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
	repo FreightRequestStore
}

func NewFreightRequestService(repo FreightRequestStore) *FreightRequestService {
	return &FreightRequestService{repo: repo}
}

func (s *FreightRequestService) CreateFromTransportOrder(ctx context.Context, in domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
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

func (s *FreightRequestService) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	if id == uuid.Nil || tenantID == uuid.Nil {
		return nil, apperrors.Validation("id and tenant_id are required", map[string]any{})
	}
	return s.repo.GetByID(ctx, id, tenantID)
}

func (s *FreightRequestService) List(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListFreightRequestsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, filter)
}

func (s *FreightRequestService) Publish(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	fr, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePublishFreightRequest(fr.Status); err != nil {
		return nil, err
	}
	return s.repo.UpdateStatus(ctx, id, tenantID, domain.FreightRequestStatusDraft, domain.FreightRequestStatusPublished)
}
