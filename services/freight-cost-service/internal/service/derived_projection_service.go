package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/client/billing_register"
	"github.com/freight-platform/freight-cost-service/internal/domain"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type DerivedProjectionService struct {
	pool         *pgxpool.Pool
	projections  *repository.CostSummaryProjectionRepository
	attributions *repository.VarianceAttributionRepository
	findings     *repository.ReconciliationFindingRepository
	mappings     *repository.ChargeCodeMappingRepository
	billing      *billing_register.Client
	metrics      *fcmetrics.Metrics
}

func NewDerivedProjectionService(
	pool *pgxpool.Pool,
	projections *repository.CostSummaryProjectionRepository,
	attributions *repository.VarianceAttributionRepository,
	findings *repository.ReconciliationFindingRepository,
	mappings *repository.ChargeCodeMappingRepository,
	billing *billing_register.Client,
	metrics *fcmetrics.Metrics,
) *DerivedProjectionService {
	return &DerivedProjectionService{
		pool:         pool,
		projections:  projections,
		attributions: attributions,
		findings:     findings,
		mappings:     mappings,
		billing:      billing,
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
	priorForecast := projection.ForecastExposure
	priorMappingVersion := projection.AttributionMappingVersion

	if err := domain.RecomputeDerivedProjection(projection, proposed, priorForecast); err != nil {
		return err
	}

	platformMappings, tenantMappings, tableVersion, err := s.mappings.LoadActiveMappings(ctx, tx, projection.TenantID)
	if err != nil {
		return err
	}
	mappingVersion := tableVersion
	if priorMappingVersion != nil {
		mappingVersion = *priorMappingVersion
	} else if mappingVersion == 0 {
		mappingVersion = 1
	}
	if projection.AttributionMappingVersion == nil {
		pinned := mappingVersion
		projection.AttributionMappingVersion = &pinned
	}

	if driverCtx.MappingVersion == 0 {
		driverCtx.MappingVersion = mappingVersion
	}
	driverCtx.PlatformMappings = platformMappings
	driverCtx.TenantMappings = tenantMappings

	if err := s.attributions.MarkSuperseded(ctx, tx, projection.TenantID, projection.TransportOrderID, projection.ProjectionRevision); err != nil {
		return err
	}

	var attributionRows []domain.VarianceAttribution
	for _, kind := range []string{domain.VarianceKindCurrent, domain.VarianceKindFinal} {
		attributionRows = append(attributionRows, domain.BuildAvailabilityReasons(projection, kind)...)
		attributionRows = append(attributionRows, domain.BuildVarianceDrivers(projection, kind, driverCtx)...)
	}
	inserted, err := s.attributions.InsertBatch(ctx, tx, attributionRows)
	if err != nil {
		return err
	}
	_ = inserted

	detected := domain.DetectReconciliationFindings(projection)
	activeIDs := make([]uuid.UUID, 0, len(detected))
	for _, finding := range detected {
		activeIDs = append(activeIDs, finding.FindingID)
	}
	if err := s.findings.UpsertBatch(ctx, tx, detected); err != nil {
		return err
	}
	if err := s.findings.ResolveAbsentFindings(ctx, tx, projection.TenantID, projection.TransportOrderID, activeIDs); err != nil {
		return err
	}

	if s.metrics != nil {
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
		for _, finding := range detected {
			s.metrics.ObserveReconciliationFinding(finding.FindingKind, "info")
		}
	}
	return nil
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
	proposed := domain.ProposedAccessorialInput{SourceStatus: domain.ProposedSourceUnknown}
	if settlement.ProposedAccessorialSourceStatus == domain.ProposedSourceKnown {
		proposed.SourceStatus = domain.ProposedSourceKnown
		proposed.TotalExVAT = settlement.ProposedAccessorialTotalExVAT
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
	priorForecast := projection.ForecastExposure
	if err := domain.RecomputeDerivedProjection(projection, proposed, priorForecast); err != nil {
		return err
	}
	if err := s.projections.Upsert(ctx, tx, projection); err != nil {
		return err
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
