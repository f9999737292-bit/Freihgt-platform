package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestUpdateStatusDeniesAssignmentOwnedManualTransitions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	shipmentID := uuid.New()
	cases := []struct {
		current string
		next    string
	}{
		{domain.ShipmentStatusCarrierAssigned, domain.ShipmentStatusAcceptedByCarrier},
		{domain.ShipmentStatusAcceptedByCarrier, domain.ShipmentStatusVehicleAssigned},
		{domain.ShipmentStatusAcceptedByCarrier, domain.ShipmentStatusDriverAssigned},
		{domain.ShipmentStatusVehicleAssigned, domain.ShipmentStatusDriverAssigned},
	}

	for _, tc := range cases {
		svc := NewShipmentService(&mockShipmentStore{
			getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
				return &domain.Shipment{
					ID: shipmentID, TenantID: tenantID, Status: tc.current, Version: 1,
				}, nil
			},
		}, &mockDriverLookup{}, &mockVehicleLookup{})

		_, err := svc.UpdateStatus(context.Background(), tenantID, shipmentID, domain.UpdateShipmentStatusInput{
			Status: tc.next,
		}, testUserTransition())
		if err == nil {
			t.Fatalf("expected manual transition %s -> %s to be denied", tc.current, tc.next)
		}
	}
}

func TestUpdateStatusDeniesExecutionWithoutAssignmentIDs(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	shipmentID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusDriverAssigned, Version: 1,
			}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.UpdateStatus(context.Background(), tenantID, shipmentID, domain.UpdateShipmentStatusInput{
		Status: domain.ShipmentStatusPickupSlotBooked,
	}, testUserTransition())
	if err == nil {
		t.Fatalf("expected progression without assignment IDs to be denied")
	}
}

func TestAssignmentFlowVehicleFirst(t *testing.T) {
	t.Parallel()

	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()
	vehicleID := uuid.New()

	var current domain.Shipment
	current = domain.Shipment{
		ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusAcceptedByCarrier,
		CarrierCompanyID: &carrierID, Version: 1,
	}

	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			copy := current
			return &copy, nil
		},
		assignVehicleFn: func(_ context.Context, id, gotTenantID, gotVehicleID uuid.UUID, fromStatus, newStatus string, version int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			if fromStatus != domain.ShipmentStatusAcceptedByCarrier || newStatus != domain.ShipmentStatusVehicleAssigned {
				t.Fatalf("unexpected vehicle assign transition: %s -> %s", fromStatus, newStatus)
			}
			current.Status = newStatus
			current.VehicleID = &vehicleID
			copy := current
			return &copy, nil
		},
		assignDriverFn: func(_ context.Context, id, gotTenantID, gotDriverID uuid.UUID, fromStatus, newStatus string, version int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			if fromStatus != domain.ShipmentStatusVehicleAssigned || newStatus != domain.ShipmentStatusDriverAssigned {
				t.Fatalf("unexpected driver assign transition: %s -> %s", fromStatus, newStatus)
			}
			current.Status = newStatus
			current.DriverID = &driverID
			copy := current
			return &copy, nil
		},
		updateStatusFn: func(_ context.Context, id, gotTenantID uuid.UUID, fromStatus, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, version int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			current.Status = newStatus
			copy := current
			return &copy, nil
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{ID: driverID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	}, &mockVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return &domain.Vehicle{ID: vehicleID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	})

	vehicleResult, err := svc.AssignVehicle(context.Background(), tenantID, shipmentID, vehicleID, testUserTransition())
	if err != nil {
		t.Fatalf("assign vehicle: %v", err)
	}
	if vehicleResult.VehicleID == nil || vehicleResult.Status != domain.ShipmentStatusVehicleAssigned {
		t.Fatalf("expected vehicle_id and VEHICLE_ASSIGNED, got status=%s vehicle=%v", vehicleResult.Status, vehicleResult.VehicleID)
	}

	driverResult, err := svc.AssignDriver(context.Background(), tenantID, shipmentID, driverID, testUserTransition())
	if err != nil {
		t.Fatalf("assign driver: %v", err)
	}
	if driverResult.DriverID == nil || driverResult.Status != domain.ShipmentStatusDriverAssigned {
		t.Fatalf("expected driver_id and DRIVER_ASSIGNED, got status=%s driver=%v", driverResult.Status, driverResult.DriverID)
	}

	progressed, err := svc.UpdateStatus(context.Background(), tenantID, shipmentID, domain.UpdateShipmentStatusInput{
		Status: domain.ShipmentStatusPickupSlotBooked,
	}, testUserTransition())
	if err != nil {
		t.Fatalf("expected execution progression after complete assignment, got %v", err)
	}
	if progressed.Status != domain.ShipmentStatusPickupSlotBooked {
		t.Fatalf("expected PICKUP_SLOT_BOOKED, got %s", progressed.Status)
	}
}

func TestAssignmentFlowDriverFirst(t *testing.T) {
	t.Parallel()

	carrierID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()
	vehicleID := uuid.New()

	var current domain.Shipment
	current = domain.Shipment{
		ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusAcceptedByCarrier,
		CarrierCompanyID: &carrierID, Version: 1,
	}

	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			copy := current
			return &copy, nil
		},
		assignDriverFn: func(_ context.Context, id, gotTenantID, gotDriverID uuid.UUID, fromStatus, newStatus string, version int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			if newStatus != domain.ShipmentStatusAcceptedByCarrier {
				t.Fatalf("expected ACCEPTED_BY_CARRIER after driver-first assign, got %s", newStatus)
			}
			current.DriverID = &driverID
			current.Status = newStatus
			copy := current
			return &copy, nil
		},
		assignVehicleFn: func(_ context.Context, id, gotTenantID, gotVehicleID uuid.UUID, fromStatus, newStatus string, version int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			if newStatus != domain.ShipmentStatusDriverAssigned {
				t.Fatalf("expected DRIVER_ASSIGNED after vehicle assign, got %s", newStatus)
			}
			current.VehicleID = &vehicleID
			current.Status = newStatus
			copy := current
			return &copy, nil
		},
	}, &mockDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{ID: driverID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	}, &mockVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return &domain.Vehicle{ID: vehicleID, TenantID: tenantID, CarrierCompanyID: carrierID}, nil
		},
	})

	driverResult, err := svc.AssignDriver(context.Background(), tenantID, shipmentID, driverID, testUserTransition())
	if err != nil {
		t.Fatalf("assign driver: %v", err)
	}
	if driverResult.DriverID == nil || driverResult.Status != domain.ShipmentStatusAcceptedByCarrier {
		t.Fatalf("expected driver_id with ACCEPTED_BY_CARRIER, got status=%s driver=%v", driverResult.Status, driverResult.DriverID)
	}

	vehicleResult, err := svc.AssignVehicle(context.Background(), tenantID, shipmentID, vehicleID, testUserTransition())
	if err != nil {
		t.Fatalf("assign vehicle: %v", err)
	}
	if vehicleResult.VehicleID == nil || vehicleResult.DriverID == nil || vehicleResult.Status != domain.ShipmentStatusDriverAssigned {
		t.Fatalf("expected both IDs and DRIVER_ASSIGNED, got status=%s driver=%v vehicle=%v", vehicleResult.Status, vehicleResult.DriverID, vehicleResult.VehicleID)
	}
}

func TestUpdateStatusOperationalChain(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()
	vehicleID := uuid.New()

	chain := []string{
		domain.ShipmentStatusPickupSlotBooked,
		domain.ShipmentStatusInPickup,
		domain.ShipmentStatusLoaded,
		domain.ShipmentStatusInTransit,
	}

	current := domain.Shipment{
		ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusDriverAssigned,
		DriverID: &driverID, VehicleID: &vehicleID, Version: 1,
	}

	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			copy := current
			return &copy, nil
		},
		updateStatusFn: func(_ context.Context, id, gotTenantID uuid.UUID, fromStatus, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, version int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			current.Status = newStatus
			copy := current
			return &copy, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	for _, next := range chain {
		input := domain.UpdateShipmentStatusInput{Status: next}
		if next == domain.ShipmentStatusLoaded {
			input.ActualTime = ptrTime(time.Now().UTC())
		}
		updated, err := svc.UpdateStatus(context.Background(), tenantID, shipmentID, input, testUserTransition())
		if err != nil {
			t.Fatalf("expected %s -> %s to succeed, got %v", current.Status, next, err)
		}
		if updated.Status != next {
			t.Fatalf("expected status %s, got %s", next, updated.Status)
		}
	}
}

func TestUpdateStatusIncompleteDriverAssignedRecordDenied(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()

	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID, Status: domain.ShipmentStatusDriverAssigned,
				DriverID: &driverID, Version: 1,
			}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.UpdateStatus(context.Background(), tenantID, shipmentID, domain.UpdateShipmentStatusInput{
		Status: domain.ShipmentStatusPickupSlotBooked,
	}, testUserTransition())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error for missing vehicle_id, got %v", err)
	}
}
