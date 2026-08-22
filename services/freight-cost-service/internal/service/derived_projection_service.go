package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/client/billing_register"
	"github.com/freight-platform/freight-cost-service/internal/client/transport_order"
	"github.com/freight-platform/freight-cost-service/internal/domain"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type DerivedProjectionService struct {
	pool         *pgxpool.Pool
	projections  *repository.CostSummaryProjectionRepository
	attributions *repository.VarianceAttributionRepository
	findings     *repository.ReconciliationFindingRepository
	mappings     *repository.ChargeCodeMappingRepository
	cursors      *repository.SourceCursorRepository
	billing      *billing_register.Client
	transport    *transport_order.Client
	metrics      *fcmetrics.Metrics
}

func NewDerivedProjectionService(
	pool *pgxpool.Pool,
	projections *repository.CostSummaryProjectionRepository,
	attributions *repository.VarianceAttributionRepository,
	findings *repository.ReconciliationFindingRepository,
	mappings *repository.ChargeCodeMappingRepository,
	cursors *repository.SourceCursorRepository,
	billing *billing_register.Client,
	transport *transport_order.Client,
	metrics *fcmetrics.Metrics,
) *DerivedProjectionService {
	return &DerivedProjectionService{
		pool:         pool,
		projections:  projections,
		attributions: attributions,
		findings:     findings,
		mappings:     mappings,
		cursors:      cursors,
		billing:      billing,
		transport:    transport,
		metrics:      metrics,
	}
}

func (s *DerivedProjectionService) RecomputeInTransaction(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.CostSummaryProjection,
	proposed domain.ProposedAccessorialInput,
	driverCtx domain.DriverAttributionContext,
) error {
	if projection == nil {
		return nil
	}
	stateChanged, err := domain.RecomputeDerivedProjection(projection, proposed)
	if err != nil {
		return err
	}
	if !stateChanged {
		return nil
	}

	evalTime := time.Now().UTC()
	platformMappings, tenantMappings, mappingVersion, _, err := s.resolveMappingContext(ctx, tx, projection, evalTime)
	if err != nil {
		return err
	}

	if driverCtx.MappingVersion == 0 {
		driverCtx.MappingVersion = mappingVersion
	}
	driverCtx.PlatformMappings = platformMappings
	driverCtx.TenantMappings = tenantMappings

	if len(driverCtx.ApprovedAccessorials) == 0 && driverCtx.BaseFreightAmount == nil {
		canonical, _, hydrateErr := s.buildCanonicalDriverContext(ctx, projection.TenantID, projection.TransportOrderID)
		if hydrateErr != nil {
			return hydrateErr
		}
		driverCtx.ApprovedAccessorials = canonical.ApprovedAccessorials
		driverCtx.BaseFreightAmount = canonical.BaseFreightAmount
	}

	if err := s.attributions.MarkSuperseded(ctx, tx, projection.TenantID, projection.TransportOrderID, projection.ProjectionRevision); err != nil {
		return err
	}

	attributionRows := s.buildAttributionRows(projection, driverCtx)
	inserted, err := s.attributions.InsertBatch(ctx, tx, attributionRows)
	if err != nil {
		return err
	}
	_ = inserted

	s.observeRecompute(projection, proposed, attributionRows)
	return nil
}

func (s *DerivedProjectionService) ReconcileTransportOrder(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) (int, error) {
	if s == nil || s.pool == nil || s.projections == nil {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	projection, err := s.projections.GetByTransportOrderTx(ctx, tx, tenantID, transportOrderID)
	if err != nil {
		return 0, err
	}
	beforeRevision := projection.ProjectionRevision

	findings, err := s.detectCanonicalFindings(ctx, tx, projection)
	if err != nil {
		return 0, err
	}
	count, err := s.persistFindings(ctx, tx, projection, findings)
	if err != nil {
		return 0, err
	}
	if projection.ProjectionRevision != beforeRevision {
		return 0, domain.ErrReconciliationMutatedProjection
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *DerivedProjectionService) ReclassifyAttribution(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) (int, error) {
	if s == nil || s.pool == nil {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	projection, err := s.projections.GetByTransportOrderTx(ctx, tx, tenantID, transportOrderID)
	if err != nil {
		return 0, err
	}
	beforeVariance := cloneDecimal(projection.CurrentVarianceAmount)
	beforeFinalVariance := cloneDecimal(projection.FinalVarianceAmount)

	driverCtx, _, err := s.buildCanonicalDriverContext(ctx, tenantID, transportOrderID)
	if err != nil {
		return 0, err
	}

	evalTime := time.Now().UTC()
	platformMappings, tenantMappings, loadedVersion, err := s.mappings.LoadActiveMappings(ctx, tx, tenantID, evalTime)
	if err != nil {
		return 0, err
	}
	platformVersion, err := s.mappings.CurrentPlatformMappingVersion(ctx)
	if err != nil {
		return 0, err
	}
	pinVersion := platformVersion
	if loadedVersion > pinVersion {
		pinVersion = loadedVersion
	}
	driverCtx.MappingVersion = pinVersion
	driverCtx.PlatformMappings = platformMappings
	driverCtx.TenantMappings = tenantMappings
	pinnedVersion := pinVersion
	projection.AttributionMappingVersion = &pinnedVersion
	projection.AttributionMappingEvaluatedAt = &evalTime
	if err := s.attributions.MarkDriversSuperseded(ctx, tx, tenantID, transportOrderID); err != nil {
		return 0, err
	}

	var attributionRows []domain.VarianceAttribution
	for _, kind := range []string{domain.VarianceKindCurrent, domain.VarianceKindFinal} {
		attributionRows = append(attributionRows, domain.BuildVarianceDrivers(projection, kind, driverCtx)...)
	}
	inserted, err := s.attributions.UpsertReclassifyBatch(ctx, tx, attributionRows)
	if err != nil {
		return 0, err
	}
	if err := s.projections.Upsert(ctx, tx, projection); err != nil {
		return 0, err
	}
	if !decimalEqual(beforeVariance, projection.CurrentVarianceAmount) || !decimalEqual(beforeFinalVariance, projection.FinalVarianceAmount) {
		return 0, domain.ErrReclassificationChangedFinancialAmounts
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	if s.metrics != nil {
		s.metrics.ObserveVarianceRecomputed("reclassified")
	}
	return inserted, nil
}

func (s *DerivedProjectionService) EnsureLegacyDerivedStateBootstrapped(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) error {
	if s == nil || s.pool == nil || s.projections == nil {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	projection, err := s.projections.GetByTransportOrderTx(ctx, tx, tenantID, transportOrderID)
	if err != nil {
		return err
	}
	changed := false
	if projection.DerivedStateFingerprint == nil || *projection.DerivedStateFingerprint == "" {
		if _, err := domain.RecomputeDerivedProjection(projection, domain.ProposedAccessorialInput{
			SourceStatus: domain.ProposedSourceUnknown,
		}); err != nil {
			return err
		}
		changed = true
	}
	if projection.AttributionMappingVersion != nil && projection.AttributionMappingEvaluatedAt == nil {
		evalTime := time.Now().UTC()
		if err := s.bootstrapLegacyMappingPin(ctx, tx, projection, evalTime); err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	if err := s.projections.Upsert(ctx, tx, projection); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DerivedProjectionService) resolveMappingContext(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.CostSummaryProjection,
	currentEvalTime time.Time,
) (platformMappings []domain.ChargeCodeMapping, tenantMappings []domain.ChargeCodeMapping, mappingVersion int64, pinEvalTime time.Time, err error) {
	if projection == nil {
		return nil, nil, 0, currentEvalTime, nil
	}

	if projection.AttributionMappingVersion != nil && projection.AttributionMappingEvaluatedAt != nil {
		pinEvalTime = projection.AttributionMappingEvaluatedAt.UTC()
		platformMappings, tenantMappings, _, err = s.mappings.LoadPinnedMappings(
			ctx, tx, projection.TenantID, *projection.AttributionMappingVersion, pinEvalTime,
		)
		if err != nil {
			return nil, nil, 0, pinEvalTime, err
		}
		return platformMappings, tenantMappings, *projection.AttributionMappingVersion, pinEvalTime, nil
	}

	evalTime := currentEvalTime.UTC()
	if projection.AttributionMappingVersion != nil && projection.AttributionMappingEvaluatedAt == nil {
		if err := s.bootstrapLegacyMappingPin(ctx, tx, projection, evalTime); err != nil {
			return nil, nil, 0, evalTime, err
		}
		platformMappings, tenantMappings, _, err = s.mappings.LoadPinnedMappings(
			ctx, tx, projection.TenantID, *projection.AttributionMappingVersion, evalTime,
		)
		if err != nil {
			return nil, nil, 0, evalTime, err
		}
		return platformMappings, tenantMappings, *projection.AttributionMappingVersion, evalTime, nil
	}

	platformMappings, tenantMappings, loadedVersion, err := s.mappings.LoadActiveMappings(ctx, tx, projection.TenantID, evalTime)
	if err != nil {
		return nil, nil, 0, evalTime, err
	}
	pinVersion, err := s.mappingPinVersion(ctx, loadedVersion)
	if err != nil {
		return nil, nil, 0, evalTime, err
	}
	if projection.AttributionMappingVersion == nil {
		projection.AttributionMappingVersion = &pinVersion
		projection.AttributionMappingEvaluatedAt = &evalTime
	}
	return platformMappings, tenantMappings, pinVersion, evalTime, nil
}

func (s *DerivedProjectionService) bootstrapLegacyMappingPin(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.CostSummaryProjection,
	evalTime time.Time,
) error {
	if projection == nil || projection.AttributionMappingVersion == nil || projection.AttributionMappingEvaluatedAt != nil {
		return nil
	}
	platformMappings, tenantMappings, loadedVersion, err := s.mappings.LoadActiveMappings(ctx, tx, projection.TenantID, evalTime)
	if err != nil {
		return err
	}
	pinVersion, err := s.mappingPinVersion(ctx, loadedVersion)
	if err != nil {
		return err
	}
	_ = platformMappings
	_ = tenantMappings
	projection.AttributionMappingVersion = &pinVersion
	eval := evalTime.UTC()
	projection.AttributionMappingEvaluatedAt = &eval
	return nil
}

func (s *DerivedProjectionService) mappingPinVersion(ctx context.Context, loadedVersion int64) (int64, error) {
	platformVersion, err := s.mappings.CurrentPlatformMappingVersion(ctx)
	if err != nil {
		return 0, err
	}
	if loadedVersion > platformVersion {
		return loadedVersion, nil
	}
	return platformVersion, nil
}

func (s *DerivedProjectionService) buildCanonicalDriverContext(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) (domain.DriverAttributionContext, int64, error) {
	var snapshot *provider.RateSnapshotFact
	if s.transport != nil {
		snap, err := s.transport.GetRateSnapshot(ctx, tenantID, transportOrderID)
		if err != nil && !isNotFoundErr(err) {
			return domain.DriverAttributionContext{}, 0, err
		}
		snapshot = snap
	}
	var settlement *billing_register.SettlementFact
	if s.billing != nil {
		settle, err := s.billing.GetSettlementByTransportOrder(ctx, tenantID, transportOrderID)
		if err != nil && !isNotFoundErr(err) {
			return domain.DriverAttributionContext{}, 0, err
		}
		settlement = settle
	}
	return buildDriverContextFromCanonical(settlement, snapshot), 0, nil
}

func (s *DerivedProjectionService) detectCanonicalFindings(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.CostSummaryProjection,
) ([]domain.ReconciliationFinding, error) {
	if projection == nil {
		return nil, nil
	}
	var snapshot *provider.RateSnapshotFact
	if s.transport != nil {
		snap, err := s.transport.GetRateSnapshot(ctx, projection.TenantID, projection.TransportOrderID)
		if err != nil && !isNotFoundErr(err) {
			return nil, err
		}
		snapshot = snap
	}
	var settlement *billing_register.SettlementFact
	var billingLink *billing_register.BillingLinkFact
	if s.billing != nil {
		settle, err := s.billing.GetSettlementByTransportOrder(ctx, projection.TenantID, projection.TransportOrderID)
		if err != nil && !isNotFoundErr(err) {
			return nil, err
		}
		settlement = settle
		if settlement != nil {
			link, linkErr := s.billing.GetBillingLink(ctx, projection.TenantID, settlement.SettlementID)
			if linkErr != nil && !isNotFoundErr(linkErr) {
				return nil, linkErr
			}
			billingLink = link
		}
	}
	var cursors []domain.SourceCursor
	if s.cursors != nil {
		listed, err := s.cursors.ListByTransportOrder(ctx, tx, projection.TenantID, projection.TransportOrderID)
		if err != nil {
			return nil, err
		}
		cursors = listed
	}
	return detectCanonicalReconciliationFindings(canonicalReconciliationContext{
		stored:      projection,
		snapshot:    snapshot,
		settlement:  settlement,
		billingLink: billingLink,
		cursors:     cursors,
	}), nil
}

func (s *DerivedProjectionService) persistFindings(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.CostSummaryProjection,
	findings []domain.ReconciliationFinding,
) (int, error) {
	if findings == nil {
		var err error
		findings, err = s.detectCanonicalFindings(ctx, tx, projection)
		if err != nil {
			return 0, err
		}
	}
	activeIDs := make([]uuid.UUID, 0, len(findings))
	for _, finding := range findings {
		activeIDs = append(activeIDs, finding.FindingID)
	}
	if err := s.findings.UpsertBatch(ctx, tx, findings); err != nil {
		return 0, err
	}
	if err := s.findings.ResolveAbsentFindings(ctx, tx, projection.TenantID, projection.TransportOrderID, activeIDs); err != nil {
		return 0, err
	}
	if s.metrics != nil {
		for _, finding := range findings {
			s.metrics.ObserveReconciliationFinding(finding.FindingKind, "info")
		}
	}
	return len(findings), nil
}

func (s *DerivedProjectionService) buildAttributionRows(
	projection *domain.CostSummaryProjection,
	driverCtx domain.DriverAttributionContext,
) []domain.VarianceAttribution {
	var rows []domain.VarianceAttribution
	for _, kind := range []string{domain.VarianceKindCurrent, domain.VarianceKindFinal} {
		rows = append(rows, domain.BuildAvailabilityReasons(projection, kind)...)
		rows = append(rows, domain.BuildVarianceDrivers(projection, kind, driverCtx)...)
	}
	return rows
}

func (s *DerivedProjectionService) observeRecompute(
	projection *domain.CostSummaryProjection,
	proposed domain.ProposedAccessorialInput,
	attributionRows []domain.VarianceAttribution,
) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveVarianceRecomputed("success")
	if proposed.SourceStatus == domain.ProposedSourceKnown {
		s.metrics.ObserveForecastRecomputed("success")
	} else if proposed.SourceStatus == domain.ProposedSourceUnknown {
		s.metrics.ObserveForecastProposedSourceUnknown()
	}
	for _, row := range attributionRows {
		if row.ReasonCode == domain.ReasonUnattributed {
			continue
		}
		if category, ok := row.EvidenceJSON["category"].(string); ok && category == domain.CategoryOther {
			s.metrics.ObserveChargeCodeUnmapped()
		}
	}
}

func (s *DerivedProjectionService) EnrichForecastFromBilling(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) error {
	if s == nil || s.billing == nil || s.projections == nil {
		return nil
	}
	settlement, err := s.billing.GetSettlementByTransportOrder(ctx, tenantID, transportOrderID)
	if err != nil {
		if isNotFoundErr(err) {
			return nil
		}
		if s.metrics != nil {
			s.metrics.ObserveForecastProposedSourceUnknown()
		}
		return err
	}
	return s.enrichForecastFromSettlement(ctx, tenantID, transportOrderID, settlement)
}

func (s *DerivedProjectionService) EnrichForecastFromSettlement(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
	settlement *billing_register.SettlementFact,
) error {
	return s.enrichForecastFromSettlement(ctx, tenantID, transportOrderID, settlement)
}

func (s *DerivedProjectionService) enrichForecastFromSettlement(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
	settlement *billing_register.SettlementFact,
) error {
	if s == nil || s.projections == nil || settlement == nil {
		return nil
	}
	proposed := ProposedInputFromSettlement(settlement)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	projection, err := s.projections.GetByTransportOrderTx(ctx, tx, tenantID, transportOrderID)
	if err != nil {
		return err
	}
	stateChanged, err := domain.RecomputeDerivedProjection(projection, proposed)
	if err != nil {
		return err
	}
	if err := s.projections.Upsert(ctx, tx, projection); err != nil {
		return err
	}
	if stateChanged {
		if err := s.RecomputeInTransaction(ctx, tx, projection, proposed, domain.DriverAttributionContext{}); err != nil {
			return err
		}
		if err := s.projections.Upsert(ctx, tx, projection); err != nil {
			return err
		}
	}
	if s.metrics != nil {
		if proposed.SourceStatus == domain.ProposedSourceKnown {
			s.metrics.ObserveForecastRecomputed("success")
		} else {
			s.metrics.ObserveForecastProposedSourceUnknown()
		}
	}
	return tx.Commit(ctx)
}

func ProposedInputFromSettlement(settlement *billing_register.SettlementFact) domain.ProposedAccessorialInput {
	if settlement == nil || settlement.ProposedAccessorialSourceStatus != domain.ProposedSourceKnown {
		return domain.ProposedAccessorialInput{SourceStatus: domain.ProposedSourceUnknown}
	}
	return domain.ProposedAccessorialInput{
		SourceStatus: domain.ProposedSourceKnown,
		TotalExVAT:   settlement.ProposedAccessorialTotalExVAT,
	}
}

func cloneDecimal(value *decimal.Decimal) *decimal.Decimal {
	if value == nil {
		return nil
	}
	v := value.Round(domain.MoneyScale)
	return &v
}

func decimalEqual(a, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Round(domain.MoneyScale).Equal(b.Round(domain.MoneyScale))
}
