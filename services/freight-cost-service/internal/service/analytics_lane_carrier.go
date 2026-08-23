package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type laneCoverageStats struct {
	sourceCount            int
	eligibleCount          int
	excludedCount          int
	excludedMissingOrigin  int
	excludedMissingDest    int
	excludedMissingCountry int
	excludedMissingMode    int
}

type carrierCoverageStats struct {
	sourceCount            int
	eligibleCount          int
	excludedCount          int
	excludedMissingCarrier int
}

type dirtyProcessResult struct {
	periods      []domain.AnalyticsPeriodKey
	lanes        []domain.AnalyticsLanePeriodKey
	carriers     []domain.AnalyticsCarrierPeriodKey
	accessorials []domain.AnalyticsAccessorialPeriodKey
	benchmarks   []domain.AnalyticsBenchmarkKey
}

func (s *AnalyticsProjectionService) hydrateOrderFactDimensions(
	fact *domain.AnalyticsOrderFact,
	dimensions map[uuid.UUID]provider.TransportOrderAnalyticsDimension,
) laneCoverageStats {
	stats := laneCoverageStats{sourceCount: 1}
	if fact == nil {
		return stats
	}
	dim, ok := dimensions[fact.TransportOrderID]
	if !ok {
		stats.excludedCount++
		stats.excludedMissingCountry++
		return stats
	}
	domain.ApplyTransportDimension(
		fact,
		dim.OriginCountry,
		dim.OriginCity,
		dim.DestinationCity,
		dim.DestinationCountry,
		dim.TransportMode,
		dim.EquipmentType,
	)
	if !fact.LaneEligible {
		stats.excludedCount++
		switch classifyLaneExclusion(dim) {
		case domain.LaneExclusionMissingOriginCity:
			stats.excludedMissingOrigin++
		case domain.LaneExclusionMissingDestinationCity:
			stats.excludedMissingDest++
		case domain.LaneExclusionMissingOriginCountry, domain.LaneExclusionMissingDestCountry:
			stats.excludedMissingCountry++
		case domain.LaneExclusionMissingTransportMode:
			stats.excludedMissingMode++
		}
		return stats
	}
	stats.eligibleCount = 1
	return stats
}

func classifyLaneExclusion(dim provider.TransportOrderAnalyticsDimension) string {
	result := domain.BuildLaneKey(domain.LaneKeyInput{
		OriginCountry:      dim.OriginCountry,
		OriginCity:         stringValue(dim.OriginCity),
		DestinationCountry: dim.DestinationCountry,
		DestinationCity:    stringValue(dim.DestinationCity),
		TransportMode:      dim.TransportMode,
		EquipmentType:      stringValue(dim.EquipmentType),
	})
	return result.ExclusionReason
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *AnalyticsProjectionService) hydrateTenantOrderFacts(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
) (laneCoverageStats, carrierCoverageStats, error) {
	var laneStats laneCoverageStats
	var carrierStats carrierCoverageStats
	if s.dimensions == nil {
		return laneStats, carrierStats, nil
	}
	facts, err := s.orderFacts.ListForTenant(ctx, tx, tenantID)
	if err != nil {
		return laneStats, carrierStats, err
	}
	laneStats.sourceCount = len(facts)
	carrierStats.sourceCount = len(facts)

	orderIDs := make([]uuid.UUID, 0, len(facts))
	for _, fact := range facts {
		orderIDs = append(orderIDs, fact.TransportOrderID)
	}
	dimensions, err := s.dimensions.BatchGetAnalyticsDimensions(ctx, tenantID, orderIDs)
	if err != nil {
		return laneStats, carrierStats, err
	}

	for _, fact := range facts {
		ls := s.hydrateOrderFactDimensions(fact, dimensions)
		laneStats.eligibleCount += ls.eligibleCount
		laneStats.excludedCount += ls.excludedCount
		laneStats.excludedMissingOrigin += ls.excludedMissingOrigin
		laneStats.excludedMissingDest += ls.excludedMissingDest
		laneStats.excludedMissingCountry += ls.excludedMissingCountry
		laneStats.excludedMissingMode += ls.excludedMissingMode

		if fact.CarrierCompanyID != uuid.Nil {
			carrierStats.eligibleCount++
		} else {
			carrierStats.excludedCount++
			carrierStats.excludedMissingCarrier++
		}
		fact.CalculatedAt = now.UTC()
		if err := s.orderFacts.Upsert(ctx, tx, fact); err != nil {
			return laneStats, carrierStats, err
		}
	}
	return laneStats, carrierStats, nil
}

func (s *AnalyticsProjectionService) hydrateSingleOrderFact(
	ctx context.Context,
	fact *domain.AnalyticsOrderFact,
) error {
	if s.dimensions == nil || fact == nil {
		return nil
	}
	dimensions, err := s.dimensions.BatchGetAnalyticsDimensions(ctx, fact.TenantID, []uuid.UUID{fact.TransportOrderID})
	if err != nil {
		return err
	}
	s.hydrateOrderFactDimensions(fact, dimensions)
	return nil
}

func (s *AnalyticsProjectionService) reaggregateLanePeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsLanePeriodKey,
	now time.Time,
) error {
	if s.lanes == nil {
		return nil
	}
	agg, err := s.orderFacts.AggregateLanePeriod(ctx, tx, key)
	if err != nil {
		return err
	}
	if agg.OrderCount == 0 {
		return s.lanes.DeleteByKey(ctx, tx, key)
	}
	projection := &domain.AnalyticsLanePeriodProjection{
		TenantID:             key.TenantID,
		BuyerCompanyID:       key.BuyerCompanyID,
		LaneKey:              key.LaneKey,
		TransportMode:        key.TransportMode,
		EquipmentType:        key.EquipmentType,
		PeriodStart:          key.PeriodStart,
		PeriodGrain:          key.PeriodGrain,
		CurrencyCode:         key.CurrencyCode,
		OrderCount:           agg.OrderCount,
		CarrierCount:         agg.CarrierCount,
		PlannedTotal:         agg.PlannedTotal,
		AccruedTotal:         agg.AccruedTotal,
		CurrentActualTotal:   agg.CurrentActualTotal,
		FinalActualTotal:     agg.FinalActualTotal,
		CurrentVarianceTotal: agg.CurrentVarianceTotal,
		FinalVarianceTotal:   agg.FinalVarianceTotal,
		CalculatedAt:         now.UTC(),
		DataThrough:          agg.MaxSourceUpdatedAt.UTC(),
		ProjectionVersion:    domain.AnalyticsLaneProjectionVersion,
	}
	return s.lanes.Upsert(ctx, tx, projection)
}

func (s *AnalyticsProjectionService) reaggregateCarrierPeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsCarrierPeriodKey,
	now time.Time,
) error {
	if s.carriers == nil {
		return nil
	}
	agg, err := s.orderFacts.AggregateCarrierPeriod(ctx, tx, key)
	if err != nil {
		return err
	}
	if agg.OrderCount == 0 {
		return s.carriers.DeleteByKey(ctx, tx, key)
	}
	projection := &domain.AnalyticsCarrierPeriodProjection{
		TenantID:             key.TenantID,
		BuyerCompanyID:       key.BuyerCompanyID,
		CarrierCompanyID:     key.CarrierCompanyID,
		PeriodStart:          key.PeriodStart,
		PeriodGrain:          key.PeriodGrain,
		CurrencyCode:         key.CurrencyCode,
		OrderCount:           agg.OrderCount,
		LaneCount:            agg.LaneCount,
		PlannedTotal:         agg.PlannedTotal,
		AccruedTotal:         agg.AccruedTotal,
		CurrentActualTotal:   agg.CurrentActualTotal,
		FinalActualTotal:     agg.FinalActualTotal,
		CurrentVarianceTotal: agg.CurrentVarianceTotal,
		FinalVarianceTotal:   agg.FinalVarianceTotal,
		CalculatedAt:         now.UTC(),
		DataThrough:          agg.MaxSourceUpdatedAt.UTC(),
		ProjectionVersion:    domain.AnalyticsCarrierProjectionVersion,
	}
	return s.carriers.Upsert(ctx, tx, projection)
}

func (s *AnalyticsProjectionService) rebuildLaneCarrierProjections(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
	laneStats laneCoverageStats,
	carrierStats carrierCoverageStats,
) error {
	if s.lanes == nil || s.carriers == nil {
		return nil
	}
	laneKeys, err := s.lanes.ListDistinctKeysForTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	for _, key := range laneKeys {
		if err := s.reaggregateLanePeriod(ctx, tx, key, now); err != nil {
			return err
		}
	}
	carrierKeys, err := s.carriers.ListDistinctKeysForTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	for _, key := range carrierKeys {
		if err := s.reaggregateCarrierPeriod(ctx, tx, key, now); err != nil {
			return err
		}
	}
	if err := s.persistCoverage(ctx, tx, tenantID, now, laneStats, carrierStats); err != nil {
		return err
	}
	if err := s.markProjectionStateSuccess(ctx, tx, domain.AnalyticsProjectionNameLane, tenantID, now, now); err != nil {
		return err
	}
	return s.markProjectionStateSuccess(ctx, tx, domain.AnalyticsProjectionNameCarrier, tenantID, now, now)
}

func (s *AnalyticsProjectionService) persistCoverage(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	now time.Time,
	laneStats laneCoverageStats,
	carrierStats carrierCoverageStats,
) error {
	if s.coverage == nil {
		return nil
	}
	laneQuality := domain.DataQualityAvailable
	if laneStats.excludedCount > 0 && laneStats.eligibleCount > 0 {
		laneQuality = domain.DataQualityPartial
	} else if laneStats.eligibleCount == 0 && laneStats.sourceCount > 0 {
		laneQuality = domain.DataQualityNotAvailable
	}
	carrierQuality := domain.DataQualityAvailable
	if carrierStats.excludedCount > 0 && carrierStats.eligibleCount > 0 {
		carrierQuality = domain.DataQualityPartial
	} else if carrierStats.eligibleCount == 0 && carrierStats.sourceCount > 0 {
		carrierQuality = domain.DataQualityNotAvailable
	}
	if err := s.coverage.Upsert(ctx, tx, &domain.AnalyticsProjectionCoverage{
		ProjectionName:                 domain.AnalyticsProjectionNameLane,
		TenantID:                       tenantID,
		CalculatedAt:                   now.UTC(),
		SourceOrderCount:               laneStats.sourceCount,
		EligibleOrderCount:             laneStats.eligibleCount,
		ExcludedOrderCount:             laneStats.excludedCount,
		ExcludedMissingOriginCity:      laneStats.excludedMissingOrigin,
		ExcludedMissingDestinationCity: laneStats.excludedMissingDest,
		ExcludedMissingCountry:         laneStats.excludedMissingCountry,
		ExcludedMissingMode:            laneStats.excludedMissingMode,
		DataQuality:                    laneQuality,
	}); err != nil {
		return err
	}
	return s.coverage.Upsert(ctx, tx, &domain.AnalyticsProjectionCoverage{
		ProjectionName:         domain.AnalyticsProjectionNameCarrier,
		TenantID:               tenantID,
		CalculatedAt:           now.UTC(),
		SourceOrderCount:       carrierStats.sourceCount,
		EligibleOrderCount:     carrierStats.eligibleCount,
		ExcludedOrderCount:     carrierStats.excludedCount,
		ExcludedMissingCarrierID: carrierStats.excludedMissingCarrier,
		DataQuality:            carrierQuality,
	})
}

func (s *AnalyticsProjectionService) markProjectionStateSuccess(
	ctx context.Context,
	tx pgx.Tx,
	projectionName string,
	tenantID uuid.UUID,
	calculatedAt, dataThrough time.Time,
) error {
	calc := calculatedAt.UTC()
	var dataThroughPtr *time.Time
	if !dataThrough.IsZero() {
		value := dataThrough.UTC()
		dataThroughPtr = &value
	}
	version := domain.AnalyticsProjectionVersion
	switch projectionName {
	case domain.AnalyticsProjectionNameLane:
		version = domain.AnalyticsLaneProjectionVersion
	case domain.AnalyticsProjectionNameCarrier:
		version = domain.AnalyticsCarrierProjectionVersion
	case domain.AnalyticsProjectionNameAccessorial:
		version = domain.AnalyticsAccessorialProjectionVersion
	case domain.AnalyticsProjectionNameBenchmark:
		version = domain.AnalyticsBenchmarkProjectionVersion
	case domain.AnalyticsProjectionNameOpportunity:
		version = domain.AnalyticsOpportunityProjectionVersion
	}
	state := &domain.AnalyticsProjectionState{
		ProjectionName:      projectionName,
		TenantID:            tenantID,
		ProjectionVersion:   version,
		LastSuccessfulRunAt: &calc,
		CalculatedAt:        &calc,
		DataThrough:         dataThroughPtr,
		Status:              domain.AnalyticsProjectionStatusReady,
		UpdatedAt:           calc,
	}
	return s.state.Upsert(ctx, tx, state)
}

func collectAffectedSlices(oldFact, newFact *domain.AnalyticsOrderFact) dirtyProcessResult {
	result := dirtyProcessResult{}
	seenPeriod := map[string]struct{}{}
	seenLane := map[string]struct{}{}
	seenCarrier := map[string]struct{}{}
	addPeriod := func(f *domain.AnalyticsOrderFact) {
		if key := f.PeriodKey(); key != nil {
			id := periodKeyString(*key)
			if _, ok := seenPeriod[id]; !ok {
				seenPeriod[id] = struct{}{}
				result.periods = append(result.periods, *key)
			}
		}
	}
	addLane := func(f *domain.AnalyticsOrderFact) {
		if key := f.LanePeriodKey(); key != nil {
			id := laneSliceID(*key)
			if _, ok := seenLane[id]; !ok {
				seenLane[id] = struct{}{}
				result.lanes = append(result.lanes, *key)
			}
		}
	}
	addCarrier := func(f *domain.AnalyticsOrderFact) {
		if key := f.CarrierPeriodKey(); key != nil {
			id := carrierSliceID(*key)
			if _, ok := seenCarrier[id]; !ok {
				seenCarrier[id] = struct{}{}
				result.carriers = append(result.carriers, *key)
			}
		}
	}
	if oldFact != nil {
		addPeriod(oldFact)
		addLane(oldFact)
		addCarrier(oldFact)
	}
	if newFact != nil {
		addPeriod(newFact)
		addLane(newFact)
		addCarrier(newFact)
	}
	return result
}

func laneSliceID(key domain.AnalyticsLanePeriodKey) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		key.TenantID, key.BuyerCompanyID, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart.Format("2006-01-02"), key.PeriodGrain, key.CurrencyCode)
}

func carrierSliceID(key domain.AnalyticsCarrierPeriodKey) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		key.TenantID, key.BuyerCompanyID, key.CarrierCompanyID,
		key.PeriodStart.Format("2006-01-02"), key.PeriodGrain, key.CurrencyCode)
}

func (s *AnalyticsProjectionService) reaggregateAffectedSlices(
	ctx context.Context,
	periods map[string]domain.AnalyticsPeriodKey,
	lanes map[string]domain.AnalyticsLanePeriodKey,
	carriers map[string]domain.AnalyticsCarrierPeriodKey,
) error {
	now := time.Now().UTC()
	for _, key := range periods {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, key.TenantID); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := s.reaggregatePeriod(ctx, tx, key, now); err != nil {
			tx.Rollback(ctx)
			return err
		}
		var maxUpdated time.Time
		if agg, aggErr := s.orderFacts.AggregatePeriod(ctx, tx, key); aggErr == nil && agg != nil {
			maxUpdated = agg.MaxSourceUpdatedAt
		}
		if err := s.markStateSuccess(ctx, tx, key.TenantID, now, maxUpdated); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	for _, key := range lanes {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, key.TenantID); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := s.reaggregateLanePeriod(ctx, tx, key, now); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	for _, key := range carriers {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, key.TenantID); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := s.reaggregateCarrierPeriod(ctx, tx, key, now); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *AnalyticsProjectionService) ListLaneProjections(
	ctx context.Context,
	tenantID uuid.UUID,
	filter repository.AnalyticsLaneListFilter,
) ([]domain.AnalyticsLanePeriodProjection, error) {
	if s.lanes == nil {
		return nil, nil
	}
	return s.lanes.List(ctx, tenantID, filter)
}

func (s *AnalyticsProjectionService) ListCarrierProjections(
	ctx context.Context,
	tenantID uuid.UUID,
	filter repository.AnalyticsCarrierListFilter,
) ([]domain.AnalyticsCarrierPeriodProjection, error) {
	if s.carriers == nil {
		return nil, nil
	}
	return s.carriers.List(ctx, tenantID, filter)
}

func (s *AnalyticsProjectionService) GetCoverage(
	ctx context.Context,
	projectionName string,
	tenantID uuid.UUID,
) (*domain.AnalyticsProjectionCoverage, error) {
	if s.coverage == nil {
		return nil, nil
	}
	return s.coverage.Get(ctx, projectionName, tenantID)
}
