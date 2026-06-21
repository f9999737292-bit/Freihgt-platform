package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type BidStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	CreateBid(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
	ListByFreightRequest(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error)
	SubmitBid(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.Bid, error)
	AcceptBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
}

type BidService struct {
	bids     BidStore
	requests FreightRequestStore
}

func NewBidService(bids BidStore, requests FreightRequestStore) *BidService {
	return &BidService{bids: bids, requests: requests}
}

func (s *BidService) CreateBid(ctx context.Context, freightRequestID uuid.UUID, in domain.CreateBidInput) (*domain.Bid, error) {
	in.FreightRequestID = freightRequestID
	if err := domain.ValidateCreateBidInput(in); err != nil {
		return nil, err
	}
	fr, err := s.requests.GetByID(ctx, freightRequestID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateFreightRequestForBid(fr.Status); err != nil {
		return nil, err
	}
	exists, err := s.bids.CompanyExists(ctx, in.CarrierCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("carrier_company_id not found")
	}
	return s.bids.CreateBid(ctx, in)
}

func (s *BidService) ListBids(ctx context.Context, freightRequestID, tenantID uuid.UUID, status *string) ([]domain.Bid, error) {
	if _, err := s.requests.GetByID(ctx, freightRequestID, tenantID); err != nil {
		return nil, err
	}
	bids, err := s.bids.ListByFreightRequest(ctx, freightRequestID, tenantID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return bids, nil
	}
	filtered := make([]domain.Bid, 0)
	for _, b := range bids {
		if b.Status == *status {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

func (s *BidService) SubmitBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	bid, err := s.bids.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSubmitBid(bid.Status); err != nil {
		return nil, err
	}
	return s.bids.SubmitBid(ctx, id, tenantID, nil)
}

func (s *BidService) AcceptBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	bid, err := s.bids.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAcceptBid(bid.Status); err != nil {
		return nil, err
	}
	return s.bids.AcceptBid(ctx, id, tenantID)
}
