package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type VehicleStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	Create(ctx context.Context, in domain.CreateVehicleInput) (*domain.Vehicle, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error)
	List(ctx context.Context, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error)
}

type VehicleService struct {
	vehicles VehicleStore
}

func NewVehicleService(vehicles VehicleStore) *VehicleService {
	return &VehicleService{vehicles: vehicles}
}

func (s *VehicleService) Create(ctx context.Context, in domain.CreateVehicleInput) (*domain.Vehicle, error) {
	in.VehicleType = domain.NormalizeVehicleType(in.VehicleType)
	in.RegistrationCountry = domain.NormalizeCountryCode(in.RegistrationCountry)
	if err := domain.ValidateCreateVehicleInput(in); err != nil {
		return nil, err
	}
	exists, err := s.vehicles.CompanyExists(ctx, in.CarrierCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("carrier_company_id not found")
	}
	return s.vehicles.Create(ctx, in)
}

func (s *VehicleService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.vehicles.GetByID(ctx, id)
}

func (s *VehicleService) List(ctx context.Context, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListVehiclesFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.vehicles.List(ctx, filter)
}
