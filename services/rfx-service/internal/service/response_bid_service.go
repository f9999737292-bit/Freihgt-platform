package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ResponseRevisionStore interface {
	SubmitRevision(ctx context.Context, in domain.SubmitResponseRevisionInput) (*domain.RfxResponseRevision, error)
	GetActiveRevision(ctx context.Context, responseID, tenantID uuid.UUID) (*domain.RfxResponseRevision, error)
	ListRevisions(ctx context.Context, responseID, tenantID uuid.UUID) ([]domain.RfxResponseRevision, error)
	ListEventBids(ctx context.Context, eventID, tenantID uuid.UUID, carrierScope *uuid.UUID) ([]domain.RfxResponseRevision, error)
	GetResponseOwner(ctx context.Context, responseID, tenantID uuid.UUID) (uuid.UUID, uuid.UUID, error)
}

type ResponseBidService struct {
	store ResponseRevisionStore
}

func NewResponseBidService(store ResponseRevisionStore) *ResponseBidService {
	return &ResponseBidService{store: store}
}

func (s *ResponseBidService) SubmitRevision(ctx context.Context, in domain.SubmitResponseRevisionInput) (*domain.RfxResponseRevision, error) {
	if err := domain.ValidateSubmitResponseRevisionInput(in); err != nil {
		return nil, err
	}
	return s.store.SubmitRevision(ctx, in)
}

func (s *ResponseBidService) GetActiveRevision(ctx context.Context, responseID, tenantID uuid.UUID, carrierScope *uuid.UUID) (*domain.RfxResponseRevision, error) {
	if carrierScope != nil {
		_, owner, err := s.store.GetResponseOwner(ctx, responseID, tenantID)
		if err != nil {
			return nil, err
		}
		if owner != *carrierScope {
			return nil, apperrors.Forbidden("cannot access another carrier bid")
		}
	}
	return s.store.GetActiveRevision(ctx, responseID, tenantID)
}

func (s *ResponseBidService) ListRevisions(ctx context.Context, responseID, tenantID uuid.UUID, carrierScope *uuid.UUID) ([]domain.RfxResponseRevision, error) {
	if carrierScope != nil {
		_, owner, err := s.store.GetResponseOwner(ctx, responseID, tenantID)
		if err != nil {
			return nil, err
		}
		if owner != *carrierScope {
			return nil, apperrors.Forbidden("cannot access another carrier bid history")
		}
	}
	return s.store.ListRevisions(ctx, responseID, tenantID)
}

func (s *ResponseBidService) ListEventBids(ctx context.Context, eventID, tenantID uuid.UUID, carrierScope *uuid.UUID) ([]domain.RfxResponseRevision, error) {
	return s.store.ListEventBids(ctx, eventID, tenantID, carrierScope)
}

func (s *ResponseBidService) GetResponseForCarrier(ctx context.Context, eventID, tenantID, carrierID uuid.UUID) (*domain.RfxResponseRevision, error) {
	bids, err := s.store.ListEventBids(ctx, eventID, tenantID, &carrierID)
	if err != nil {
		return nil, err
	}
	if len(bids) == 0 {
		return nil, apperrors.NotFound("carrier response not found")
	}
	return &bids[0], nil
}
