package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCommandDigestRejectsDifferentBody(t *testing.T) {
	cmd := sampleCommand()
	left, err := CommandDigest(cmd)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CommandDigest(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if left != again {
		t.Fatal("digest changed for the same body")
	}
	cmd.OccurredAt = cmd.OccurredAt.Add(time.Minute)
	right, err := CommandDigest(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("different body produced the same digest")
	}
}

func TestDepartedPickupReusesInTransit(t *testing.T) {
	target, ok, informational := MapDriverEventToTargetStatus("DEPARTED_PICKUP")
	if !ok || informational || target != ShipmentStatusInTransit {
		t.Fatalf("DEPARTED_PICKUP target=%s ok=%v informational=%v", target, ok, informational)
	}
	if err := ValidateStatusTransition(ShipmentStatusLoaded, ShipmentStatusInTransit); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStatusTransition(ShipmentStatusInTransit, "EN_ROUTE"); err == nil {
		t.Fatal("EN_ROUTE transition was accepted")
	}
	if err := ValidateStatusTransition(ShipmentStatusInTransit, ShipmentStatusDelivered); err == nil {
		t.Fatal("IN_TRANSIT to DELIVERED was accepted")
	}
}

func TestDriverExceptionCatalogueReused(t *testing.T) {
	if !IsDriverExceptionCategory("route_blocked") || !IsDriverExceptionCategory("CARGO_ISSUE") {
		t.Fatal("existing exception category was not recognized")
	}
	if IsDriverExceptionCategory("NEW_STATUS") || KnownExecutionReason("EN_ROUTE") {
		t.Fatal("unknown category was accepted")
	}
}

func TestShipmentChainStaysClosed(t *testing.T) {
	chain := []string{
		ShipmentStatusPickupSlotBooked,
		ShipmentStatusInPickup,
		ShipmentStatusLoaded,
		ShipmentStatusInTransit,
		ShipmentStatusArrivedAtConsignee,
		ShipmentStatusUnloading,
		ShipmentStatusDelivered,
	}
	for i := 0; i < len(chain)-1; i++ {
		if err := ValidateStatusTransition(chain[i], chain[i+1]); err != nil {
			t.Fatalf("%s -> %s: %v", chain[i], chain[i+1], err)
		}
	}
}

func sampleCommand() ExecutionCommand {
	return ExecutionCommand{
		Name:                CommandArriveStop,
		ExecutionID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		RevisionID:          uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		StopID:              uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		IdempotencyKey:      "arrive-1",
		OccurredAt:          time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC),
		ExpectedStopVersion: 1,
		ActorKind:           ActorKindDriver,
		ActorID:             uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		OperatingTenantID:   uuid.MustParse("55555555-5555-5555-5555-555555555555"),
	}
}
