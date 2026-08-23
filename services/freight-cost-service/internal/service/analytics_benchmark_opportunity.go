package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

func (s *AnalyticsProjectionService) rebuildTenantBenchmarksAndOpportunities(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
) error {
	if s.benchmarks == nil || s.opportunities == nil {
		return nil
	}
	if err := s.benchmarks.DeleteByTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	keys, err := s.benchmarks.ListDistinctLaneKeysForTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	benchmarkByKey := make(map[string]*domain.AnalyticsBenchmarkProjection, len(keys))
	for _, key := range keys {
		projection, err := s.computeLaneBenchmark(ctx, tx, key, now)
		if err != nil {
			return err
		}
		if projection == nil {
			continue
		}
		if err := s.benchmarks.Upsert(ctx, tx, projection); err != nil {
			return err
		}
		if s.metrics != nil {
			s.metrics.ObserveBenchmarkCohort(projection.DataQuality)
		}
		benchmarkByKey[benchmarkSliceID(key)] = projection
	}
	if err := s.evaluateAndPersistOpportunities(ctx, tx, tenantID, keys, benchmarkByKey, now, true); err != nil {
		return err
	}
	if err := s.markProjectionStateSuccess(ctx, tx, domain.AnalyticsProjectionNameBenchmark, tenantID, now, now); err != nil {
		return err
	}
	return s.markProjectionStateSuccess(ctx, tx, domain.AnalyticsProjectionNameOpportunity, tenantID, now, now)
}

func (s *AnalyticsProjectionService) recomputeBenchmarksForLaneKeys(
	ctx context.Context,
	keys map[string]domain.AnalyticsBenchmarkKey,
) error {
	if s.benchmarks == nil || s.opportunities == nil || len(keys) == 0 {
		return nil
	}
	now := time.Now().UTC()
	tenantIDs := map[uuid.UUID]struct{}{}
	keyList := make([]domain.AnalyticsBenchmarkKey, 0, len(keys))
	for _, key := range keys {
		tenantIDs[key.TenantID] = struct{}{}
		keyList = append(keyList, key)
	}
	benchmarkByKey := make(map[string]*domain.AnalyticsBenchmarkProjection, len(keyList))
	for _, key := range keyList {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, key.TenantID); err != nil {
			tx.Rollback(ctx)
			return err
		}
		projection, err := s.computeLaneBenchmark(ctx, tx, key, now)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		if projection == nil {
			if err := s.benchmarks.DeleteByKey(ctx, tx, key); err != nil {
				tx.Rollback(ctx)
				return err
			}
		} else if err := s.benchmarks.Upsert(ctx, tx, projection); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if projection != nil {
			benchmarkByKey[benchmarkSliceID(key)] = projection
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	for tenantID := range tenantIDs {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, tenantID); err != nil {
			tx.Rollback(ctx)
			return err
		}
		var tenantKeys []domain.AnalyticsBenchmarkKey
		for _, key := range keyList {
			if key.TenantID == tenantID {
				tenantKeys = append(tenantKeys, key)
			}
		}
		if err := s.evaluateAndPersistOpportunities(ctx, tx, tenantID, tenantKeys, benchmarkByKey, now, false); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *AnalyticsProjectionService) computeLaneBenchmark(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsBenchmarkKey,
	now time.Time,
) (*domain.AnalyticsBenchmarkProjection, error) {
	stats, err := s.benchmarks.ComputeLaneBenchmarkStats(ctx, tx, key)
	if err != nil {
		return nil, err
	}
	minSample := s.benchmarkConfig.EffectiveMinSample()
	quality := domain.BenchmarkDataQualityForSample(stats.SampleCount, minSample)
	projection := &domain.AnalyticsBenchmarkProjection{
		TenantID:          key.TenantID,
		BuyerCompanyID:    key.BuyerCompanyID,
		CohortType:        domain.BenchmarkCohortTypeLane,
		LaneKey:           key.LaneKey,
		TransportMode:     key.TransportMode,
		EquipmentType:     key.EquipmentType,
		PeriodStart:       key.PeriodStart,
		PeriodGrain:       key.PeriodGrain,
		CurrencyCode:      key.CurrencyCode,
		SampleCount:       stats.SampleCount,
		DataQuality:       quality,
		RuleVersion:       domain.BenchmarkRuleVersion,
		CalculatedAt:      now.UTC(),
		DataThrough:       stats.MaxUpdated.UTC(),
		ProjectionVersion: domain.AnalyticsBenchmarkProjectionVersion,
	}
	if quality == domain.DataQualityAvailable {
		projection.MeanAmount = roundDecimalPtr(stats.MeanAmount)
		projection.MedianAmount = roundDecimalPtr(stats.MedianAmount)
		projection.P25Amount = roundDecimalPtr(stats.P25Amount)
		projection.P75Amount = roundDecimalPtr(stats.P75Amount)
		projection.P90Amount = roundDecimalPtr(stats.P90Amount)
		projection.MinAmount = roundDecimalPtr(stats.MinAmount)
		projection.MaxAmount = roundDecimalPtr(stats.MaxAmount)
	}
	return projection, nil
}

func roundDecimalPtr(value *decimal.Decimal) *decimal.Decimal {
	if value == nil {
		return nil
	}
	rounded := value.Round(domain.MoneyScale)
	return &rounded
}

func (s *AnalyticsProjectionService) evaluateAndPersistOpportunities(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	laneKeys []domain.AnalyticsBenchmarkKey,
	benchmarkByKey map[string]*domain.AnalyticsBenchmarkProjection,
	now time.Time,
	fullTenant bool,
) error {
	minSample := s.benchmarkConfig.EffectiveMinSample()
	var generated []domain.AnalyticsOpportunityProjection
	seenIDs := map[uuid.UUID]struct{}{}

	for _, key := range laneKeys {
		if key.TenantID != tenantID {
			continue
		}
		benchmark := benchmarkByKey[benchmarkSliceID(key)]
		if benchmark == nil || benchmark.DataQuality != domain.DataQualityAvailable {
			continue
		}
		orders, err := s.orderFacts.ListForLaneBenchmarkKey(ctx, tx, key)
		if err != nil {
			return err
		}
		opps, err := s.evaluateLaneOrderOpportunities(benchmark, orders, now)
		if err != nil {
			return err
		}
		for _, opp := range opps {
			if _, ok := seenIDs[opp.OpportunityID]; ok {
				continue
			}
			seenIDs[opp.OpportunityID] = struct{}{}
			generated = append(generated, opp)
		}
	}

	carrierOpps, err := s.evaluateCarrierLaneNormalizedOpportunities(ctx, tx, tenantID, benchmarkByKey, minSample, now)
	if err != nil {
		return err
	}
	for _, opp := range carrierOpps {
		if _, ok := seenIDs[opp.OpportunityID]; ok {
			continue
		}
		seenIDs[opp.OpportunityID] = struct{}{}
		generated = append(generated, opp)
	}

	if s.attributions != nil {
		varianceOpps, err := s.evaluateRepeatedVarianceOpportunities(ctx, tx, tenantID, now)
		if err != nil {
			return err
		}
		for _, opp := range varianceOpps {
			if _, ok := seenIDs[opp.OpportunityID]; ok {
				continue
			}
			seenIDs[opp.OpportunityID] = struct{}{}
			generated = append(generated, opp)
		}
	}

	accessorialOpps, err := s.evaluateHighAccessorialRateOpportunities(ctx, tx, tenantID, minSample, now)
	if err != nil {
		return err
	}
	for _, opp := range accessorialOpps {
		if _, ok := seenIDs[opp.OpportunityID]; ok {
			continue
		}
		seenIDs[opp.OpportunityID] = struct{}{}
		generated = append(generated, opp)
	}

	keepIDs := make([]uuid.UUID, 0, len(generated))
	for _, opp := range generated {
		if err := s.opportunities.Upsert(ctx, tx, &opp); err != nil {
			return err
		}
		if s.metrics != nil {
			s.metrics.ObserveOpportunityGenerated(opp.OpportunityType)
		}
		keepIDs = append(keepIDs, opp.OpportunityID)
	}
	if fullTenant {
		return s.opportunities.DeleteExceptIDs(ctx, tx, tenantID, keepIDs)
	}
	return s.pruneStaleOpportunitiesForLanes(ctx, tx, tenantID, laneKeys, keepIDs)
}

func (s *AnalyticsProjectionService) pruneStaleOpportunitiesForLanes(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	laneKeys []domain.AnalyticsBenchmarkKey,
	keepIDs []uuid.UUID,
) error {
	if len(laneKeys) == 0 {
		return nil
	}
	laneSet := map[string]struct{}{}
	periodSet := map[string]struct{}{}
	for _, key := range laneKeys {
		laneSet[key.LaneKey] = struct{}{}
		periodSet[key.PeriodStart.Format("2006-01-02")+"|"+key.CurrencyCode] = struct{}{}
	}
	existing, err := s.opportunities.List(ctx, tenantID, repository.AnalyticsOpportunityListFilter{Limit: 5000})
	if err != nil {
		return err
	}
	keepSet := map[uuid.UUID]struct{}{}
	for _, id := range keepIDs {
		keepSet[id] = struct{}{}
	}
	for _, opp := range existing {
		if opp.LaneKey == nil {
			continue
		}
		if _, laneOK := laneSet[*opp.LaneKey]; !laneOK {
			continue
		}
		periodKey := opp.PeriodStart.Format("2006-01-02") + "|" + opp.CurrencyCode
		if _, periodOK := periodSet[periodKey]; !periodOK {
			continue
		}
		if _, keep := keepSet[opp.OpportunityID]; keep {
			continue
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM freight_cost.cost_analytics_opportunity_projection
			WHERE tenant_id = $1 AND opportunity_id = $2`, tenantID, opp.OpportunityID); err != nil {
			return err
		}
	}
	return nil
}

func (s *AnalyticsProjectionService) evaluateLaneOrderOpportunities(
	benchmark *domain.AnalyticsBenchmarkProjection,
	orders []*domain.AnalyticsOrderFact,
	now time.Time,
) ([]domain.AnalyticsOpportunityProjection, error) {
	if benchmark == nil || benchmark.MedianAmount == nil || benchmark.P90Amount == nil {
		return nil, nil
	}
	var opps []domain.AnalyticsOpportunityProjection
	for _, order := range orders {
		if order == nil {
			continue
		}
		amount := domain.BenchmarkEligibleCostAmount(order)
		if amount == nil {
			continue
		}
		if amount.GreaterThan(*benchmark.P90Amount) {
			opp, err := s.buildOrderOpportunity(
				order, benchmark, *amount, *benchmark.P90Amount,
				domain.OpportunityTypeLaneCostOutlier, domain.OpportunityScopeOrder, now,
			)
			if err != nil {
				return nil, err
			}
			if opp != nil {
				opps = append(opps, *opp)
			}
		}
		if amount.GreaterThan(*benchmark.MedianAmount) {
			opp, err := s.buildOrderOpportunity(
				order, benchmark, *amount, *benchmark.MedianAmount,
				domain.OpportunityTypeCostAboveLaneMedian, domain.OpportunityScopeOrder, now,
			)
			if err != nil {
				return nil, err
			}
			if opp != nil {
				opps = append(opps, *opp)
			}
		}
	}
	return opps, nil
}

func (s *AnalyticsProjectionService) buildOrderOpportunity(
	order *domain.AnalyticsOrderFact,
	benchmark *domain.AnalyticsBenchmarkProjection,
	observed, baseline decimal.Decimal,
	opportunityType, scope string,
	now time.Time,
) (*domain.AnalyticsOpportunityProjection, error) {
	delta := observed.Sub(baseline).Round(domain.MoneyScale)
	if !delta.IsPositive() {
		return nil, nil
	}
	entityKey := order.TransportOrderID.String()
	if order.LaneKey != nil {
		entityKey = fmt.Sprintf("%s|%s", *order.LaneKey, order.TransportOrderID.String())
	}
	oppID := domain.DeriveOpportunityID(
		order.TenantID, order.BuyerCompanyID,
		opportunityType, scope, entityKey, order.CurrencyCode,
		order.PeriodStart, domain.OpportunityRuleVersion,
	)
	evidence := domain.OpportunityEvidence{
		SchemaVersion:  1,
		ObservedCost:   domain.MoneyString(observed),
		BaselineCost:   domain.MoneyString(baseline),
		PotentialDelta: domain.MoneyString(delta),
		SampleSize:     benchmark.SampleCount,
		CurrencyCode:   order.CurrencyCode,
		LaneKey:        derefString(order.LaneKey),
	}
	if benchmark.MedianAmount != nil {
		evidence.CohortMedian = domain.MoneyString(*benchmark.MedianAmount)
	}
	if benchmark.P90Amount != nil {
		evidence.CohortP90 = domain.MoneyString(*benchmark.P90Amount)
	}
	evidenceRaw, err := evidence.JSON()
	if err != nil {
		return nil, err
	}
	laneKey := order.LaneKey
	carrierID := order.CarrierCompanyID
	orderID := order.TransportOrderID
	return &domain.AnalyticsOpportunityProjection{
		TenantID:          order.TenantID,
		BuyerCompanyID:    order.BuyerCompanyID,
		OpportunityID:     oppID,
		OpportunityType:   opportunityType,
		Scope:             scope,
		EntityKey:         entityKey,
		CurrencyCode:      order.CurrencyCode,
		TransportOrderID:  &orderID,
		CarrierCompanyID:  &carrierID,
		LaneKey:           laneKey,
		PeriodStart:       order.PeriodStart,
		PeriodGrain:       order.PeriodGrain,
		ObservedAmount:    observed,
		BaselineAmount:    baseline,
		EstimatedDelta:    delta,
		SampleSize:        benchmark.SampleCount,
		DataQuality:       domain.DataQualityAvailable,
		RuleVersion:       domain.OpportunityRuleVersion,
		EvidenceJSON:      evidenceRaw,
		CalculatedAt:      now.UTC(),
		DataThrough:       benchmark.DataThrough,
		ProjectionVersion: domain.AnalyticsOpportunityProjectionVersion,
	}, nil
}

func (s *AnalyticsProjectionService) evaluateCarrierLaneNormalizedOpportunities(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	benchmarkByKey map[string]*domain.AnalyticsBenchmarkProjection,
	minSample int,
	now time.Time,
) ([]domain.AnalyticsOpportunityProjection, error) {
	type carrierKey struct {
		buyerID   uuid.UUID
		carrierID uuid.UUID
		period    time.Time
		currency  string
	}
	deltasByCarrier := map[carrierKey][]decimal.Decimal{}
	sampleByCarrier := map[carrierKey]int{}

	for _, benchmark := range benchmarkByKey {
		if benchmark.TenantID != tenantID || benchmark.DataQuality != domain.DataQualityAvailable || benchmark.MedianAmount == nil {
			continue
		}
		key := domain.AnalyticsBenchmarkKey{
			TenantID: benchmark.TenantID, BuyerCompanyID: benchmark.BuyerCompanyID,
			LaneKey: benchmark.LaneKey, TransportMode: benchmark.TransportMode,
			EquipmentType: benchmark.EquipmentType, PeriodStart: benchmark.PeriodStart,
			PeriodGrain: benchmark.PeriodGrain, CurrencyCode: benchmark.CurrencyCode,
		}
		orders, err := s.orderFacts.ListForLaneBenchmarkKey(ctx, tx, key)
		if err != nil {
			return nil, err
		}
		for _, order := range orders {
			if order == nil || order.CarrierCompanyID == uuid.Nil {
				continue
			}
			amount := domain.BenchmarkEligibleCostAmount(order)
			if amount == nil {
				continue
			}
			delta := amount.Sub(*benchmark.MedianAmount).Round(domain.MoneyScale)
			ck := carrierKey{
				buyerID: order.BuyerCompanyID, carrierID: order.CarrierCompanyID,
				period: order.PeriodStart, currency: order.CurrencyCode,
			}
			deltasByCarrier[ck] = append(deltasByCarrier[ck], delta)
			sampleByCarrier[ck]++
		}
	}

	var opps []domain.AnalyticsOpportunityProjection
	for ck, deltas := range deltasByCarrier {
		if sampleByCarrier[ck] < minSample {
			continue
		}
		sort.Slice(deltas, func(i, j int) bool { return deltas[i].LessThan(deltas[j]) })
		medianDelta := percentileSorted(deltas, 0.5)
		if !medianDelta.IsPositive() {
			continue
		}
		positiveCount := 0
		totalPositive := decimal.Zero
		for _, d := range deltas {
			if d.IsPositive() {
				positiveCount++
				totalPositive = totalPositive.Add(d)
			}
		}
		if positiveCount == 0 {
			continue
		}
		observed := totalPositive.Div(decimal.NewFromInt(int64(positiveCount))).Round(domain.MoneyScale)
		baseline := decimal.Zero
		estimated := observed.Sub(baseline).Round(domain.MoneyScale)
		if !estimated.IsPositive() {
			continue
		}
		entityKey := fmt.Sprintf("%s|%s", ck.carrierID.String(), ck.period.Format("2006-01-02"))
		oppID := domain.DeriveOpportunityID(
			tenantID, ck.buyerID, domain.OpportunityTypeCarrierCostOutlier,
			domain.OpportunityScopeCarrier, entityKey, ck.currency,
			ck.period, domain.OpportunityRuleVersion,
		)
		evidence := domain.OpportunityEvidence{
			SchemaVersion:    1,
			ObservedCost:     domain.MoneyString(observed),
			BaselineCost:     domain.MoneyString(baseline),
			PotentialDelta:   domain.MoneyString(estimated),
			SampleSize:       sampleByCarrier[ck],
			CurrencyCode:     ck.currency,
			CarrierCompanyID: ck.carrierID.String(),
		}
		evidenceRaw, err := evidence.JSON()
		if err != nil {
			return nil, err
		}
		carrierID := ck.carrierID
		opps = append(opps, domain.AnalyticsOpportunityProjection{
			TenantID:          tenantID,
			BuyerCompanyID:    ck.buyerID,
			OpportunityID:     oppID,
			OpportunityType:   domain.OpportunityTypeCarrierCostOutlier,
			Scope:             domain.OpportunityScopeCarrier,
			EntityKey:         entityKey,
			CurrencyCode:      ck.currency,
			CarrierCompanyID:  &carrierID,
			PeriodStart:       ck.period,
			PeriodGrain:       domain.AnalyticsPeriodGrainMonth,
			ObservedAmount:    observed,
			BaselineAmount:    baseline,
			EstimatedDelta:    estimated,
			SampleSize:        sampleByCarrier[ck],
			DataQuality:       domain.DataQualityAvailable,
			RuleVersion:       domain.OpportunityRuleVersion,
			EvidenceJSON:      evidenceRaw,
			CalculatedAt:      now.UTC(),
			DataThrough:       now.UTC(),
			ProjectionVersion: domain.AnalyticsOpportunityProjectionVersion,
		})
	}
	return opps, nil
}

func percentileSorted(values []decimal.Decimal, p float64) decimal.Decimal {
	if len(values) == 0 {
		return decimal.Zero
	}
	if len(values) == 1 {
		return values[0]
	}
	idx := p * float64(len(values)-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= len(values) {
		return values[len(values)-1]
	}
	weight := decimal.NewFromFloat(idx - float64(lower))
	return values[lower].Add(values[upper].Sub(values[lower]).Mul(weight)).Round(domain.MoneyScale)
}

func (s *AnalyticsProjectionService) evaluateRepeatedVarianceOpportunities(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
) ([]domain.AnalyticsOpportunityProjection, error) {
	minOcc := s.benchmarkConfig.EffectiveRepeatedVarianceMin()
	groups, err := s.attributions.ListRepeatedVarianceGroups(ctx, tx, tenantID, minOcc)
	if err != nil {
		return nil, err
	}
	var opps []domain.AnalyticsOpportunityProjection
	for _, group := range groups {
		entityKey := fmt.Sprintf("%s|%s|%s|%s",
			group.CarrierCompanyID.String(), group.LaneKey, group.ReasonCode, group.PeriodStart.Format("2006-01-02"))
		oppID := domain.DeriveOpportunityID(
			tenantID, group.BuyerCompanyID, domain.OpportunityTypeRepeatedVariance,
			domain.OpportunityScopeVariance, entityKey, group.CurrencyCode,
			group.PeriodStart, domain.OpportunityRuleVersion,
		)
		evidence := domain.OpportunityEvidence{
			SchemaVersion:    1,
			SampleSize:       group.OccurrenceCount,
			CurrencyCode:     group.CurrencyCode,
			LaneKey:          group.LaneKey,
			CarrierCompanyID: group.CarrierCompanyID.String(),
			ReasonCode:       group.ReasonCode,
			OccurrenceCount:  group.OccurrenceCount,
		}
		evidenceRaw, err := evidence.JSON()
		if err != nil {
			return nil, err
		}
		carrierID := group.CarrierCompanyID
		laneKey := group.LaneKey
		opps = append(opps, domain.AnalyticsOpportunityProjection{
			TenantID:          tenantID,
			BuyerCompanyID:    group.BuyerCompanyID,
			OpportunityID:     oppID,
			OpportunityType:   domain.OpportunityTypeRepeatedVariance,
			Scope:             domain.OpportunityScopeVariance,
			EntityKey:         entityKey,
			CurrencyCode:      group.CurrencyCode,
			CarrierCompanyID:  &carrierID,
			LaneKey:           &laneKey,
			PeriodStart:       group.PeriodStart,
			PeriodGrain:       domain.AnalyticsPeriodGrainMonth,
			ObservedAmount:    decimal.Zero,
			BaselineAmount:    decimal.Zero,
			EstimatedDelta:    decimal.Zero,
			SampleSize:        group.OccurrenceCount,
			DataQuality:       domain.DataQualityAvailable,
			RuleVersion:       domain.OpportunityRuleVersion,
			EvidenceJSON:      evidenceRaw,
			CalculatedAt:      now.UTC(),
			DataThrough:       now.UTC(),
			ProjectionVersion: domain.AnalyticsOpportunityProjectionVersion,
		})
	}
	return opps, nil
}

func (s *AnalyticsProjectionService) evaluateHighAccessorialRateOpportunities(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	minSample int,
	now time.Time,
) ([]domain.AnalyticsOpportunityProjection, error) {
	if s.accessorialPeriods == nil {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT buyer_company_id, period_start, period_grain, currency_code,
			accessorial_order_rate
		FROM freight_cost.cost_analytics_accessorial_period_projection
		WHERE tenant_id = $1
		  AND accessorial_order_rate IS NOT NULL
		  AND order_count >= $2`, tenantID, minSample)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rateRow struct {
		buyerID    uuid.UUID
		period     time.Time
		grain      string
		currency   string
		rate       decimal.Decimal
	}
	var rates []rateRow
	for rows.Next() {
		var row rateRow
		if err := rows.Scan(&row.buyerID, &row.period, &row.grain, &row.currency, &row.rate); err != nil {
			return nil, err
		}
		rates = append(rates, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(rates) == 0 {
		return nil, nil
	}

	type scopeKey struct {
		buyerID  uuid.UUID
		period   time.Time
		currency string
	}
	byScope := map[scopeKey][]decimal.Decimal{}
	for _, row := range rates {
		sk := scopeKey{buyerID: row.buyerID, period: row.period, currency: row.currency}
		byScope[sk] = append(byScope[sk], row.rate)
	}

	p75ByScope := map[scopeKey]decimal.Decimal{}
	for sk, values := range byScope {
		if len(values) < minSample {
			continue
		}
		sort.Slice(values, func(i, j int) bool { return values[i].LessThan(values[j]) })
		p75ByScope[sk] = percentileSorted(values, 0.75)
	}

	var opps []domain.AnalyticsOpportunityProjection
	for _, row := range rates {
		sk := scopeKey{buyerID: row.buyerID, period: row.period, currency: row.currency}
		p75, ok := p75ByScope[sk]
		if !ok || !row.rate.GreaterThan(p75) {
			continue
		}
		entityKey := fmt.Sprintf("%s|%s|ACCESSORIAL_RATE", row.buyerID.String(), row.period.Format("2006-01-02"))
		oppID := domain.DeriveOpportunityID(
			tenantID, row.buyerID, domain.OpportunityTypeHighAccessorialRate,
			domain.OpportunityScopeAccessorial, entityKey, row.currency,
			row.period, domain.OpportunityRuleVersion,
		)
		delta := row.rate.Sub(p75).Round(6)
		if !delta.IsPositive() {
			continue
		}
		evidence := domain.OpportunityEvidence{
			SchemaVersion:   1,
			SampleSize:      len(byScope[sk]),
			CurrencyCode:    row.currency,
			AccessorialRate: row.rate.StringFixed(6),
			BaselineP75Rate: p75.StringFixed(6),
			PotentialDelta:  delta.StringFixed(6),
		}
		evidenceRaw, err := evidence.JSON()
		if err != nil {
			return nil, err
		}
		opps = append(opps, domain.AnalyticsOpportunityProjection{
			TenantID:          tenantID,
			BuyerCompanyID:    row.buyerID,
			OpportunityID:     oppID,
			OpportunityType:   domain.OpportunityTypeHighAccessorialRate,
			Scope:             domain.OpportunityScopeAccessorial,
			EntityKey:         entityKey,
			CurrencyCode:      row.currency,
			PeriodStart:       row.period,
			PeriodGrain:       row.grain,
			ObservedAmount:    row.rate.Round(domain.MoneyScale),
			BaselineAmount:    p75.Round(domain.MoneyScale),
			EstimatedDelta:    delta.Round(domain.MoneyScale),
			SampleSize:        len(byScope[sk]),
			DataQuality:       domain.DataQualityAvailable,
			RuleVersion:       domain.OpportunityRuleVersion,
			EvidenceJSON:      evidenceRaw,
			CalculatedAt:      now.UTC(),
			DataThrough:       now.UTC(),
			ProjectionVersion: domain.AnalyticsOpportunityProjectionVersion,
		})
	}
	return opps, nil
}

func benchmarkSliceID(key domain.AnalyticsBenchmarkKey) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		key.TenantID, key.BuyerCompanyID, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart.Format("2006-01-02"), key.PeriodGrain, key.CurrencyCode)
}

func benchmarkKeyFromLane(key domain.AnalyticsLanePeriodKey) domain.AnalyticsBenchmarkKey {
	return domain.AnalyticsBenchmarkKey{
		TenantID:       key.TenantID,
		BuyerCompanyID: key.BuyerCompanyID,
		CohortType:     domain.BenchmarkCohortTypeLane,
		LaneKey:        key.LaneKey,
		TransportMode:  key.TransportMode,
		EquipmentType:  key.EquipmentType,
		PeriodStart:    key.PeriodStart,
		PeriodGrain:    key.PeriodGrain,
		CurrencyCode:   key.CurrencyCode,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *AnalyticsProjectionService) ListBenchmarkProjections(
	ctx context.Context,
	tenantID uuid.UUID,
	filter repository.AnalyticsBenchmarkListFilter,
) ([]domain.AnalyticsBenchmarkProjection, error) {
	if s.benchmarks == nil {
		return nil, nil
	}
	return s.benchmarks.List(ctx, tenantID, filter)
}

func (s *AnalyticsProjectionService) ListOpportunityProjections(
	ctx context.Context,
	tenantID uuid.UUID,
	filter repository.AnalyticsOpportunityListFilter,
) ([]domain.AnalyticsOpportunityProjection, error) {
	if s.opportunities == nil {
		return nil, nil
	}
	return s.opportunities.List(ctx, tenantID, filter)
}

func collectAffectedBenchmarkKeys(
	oldFact, newFact *domain.AnalyticsOrderFact,
) []domain.AnalyticsBenchmarkKey {
	seen := map[string]struct{}{}
	var keys []domain.AnalyticsBenchmarkKey
	add := func(f *domain.AnalyticsOrderFact) {
		if f == nil {
			return
		}
		if laneKey := f.LanePeriodKey(); laneKey != nil {
			bk := benchmarkKeyFromLane(*laneKey)
			id := benchmarkSliceID(bk)
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				keys = append(keys, bk)
			}
		}
	}
	add(oldFact)
	add(newFact)
	return keys
}
