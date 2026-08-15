package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type BidRevisionStore interface {
	SubmitRevision(ctx context.Context, in domain.SubmitBidRevisionInput) (*domain.BidRevision, error)
	GetActiveRevision(ctx context.Context, bidID, tenantID uuid.UUID) (*domain.BidRevision, error)
	ListRevisions(ctx context.Context, bidID, tenantID uuid.UUID) ([]domain.BidRevision, error)
}

type BidRevisionService struct {
	bids      BidStore
	revisions BidRevisionStore
}

func NewBidRevisionService(bids BidStore, revisions BidRevisionStore) *BidRevisionService {
	return &BidRevisionService{bids: bids, revisions: revisions}
}

func (s *BidRevisionService) SubmitRevision(ctx context.Context, in domain.SubmitBidRevisionInput) (*domain.BidRevision, error) {
	if err := domain.ValidateSubmitBidRevisionInput(in); err != nil {
		return nil, err
	}
	return s.revisions.SubmitRevision(ctx, in)
}

func (s *BidRevisionService) GetActiveRevision(ctx context.Context, bidID, tenantID uuid.UUID, carrierScope *uuid.UUID) (*domain.BidRevision, error) {
	bid, err := s.bids.GetByID(ctx, bidID, tenantID)
	if err != nil {
		return nil, err
	}
	if carrierScope != nil && bid.CarrierCompanyID != *carrierScope {
		return nil, apperrors.Forbidden("cannot access another carrier bid")
	}
	return s.revisions.GetActiveRevision(ctx, bidID, tenantID)
}

func (s *BidRevisionService) ListRevisions(ctx context.Context, bidID, tenantID uuid.UUID, carrierScope *uuid.UUID) ([]domain.BidRevision, error) {
	bid, err := s.bids.GetByID(ctx, bidID, tenantID)
	if err != nil {
		return nil, err
	}
	if carrierScope != nil && bid.CarrierCompanyID != *carrierScope {
		return nil, apperrors.Forbidden("cannot access another carrier bid history")
	}
	return s.revisions.ListRevisions(ctx, bidID, tenantID)
}
