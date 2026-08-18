package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildExecutionReadinessRequiresDriverAndVehicle(t *testing.T) {
	carrierID := uuid.New()
	driverID := uuid.New()
	shipment := &Shipment{
		Status:           ShipmentStatusAcceptedByCarrier,
		CarrierCompanyID: &carrierID,
	}
	readiness := BuildExecutionReadiness(shipment)
	if readiness.CarrierAccepted != true {
		t.Fatalf("expected carrier accepted")
	}
	if readiness.DriverAssigned || readiness.VehicleAssigned || readiness.ReadyToStart {
		t.Fatalf("expected missing driver and vehicle")
	}

	shipment.DriverID = &driverID
	shipment.VehicleID = &driverID
	shipment.Status = ShipmentStatusDriverAssigned
	readiness = BuildExecutionReadiness(shipment)
	if !readiness.ReadyToStart {
		t.Fatalf("expected ready to start")
	}
}

func TestValidateStartExecutionDeniedWithoutAssignments(t *testing.T) {
	carrierID := uuid.New()
	shipment := &Shipment{
		Status:           ShipmentStatusAcceptedByCarrier,
		CarrierCompanyID: &carrierID,
	}
	if err := ValidateStartExecution(shipment, carrierID); err == nil {
		t.Fatal("expected validation error")
	}
}
