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
	actors   ActorResolver
	audit    AuditRecorder
}

func NewBidService(bids BidStore, requests FreightRequestStore, actors ActorResolver, audit AuditRecorder) *BidService {
	return &BidService{bids: bids, requests: requests, actors: actors, audit: audit}
}

func (s *BidService) resolveActor(ctx context.Context, actor domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	if err := actor.Validate(); err != nil {
		return domain.ActorKindUnknown, nil, err
	}
	if s.actors == nil {
		return domain.ActorKindBuyer, nil, nil
	}
	return s.actors.ResolveActorKind(ctx, actor)
}

func carrierCanViewBid(carrierIDs []uuid.UUID, bidCarrierCompanyID uuid.UUID) bool {
	for _, id := range carrierIDs {
		if domain.CanViewBid(domain.ActorKindCarrier, id, bidCarrierCompanyID) {
			return true
		}
	}
	return false
}

func (s *BidService) CreateBid(ctx context.Context, actor domain.ActorContext, freightRequestID uuid.UUID, in domain.CreateBidInput) (*domain.Bid, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	in.FreightRequestID = freightRequestID

	_, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	carrierCompanyID, err := domain.ResolveCarrierCompanyID(in.CarrierCompanyID, carrierIDs)
	if err != nil {
		return nil, err
	}
	in.CarrierCompanyID = carrierCompanyID

	if err := domain.ValidateCreateBidInput(in); err != nil {
		return nil, err
	}
	fr, err := s.requests.GetByID(ctx, freightRequestID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateFreightRequestForBid(fr.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateResponseDeadlineOpen(fr.ResponseDeadline, nowUTC()); err != nil {
		return nil, err
	}
	bid, err := s.bids.CreateBid(ctx, in)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "bid", bid.ID, "create", map[string]any{
		"freight_request_id": freightRequestID.String(),
		"carrier_company_id": carrierCompanyID.String(),
	})
	return bid, nil
}

func (s *BidService) ListBids(ctx context.Context, actor domain.ActorContext, freightRequestID uuid.UUID, status *string) ([]domain.Bid, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.requests.GetByID(ctx, freightRequestID, actor.TenantID); err != nil {
		return nil, err
	}
	bids, err := s.bids.ListByFreightRequest(ctx, freightRequestID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !domain.CanViewAllBids(kind) {
		filtered := make([]domain.Bid, 0)
		for _, b := range bids {
			if carrierCanViewBid(carrierIDs, b.CarrierCompanyID) {
				filtered = append(filtered, b)
			}
		}
		bids = filtered
	}
	if status == nil {
		return bids, nil
	}
	filtered := make([]domain.Bid, 0, len(bids))
	for _, b := range bids {
		if b.Status == *status {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

func (s *BidService) SubmitBid(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.Bid, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	bid, err := s.bids.GetByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !domain.CanViewAllBids(kind) && !carrierCanViewBid(carrierIDs, bid.CarrierCompanyID) {
		return nil, apperrors.NotFound("bid not found")
	}
	if err := domain.ValidateSubmitBid(bid.Status); err != nil {
		return nil, err
	}
	fr, err := s.requests.GetByID(ctx, bid.FreightRequestID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSubmissionBeforeDeadline(fr.ResponseDeadline, nowUTC()); err != nil {
		return nil, err
	}
	submitted, err := s.bids.SubmitBid(ctx, id, actor.TenantID, auditUser(actor))
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "bid", id, "submit", nil)
	return submitted, nil
}

func (s *BidService) AcceptBid(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.Bid, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	bid, err := s.bids.GetByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAcceptBid(bid.Status); err != nil {
		return nil, err
	}
	fr, err := s.requests.GetByID(ctx, bid.FreightRequestID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if fr.Status == domain.FreightRequestStatusAwarded {
		return nil, apperrors.Conflict("freight request is already awarded", map[string]any{"field": "status"})
	}
	accepted, err := s.bids.AcceptBid(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	recordAudit(ctx, s.audit, actor, "bid", id, "accept", map[string]any{
		"freight_request_id": bid.FreightRequestID.String(),
	})
	return accepted, nil
}

func (s *BidService) GetByID(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.Bid, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	bid, err := s.bids.GetByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !domain.CanViewAllBids(kind) && !carrierCanViewBid(carrierIDs, bid.CarrierCompanyID) {
		return nil, apperrors.NotFound("bid not found")
	}
	return bid, nil
}
