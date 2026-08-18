package domain

import (
	"testing"
	"time"
)

func TestAllowedDriverMilestoneActionsForInPickup(t *testing.T) {
	actions := AllowedDriverMilestoneActions(ShipmentStatusInPickup)
	if len(actions) != 2 || actions[0].Type != "LOADING_STARTED" {
		t.Fatalf("actions=%v", actions)
	}
}

func TestBuildExecutionSLASignalsPickupLateness(t *testing.T) {
	planned := time.Now().UTC().Add(-2 * time.Hour)
	shipment := &Shipment{
		Status:          ShipmentStatusInTransit,
		PlannedPickupAt: &planned,
	}
	signals := BuildExecutionSLASignals(shipment, time.Now().UTC())
	found := false
	for _, signal := range signals {
		if signal.Code == "PICKUP_LATE" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected pickup lateness signal")
	}
}

func TestBuildExecutionSLASignalsNoDuplicateActualPickup(t *testing.T) {
	planned := time.Now().UTC().Add(-2 * time.Hour)
	actual := planned.Add(-30 * time.Minute)
	delivered := time.Now().UTC()
	shipment := &Shipment{
		Status:           ShipmentStatusDelivered,
		PlannedPickupAt:  &planned,
		ActualPickupAt:   &actual,
		ActualDeliveryAt: &delivered,
		PlannedDeliveryAt: func() *time.Time {
			v := time.Now().UTC().Add(-1 * time.Hour)
			return &v
		}(),
	}
	signals := BuildExecutionSLASignals(shipment, time.Now().UTC())
	for _, signal := range signals {
		if signal.Code == "PICKUP_LATE" {
			t.Fatal("should not warn after on-time pickup recorded")
		}
	}
}
