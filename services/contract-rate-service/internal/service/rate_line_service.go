package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/repository"
)

type RateLineService struct {
	rateLines *repository.RateLineRepository
	rateCards *repository.RateCardRepository
	contracts *repository.ContractRepository
}

func NewRateLineService(rateLines *repository.RateLineRepository, rateCards *repository.RateCardRepository, contracts *repository.ContractRepository) *RateLineService {
	return &RateLineService{rateLines: rateLines, rateCards: rateCards, contracts: contracts}
}

func (s *RateLineService) Create(ctx context.Context, in domain.CreateRateLineInput, correlationID *string) (*domain.RateLine, error) {
	if err := in.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	if err := s.authorizeVersionWrite(ctx, in.TenantID, in.RateCardVersionID, in.Actor); err != nil {
		return nil, err
	}
	return s.rateLines.Create(ctx, in, correlationID)
}

func (s *RateLineService) ListByVersion(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput) ([]domain.RateLine, error) {
	if err := s.authorizeVersionRead(ctx, tenantID, versionID, actor); err != nil {
		return nil, err
	}
	return s.rateLines.ListByVersion(ctx, tenantID, versionID)
}

func (s *RateLineService) Get(ctx context.Context, tenantID, lineID uuid.UUID, actor domain.ActorInput) (*domain.RateLine, error) {
	line, err := s.rateLines.GetByIDAndTenant(ctx, tenantID, lineID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeVersionRead(ctx, tenantID, line.RateCardVersionID, actor); err != nil {
		return nil, err
	}
	return line, nil
}

func (s *RateLineService) Update(ctx context.Context, tenantID, lineID uuid.UUID, patch domain.UpdateRateLineInput, correlationID *string) (*domain.RateLine, error) {
	if err := patch.Actor.RequireBuyerMutation(); err != nil {
		return nil, err
	}
	line, err := s.rateLines.GetByIDAndTenant(ctx, tenantID, lineID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeVersionWrite(ctx, tenantID, line.RateCardVersionID, patch.Actor); err != nil {
		return nil, err
	}
	return s.rateLines.Update(ctx, tenantID, lineID, patch, correlationID)
}

func (s *RateLineService) Delete(ctx context.Context, tenantID, lineID uuid.UUID, actor domain.ActorInput, correlationID *string) error {
	if err := actor.RequireBuyerMutation(); err != nil {
		return err
	}
	line, err := s.rateLines.GetByIDAndTenant(ctx, tenantID, lineID)
	if err != nil {
		return err
	}
	if err := s.authorizeVersionWrite(ctx, tenantID, line.RateCardVersionID, actor); err != nil {
		return err
	}
	return s.rateLines.Delete(ctx, tenantID, lineID, actor, correlationID)
}

func (s *RateLineService) authorizeVersionRead(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput) error {
	version, err := s.rateCards.GetVersionByIDAndTenant(ctx, tenantID, versionID)
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

func (s *RateLineService) authorizeVersionWrite(ctx context.Context, tenantID, versionID uuid.UUID, actor domain.ActorInput) error {
	if err := actor.RequireBuyerMutation(); err != nil {
		return err
	}
	return s.authorizeVersionRead(ctx, tenantID, versionID, actor)
}
