package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type DriverStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	Create(ctx context.Context, in domain.CreateDriverInput) (*domain.Driver, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Driver, error)
	List(ctx context.Context, filter domain.ListDriversFilter) ([]domain.Driver, int, error)
}

type DriverService struct {
	drivers DriverStore
}

func NewDriverService(drivers DriverStore) *DriverService {
	return &DriverService{drivers: drivers}
}

func (s *DriverService) Create(ctx context.Context, in domain.CreateDriverInput) (*domain.Driver, error) {
	in.LicenseCountry = domain.NormalizeCountryCode(in.LicenseCountry)
	in.PreferredLocale = domain.NormalizeTimezone(in.PreferredLocale)
	if err := domain.ValidateCreateDriverInput(in); err != nil {
		return nil, err
	}
	exists, err := s.drivers.CompanyExists(ctx, in.CarrierCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("carrier_company_id not found")
	}
	return s.drivers.Create(ctx, in)
}

func (s *DriverService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Driver, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.drivers.GetByID(ctx, id)
}

func (s *DriverService) List(ctx context.Context, filter domain.ListDriversFilter) ([]domain.Driver, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListDriversFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.drivers.List(ctx, filter)
}
