package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type mockShipmentStore struct {
	getTransportOrderFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrderSnapshot, error)
	getBidFn            func(ctx context.Context, id, tenantID uuid.UUID) (*domain.BidSnapshot, error)
	createFn            func(ctx context.Context, params repository.CreateShipmentParams) (*domain.Shipment, error)
	getByIDAndTenantFn  func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	assignDriverFn      func(ctx context.Context, id, tenantID, driverID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error)
	assignVehicleFn     func(ctx context.Context, id, tenantID, vehicleID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error)
	updateStatusFn      func(ctx context.Context, id, tenantID uuid.UUID, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int) (*domain.Shipment, error)
	cancelFn            func(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.Shipment, error)
}

func (m *mockShipmentStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockShipmentStore) GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrderSnapshot, error) {
	return m.getTransportOrderFn(ctx, id, tenantID)
}
func (m *mockShipmentStore) GetBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.BidSnapshot, error) {
	return m.getBidFn(ctx, id, tenantID)
}
func (m *mockShipmentStore) CreateShipment(ctx context.Context, params repository.CreateShipmentParams) (*domain.Shipment, error) {
	return m.createFn(ctx, params)
}
func (m *mockShipmentStore) GetByID(context.Context, uuid.UUID) (*domain.Shipment, error) {
	return nil, nil
}
func (m *mockShipmentStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	return m.getByIDAndTenantFn(ctx, id, tenantID)
}
func (m *mockShipmentStore) List(context.Context, domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
	return nil, 0, nil
}
func (m *mockShipmentStore) AssignDriver(ctx context.Context, id, tenantID, driverID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error) {
	return m.assignDriverFn(ctx, id, tenantID, driverID, newStatus, expectedVersion)
}
func (m *mockShipmentStore) AssignVehicle(ctx context.Context, id, tenantID, vehicleID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error) {
	return m.assignVehicleFn(ctx, id, tenantID, vehicleID, newStatus, expectedVersion)
}
func (m *mockShipmentStore) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int) (*domain.Shipment, error) {
	return m.updateStatusFn(ctx, id, tenantID, newStatus, actualPickupAt, actualDeliveryAt, expectedVersion)
}
func (m *mockShipmentStore) Accept(context.Context, uuid.UUID, uuid.UUID, int) (*domain.Shipment, error) {
	return nil, nil
}
func (m *mockShipmentStore) Cancel(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.Shipment, error) {
	return m.cancelFn(ctx, id, tenantID, expectedVersion)
}

type mockDriverLookup struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Driver, error)
}

func (m *mockDriverLookup) GetByID(ctx context.Context, id uuid.UUID) (*domain.Driver, error) {
	return m.getByIDFn(ctx, id)
}

type mockVehicleLookup struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error)
}

func (m *mockVehicleLookup) GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error) {
	return m.getByIDFn(ctx, id)
}

func TestShipmentServiceCreateFromTransportOrderValidation(t *testing.T) {
	t.Parallel()
	svc := NewShipmentService(&mockShipmentStore{}, &mockDriverLookup{}, &mockVehicleLookup{})
	_, err := svc.CreateFromTransportOrder(context.Background(), domain.CreateShipmentFromOrderInput{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestShipmentServiceCreateFromTransportOrderInvalidDates(t *testing.T) {
	t.Parallel()
	pickup := time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC)
	delivery := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	svc := NewShipmentService(&mockShipmentStore{}, &mockDriverLookup{}, &mockVehicleLookup{})
	_, err := svc.CreateFromTransportOrder(context.Background(), domain.CreateShipmentFromOrderInput{
		TenantID: uuid.New(), ShipmentNumber: "SH-1", TransportOrderID: uuid.New(),
		CarrierCompanyID: uuid.New(), PlannedPickupAt: &pickup, PlannedDeliveryAt: &delivery,
	})
	if err == nil {
		t.Fatalf("expected validation error for invalid dates")
	}
}

func TestShipmentServiceCreateFromBid(t *testing.T) {
	t.Parallel()
	carrierID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	svc := NewShipmentService(&mockShipmentStore{
		getBidFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BidSnapshot, error) {
			return &domain.BidSnapshot{Status: domain.BidStatusAccepted, CarrierCompanyID: carrierID}, nil
		},
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrderSnapshot, error) {
			return &domain.TransportOrderSnapshot{
				Status: domain.TransportOrderStatusAssigned,
				ShipperCompanyID: uuid.New(), ConsigneeCompanyID: uuid.New(),
				OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
				TransportMode: "ROAD",
			}, nil
		},
		createFn: func(_ context.Context, params repository.CreateShipmentParams) (*domain.Shipment, error) {
			if params.CarrierCompanyID != carrierID {
				t.Fatalf("expected carrier from bid")
			}
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned, CarrierCompanyID: &carrierID}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	shipment, err := svc.CreateFromBid(context.Background(), domain.CreateShipmentFromBidInput{
		TenantID: uuid.New(), ShipmentNumber: "SH-2", BidID: uuid.New(), TransportOrderID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shipment.Status != domain.ShipmentStatusCarrierAssigned {
		t.Fatalf("unexpected status: %s", shipment.Status)
	}
}

func TestShipmentServiceAssignDriver(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusCarrierAssigned,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignDriverFn: func(_ context.Context, id, gotTenantID, gotDriverID uuid.UUID, newStatus string, version int) (*domain.Shipment, error) {
			if newStatus != domain.ShipmentStatusAcceptedByCarrier {
				t.Fatalf("expected ACCEPTED_BY_CARRIER, got %s", newStatus)
			}
			return &domain.Shipment{Status: newStatus, DriverID: &driverID}, nil
		},
	}, &mockDriverLookup{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{ID: driverID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	}, &mockVehicleLookup{})

	shipment, err := svc.AssignDriver(context.Background(), shipmentID, domain.AssignDriverInput{
		TenantID: tenantID, DriverID: driverID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shipment.Status != domain.ShipmentStatusAcceptedByCarrier {
		t.Fatalf("unexpected status: %s", shipment.Status)
	}
}

func TestShipmentServiceAssignVehicle(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	vehicleID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusAcceptedByCarrier,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignVehicleFn: func(_ context.Context, id, gotTenantID, gotVehicleID uuid.UUID, newStatus string, version int) (*domain.Shipment, error) {
			if newStatus != domain.ShipmentStatusVehicleAssigned {
				t.Fatalf("expected VEHICLE_ASSIGNED, got %s", newStatus)
			}
			return &domain.Shipment{Status: newStatus, VehicleID: &vehicleID}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.Vehicle, error) {
			return &domain.Vehicle{ID: vehicleID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	})

	shipment, err := svc.AssignVehicle(context.Background(), shipmentID, domain.AssignVehicleInput{
		TenantID: tenantID, VehicleID: vehicleID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shipment.Status != domain.ShipmentStatusVehicleAssigned {
		t.Fatalf("unexpected status: %s", shipment.Status)
	}
}

func TestShipmentServiceInvalidStatusTransition(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned, Version: 1}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.UpdateStatus(context.Background(), uuid.New(), domain.UpdateShipmentStatusInput{
		TenantID: tenantID, Status: domain.ShipmentStatusLoaded, ActualTime: ptrTime(time.Now().UTC()),
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestShipmentServiceCancelForbiddenAfterDelivered(t *testing.T) {
	t.Parallel()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{Status: domain.ShipmentStatusDelivered, Version: 1}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.Cancel(context.Background(), uuid.New(), domain.CancelShipmentInput{TenantID: uuid.New()})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
