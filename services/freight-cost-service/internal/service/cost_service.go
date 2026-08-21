package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/security"
)

type CostService struct {
	transport provider.TransportOrderPricingProvider
}

func NewCostService(transport provider.TransportOrderPricingProvider) *CostService {
	return &CostService{transport: transport}
}

func (s *CostService) GetCostSummaryByTransportOrder(ctx context.Context, actor security.TrustedActor, transportOrderID uuid.UUID) (*domain.CostSummary, error) {
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
		AccruedAmount:               nil,
		ForecastExposure:            nil,
		CurrentActualAmount:         nil,
		FinalActualAmount:           nil,
		BillingRegisterAmount:       nil,
		PaidAmount:                  nil,
		CurrentVarianceAmount:       nil,
		FinalVarianceAmount:         nil,
		BillingReconciliationStatus: nil,
		SourcesAvailable:            []string{domain.SourceTypeTORateSnapshot},
	}

	scope := domain.ViewScopeForActorKind(actor.ActorKind)
	return domain.ApplyViewScope(scope, summary), nil
}
