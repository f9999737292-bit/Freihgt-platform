package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type FreightRequestStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (string, error)
	CreateFromTransportOrder(ctx context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error)
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error)
	List(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error)
	UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, expectedStatus, newStatus string) (*domain.FreightRequest, error)
}

type FreightRequestService struct {
	repo   FreightRequestStore
	actors ActorResolver
}

func NewFreightRequestService(repo FreightRequestStore) *FreightRequestService {
	return &FreightRequestService{repo: repo}
}

func NewFreightRequestServiceWithAuth(repo FreightRequestStore, actors ActorResolver) *FreightRequestService {
	return &FreightRequestService{repo: repo, actors: actors}
}

func (s *FreightRequestService) applyBuyerListScope(ctx context.Context, actor domain.ActorContext, filter *domain.ListFreightRequestsFilter) error {
	if s.actors == nil {
		return apperrors.Forbidden("buyer company membership is required")
	}
	kind, _, err := s.actors.ResolveActorKind(ctx, actor)
	if err != nil {
		return err
	}
	if kind != domain.ActorKindBuyer {
		return apperrors.Forbidden("buyer authorization required")
	}
	resolver, ok := s.actors.(CompanyMembershipResolver)
	if !ok {
		return apperrors.Forbidden("buyer company membership is required")
	}
	buyerCompanyIDs, err := resolver.ListBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return err
	}
	if len(buyerCompanyIDs) == 0 {
		return apperrors.Forbidden("buyer company membership is required")
	}
	if filter.ShipperCompanyID != nil {
		if !domain.ContainsCompanyID(buyerCompanyIDs, *filter.ShipperCompanyID) {
			filter.ShipperCompanyIDs = []uuid.UUID{uuid.Nil}
			return nil
		}
		filter.ShipperCompanyIDs = []uuid.UUID{*filter.ShipperCompanyID}
		return nil
	}
	filter.ShipperCompanyIDs = buyerCompanyIDs
	return nil
}

func (s *FreightRequestService) CreateFromTransportOrder(ctx context.Context, actor domain.ActorContext, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	in.TenantID = actor.TenantID
	shipperCompanyID, err := requireBuyerCompanyAccess(ctx, s.actors, actor, in.ShipperCompanyID)
	if err != nil {
		return nil, err
	}
	in.ShipperCompanyID = shipperCompanyID
	if err := domain.ValidateCreateFreightRequestInput(in); err != nil {
		return nil, err
	}
	status, err := s.repo.GetTransportOrder(ctx, in.TransportOrderID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateTransportOrderForFreightRequest(status); err != nil {
		return nil, err
	}
	exists, err := s.repo.CompanyExists(ctx, in.ShipperCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("shipper_company_id not found")
	}
	return s.repo.CreateFromTransportOrder(ctx, in)
}

func (s *FreightRequestService) GetByID(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.FreightRequest, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	fr, err := s.repo.GetByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if s.actors != nil {
		if err := s.authorizeFreightRequestRead(ctx, actor, fr); err != nil {
			return nil, err
		}
	}
	return fr, nil
}

func (s *FreightRequestService) List(ctx context.Context, actor domain.ActorContext, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	if err := actor.Validate(); err != nil {
		return nil, 0, err
	}
	filter.TenantID = actor.TenantID
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	empty, err := s.applyActorListScope(ctx, actor, &filter)
	if err != nil {
		return nil, 0, err
	}
	if empty {
		return []domain.FreightRequest{}, 0, nil
	}
	if err := domain.ValidateListFreightRequestsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, filter)
}

func (s *FreightRequestService) Publish(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.FreightRequest, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	fr, err := s.repo.GetByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if _, err := requireBuyerCompanyAccess(ctx, s.actors, actor, fr.ShipperCompanyID); err != nil {
		return nil, err
	}
	if err := domain.ValidatePublishFreightRequest(fr.Status); err != nil {
		return nil, err
	}
	return s.repo.UpdateStatus(ctx, id, actor.TenantID, domain.FreightRequestStatusDraft, domain.FreightRequestStatusPublished)
}
