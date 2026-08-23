package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type enrichmentCoverageStats struct {
	sourceCount           int
	missingCarrierDisplay int
	missingOrderReference int
}

type accessorialCoverageStats struct {
	sourceLineCount    int
	eligibleLineCount  int
	excludedProposed   int
	excludedRejected   int
	excludedCancelled  int
	unmappedChargeCode int
}

type accessorialMappingContext struct {
	platformMappings   []domain.ChargeCodeMapping
	tenantMappings     []domain.ChargeCodeMapping
	mappingVersion     int64
	mappingEvaluatedAt time.Time
}

func (s *AnalyticsProjectionService) hydrateTenantOrderFactEnrichment(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
) (enrichmentCoverageStats, error) {
	stats := enrichmentCoverageStats{}
	if s.companies == nil && s.dimensions == nil {
		return stats, nil
	}
	facts, err := s.orderFacts.ListForTenant(ctx, tx, tenantID)
	if err != nil {
		return stats, err
	}
	if len(facts) == 0 {
		return stats, nil
	}

	orderIDs := make([]uuid.UUID, 0, len(facts))
	carrierIDs := make([]uuid.UUID, 0, len(facts))
	carrierSeen := map[uuid.UUID]struct{}{}
	for _, fact := range facts {
		orderIDs = append(orderIDs, fact.TransportOrderID)
		if fact.CarrierCompanyID != uuid.Nil {
			if _, ok := carrierSeen[fact.CarrierCompanyID]; !ok {
				carrierSeen[fact.CarrierCompanyID] = struct{}{}
				carrierIDs = append(carrierIDs, fact.CarrierCompanyID)
			}
		}
	}

	var companies map[uuid.UUID]provider.CompanyDisplay
	if s.companies != nil {
		companies, err = s.companies.BatchGetCompanyDisplay(ctx, tenantID, carrierIDs)
		if err != nil {
			return stats, err
		}
	}
	var dimensions map[uuid.UUID]provider.TransportOrderAnalyticsDimension
	if s.dimensions != nil {
		dimensions, err = s.dimensions.BatchGetAnalyticsDimensions(ctx, tenantID, orderIDs)
		if err != nil {
			return stats, err
		}
	}

	stats = enrichmentCoverageStats{sourceCount: len(facts)}
	for _, fact := range facts {
		ls := s.hydrateOrderFactEnrichment(fact, companies, dimensions)
		stats.missingCarrierDisplay += ls.missingCarrierDisplay
		stats.missingOrderReference += ls.missingOrderReference
		fact.CalculatedAt = now.UTC()
		if err := s.orderFacts.Upsert(ctx, tx, fact); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func (s *AnalyticsProjectionService) hydrateSingleOrderFactEnrichment(
	ctx context.Context,
	fact *domain.AnalyticsOrderFact,
) error {
	if fact == nil || (s.companies == nil && s.dimensions == nil) {
		return nil
	}
	var companies map[uuid.UUID]provider.CompanyDisplay
	var dimensions map[uuid.UUID]provider.TransportOrderAnalyticsDimension
	var err error
	if s.companies != nil && fact.CarrierCompanyID != uuid.Nil {
		companies, err = s.companies.BatchGetCompanyDisplay(ctx, fact.TenantID, []uuid.UUID{fact.CarrierCompanyID})
		if err != nil {
			return err
		}
	}
	if s.dimensions != nil {
		dimensions, err = s.dimensions.BatchGetAnalyticsDimensions(ctx, fact.TenantID, []uuid.UUID{fact.TransportOrderID})
		if err != nil {
			return err
		}
	}
	s.hydrateOrderFactEnrichment(fact, companies, dimensions)
	return nil
}

func (s *AnalyticsProjectionService) hydrateOrderFactEnrichment(
	fact *domain.AnalyticsOrderFact,
	companies map[uuid.UUID]provider.CompanyDisplay,
	dimensions map[uuid.UUID]provider.TransportOrderAnalyticsDimension,
) enrichmentCoverageStats {
	stats := enrichmentCoverageStats{sourceCount: 1}
	if fact == nil {
		return stats
	}
	if dim, ok := dimensions[fact.TransportOrderID]; ok && dim.OrderNumber != nil {
		ref := stringsTrimDisplay(*dim.OrderNumber)
		if ref != "" {
			fact.OrderReference = stringPtrEnrichment(ref)
		}
	}
	if fact.OrderReference == nil || stringsTrimDisplay(*fact.OrderReference) == "" {
		stats.missingOrderReference = 1
	}
	if fact.CarrierCompanyID != uuid.Nil {
		if company, ok := companies[fact.CarrierCompanyID]; ok {
			legalName := company.LegalName
			name := domain.ResolveCompanyDisplayName(company.ShortName, &legalName)
			if name != "" {
				fact.CarrierDisplayName = stringPtrEnrichment(name)
			}
		}
	}
	if fact.CarrierDisplayName == nil || stringsTrimDisplay(*fact.CarrierDisplayName) == "" {
		stats.missingCarrierDisplay = 1
	}
	laneLabel := domain.BuildLaneLabel(
		stringValueEnrichment(fact.OriginCity),
		stringValueEnrichment(fact.DestinationCity),
		stringValueEnrichment(fact.TransportMode),
		stringValueEnrichment(fact.EquipmentType),
	)
	if laneLabel != "" {
		fact.LaneLabel = stringPtrEnrichment(laneLabel)
	}
	return stats
}

func (s *AnalyticsProjectionService) rebuildTenantAccessorialFacts(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
) (accessorialCoverageStats, error) {
	stats := accessorialCoverageStats{}
	if s.accessorialFacts == nil || s.settlements == nil || s.mappings == nil {
		return stats, nil
	}
	if err := s.accessorialFacts.DeleteByTenant(ctx, tx, tenantID); err != nil {
		return stats, err
	}
	facts, err := s.orderFacts.ListForTenant(ctx, tx, tenantID)
	if err != nil {
		return stats, err
	}
	orderIDs := make([]uuid.UUID, 0, len(facts))
	factByOrder := make(map[uuid.UUID]*domain.AnalyticsOrderFact, len(facts))
	for _, fact := range facts {
		orderIDs = append(orderIDs, fact.TransportOrderID)
		factByOrder[fact.TransportOrderID] = fact
	}
	settlements, err := s.settlements.BatchGetSettlementsByTransportOrder(ctx, tenantID, orderIDs)
	if err != nil {
		return stats, err
	}
	for orderID, settlement := range settlements {
		orderFact := factByOrder[orderID]
		if orderFact == nil {
			continue
		}
		mappingCtx, err := s.loadAccessorialMappingContext(ctx, tx, tenantID, orderID)
		if err != nil {
			return stats, err
		}
		lineStats, err := s.persistAccessorialFactsForOrder(
			ctx, tx, orderFact, settlement, mappingCtx, now,
		)
		if err != nil {
			return stats, err
		}
		stats.sourceLineCount += lineStats.sourceLineCount
		stats.eligibleLineCount += lineStats.eligibleLineCount
		stats.excludedProposed += lineStats.excludedProposed
		stats.excludedRejected += lineStats.excludedRejected
		stats.excludedCancelled += lineStats.excludedCancelled
		stats.unmappedChargeCode += lineStats.unmappedChargeCode
	}
	return stats, nil
}

func (s *AnalyticsProjectionService) rebuildOrderAccessorialFacts(
	ctx context.Context,
	tx pgx.Tx,
	orderFact *domain.AnalyticsOrderFact,
	now time.Time,
) (accessorialCoverageStats, error) {
	stats := accessorialCoverageStats{}
	if s.accessorialFacts == nil || s.settlements == nil || s.mappings == nil || orderFact == nil {
		return stats, nil
	}
	if err := s.accessorialFacts.DeleteByTransportOrder(ctx, tx, orderFact.TenantID, orderFact.TransportOrderID); err != nil {
		return stats, err
	}
	settlements, err := s.settlements.BatchGetSettlementsByTransportOrder(
		ctx, orderFact.TenantID, []uuid.UUID{orderFact.TransportOrderID},
	)
	if err != nil {
		return stats, err
	}
	settlement, ok := settlements[orderFact.TransportOrderID]
	if !ok {
		return stats, nil
	}
	mappingCtx, err := s.loadAccessorialMappingContext(ctx, tx, orderFact.TenantID, orderFact.TransportOrderID)
	if err != nil {
		return stats, err
	}
	return s.persistAccessorialFactsForOrder(ctx, tx, orderFact, settlement, mappingCtx, now)
}

func (s *AnalyticsProjectionService) persistAccessorialFactsForOrder(
	ctx context.Context,
	tx pgx.Tx,
	orderFact *domain.AnalyticsOrderFact,
	settlement provider.SettlementAccessorialBatchItem,
	mappingCtx accessorialMappingContext,
	now time.Time,
) (accessorialCoverageStats, error) {
	stats := accessorialCoverageStats{}
	for _, line := range settlement.Accessorials {
		stats.sourceLineCount++
		switch line.Status {
		case domain.AccessorialStatusProposed:
			stats.excludedProposed++
		case domain.AccessorialStatusRejected:
			stats.excludedRejected++
		case domain.AccessorialStatusDisputed:
			stats.excludedCancelled++
		}
		currency := strings.ToUpper(strings.TrimSpace(line.CurrencyCode))
		if currency == "" {
			currency = orderFact.CurrencyCode
		}
		category := domain.ResolveChargeCategory(line.ChargeCode, mappingCtx.platformMappings, mappingCtx.tenantMappings)
		if isUnmappedChargeCode(line.ChargeCode, mappingCtx.platformMappings, mappingCtx.tenantMappings) {
			stats.unmappedChargeCode++
		}
		eligible := line.Status == domain.AccessorialStatusApproved && currency == orderFact.CurrencyCode
		if eligible {
			stats.eligibleLineCount++
		}
		fact := &domain.AnalyticsAccessorialFact{
			TenantID:           orderFact.TenantID,
			AccessorialID:      line.AccessorialID,
			CurrencyCode:       currency,
			TransportOrderID:   orderFact.TransportOrderID,
			BuyerCompanyID:     orderFact.BuyerCompanyID,
			SettlementID:       settlement.SettlementID,
			ChargeCode:         line.ChargeCode,
			NormalizedCategory: category,
			Amount:             line.Amount.Round(domain.MoneyScale),
			Status:             line.Status,
			MappingVersion:     mappingCtx.mappingVersion,
			MappingEvaluatedAt: mappingCtx.mappingEvaluatedAt,
			PeriodStart:        orderFact.PeriodStart,
			PeriodGrain:        orderFact.PeriodGrain,
			Eligible:           eligible,
			CalculatedAt:       now.UTC(),
		}
		if err := s.accessorialFacts.Upsert(ctx, tx, fact); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func (s *AnalyticsProjectionService) loadAccessorialMappingContext(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) (accessorialMappingContext, error) {
	result := accessorialMappingContext{mappingEvaluatedAt: time.Now().UTC()}
	if s.mappings == nil || s.summaries == nil {
		return result, nil
	}
	summaryRow, err := s.summaries.GetSummaryRowByTransportOrderTx(ctx, tx, tenantID, transportOrderID)
	if err != nil {
		return result, err
	}
	projection := summaryRow.Projection
	evalTime := time.Now().UTC()
	if projection.AttributionMappingEvaluatedAt != nil {
		evalTime = projection.AttributionMappingEvaluatedAt.UTC()
	}
	if projection.AttributionMappingVersion != nil {
		platform, tenant, _, err := s.mappings.LoadPinnedMappings(
			ctx, tx, tenantID, *projection.AttributionMappingVersion, evalTime,
		)
		if err != nil {
			return result, err
		}
		result.platformMappings = platform
		result.tenantMappings = tenant
		result.mappingVersion = *projection.AttributionMappingVersion
		result.mappingEvaluatedAt = evalTime
		return result, nil
	}
	platform, tenant, loadedVersion, err := s.mappings.LoadActiveMappings(ctx, tx, tenantID, evalTime)
	if err != nil {
		return result, err
	}
	result.platformMappings = platform
	result.tenantMappings = tenant
	result.mappingVersion = loadedVersion
	result.mappingEvaluatedAt = evalTime
	return result, nil
}

func (s *AnalyticsProjectionService) rebuildAccessorialPeriodProjections(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
) error {
	if s.accessorialPeriods == nil || s.accessorialFacts == nil {
		return nil
	}
	keys, err := s.accessorialPeriods.ListDistinctKeysForTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := s.reaggregateAccessorialPeriod(ctx, tx, key, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *AnalyticsProjectionService) reaggregateAccessorialPeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsAccessorialPeriodKey,
	now time.Time,
) error {
	if s.accessorialPeriods == nil || s.accessorialFacts == nil {
		return nil
	}
	agg, err := s.accessorialFacts.AggregatePeriod(ctx, tx, key)
	if err != nil {
		return err
	}
	if agg.LineCount == 0 {
		return s.accessorialPeriods.DeleteByKey(ctx, tx, key)
	}
	periodKey := domain.AnalyticsPeriodKey{
		TenantID:       key.TenantID,
		BuyerCompanyID: key.BuyerCompanyID,
		PeriodStart:    key.PeriodStart,
		PeriodGrain:    key.PeriodGrain,
		CurrencyCode:   key.CurrencyCode,
	}
	freightAgg, err := s.orderFacts.AggregatePeriod(ctx, tx, periodKey)
	if err != nil {
		return err
	}
	var freightSpend *decimal.Decimal
	if freightAgg.CurrentActualTotal != nil {
		freightSpend = freightAgg.CurrentActualTotal
	}
	var shareOfSpend, accessorialOrderRate *decimal.Decimal
	if freightSpend != nil && !freightSpend.IsZero() && agg.TotalAmount != nil {
		ratio := agg.TotalAmount.Div(*freightSpend).Round(6)
		shareOfSpend = &ratio
	}
	if freightAgg.OrderCount > 0 && agg.OrderCount > 0 {
		rate := decimal.NewFromInt(int64(agg.OrderCount)).
			Div(decimal.NewFromInt(int64(freightAgg.OrderCount))).
			Round(6)
		accessorialOrderRate = &rate
	}
	projection := &domain.AnalyticsAccessorialPeriodProjection{
		TenantID:             key.TenantID,
		BuyerCompanyID:       key.BuyerCompanyID,
		NormalizedCategory:   key.NormalizedCategory,
		PeriodStart:          key.PeriodStart,
		PeriodGrain:          key.PeriodGrain,
		CurrencyCode:         key.CurrencyCode,
		TotalAmount:          agg.TotalAmount,
		OrderCount:           agg.OrderCount,
		LineCount:            agg.LineCount,
		ShareOfSpend:         shareOfSpend,
		AccessorialOrderRate: accessorialOrderRate,
		FreightSpendTotal:    freightSpend,
		CalculatedAt:         now.UTC(),
		DataThrough:          agg.MaxCalculatedAt.UTC(),
		ProjectionVersion:    domain.AnalyticsAccessorialProjectionVersion,
	}
	return s.accessorialPeriods.Upsert(ctx, tx, projection)
}

func (s *AnalyticsProjectionService) rebuildAccessorialAndEnrichment(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
	enrichmentStats enrichmentCoverageStats,
	accessorialStats accessorialCoverageStats,
) error {
	if err := s.persistEnrichmentCoverage(ctx, tx, tenantID, now, enrichmentStats); err != nil {
		return err
	}
	if err := s.persistAccessorialCoverage(ctx, tx, tenantID, now, accessorialStats); err != nil {
		return err
	}
	if err := s.markProjectionStateSuccess(ctx, tx, domain.AnalyticsProjectionNameAccessorial, tenantID, now, now); err != nil {
		return err
	}
	return nil
}

func (s *AnalyticsProjectionService) persistEnrichmentCoverage(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
	stats enrichmentCoverageStats,
) error {
	if s.coverage == nil {
		return nil
	}
	eligible := stats.sourceCount - stats.missingCarrierDisplay
	if stats.missingOrderReference > 0 {
		eligible = stats.sourceCount - stats.missingOrderReference
		if eligible < 0 {
			eligible = 0
		}
	}
	excluded := stats.missingCarrierDisplay + stats.missingOrderReference
	quality := domain.DataQualityAvailable
	if excluded > 0 && eligible > 0 {
		quality = domain.DataQualityPartial
	} else if eligible == 0 && stats.sourceCount > 0 {
		quality = domain.DataQualityNotAvailable
	}
	return s.coverage.Upsert(ctx, tx, &domain.AnalyticsProjectionCoverage{
		ProjectionName:             "cost_analytics_order_fact_enrichment",
		TenantID:                   tenantID,
		CalculatedAt:               now.UTC(),
		SourceOrderCount:           stats.sourceCount,
		EligibleOrderCount:         eligible,
		ExcludedOrderCount:         excluded,
		MissingCarrierDisplayCount: stats.missingCarrierDisplay,
		MissingOrderReferenceCount: stats.missingOrderReference,
		DataQuality:                quality,
	})
}

func (s *AnalyticsProjectionService) persistAccessorialCoverage(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
	stats accessorialCoverageStats,
) error {
	if s.coverage == nil {
		return nil
	}
	excluded := stats.excludedProposed + stats.excludedRejected + stats.excludedCancelled
	quality := domain.DataQualityAvailable
	if excluded > 0 && stats.eligibleLineCount > 0 {
		quality = domain.DataQualityPartial
	} else if stats.eligibleLineCount == 0 && stats.sourceLineCount > 0 {
		quality = domain.DataQualityNotAvailable
	}
	return s.coverage.Upsert(ctx, tx, &domain.AnalyticsProjectionCoverage{
		ProjectionName:          domain.AnalyticsProjectionNameAccessorial,
		TenantID:                tenantID,
		CalculatedAt:            now.UTC(),
		SourceOrderCount:        stats.sourceLineCount,
		EligibleOrderCount:      stats.eligibleLineCount,
		ExcludedOrderCount:      excluded,
		ExcludedProposedCount:   stats.excludedProposed,
		ExcludedRejectedCount:   stats.excludedRejected,
		ExcludedCancelledCount:  stats.excludedCancelled,
		UnmappedChargeCodeCount: stats.unmappedChargeCode,
		DataQuality:             quality,
	})
}

func collectAffectedAccessorialSlices(
	oldFacts, newFacts []domain.AnalyticsAccessorialFact,
) []domain.AnalyticsAccessorialPeriodKey {
	seen := map[string]struct{}{}
	var keys []domain.AnalyticsAccessorialPeriodKey
	add := func(f domain.AnalyticsAccessorialFact) {
		if !f.Eligible {
			return
		}
		key := f.PeriodKey()
		id := accessorialSliceID(key)
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		keys = append(keys, key)
	}
	for _, f := range oldFacts {
		add(f)
	}
	for _, f := range newFacts {
		add(f)
	}
	return keys
}

func accessorialSliceID(key domain.AnalyticsAccessorialPeriodKey) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		key.TenantID, key.BuyerCompanyID, key.NormalizedCategory,
		key.PeriodStart.Format("2006-01-02"), key.PeriodGrain, key.CurrencyCode)
}

func (s *AnalyticsProjectionService) reaggregateAffectedAccessorialSlices(
	ctx context.Context,
	accessorials map[string]domain.AnalyticsAccessorialPeriodKey,
) error {
	now := time.Now().UTC()
	for _, key := range accessorials {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, key.TenantID); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := s.reaggregateAccessorialPeriod(ctx, tx, key, now); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func isUnmappedChargeCode(sourceCode string, platform, tenant []domain.ChargeCodeMapping) bool {
	normalized, err := domain.NormalizeChargeCode(sourceCode)
	if err != nil {
		return true
	}
	for _, m := range tenant {
		if m.SourceChargeCodeNormalized == normalized {
			return false
		}
	}
	for _, m := range platform {
		if m.SourceChargeCodeNormalized == normalized {
			return false
		}
	}
	return true
}

func stringPtrEnrichment(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func stringValueEnrichment(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringsTrimDisplay(value string) string {
	return strings.TrimSpace(value)
}

func (s *AnalyticsProjectionService) ListAccessorialProjections(
	ctx context.Context,
	tenantID uuid.UUID,
	filter repository.AnalyticsAccessorialListFilter,
) ([]domain.AnalyticsAccessorialPeriodProjection, error) {
	if s.accessorialPeriods == nil {
		return nil, nil
	}
	return s.accessorialPeriods.List(ctx, tenantID, filter)
}
