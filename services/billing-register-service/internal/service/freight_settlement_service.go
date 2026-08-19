package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/repository"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type FreightSettlementStore interface {
	LoadShipmentContext(ctx context.Context, tenantID, shipmentID uuid.UUID) (*repository.ShipmentSettlementContext, error)
	CreateSettlement(ctx context.Context, in domain.CreateFreightSettlementInput, ctxData *repository.ShipmentSettlementContext) (*domain.FreightSettlement, error)
	GetDetail(ctx context.Context, id, tenantID uuid.UUID) (*repository.SettlementDetail, error)
	List(ctx context.Context, filter domain.ListFreightSettlementsFilter) ([]domain.FreightSettlement, int, error)
	ProposeAccessorial(ctx context.Context, settlementID uuid.UUID, in domain.ProposeAccessorialInput) (*domain.SettlementAccessorial, error)
	ReviewAccessorial(ctx context.Context, settlementID, accessorialID uuid.UUID, in domain.SettlementActorInput, approve bool) (*repository.SettlementDetail, error)
	RaiseDispute(ctx context.Context, settlementID uuid.UUID, in domain.RaiseDisputeInput) (*domain.SettlementDispute, error)
	ResolveDispute(ctx context.Context, settlementID, disputeID uuid.UUID, in domain.ResolveDisputeInput) (*repository.SettlementDetail, error)
	TransitionStatus(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput, toStatus string) (*domain.FreightSettlement, error)
	IncludeInRegister(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput, registerNumber string) (*domain.FreightSettlement, error)
}

type FreightSettlementService struct {
	store FreightSettlementStore
}

func NewFreightSettlementService(store FreightSettlementStore) *FreightSettlementService {
	return &FreightSettlementService{store: store}
}

func (s *FreightSettlementService) Create(ctx context.Context, in domain.CreateFreightSettlementInput) (*domain.FreightSettlement, error) {
	ctxData, err := s.store.LoadShipmentContext(ctx, in.TenantID, in.ShipmentID)
	if err != nil {
		return nil, err
	}
	return s.store.CreateSettlement(ctx, in, ctxData)
}

func (s *FreightSettlementService) GetDetail(ctx context.Context, id, tenantID uuid.UUID, actorCompanyID uuid.UUID, actorKind string) (*repository.SettlementDetail, error) {
	detail, err := s.store.GetDetail(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSettlementAccess(&detail.Settlement, actorCompanyID, actorKind); err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *FreightSettlementService) List(ctx context.Context, filter domain.ListFreightSettlementsFilter, actor domain.SettlementActorInput) ([]domain.FreightSettlement, int, error) {
	if err := domain.ValidateSettlementActor(actor); err != nil {
		return nil, 0, err
	}
	switch actor.ActorKind {
	case domain.SettlementActorBuyer:
		if filter.BuyerCompanyID != nil && *filter.BuyerCompanyID != actor.ActorCompanyID {
			return nil, 0, apperrors.Forbidden("buyer cannot list another buyer's settlements")
		}
		filter.BuyerCompanyID = &actor.ActorCompanyID
	case domain.SettlementActorCarrier:
		if filter.CarrierCompanyID != nil && *filter.CarrierCompanyID != actor.ActorCompanyID {
			return nil, 0, apperrors.Forbidden("carrier cannot list another carrier's settlements")
		}
		filter.CarrierCompanyID = &actor.ActorCompanyID
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	return s.store.List(ctx, filter)
}

func (s *FreightSettlementService) ProposeAccessorial(ctx context.Context, settlementID uuid.UUID, in domain.ProposeAccessorialInput) (*domain.SettlementAccessorial, error) {
	if err := domain.ValidateProposeAccessorialInput(in); err != nil {
		return nil, err
	}
	return s.store.ProposeAccessorial(ctx, settlementID, in)
}

func (s *FreightSettlementService) ApproveAccessorial(ctx context.Context, settlementID, accessorialID uuid.UUID, in domain.SettlementActorInput) (*repository.SettlementDetail, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.ReviewAccessorial(ctx, settlementID, accessorialID, in, true)
}

func (s *FreightSettlementService) RejectAccessorial(ctx context.Context, settlementID, accessorialID uuid.UUID, in domain.SettlementActorInput) (*repository.SettlementDetail, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.ReviewAccessorial(ctx, settlementID, accessorialID, in, false)
}

func (s *FreightSettlementService) RaiseDispute(ctx context.Context, settlementID uuid.UUID, in domain.RaiseDisputeInput) (*domain.SettlementDispute, error) {
	if err := domain.ValidateRaiseDisputeInput(in); err != nil {
		return nil, err
	}
	return s.store.RaiseDispute(ctx, settlementID, in)
}

func (s *FreightSettlementService) ResolveDispute(ctx context.Context, settlementID, disputeID uuid.UUID, in domain.ResolveDisputeInput) (*repository.SettlementDetail, error) {
	if err := domain.ValidateSettlementActor(in.SettlementActorInput); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ResolutionNote) == "" {
		return nil, apperrors.Validation("resolution_note is required", map[string]any{"field": "resolution_note"})
	}
	return s.store.ResolveDispute(ctx, settlementID, disputeID, in)
}

func (s *FreightSettlementService) SubmitForReview(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput) (*domain.FreightSettlement, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.TransitionStatus(ctx, settlementID, in, domain.SettlementStatusUnderReview)
}

func (s *FreightSettlementService) Approve(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput) (*domain.FreightSettlement, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.TransitionStatus(ctx, settlementID, in, domain.SettlementStatusApproved)
}

func (s *FreightSettlementService) MarkDocumentsReady(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput) (*domain.FreightSettlement, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.TransitionStatus(ctx, settlementID, in, domain.SettlementStatusDocumentsReady)
}

func (s *FreightSettlementService) MarkReadyForPayment(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput) (*domain.FreightSettlement, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.TransitionStatus(ctx, settlementID, in, domain.SettlementStatusReadyForPayment)
}

func (s *FreightSettlementService) IncludeInRegister(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput, registerNumber string) (*domain.FreightSettlement, error) {
	if err := domain.ValidateSettlementActor(in); err != nil {
		return nil, err
	}
	return s.store.IncludeInRegister(ctx, settlementID, in, registerNumber)
}
