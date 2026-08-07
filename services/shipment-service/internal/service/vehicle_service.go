package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type VehicleStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	Create(ctx context.Context, tenantID uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error)
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Vehicle, error)
	List(ctx context.Context, tenantID uuid.UUID, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error)
}

type VehicleService struct {
	vehicles VehicleStore
}

func NewVehicleService(vehicles VehicleStore) *VehicleService {
	return &VehicleService{vehicles: vehicles}
}

func (s *VehicleService) Create(ctx context.Context, tenantID uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error) {
	if tenantID == uuid.Nil {
		return nil, apperrors.Unauthorized("tenant context is required")
	}
	in.VehicleType = domain.NormalizeVehicleType(in.VehicleType)
	in.RegistrationCountry = domain.NormalizeCountryCode(in.RegistrationCountry)
	if err := domain.ValidateCreateVehicleInput(in); err != nil {
		return nil, err
	}
	exists, err := s.vehicles.CompanyExists(ctx, in.CarrierCompanyID, tenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("carrier_company_id not found")
	}
	return s.vehicles.Create(ctx, tenantID, in)
}

func (s *VehicleService) GetByIDAndTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if tenantID == uuid.Nil {
		return nil, apperrors.Unauthorized("tenant context is required")
	}
	return s.vehicles.GetByIDAndTenant(ctx, id, tenantID)
}

func (s *VehicleService) List(ctx context.Context, tenantID uuid.UUID, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, apperrors.Unauthorized("tenant context is required")
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListVehiclesFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.vehicles.List(ctx, tenantID, filter)
}
