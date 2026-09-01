package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateManualStatusTransitionBlocksAssignmentOwnedTargets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from string
		to   string
	}{
		{ShipmentStatusCarrierAssigned, ShipmentStatusAcceptedByCarrier},
		{ShipmentStatusAcceptedByCarrier, ShipmentStatusVehicleAssigned},
		{ShipmentStatusAcceptedByCarrier, ShipmentStatusDriverAssigned},
		{ShipmentStatusVehicleAssigned, ShipmentStatusDriverAssigned},
	}

	for _, tc := range cases {
		if err := ValidateManualStatusTransition(tc.from, tc.to); err == nil {
			t.Fatalf("expected manual transition %s -> %s to be denied", tc.from, tc.to)
		}
	}
}

func TestValidateManualStatusTransitionAllowsOperationalProgression(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from string
		to   string
	}{
		{ShipmentStatusDriverAssigned, ShipmentStatusPickupSlotBooked},
		{ShipmentStatusPickupSlotBooked, ShipmentStatusInPickup},
		{ShipmentStatusInPickup, ShipmentStatusLoaded},
		{ShipmentStatusLoaded, ShipmentStatusInTransit},
	}

	for _, tc := range cases {
		if err := ValidateManualStatusTransition(tc.from, tc.to); err != nil {
			t.Fatalf("expected manual transition %s -> %s to be allowed, got %v", tc.from, tc.to, err)
		}
	}
}

func TestValidateExecutionAssignmentReadiness(t *testing.T) {
	t.Parallel()

	driverID := uuid.New()
	vehicleID := uuid.New()

	if err := ValidateExecutionAssignmentReadiness(&Shipment{
		Status: ShipmentStatusDriverAssigned,
	}, ShipmentStatusPickupSlotBooked); err == nil {
		t.Fatalf("expected missing assignment IDs to be denied")
	}

	if err := ValidateExecutionAssignmentReadiness(&Shipment{
		Status:    ShipmentStatusDriverAssigned,
		DriverID:  &driverID,
		VehicleID: &vehicleID,
	}, ShipmentStatusPickupSlotBooked); err != nil {
		t.Fatalf("expected complete assignment to allow progression, got %v", err)
	}
}
