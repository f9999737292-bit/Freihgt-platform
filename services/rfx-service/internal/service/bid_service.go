package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type BidStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	CreateBid(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
	ListByFreightRequest(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error)
	SubmitBid(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.Bid, error)
	AcceptBid(ctx context.Context, id, tenantID uuid.UUID, preCommit func(context.Context, pgx.Tx) error) (*domain.Bid, error)
}

type BidService struct {
	bids     BidStore
	requests FreightRequestStore
	actors   ActorResolver
	audit    AuditRecorder
	atomic   *atomicServices
	bidRepo  *repository.BidRepository
	auditRepo *repository.AuditRepository
}

func NewBidService(bids BidStore, requests FreightRequestStore, actors ActorResolver, audit AuditRecorder) *BidService {
	return &BidService{bids: bids, requests: requests, actors: actors, audit: audit}
}

func NewBidServiceWithAtomic(pool *pgxpool.Pool, bidRepo *repository.BidRepository, frRepo *repository.FreightRequestRepository, actors ActorResolver, auditRepo *repository.AuditRepository) *BidService {
	s := NewBidService(bidRepo, frRepo, actors, auditRepo)
	s.bidRepo = bidRepo
	s.auditRepo = auditRepo
	if pool != nil {
		s.atomic = newAtomicServices(pool, nil, auditRepo, bidRepo, frRepo)
	}
	return s
}

func (s *BidService) requireShipperCompanyAccess(ctx context.Context, actor domain.ActorContext, shipperCompanyID uuid.UUID) (uuid.UUID, error) {
	return requireBuyerCompanyAccess(ctx, s.actors, actor, shipperCompanyID)
}

func requireBuyerCompanyAccess(ctx context.Context, actors ActorResolver, actor domain.ActorContext, companyID uuid.UUID) (uuid.UUID, error) {
	if err := actor.Validate(); err != nil {
		return uuid.Nil, err
	}
	if actors == nil {
		return uuid.Nil, apperrors.Forbidden("buyer company membership is required")
	}
	kind, _, err := actors.ResolveActorKind(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	if kind != domain.ActorKindBuyer {
		return uuid.Nil, apperrors.Forbidden("buyer authorization required")
	}
	if actor.UserID == uuid.Nil {
		return uuid.Nil, apperrors.Forbidden("user context is required")
	}
	resolver, ok := actors.(CompanyMembershipResolver)
	if !ok {
		return uuid.Nil, apperrors.Forbidden("buyer company membership is required")
	}
	roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	if !domain.HasBuyerRole(roles) {
		return uuid.Nil, apperrors.Forbidden("buyer role is required")
	}
	buyerCompanyIDs, err := resolver.ListBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	if !domain.ContainsCompanyID(buyerCompanyIDs, companyID) {
		return uuid.Nil, apperrors.NotFound("freight request not found")
	}
	return companyID, nil
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

func (s *BidService) runBid(ctx context.Context, fn func(bids BidStore, requests FreightRequestStore, audit AuditRecorder) error) error {
	if s.atomic != nil {
		return s.atomic.runBid(ctx, fn)
	}
	return fn(s.bids, s.requests, s.audit)
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
	var bid *domain.Bid
	err = s.runBid(ctx, func(bids BidStore, _ FreightRequestStore, audit AuditRecorder) error {
		created, err := bids.CreateBid(ctx, in)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, carrierCompanyID, "bid", created.ID, "create", map[string]any{
			"freight_request_id":   freightRequestID.String(),
			"carrier_company_id": carrierCompanyID.String(),
		}); err != nil {
			return err
		}
		bid = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return bid, nil
}

func (s *BidService) ListBids(ctx context.Context, actor domain.ActorContext, freightRequestID uuid.UUID, status *string) ([]domain.Bid, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	fr, err := s.requests.GetByID(ctx, freightRequestID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind == domain.ActorKindBuyer {
		if _, err := s.requireShipperCompanyAccess(ctx, actor, fr.ShipperCompanyID); err != nil {
			return nil, err
		}
	}
	bids, err := s.bids.ListByFreightRequest(ctx, freightRequestID, actor.TenantID)
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
	carrierCompanyID := bid.CarrierCompanyID
	if kind == domain.ActorKindCarrier {
		var resolveErr error
		carrierCompanyID, resolveErr = domain.ResolveCarrierCompanyID(bid.CarrierCompanyID, carrierIDs)
		if resolveErr != nil {
			return nil, apperrors.NotFound("bid not found")
		}
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
	var submitted *domain.Bid
	err = s.runBid(ctx, func(bids BidStore, _ FreightRequestStore, audit AuditRecorder) error {
		result, err := bids.SubmitBid(ctx, id, actor.TenantID, auditUser(actor))
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, audit, actor, carrierCompanyID, "bid", id, "submit", nil); err != nil {
			return err
		}
		submitted = result
		return nil
	})
	if err != nil {
		return nil, err
	}
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
	shipperCompanyID, err := s.requireShipperCompanyAccess(ctx, actor, fr.ShipperCompanyID)
	if err != nil {
		return nil, err
	}
	if fr.Status == domain.FreightRequestStatusAwarded {
		return nil, apperrors.Conflict("freight request is already awarded", map[string]any{"field": "status"})
	}
	accepted, err := s.bids.AcceptBid(ctx, id, actor.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		if s.auditRepo == nil {
			return recordAudit(ctx, s.audit, actor, shipperCompanyID, "bid", id, "accept", map[string]any{
				"freight_request_id": bid.FreightRequestID.String(),
			})
		}
		return s.auditRepo.WithTx(tx).Record(ctx, repository.AuditRecord{
			TenantID:       actor.TenantID,
			EntityType:     "bid",
			EntityID:       id,
			Action:         "accept",
			ActorUserID:    auditUser(actor),
			ActorCompanyID: verifiedActorCompany(actor, shipperCompanyID),
			Metadata: map[string]any{
				"freight_request_id": bid.FreightRequestID.String(),
			},
		})
	})
	if err != nil {
		return nil, err
	}
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
	if kind == domain.ActorKindBuyer {
		fr, err := s.requests.GetByID(ctx, bid.FreightRequestID, actor.TenantID)
		if err != nil {
			return nil, err
		}
		if _, err := s.requireShipperCompanyAccess(ctx, actor, fr.ShipperCompanyID); err != nil {
			return nil, err
		}
	}
	if !domain.CanViewAllBids(kind) && !carrierCanViewBid(carrierIDs, bid.CarrierCompanyID) {
		return nil, apperrors.NotFound("bid not found")
	}
	return bid, nil
}
