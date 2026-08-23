package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type AnalyticsProjectionService struct {
	pool               *pgxpool.Pool
	summaries          *repository.CostSummaryProjectionRepository
	orderFacts         *repository.AnalyticsOrderFactRepository
	periods            *repository.AnalyticsPeriodProjectionRepository
	lanes              *repository.AnalyticsLanePeriodProjectionRepository
	carriers           *repository.AnalyticsCarrierPeriodProjectionRepository
	accessorialFacts   *repository.AnalyticsAccessorialFactRepository
	accessorialPeriods *repository.AnalyticsAccessorialPeriodProjectionRepository
	coverage           *repository.AnalyticsProjectionCoverageRepository
	state              *repository.AnalyticsProjectionStateRepository
	dirty              *repository.AnalyticsDirtyQueueRepository
	mappings           *repository.ChargeCodeMappingRepository
	dimensions         provider.TransportDimensionReader
	companies          provider.CompanyDisplayReader
	settlements        provider.SettlementAccessorialReader
	metrics            *fcmetrics.Metrics
	projectionVersion  int
}

func NewAnalyticsProjectionService(
	pool *pgxpool.Pool,
	summaries *repository.CostSummaryProjectionRepository,
	orderFacts *repository.AnalyticsOrderFactRepository,
	periods *repository.AnalyticsPeriodProjectionRepository,
	lanes *repository.AnalyticsLanePeriodProjectionRepository,
	carriers *repository.AnalyticsCarrierPeriodProjectionRepository,
	accessorialFacts *repository.AnalyticsAccessorialFactRepository,
	accessorialPeriods *repository.AnalyticsAccessorialPeriodProjectionRepository,
	coverage *repository.AnalyticsProjectionCoverageRepository,
	state *repository.AnalyticsProjectionStateRepository,
	dirty *repository.AnalyticsDirtyQueueRepository,
	mappings *repository.ChargeCodeMappingRepository,
	dimensions provider.TransportDimensionReader,
	companies provider.CompanyDisplayReader,
	settlements provider.SettlementAccessorialReader,
	metrics *fcmetrics.Metrics,
) *AnalyticsProjectionService {
	return &AnalyticsProjectionService{
		pool:               pool,
		summaries:          summaries,
		orderFacts:         orderFacts,
		periods:            periods,
		lanes:              lanes,
		carriers:           carriers,
		accessorialFacts:   accessorialFacts,
		accessorialPeriods: accessorialPeriods,
		coverage:           coverage,
		state:              state,
		dirty:              dirty,
		mappings:           mappings,
		dimensions:         dimensions,
		companies:          companies,
		settlements:        settlements,
		metrics:            metrics,
		projectionVersion:  domain.AnalyticsProjectionVersion,
	}
}

type AnalyticsChangeInput struct {
	TenantID         uuid.UUID
	TransportOrderID uuid.UUID
	BuyerCompanyID   uuid.UUID
	CurrencyCode     string
	SummaryUpdatedAt time.Time
	SourceEventID    uuid.UUID
}

// MarkCostSummaryChanged records a dirty analytics slice in the same transaction as canonical ingest.
func (s *AnalyticsProjectionService) MarkCostSummaryChanged(
	ctx context.Context,
	tx pgx.Tx,
	input AnalyticsChangeInput,
) error {
	if s == nil {
		return nil
	}
	currency := strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if currency == "" {
		return nil
	}
	now := time.Now().UTC()
	entry := domain.AnalyticsDirtyEntry{
		TenantID:         input.TenantID,
		TransportOrderID: input.TransportOrderID,
		BuyerCompanyID:   input.BuyerCompanyID,
		CurrencyCode:     currency,
		PeriodStart:      domain.PeriodStartFromSummaryUpdatedAt(input.SummaryUpdatedAt),
		PeriodGrain:      domain.AnalyticsPeriodGrainMonth,
		DirtyAt:          now,
	}
	if input.SourceEventID != uuid.Nil {
		entry.SourceEventID = &input.SourceEventID
	}
	return s.dirty.MarkDirty(ctx, tx, entry)
}

func (s *AnalyticsProjectionService) RebuildTenant(ctx context.Context, tenantID uuid.UUID) error {
	start := time.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperrors.Internal("begin tenant analytics rebuild", err)
	}
	defer tx.Rollback(ctx)

	if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, tenantID); err != nil {
		return apperrors.Internal("acquire analytics rebuild lock", err)
	}

	now := time.Now().UTC()
	if err := s.setStateRunning(ctx, tx, tenantID, now); err != nil {
		return err
	}

	if err := s.orderFacts.DeleteByTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if err := s.periods.DeleteByTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if s.lanes != nil {
		if err := s.lanes.DeleteByTenant(ctx, tx, tenantID); err != nil {
			return err
		}
	}
	if s.carriers != nil {
		if err := s.carriers.DeleteByTenant(ctx, tx, tenantID); err != nil {
			return err
		}
	}
	if s.accessorialFacts != nil {
		if err := s.accessorialFacts.DeleteByTenant(ctx, tx, tenantID); err != nil {
			return err
		}
	}
	if s.accessorialPeriods != nil {
		if err := s.accessorialPeriods.DeleteByTenant(ctx, tx, tenantID); err != nil {
			return err
		}
	}
	if s.coverage != nil {
		if err := s.coverage.DeleteByTenant(ctx, tx, tenantID); err != nil {
			return err
		}
	}

	rows, err := s.summaries.ListSummariesForTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}

	var maxDataThrough time.Time
	for _, row := range rows {
		fact := domain.OrderFactFromCostSummary(row.Projection, row.UpdatedAt, now)
		if fact == nil {
			continue
		}
		if err := s.orderFacts.Upsert(ctx, tx, fact); err != nil {
			return err
		}
		if row.UpdatedAt.After(maxDataThrough) {
			maxDataThrough = row.UpdatedAt.UTC()
		}
	}

	periodKeys, err := s.orderFacts.ListDistinctPeriodKeysForTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	for _, key := range periodKeys {
		if err := s.reaggregatePeriod(ctx, tx, key, now); err != nil {
			return err
		}
	}

	laneStats, carrierStats, err := s.hydrateTenantOrderFacts(ctx, tx, tenantID, now)
	if err != nil {
		return err
	}
	if s.lanes != nil && s.carriers != nil {
		if err := s.rebuildLaneCarrierProjections(ctx, tx, tenantID, now, laneStats, carrierStats); err != nil {
			return err
		}
	}

	enrichmentStats, err := s.hydrateTenantOrderFactEnrichment(ctx, tx, tenantID, now)
	if err != nil {
		return err
	}
	accessorialStats, err := s.rebuildTenantAccessorialFacts(ctx, tx, tenantID, now)
	if err != nil {
		return err
	}
	if s.accessorialPeriods != nil {
		if err := s.rebuildAccessorialPeriodProjections(ctx, tx, tenantID, now); err != nil {
			return err
		}
	}
	if s.accessorialFacts != nil {
		if err := s.rebuildAccessorialAndEnrichment(ctx, tx, tenantID, now, enrichmentStats, accessorialStats); err != nil {
			return err
		}
	}

	if err := s.dirty.DeleteByTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if err := s.markStateSuccess(ctx, tx, tenantID, now, maxDataThrough); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		s.observeRebuild("error")
		return apperrors.Internal("commit tenant analytics rebuild", err)
	}
	s.observeRebuild("success")
	s.observeRebuildDuration(time.Since(start))
	return nil
}

func (s *AnalyticsProjectionService) ProcessDirtyBatch(ctx context.Context, limit int) (int, error) {
	entries, err := s.dirty.ListBatch(ctx, limit)
	if err != nil {
		s.observeIncremental("claim_error")
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}

	processed := 0
	periodSeen := make(map[string]domain.AnalyticsPeriodKey)
	laneSeen := make(map[string]domain.AnalyticsLanePeriodKey)
	carrierSeen := make(map[string]domain.AnalyticsCarrierPeriodKey)
	accessorialSeen := make(map[string]domain.AnalyticsAccessorialPeriodKey)
	for _, entry := range entries {
		result, err := s.processDirtyEntry(ctx, entry)
		if err != nil {
			s.observeIncremental("error")
			if stateErr := s.markTenantError(ctx, entry.TenantID, "INCREMENTAL_APPLY_FAILED", sanitizeError(err)); stateErr != nil {
				return processed, stateErr
			}
			continue
		}
		if result != nil {
			for _, key := range result.periods {
				periodSeen[periodKeyString(key)] = key
			}
			for _, key := range result.lanes {
				laneSeen[laneSliceID(key)] = key
			}
			for _, key := range result.carriers {
				carrierSeen[carrierSliceID(key)] = key
			}
			for _, key := range result.accessorials {
				accessorialSeen[accessorialSliceID(key)] = key
			}
		}
		processed++
		s.observeIncremental("processed")
	}

	if err := s.reaggregateAffectedSlices(ctx, periodSeen, laneSeen, carrierSeen); err != nil {
		return processed, err
	}
	if err := s.reaggregateAffectedAccessorialSlices(ctx, accessorialSeen); err != nil {
		return processed, err
	}
	return processed, nil
}

func (s *AnalyticsProjectionService) processDirtyEntry(ctx context.Context, entry domain.AnalyticsDirtyEntry) (*dirtyProcessResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := repository.AcquireTenantAnalyticsExclusiveLock(ctx, tx, entry.TenantID); err != nil {
		return nil, err
	}

	var previous *domain.AnalyticsOrderFact
	if prev, prevErr := s.orderFacts.GetByKey(ctx, tx, entry.TenantID, entry.TransportOrderID, entry.CurrencyCode); prevErr == nil {
		previous = prev
	}
	var previousAccessorials []domain.AnalyticsAccessorialFact
	if s.accessorialFacts != nil {
		if prevAcc, prevAccErr := s.accessorialFacts.ListByTransportOrder(ctx, tx, entry.TenantID, entry.TransportOrderID); prevAccErr == nil {
			previousAccessorials = prevAcc
		}
	}

	summaryRow, err := s.summaries.GetSummaryRowByTransportOrderTx(ctx, tx, entry.TenantID, entry.TransportOrderID)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			if err := s.dirty.Delete(ctx, tx, entry); err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}

	now := time.Now().UTC()
	fact := domain.OrderFactFromCostSummary(summaryRow.Projection, summaryRow.UpdatedAt, now)
	if fact == nil {
		if err := s.dirty.Delete(ctx, tx, entry); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err := s.hydrateSingleOrderFact(ctx, fact); err != nil {
		return nil, err
	}
	if err := s.hydrateSingleOrderFactEnrichment(ctx, fact); err != nil {
		return nil, err
	}
	if err := s.orderFacts.Upsert(ctx, tx, fact); err != nil {
		return nil, err
	}
	if _, err := s.rebuildOrderAccessorialFacts(ctx, tx, fact, now); err != nil {
		return nil, err
	}
	var newAccessorials []domain.AnalyticsAccessorialFact
	if s.accessorialFacts != nil {
		newAccessorials, err = s.accessorialFacts.ListByTransportOrder(ctx, tx, entry.TenantID, entry.TransportOrderID)
		if err != nil {
			return nil, err
		}
	}
	if err := s.dirty.Delete(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	slices := collectAffectedSlices(previous, fact)
	slices.accessorials = collectAffectedAccessorialSlices(previousAccessorials, newAccessorials)
	return &slices, nil
}

func (s *AnalyticsProjectionService) reaggregatePeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsPeriodKey,
	now time.Time,
) error {
	agg, err := s.orderFacts.AggregatePeriod(ctx, tx, key)
	if err != nil {
		return err
	}
	reconciliationCount, err := s.periods.CountOpenReconciliations(ctx, tx, key)
	if err != nil {
		return err
	}
	projection := &domain.AnalyticsPeriodProjection{
		TenantID:                key.TenantID,
		BuyerCompanyID:          key.BuyerCompanyID,
		PeriodStart:             key.PeriodStart,
		PeriodGrain:             key.PeriodGrain,
		CurrencyCode:            key.CurrencyCode,
		OrderCount:              agg.OrderCount,
		PlannedTotal:            agg.PlannedTotal,
		AccruedTotal:            agg.AccruedTotal,
		CurrentActualTotal:      agg.CurrentActualTotal,
		FinalActualTotal:        agg.FinalActualTotal,
		CurrentVarianceTotal:    agg.CurrentVarianceTotal,
		FinalVarianceTotal:      agg.FinalVarianceTotal,
		ReconciliationOpenCount: reconciliationCount,
		CalculatedAt:            now.UTC(),
		DataThrough:             agg.MaxSourceUpdatedAt.UTC(),
		ProjectionVersion:       s.projectionVersion,
	}
	return s.periods.Upsert(ctx, tx, projection)
}

func (s *AnalyticsProjectionService) setStateRunning(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, now time.Time) error {
	state := &domain.AnalyticsProjectionState{
		ProjectionName:    domain.AnalyticsProjectionNamePeriod,
		TenantID:          tenantID,
		ProjectionVersion: s.projectionVersion,
		Status:            domain.AnalyticsProjectionStatusRunning,
		UpdatedAt:         now,
	}
	return s.state.Upsert(ctx, tx, state)
}

func (s *AnalyticsProjectionService) markStateSuccess(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	calculatedAt, dataThrough time.Time,
) error {
	var watermark *time.Time
	if !dataThrough.IsZero() {
		value := dataThrough.UTC()
		watermark = &value
	}
	calc := calculatedAt.UTC()
	var dataThroughPtr *time.Time
	if !dataThrough.IsZero() {
		value := dataThrough.UTC()
		dataThroughPtr = &value
	}
	state := &domain.AnalyticsProjectionState{
		ProjectionName:      domain.AnalyticsProjectionNamePeriod,
		TenantID:            tenantID,
		ProjectionVersion:   s.projectionVersion,
		SourceWatermark:     watermark,
		LastSuccessfulRunAt: &calc,
		CalculatedAt:        &calc,
		DataThrough:         dataThroughPtr,
		Status:              domain.AnalyticsProjectionStatusReady,
		UpdatedAt:           calc,
	}
	return s.state.Upsert(ctx, tx, state)
}

func (s *AnalyticsProjectionService) markTenantError(ctx context.Context, tenantID uuid.UUID, code, message string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	codeCopy := code
	msgCopy := message
	state := &domain.AnalyticsProjectionState{
		ProjectionName:    domain.AnalyticsProjectionNamePeriod,
		TenantID:          tenantID,
		ProjectionVersion: s.projectionVersion,
		Status:            domain.AnalyticsProjectionStatusError,
		LastErrorCode:     &codeCopy,
		LastErrorMessage:  &msgCopy,
		UpdatedAt:         now,
	}
	if err := s.state.Upsert(ctx, tx, state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func periodKeyString(key domain.AnalyticsPeriodKey) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s",
		key.TenantID, key.BuyerCompanyID, key.PeriodStart.Format("2006-01-02"), key.PeriodGrain, key.CurrencyCode)
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}

func (s *AnalyticsProjectionService) observeRebuild(result string) {
	if s.metrics != nil {
		s.metrics.ObserveAnalyticsRebuild(result)
	}
}

func (s *AnalyticsProjectionService) observeRebuildDuration(d time.Duration) {
	if s.metrics != nil {
		s.metrics.ObserveAnalyticsRebuildDuration(d)
	}
}

func (s *AnalyticsProjectionService) observeIncremental(result string) {
	if s.metrics != nil {
		s.metrics.ObserveAnalyticsIncremental(result)
	}
}
