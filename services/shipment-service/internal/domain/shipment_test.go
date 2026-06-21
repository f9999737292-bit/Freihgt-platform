package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateCreateShipmentFromOrderInput(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pickup := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	delivery := time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC)

	if err := ValidateCreateShipmentFromOrderInput(CreateShipmentFromOrderInput{
		TenantID: tenantID, ShipmentNumber: "SH-1", TransportOrderID: uuid.New(),
		CarrierCompanyID: uuid.New(), PlannedPickupAt: &pickup, PlannedDeliveryAt: &delivery,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateShipmentFromOrderInput(CreateShipmentFromOrderInput{}); err == nil {
		t.Fatalf("expected validation error for missing tenant_id")
	}
	if err := ValidateCreateShipmentFromOrderInput(CreateShipmentFromOrderInput{
		TenantID: tenantID,
	}); err == nil {
		t.Fatalf("expected validation error for missing shipment_number")
	}
}

func TestValidatePlannedDeliveryBeforePickup(t *testing.T) {
	t.Parallel()
	pickup := time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC)
	delivery := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	if err := ValidateCreateShipmentFromOrderInput(CreateShipmentFromOrderInput{
		TenantID: uuid.New(), ShipmentNumber: "SH-1", TransportOrderID: uuid.New(),
		CarrierCompanyID: uuid.New(), PlannedPickupAt: &pickup, PlannedDeliveryAt: &delivery,
	}); err == nil {
		t.Fatalf("expected validation error when delivery is before pickup")
	}
}

func TestValidateStatusTransition(t *testing.T) {
	t.Parallel()
	if err := ValidateStatusTransition(ShipmentStatusCarrierAssigned, ShipmentStatusAcceptedByCarrier); err != nil {
		t.Fatalf("expected allowed transition")
	}
	if err := ValidateStatusTransition(ShipmentStatusCarrierAssigned, ShipmentStatusLoaded); err == nil {
		t.Fatalf("expected forbidden transition")
	}
}

func TestValidateCancelShipmentStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateCancelShipmentStatus(ShipmentStatusCarrierAssigned); err != nil {
		t.Fatalf("expected cancel allowed")
	}
	if err := ValidateCancelShipmentStatus(ShipmentStatusDelivered); err == nil {
		t.Fatalf("expected cancel forbidden after DELIVERED")
	}
}

func TestResolveStatusAfterAssignDriver(t *testing.T) {
	t.Parallel()
	if got := ResolveStatusAfterAssignDriver(ShipmentStatusCarrierAssigned, true); got != ShipmentStatusDriverAssigned {
		t.Fatalf("expected DRIVER_ASSIGNED, got %s", got)
	}
	if got := ResolveStatusAfterAssignDriver(ShipmentStatusCarrierAssigned, false); got != ShipmentStatusAcceptedByCarrier {
		t.Fatalf("expected ACCEPTED_BY_CARRIER, got %s", got)
	}
}

func TestResolveStatusAfterAssignVehicle(t *testing.T) {
	t.Parallel()
	if got := ResolveStatusAfterAssignVehicle(true); got != ShipmentStatusDriverAssigned {
		t.Fatalf("expected DRIVER_ASSIGNED, got %s", got)
	}
	if got := ResolveStatusAfterAssignVehicle(false); got != ShipmentStatusVehicleAssigned {
		t.Fatalf("expected VEHICLE_ASSIGNED, got %s", got)
	}
}

func TestValidateBidForShipment(t *testing.T) {
	t.Parallel()
	if err := ValidateBidForShipment(BidStatusAccepted); err != nil {
		t.Fatalf("expected accepted bid to be valid")
	}
	if err := ValidateBidForShipment("SUBMITTED"); err == nil {
		t.Fatalf("expected validation error for non-accepted bid")
	}
}
