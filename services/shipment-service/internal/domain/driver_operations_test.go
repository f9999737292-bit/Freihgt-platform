package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestCanDriverAccessShipment(t *testing.T) {
	tenantA := uuid.New()
	driverA := uuid.New()
	driverB := uuid.New()
	shipmentID := uuid.New()

	shipment := &Shipment{
		ID:       shipmentID,
		TenantID: tenantA,
		DriverID: &driverA,
	}
	if !CanDriverAccessShipment(tenantA, driverA, shipment) {
		t.Fatal("expected driver A to access assigned shipment")
	}
	if CanDriverAccessShipment(tenantA, driverB, shipment) {
		t.Fatal("expected driver B denied for driver A shipment")
	}
	if CanDriverAccessShipment(uuid.New(), driverA, shipment) {
		t.Fatal("expected cross-tenant denied")
	}
}

func TestMapDriverEventToTargetStatus(t *testing.T) {
	status, change, info := MapDriverEventToTargetStatus("ARRIVED_AT_PICKUP")
	if !change || info || status != ShipmentStatusInPickup {
		t.Fatalf("unexpected mapping: status=%q change=%v info=%v", status, change, info)
	}
	_, change, info = MapDriverEventToTargetStatus("LOADING_STARTED")
	if change || !info {
		t.Fatalf("expected informational event")
	}
	_, change, info = MapDriverEventToTargetStatus("INVALID")
	if change || info {
		t.Fatalf("expected unsupported event")
	}
}

func TestValidateDriverExceptionInputRejectsUnknownCategory(t *testing.T) {
	err := ValidateDriverExceptionInput(DriverExceptionInput{
		Category:       "SEVERITY_INJECTION",
		IdempotencyKey: "key-1",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
