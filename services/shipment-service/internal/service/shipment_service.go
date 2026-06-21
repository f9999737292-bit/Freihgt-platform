package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type ShipmentStore interface {
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrderSnapshot, error)
	GetBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.BidSnapshot, error)
	CreateShipment(ctx context.Context, params repository.CreateShipmentParams) (*domain.Shipment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Shipment, error)
	GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	List(ctx context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error)
	AssignDriver(ctx context.Context, id, tenantID, driverID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error)
	AssignVehicle(ctx context.Context, id, tenantID, vehicleID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error)
	UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int) (*domain.Shipment, error)
	Accept(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.Shipment, error)
	Cancel(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.Shipment, error)
}

type DriverLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Driver, error)
}

type VehicleLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error)
}

type ShipmentService struct {
	shipments ShipmentStore
	drivers   DriverLookup
	vehicles  VehicleLookup
}

func NewShipmentService(shipments ShipmentStore, drivers DriverLookup, vehicles VehicleLookup) *ShipmentService {
	return &ShipmentService{shipments: shipments, drivers: drivers, vehicles: vehicles}
}

func (s *ShipmentService) CreateFromTransportOrder(ctx context.Context, in domain.CreateShipmentFromOrderInput) (*domain.Shipment, error) {
	if err := domain.ValidateCreateShipmentFromOrderInput(in); err != nil {
		return nil, err
	}

	order, err := s.shipments.GetTransportOrder(ctx, in.TransportOrderID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateTransportOrderForShipment(order.Status); err != nil {
		return nil, err
	}

	exists, err := s.shipments.CompanyExists(ctx, in.CarrierCompanyID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperrors.NotFound("carrier_company_id not found")
	}

	return s.shipments.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID:              in.TenantID,
		ShipmentNumber:        in.ShipmentNumber,
		TransportOrderID:      in.TransportOrderID,
		ShipperCompanyID:      order.ShipperCompanyID,
		ConsigneeCompanyID:    order.ConsigneeCompanyID,
		CarrierCompanyID:      in.CarrierCompanyID,
		ForwarderCompanyID:    in.ForwarderCompanyID,
		OriginLocationID:      order.OriginLocationID,
		DestinationLocationID: order.DestinationLocationID,
		CargoID:               order.CargoID,
		TransportMode:         order.TransportMode,
		PlannedPickupAt:       in.PlannedPickupAt,
		PlannedDeliveryAt:     in.PlannedDeliveryAt,
	})
}

func (s *ShipmentService) CreateFromBid(ctx context.Context, in domain.CreateShipmentFromBidInput) (*domain.Shipment, error) {
	if err := domain.ValidateCreateShipmentFromBidInput(in); err != nil {
		return nil, err
	}

	bid, err := s.shipments.GetBid(ctx, in.BidID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateBidForShipment(bid.Status); err != nil {
		return nil, err
	}

	order, err := s.shipments.GetTransportOrder(ctx, in.TransportOrderID, in.TenantID)
	if err != nil {
		return nil, err
	}

	return s.shipments.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID:              in.TenantID,
		ShipmentNumber:        in.ShipmentNumber,
		TransportOrderID:      in.TransportOrderID,
		ShipperCompanyID:      order.ShipperCompanyID,
		ConsigneeCompanyID:    order.ConsigneeCompanyID,
		CarrierCompanyID:      bid.CarrierCompanyID,
		OriginLocationID:      order.OriginLocationID,
		DestinationLocationID: order.DestinationLocationID,
		CargoID:               order.CargoID,
		TransportMode:         order.TransportMode,
		PlannedPickupAt:       in.PlannedPickupAt,
		PlannedDeliveryAt:     in.PlannedDeliveryAt,
	})
}

func (s *ShipmentService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Shipment, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.shipments.GetByID(ctx, id)
}

func (s *ShipmentService) List(ctx context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListShipmentsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.shipments.List(ctx, filter)
}

func (s *ShipmentService) AssignDriver(ctx context.Context, id uuid.UUID, in domain.AssignDriverInput) (*domain.Shipment, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateAssignDriverInput(in); err != nil {
		return nil, err
	}

	shipment, err := s.shipments.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAssignDriverStatus(shipment.Status); err != nil {
		return nil, err
	}

	driver, err := s.drivers.GetByID(ctx, in.DriverID)
	if err != nil {
		return nil, err
	}
	if driver.TenantID != in.TenantID {
		return nil, apperrors.NotFound("driver not found")
	}
	if shipment.CarrierCompanyID == nil || driver.CarrierCompanyID != *shipment.CarrierCompanyID {
		return nil, apperrors.Validation("driver carrier_company_id must match shipment carrier_company_id", map[string]any{"field": "driver_id"})
	}

	newStatus := domain.ResolveStatusAfterAssignDriver(shipment.Status, shipment.VehicleID != nil)
	return s.shipments.AssignDriver(ctx, id, in.TenantID, in.DriverID, newStatus, shipment.Version)
}

func (s *ShipmentService) AssignVehicle(ctx context.Context, id uuid.UUID, in domain.AssignVehicleInput) (*domain.Shipment, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateAssignVehicleInput(in); err != nil {
		return nil, err
	}

	shipment, err := s.shipments.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAssignVehicleStatus(shipment.Status); err != nil {
		return nil, err
	}

	vehicle, err := s.vehicles.GetByID(ctx, in.VehicleID)
	if err != nil {
		return nil, err
	}
	if vehicle.TenantID != in.TenantID {
		return nil, apperrors.NotFound("vehicle not found")
	}
	if shipment.CarrierCompanyID == nil || vehicle.CarrierCompanyID != *shipment.CarrierCompanyID {
		return nil, apperrors.Validation("vehicle carrier_company_id must match shipment carrier_company_id", map[string]any{"field": "vehicle_id"})
	}

	newStatus := domain.ResolveStatusAfterAssignVehicle(shipment.DriverID != nil)
	return s.shipments.AssignVehicle(ctx, id, in.TenantID, in.VehicleID, newStatus, shipment.Version)
}

func (s *ShipmentService) Accept(ctx context.Context, id uuid.UUID, in domain.AcceptShipmentInput) (*domain.Shipment, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateAcceptShipmentInput(in); err != nil {
		return nil, err
	}

	shipment, err := s.shipments.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAcceptShipmentStatus(shipment.Status); err != nil {
		return nil, err
	}
	return s.shipments.Accept(ctx, id, in.TenantID, shipment.Version)
}

func (s *ShipmentService) UpdateStatus(ctx context.Context, id uuid.UUID, in domain.UpdateShipmentStatusInput) (*domain.Shipment, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateUpdateShipmentStatusInput(in); err != nil {
		return nil, err
	}

	shipment, err := s.shipments.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateStatusTransition(shipment.Status, in.Status); err != nil {
		return nil, err
	}

	var actualPickup, actualDelivery *time.Time
	if in.Status == domain.ShipmentStatusLoaded && shipment.ActualPickupAt == nil {
		actualPickup = in.ActualTime
	}
	if in.Status == domain.ShipmentStatusDelivered && shipment.ActualDeliveryAt == nil {
		actualDelivery = in.ActualTime
	}

	return s.shipments.UpdateStatus(ctx, id, in.TenantID, in.Status, actualPickup, actualDelivery, shipment.Version)
}

func (s *ShipmentService) Cancel(ctx context.Context, id uuid.UUID, in domain.CancelShipmentInput) (*domain.Shipment, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateCancelShipmentInput(in); err != nil {
		return nil, err
	}

	shipment, err := s.shipments.GetByIDAndTenant(ctx, id, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCancelShipmentStatus(shipment.Status); err != nil {
		return nil, err
	}
	return s.shipments.Cancel(ctx, id, in.TenantID, shipment.Version)
}
