package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/repository"
)

type RateCardService struct {
	rateCards *repository.RateCardRepository
	contracts *repository.ContractRepository
}

func NewRateCardService(rateCards *repository.RateCardRepository, contracts *repository.ContractRepository) *RateCardService {
	return &RateCardService{rateCards: rateCards, contracts: contracts}
}

func (s *RateCardService) Create(ctx context.Context, in domain.CreateRateCardInput, correlationID *string) (*domain.RateCard, error) {
	if err := in.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.rateCards.Create(ctx, in, correlationID)
}

func (s *RateCardService) Get(ctx context.Context, tenantID, rateCardID uuid.UUID, actor domain.ActorInput) (*domain.RateCard, error) {
	card, err := s.rateCards.GetByIDAndTenant(ctx, tenantID, rateCardID)
	if err != nil {
		return nil, err
	}
	contract, err := s.contracts.GetByIDAndTenant(ctx, tenantID, card.ContractID)
	if err != nil {
		return nil, err
	}
	if err := actor.CanReadContract(contract.BuyerCompanyID, contract.CarrierCompanyID); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *RateCardService) ListByContract(ctx context.Context, tenantID, contractID uuid.UUID, actor domain.ActorInput) ([]domain.RateCard, error) {
	contract, err := s.contracts.GetByIDAndTenant(ctx, tenantID, contractID)
	if err != nil {
		return nil, err
	}
	if err := actor.CanReadContract(contract.BuyerCompanyID, contract.CarrierCompanyID); err != nil {
		return nil, err
	}
	return s.rateCards.ListByContract(ctx, tenantID, contractID)
}

func (s *RateCardService) Update(ctx context.Context, tenantID, rateCardID uuid.UUID, patch domain.UpdateRateCardInput, correlationID *string) (*domain.RateCard, error) {
	if err := patch.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.rateCards.Update(ctx, tenantID, rateCardID, patch, correlationID)
}

func (s *RateCardService) CreateDraftVersion(ctx context.Context, in domain.CreateRateVersionInput, correlationID *string) (*domain.RateCardVersion, error) {
	if err := in.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.rateCards.CreateDraftVersion(ctx, in, correlationID)
}

func (s *RateCardService) GetVersion(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput) (*domain.RateCardVersion, error) {
	version, err := s.rateCards.GetVersionByIDAndTenant(ctx, tenantID, versionID)
	if err != nil {
		return nil, err
	}
	card, err := s.rateCards.GetByIDAndTenant(ctx, tenantID, version.RateCardID)
	if err != nil {
		return nil, err
	}
	contract, err := s.contracts.GetByIDAndTenant(ctx, tenantID, card.ContractID)
	if err != nil {
		return nil, err
	}
	if err := actor.CanReadContract(contract.BuyerCompanyID, contract.CarrierCompanyID); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *RateCardService) ListVersions(ctx context.Context, tenantID, rateCardID uuid.UUID, actor domain.ActorInput) ([]domain.RateCardVersion, error) {
	if _, err := s.Get(ctx, tenantID, rateCardID, actor); err != nil {
		return nil, err
	}
	return s.rateCards.ListVersions(ctx, tenantID, rateCardID)
}

func (s *RateCardService) UpdateDraftVersion(ctx context.Context, tenantID, versionID uuid.UUID, patch domain.UpdateRateVersionInput, correlationID *string) (*domain.RateCardVersion, error) {
	if err := patch.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	return s.rateCards.UpdateDraftVersion(ctx, tenantID, versionID, patch, correlationID)
}

func (s *RateCardService) DiscardDraftVersion(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput, correlationID *string) error {
	if err := actor.RequireBuyerMutation(); err != nil {
		return err
	}
	return s.rateCards.DiscardDraftVersion(ctx, tenantID, versionID, actor, correlationID)
}
