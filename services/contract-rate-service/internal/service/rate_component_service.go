package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/repository"
)

type RateComponentService struct {
	components *repository.RateComponentRepository
	rateLines  *repository.RateLineRepository
	rateCards  *repository.RateCardRepository
	contracts  *repository.ContractRepository
}

func NewRateComponentService(
	components *repository.RateComponentRepository,
	rateLines *repository.RateLineRepository,
	rateCards *repository.RateCardRepository,
	contracts *repository.ContractRepository,
) *RateComponentService {
	return &RateComponentService{
		components: components,
		rateLines:  rateLines,
		rateCards:  rateCards,
		contracts:  contracts,
	}
}

func (s *RateComponentService) Create(ctx context.Context, in domain.CreateRateComponentInput, correlationID *string) (*domain.RateComponent, error) {
	if err := in.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	if err := s.authorizeLineWrite(ctx, in.TenantID, in.RateLineID, in.Actor); err != nil {
		return nil, err
	}
	return s.components.Create(ctx, in, correlationID)
}

func (s *RateComponentService) ListByLine(ctx context.Context, tenantID, lineID uuid.UUID, actor domain.ActorInput) ([]domain.RateComponent, error) {
	if err := s.authorizeLineRead(ctx, tenantID, lineID, actor); err != nil {
		return nil, err
	}
	return s.components.ListByLine(ctx, tenantID, lineID)
}

func (s *RateComponentService) Update(ctx context.Context, tenantID, componentID uuid.UUID, patch domain.UpdateRateComponentInput, correlationID *string) (*domain.RateComponent, error) {
	if err := patch.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	component, err := s.components.GetByIDAndTenant(ctx, tenantID, componentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeLineWrite(ctx, tenantID, component.RateLineID, patch.Actor); err != nil {
		return nil, err
	}
	return s.components.Update(ctx, tenantID, componentID, patch, correlationID)
}

func (s *RateComponentService) Delete(ctx context.Context, tenantID, componentID uuid.UUID, actor domain.ActorInput, correlationID *string) error {
	if err := actor.RequireBuyerMutation(); err != nil {
		return err
	}
	component, err := s.components.GetByIDAndTenant(ctx, tenantID, componentID)
	if err != nil {
		return err
	}
	if err := s.authorizeLineWrite(ctx, tenantID, component.RateLineID, actor); err != nil {
		return err
	}
	return s.components.Delete(ctx, tenantID, componentID, actor, correlationID)
}

func (s *RateComponentService) authorizeLineRead(ctx context.Context, tenantID, lineID uuid.UUID, actor domain.ActorInput) error {
	line, err := s.rateLines.GetByIDAndTenant(ctx, tenantID, lineID)
	if err != nil {
		return err
	}
	version, err := s.rateCards.GetVersionByIDAndTenant(ctx, tenantID, line.RateCardVersionID)
	if err != nil {
		return err
	}
	card, err := s.rateCards.GetByIDAndTenant(ctx, tenantID, version.RateCardID)
	if err != nil {
		return err
	}
	contract, err := s.contracts.GetByIDAndTenant(ctx, tenantID, card.ContractID)
	if err != nil {
		return err
	}
	return actor.CanReadContract(contract.BuyerCompanyID, contract.CarrierCompanyID)
}

func (s *RateComponentService) authorizeLineWrite(ctx context.Context, tenantID, lineID uuid.UUID, actor domain.ActorInput) error {
	if err := actor.RequireBuyerMutation(); err != nil {
		return err
	}
	return s.authorizeLineRead(ctx, tenantID, lineID, actor)
}
