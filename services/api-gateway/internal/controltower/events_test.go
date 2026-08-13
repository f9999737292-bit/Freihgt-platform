package controltower

import (
	"testing"
	"time"
)

func TestDeterministicEventIDPickupDelayUsesPlannedPickupAnchor(t *testing.T) {
	t.Parallel()
	pickup := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	shipment := ControlTowerShipment{
		ID:              "11111111-1111-1111-1111-111111111111",
		PlannedPickupAt: &pickup,
	}

	first := deterministicEventID(shipment.ID, EventTypePickupDelay, canonicalEventAnchor(shipment, EventTypePickupDelay))
	second := deterministicEventID(shipment.ID, EventTypePickupDelay, canonicalEventAnchor(shipment, EventTypePickupDelay))
	if first != second {
		t.Fatalf("expected stable pickup delay id, got %q vs %q", first, second)
	}
	if !ValidateCriticalEventID(first) {
		t.Fatalf("expected valid event id format, got %q", first)
	}
}

func TestDeterministicEventIDCancelledUsesSentinelAnchor(t *testing.T) {
	t.Parallel()
	updated := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	shipment := ControlTowerShipment{
		ID:            "11111111-1111-1111-1111-111111111111",
		Status:        "CANCELLED",
		LastUpdatedAt: &updated,
	}

	withUpdated := deterministicEventID(shipment.ID, EventTypeShipmentCancelled, canonicalEventAnchor(shipment, EventTypeShipmentCancelled))
	shipment.LastUpdatedAt = ptrTime(updated.Add(time.Hour))
	withDifferentUpdated := deterministicEventID(shipment.ID, EventTypeShipmentCancelled, canonicalEventAnchor(shipment, EventTypeShipmentCancelled))
	if withUpdated != withDifferentUpdated {
		t.Fatalf("cancelled event id must not depend on LastUpdatedAt")
	}
}

func TestFindCriticalEventByID(t *testing.T) {
	t.Parallel()
	events := []ControlTowerEvent{{ID: "abc123", ShipmentID: "ship-1"}}
	if _, ok := FindCriticalEventByID(events, "missing"); ok {
		t.Fatal("expected missing event")
	}
	got, ok := FindCriticalEventByID(events, "abc123")
	if !ok || got.ShipmentID != "ship-1" {
		t.Fatalf("unexpected event: %+v ok=%v", got, ok)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
