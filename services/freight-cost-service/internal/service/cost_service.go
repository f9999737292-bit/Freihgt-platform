package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
)

type CostService struct {
	transport   provider.TransportOrderPricingProvider
	projections *repository.CostSummaryProjectionRepository
}

func NewCostService(
	transport provider.TransportOrderPricingProvider,
	projections *repository.CostSummaryProjectionRepository,
) *CostService {
	return &CostService{transport: transport, projections: projections}
}

func (s *CostService) GetCostSummaryByTransportOrder(ctx context.Context, actor security.TrustedActor, transportOrderID uuid.UUID) (*domain.CostSummary, error) {
	if s.projections != nil {
		projection, err := s.projections.GetByTransportOrder(ctx, actor.TenantID, transportOrderID)
		if err == nil {
			summary, err := domain.ProjectionToCostSummary(projection)
			if err != nil {
				return nil, err
			}
			if err := authorizeSummary(actor, summary); err != nil {
				return nil, err
			}
			scope := domain.ViewScopeForActorKind(actor.ActorKind)
			return domain.ApplyViewScope(scope, summary), nil
		}
		if !isNotFoundErr(err) {
			return nil, err
		}
	}

	return s.buildPlannedOnlyFallback(ctx, actor, transportOrderID)
}

func (s *CostService) buildPlannedOnlyFallback(ctx context.Context, actor security.TrustedActor, transportOrderID uuid.UUID) (*domain.CostSummary, error) {
	snapshot, err := s.transport.GetRateSnapshot(ctx, actor.TenantID, transportOrderID)
	if err != nil {
		return nil, err
	}

	facts := security.CanonicalCompanyFacts{
		BuyerCompanyID:   snapshot.BuyerCompanyID,
		CarrierCompanyID: snapshot.CarrierCompanyID,
	}
	if err := security.AuthorizeCompanyAccess(actor, facts); err != nil {
		return nil, err
	}

	planned, err := domain.NewMoney(snapshot.TotalAmount, snapshot.CurrencyCode)
	if err != nil {
		return nil, err
	}
	pricingModelVersion := domain.PricingModelVersionSnapshot
	plannedSource := &domain.CanonicalSourceRef{
		SourceService:       domain.SourceServiceTransportOrder,
		SourceType:          domain.SourceTypeTORateSnapshot,
		SourceID:            snapshot.SnapshotID,
		SourceVersion:       nil,
		PricingModelVersion: &pricingModelVersion,
	}

	unlinked := domain.BillingReconciliationUnlinked
	summary := &domain.CostSummary{
		TenantID:                    actor.TenantID,
		TransportOrderID:            transportOrderID,
		BuyerCompanyID:              snapshot.BuyerCompanyID,
		CarrierCompanyID:            snapshot.CarrierCompanyID,
		CurrencyCode:                snapshot.CurrencyCode,
		PlannedAmount:               planned,
		PlannedSourceRef:            plannedSource,
		DataStage:                   domain.DataStagePlannedOnly,
		FinancialFinality:           domain.FinancialFinalityNotEvaluated,
		BillingReconciliationStatus: &unlinked,
		SourcesAvailable:            []string{domain.SourceTypeTORateSnapshot},
	}

	scope := domain.ViewScopeForActorKind(actor.ActorKind)
	return domain.ApplyViewScope(scope, summary), nil
}

func authorizeSummary(actor security.TrustedActor, summary *domain.CostSummary) error {
	if summary == nil {
		return apperrors.NotFound("cost summary not found")
	}
	if summary.TenantID != actor.TenantID {
		return apperrors.NotFound("transport order not found")
	}
	return security.AuthorizeCompanyAccess(actor, security.CanonicalCompanyFacts{
		BuyerCompanyID:   summary.BuyerCompanyID,
		CarrierCompanyID: summary.CarrierCompanyID,
	})
}
