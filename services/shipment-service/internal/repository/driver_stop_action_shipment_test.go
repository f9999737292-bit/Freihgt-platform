package repository

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestSummarizeStopActionsSingleShipment(t *testing.T) {
	shipmentID := uuid.New()
	cargoID := uuid.New()
	summary, stopShipment := summarizeStopActions([]actionTaskSource{{
		ID: uuid.New(), ShipmentID: shipmentID, CargoID: cargoID, ActionType: domain.ActionTypePickup, Ordinal: 1,
	}})
	if stopShipment == nil || *stopShipment != shipmentID {
		t.Fatalf("stop shipment = %v", stopShipment)
	}
	if len(summary.Actions) != 1 || summary.Actions[0].ShipmentID != shipmentID || summary.Actions[0].CargoID != cargoID {
		t.Fatalf("action fact %+v", summary.Actions)
	}
}

func TestSummarizeStopActionsMultiShipment(t *testing.T) {
	shipmentA := uuid.New()
	shipmentB := uuid.New()
	cargoA := uuid.New()
	cargoB := uuid.New()
	summary, stopShipment := summarizeStopActions([]actionTaskSource{
		{ID: uuid.New(), ShipmentID: shipmentA, CargoID: cargoA, ActionType: domain.ActionTypeDelivery, Ordinal: 0},
		{ID: uuid.New(), ShipmentID: shipmentB, CargoID: cargoB, ActionType: domain.ActionTypeDelivery, Ordinal: 1},
	})
	if stopShipment != nil {
		t.Fatalf("multi-shipment stop stored %s", stopShipment)
	}
	if summary.Actions[0].ShipmentID != shipmentA || summary.Actions[0].CargoID != cargoA {
		t.Fatalf("action A %+v", summary.Actions[0])
	}
	if summary.Actions[1].ShipmentID != shipmentB || summary.Actions[1].CargoID != cargoB {
		t.Fatalf("action B %+v", summary.Actions[1])
	}
}

func TestApplyCanonicalActionShipmentsReplacesStaleIdentity(t *testing.T) {
	actionID := uuid.New()
	canonicalID := uuid.New()
	summary := domain.DriverStopActionSummary{Actions: []domain.DriverStopActionFact{{
		ActionID: actionID, ShipmentID: uuid.Nil, CargoID: uuid.New(), ActionType: domain.ActionTypeDelivery,
	}}}
	if err := applyCanonicalActionShipments(&summary, map[uuid.UUID]uuid.UUID{actionID: canonicalID}); err != nil {
		t.Fatal(err)
	}
	if summary.Actions[0].ShipmentID != canonicalID {
		t.Fatalf("shipment = %s", summary.Actions[0].ShipmentID)
	}

	summary.Actions[0].ShipmentID = uuid.New()
	if err := applyCanonicalActionShipments(&summary, map[uuid.UUID]uuid.UUID{actionID: canonicalID}); err != nil {
		t.Fatal(err)
	}
	if summary.Actions[0].ShipmentID != canonicalID {
		t.Fatal("stale materialized shipment id was kept")
	}
	if summary.Actions[0].ShipmentID == uuid.Nil {
		t.Fatal("zero shipment id was exposed")
	}
}

func TestApplyCanonicalActionShipmentsRejectsMissingIdentity(t *testing.T) {
	actionID := uuid.New()
	summary := domain.DriverStopActionSummary{Actions: []domain.DriverStopActionFact{{ActionID: actionID}}}
	if err := applyCanonicalActionShipments(&summary, map[uuid.UUID]uuid.UUID{}); err == nil {
		t.Fatal("missing canonical action was accepted")
	}
	if err := applyCanonicalActionShipments(&summary, map[uuid.UUID]uuid.UUID{actionID: uuid.Nil}); err == nil {
		t.Fatal("zero canonical shipment id was accepted")
	}
}
