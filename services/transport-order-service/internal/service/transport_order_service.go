package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type LocationStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	Create(ctx context.Context, in domain.CreateLocationInput) (*domain.Location, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Location, error)
	List(ctx context.Context, filter domain.ListLocationsFilter) ([]domain.Location, int, error)
}

type CargoStore interface {
	Create(ctx context.Context, in domain.CreateCargoInput) (*domain.Cargo, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Cargo, error)
	ExistsInTenant(ctx context.Context, id, tenantID uuid.UUID) (bool, error)
}

type TransportOrderStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	Create(ctx context.Context, in domain.CreateTransportOrderInput) (*domain.TransportOrder, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error)
	List(ctx context.Context, filter domain.ListTransportOrdersFilter) ([]domain.TransportOrder, int, error)
	Update(ctx context.Context, id uuid.UUID, in domain.UpdateTransportOrderInput) (*domain.TransportOrder, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, expectedStatus, newStatus string) (*domain.TransportOrder, error)
}

type LocationReferenceStore interface {
	ExistsInTenant(ctx context.Context, id, tenantID uuid.UUID) (bool, error)
}

type TransportOrderService struct {
	locations      LocationStore
	cargoes        CargoStore
	orders         TransportOrderStore
	locationLookup LocationReferenceStore
}

func NewTransportOrderService(
	locations LocationStore,
	cargoes CargoStore,
	orders TransportOrderStore,
	locationLookup LocationReferenceStore,
) *TransportOrderService {
	return &TransportOrderService{
		locations:      locations,
		cargoes:        cargoes,
		orders:         orders,
		locationLookup: locationLookup,
	}
}

func (s *TransportOrderService) CreateLocation(ctx context.Context, in domain.CreateLocationInput) (*domain.Location, error) {
	in.CountryCode = domain.NormalizeCountryCode(in.CountryCode)
	in.Timezone = domain.NormalizeTimezone(in.Timezone)
	if err := domain.ValidateCreateLocationInput(in); err != nil {
		return nil, err
	}
	if in.CompanyID != nil {
		exists, err := s.locations.CompanyExists(ctx, *in.CompanyID, in.TenantID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, apperrors.NotFound("company not found")
		}
	}
	return s.locations.Create(ctx, in)
}

func (s *TransportOrderService) GetLocation(ctx context.Context, id uuid.UUID) (*domain.Location, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.locations.GetByID(ctx, id)
}

func (s *TransportOrderService) ListLocations(ctx context.Context, filter domain.ListLocationsFilter) ([]domain.Location, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListLocationsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.locations.List(ctx, filter)
}

func (s *TransportOrderService) CreateCargo(ctx context.Context, in domain.CreateCargoInput) (*domain.Cargo, error) {
	if err := domain.ValidateCreateCargoInput(in); err != nil {
		return nil, err
	}
	return s.cargoes.Create(ctx, in)
}

func (s *TransportOrderService) GetCargo(ctx context.Context, id uuid.UUID) (*domain.Cargo, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.cargoes.GetByID(ctx, id)
}

func (s *TransportOrderService) CreateTransportOrder(ctx context.Context, in domain.CreateTransportOrderInput) (*domain.TransportOrder, error) {
	in.TransportMode = domain.NormalizeTransportMode(in.TransportMode)
	if err := domain.ValidateCreateTransportOrderInput(in); err != nil {
		return nil, err
	}
	if err := s.validateOrderReferences(ctx, in); err != nil {
		return nil, err
	}
	return s.orders.Create(ctx, in)
}

func (s *TransportOrderService) GetTransportOrder(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.orders.GetByID(ctx, id)
}

func (s *TransportOrderService) ListTransportOrders(ctx context.Context, filter domain.ListTransportOrdersFilter) ([]domain.TransportOrder, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListTransportOrdersFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.orders.List(ctx, filter)
}

func (s *TransportOrderService) UpdateTransportOrder(ctx context.Context, id uuid.UUID, in domain.UpdateTransportOrderInput) (*domain.TransportOrder, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}

	current, err := s.orders.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateUpdateTransportOrderStatus(current.Status); err != nil {
		return nil, err
	}
	if err := domain.ValidateUpdateTransportOrderInput(current, in); err != nil {
		return nil, err
	}
	return s.orders.Update(ctx, id, in)
}

func (s *TransportOrderService) SubmitTransportOrder(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}

	current, err := s.orders.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSubmitTransportOrder(current.Status); err != nil {
		return nil, err
	}
	return s.orders.UpdateStatus(ctx, id, domain.TransportOrderStatusDraft, domain.TransportOrderStatusReadyForSourcing)
}

func (s *TransportOrderService) CancelTransportOrder(ctx context.Context, id uuid.UUID) (*domain.TransportOrder, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}

	current, err := s.orders.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCancelTransportOrder(current.Status); err != nil {
		return nil, err
	}
	return s.orders.UpdateStatus(ctx, id, current.Status, domain.TransportOrderStatusCancelled)
}

func (s *TransportOrderService) validateOrderReferences(ctx context.Context, in domain.CreateTransportOrderInput) error {
	for _, ref := range []struct {
		id    uuid.UUID
		field string
	}{
		{in.ShipperCompanyID, "shipper_company_id"},
		{in.ConsigneeCompanyID, "consignee_company_id"},
	} {
		exists, err := s.orders.CompanyExists(ctx, ref.id, in.TenantID)
		if err != nil {
			return err
		}
		if !exists {
			return apperrors.NotFound(ref.field + " not found")
		}
	}

	for _, ref := range []struct {
		id    uuid.UUID
		field string
	}{
		{in.OriginLocationID, "origin_location_id"},
		{in.DestinationLocationID, "destination_location_id"},
	} {
		exists, err := s.locationLookup.ExistsInTenant(ctx, ref.id, in.TenantID)
		if err != nil {
			return err
		}
		if !exists {
			return apperrors.NotFound(ref.field + " not found")
		}
	}

	cargoExists, err := s.cargoes.ExistsInTenant(ctx, in.CargoID, in.TenantID)
	if err != nil {
		return err
	}
	if !cargoExists {
		return apperrors.NotFound("cargo_id not found")
	}
	return nil
}
