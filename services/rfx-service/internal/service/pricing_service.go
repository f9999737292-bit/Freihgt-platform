package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type PricingService struct {
	repo *repository.PricingRepository
}

func NewPricingService(repo *repository.PricingRepository) *PricingService {
	return &PricingService{repo: repo}
}

func (s *PricingService) GetAwardLinkContext(ctx context.Context, tenantID, linkID uuid.UUID) (domain.NormalizedPricingContext, error) {
	return s.repo.GetAwardLinkContext(ctx, tenantID, linkID)
}

func (s *PricingService) GetAwardScopeContext(ctx context.Context, tenantID, eventID uuid.UUID, lotID *uuid.UUID) (domain.NormalizedPricingContext, error) {
	return s.repo.GetAwardScopeContext(ctx, tenantID, eventID, lotID)
}

func (s *PricingService) GetAcceptedBidContext(ctx context.Context, tenantID, bidID uuid.UUID) (domain.NormalizedPricingContext, error) {
	return s.repo.GetAcceptedBidContext(ctx, tenantID, bidID)
}
