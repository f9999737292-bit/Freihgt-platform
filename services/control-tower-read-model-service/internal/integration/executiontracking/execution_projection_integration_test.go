//go:build integration

package executiontracking

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

func Test06CTProjectionReceivesActualPickup(t *testing.T) {
	ctEnv := setupCTEnv(t)
	tenantID := uuid.New()
	shipmentID := uuid.New()
	actualPickup := time.Date(2026, 8, 10, 9, 30, 0, 0, time.UTC)
	plannedPickup := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	plannedDelivery := time.Date(2026, 8, 10, 18, 0, 0, 0, time.UTC)

	payload := buildStatusOutboxPayload(t, outboxPayloadInput{
		TenantID:          tenantID,
		ShipmentID:        shipmentID,
		Version:           1,
		ToStatus:          domain.StatusLoaded,
		PlannedPickupAt:   &plannedPickup,
		PlannedDeliveryAt: &plannedDelivery,
		ActualPickupAt:    &actualPickup,
	})
	processOutboxPayload(t, ctEnv, payload, tenantID, shipmentID, 1)

	projection, err := ctEnv.repo.GetProjection(context.Background(), tenantID, shipmentID)
	if err != nil || projection == nil {
		t.Fatalf("projection: %v", err)
	}
	if projection.CurrentStatus != domain.StatusLoaded {
		t.Fatalf("ct status=%s", projection.CurrentStatus)
	}
	if projection.ActualPickupAt == nil || !projection.ActualPickupAt.Equal(actualPickup) {
		t.Fatalf("actual_pickup_at=%v want %v", projection.ActualPickupAt, actualPickup)
	}
	if projection.PlannedPickupAt == nil || !projection.PlannedPickupAt.Equal(plannedPickup) {
		t.Fatalf("planned_pickup_at=%v", projection.PlannedPickupAt)
	}
	if projection.PlannedDeliveryAt == nil || !projection.PlannedDeliveryAt.Equal(plannedDelivery) {
		t.Fatalf("planned_delivery_at=%v", projection.PlannedDeliveryAt)
	}
}

func Test07CTDuplicateEventFirstWriteWins(t *testing.T) {
	ctEnv := setupCTEnv(t)
	tenantID := uuid.New()
	shipmentID := uuid.New()
	firstActual := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	secondActual := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	payload1 := buildStatusOutboxPayload(t, outboxPayloadInput{
		TenantID: tenantID, ShipmentID: shipmentID, Version: 1,
		ToStatus: domain.StatusLoaded, ActualPickupAt: &firstActual,
	})
	processOutboxPayload(t, ctEnv, payload1, tenantID, shipmentID, 1)

	payload2 := buildStatusOutboxPayload(t, outboxPayloadInput{
		TenantID: tenantID, ShipmentID: shipmentID, Version: 2,
		ToStatus: domain.StatusInTransit, ActualPickupAt: &secondActual,
	})
	processOutboxPayload(t, ctEnv, payload2, tenantID, shipmentID, 2)

	projection, err := ctEnv.repo.GetProjection(context.Background(), tenantID, shipmentID)
	if err != nil || projection == nil {
		t.Fatalf("projection: %v", err)
	}
	if projection.ActualPickupAt == nil || !projection.ActualPickupAt.Equal(firstActual) {
		t.Fatalf("expected first-write-wins actual pickup, got %v", projection.ActualPickupAt)
	}
}

func Test19CTActualDeliveryProjection(t *testing.T) {
	ctEnv := setupCTEnv(t)
	tenantID := uuid.New()
	shipmentID := uuid.New()
	actualPickup := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	actualDelivery := time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC)

	steps := []struct {
		version int
		status  string
		pickup  *time.Time
		delivery *time.Time
		offset  int64
	}{
		{1, domain.StatusLoaded, &actualPickup, nil, 1},
		{2, domain.StatusInTransit, nil, nil, 2},
		{3, domain.StatusDelivered, nil, &actualDelivery, 3},
	}
	for _, step := range steps {
		payload := buildStatusOutboxPayload(t, outboxPayloadInput{
			TenantID: tenantID, ShipmentID: shipmentID, Version: step.version,
			ToStatus: step.status, ActualPickupAt: step.pickup, ActualDeliveryAt: step.delivery,
		})
		processOutboxPayload(t, ctEnv, payload, tenantID, shipmentID, step.offset)
	}

	projection, err := ctEnv.repo.GetProjection(context.Background(), tenantID, shipmentID)
	if err != nil || projection == nil {
		t.Fatalf("projection: %v", err)
	}
	if projection.CurrentStatus != domain.StatusDelivered {
		t.Fatalf("ct status=%s want DELIVERED", projection.CurrentStatus)
	}
	if projection.ActualPickupAt == nil || !projection.ActualPickupAt.Equal(actualPickup) {
		t.Fatal("ct actual_pickup_at missing or overwritten")
	}
	if projection.ActualDeliveryAt == nil || !projection.ActualDeliveryAt.Equal(actualDelivery) {
		t.Fatal("ct actual_delivery_at missing")
	}
}

type outboxPayloadInput struct {
	TenantID, ShipmentID          uuid.UUID
	Version                       int
	ToStatus                      string
	PlannedPickupAt               *time.Time
	PlannedDeliveryAt             *time.Time
	ActualPickupAt, ActualDeliveryAt *time.Time
}

func buildStatusOutboxPayload(t *testing.T, in outboxPayloadInput) []byte {
	t.Helper()
	eventID := uuid.New()
	sourceEventID := uuid.New()
	envelope := map[string]any{
		"eventId":       eventID.String(),
		"eventType":     domain.EventTypeStatusChanged,
		"schemaVersion": domain.SchemaVersionV1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId":      in.TenantID.String(),
		"aggregate": map[string]any{
			"type":    domain.AggregateTypeShipment,
			"id":      in.ShipmentID.String(),
			"version": in.Version,
		},
		"sourceEventId": sourceEventID.String(),
		"data": map[string]any{
			"toStatus":  in.ToStatus,
			"actorType": "DRIVER",
		},
	}
	data := envelope["data"].(map[string]any)
	if in.PlannedPickupAt != nil {
		data["plannedPickupAt"] = in.PlannedPickupAt.Format(time.RFC3339Nano)
	}
	if in.PlannedDeliveryAt != nil {
		data["plannedDeliveryAt"] = in.PlannedDeliveryAt.Format(time.RFC3339Nano)
	}
	if in.ActualPickupAt != nil {
		data["actualPickupAt"] = in.ActualPickupAt.Format(time.RFC3339Nano)
	}
	if in.ActualDeliveryAt != nil {
		data["actualDeliveryAt"] = in.ActualDeliveryAt.Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return raw
}

func TestCTOutboxToProjectionChainMatchesShipmentEnvelope(t *testing.T) {
	// Validates CT parser accepts the same JSON field names emitted by shipment outbox.
	ctEnv := setupCTEnv(t)
	tenantID := uuid.New()
	shipmentID := uuid.New()
	actual := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"eventId":"` + uuid.New().String() + `",
		"eventType":"shipment.status.changed",
		"schemaVersion":1,
		"occurredAt":"2026-08-11T08:00:00Z",
		"tenantId":"` + tenantID.String() + `",
		"aggregate":{"type":"SHIPMENT","id":"` + shipmentID.String() + `","version":1},
		"sourceEventId":"` + uuid.New().String() + `",
		"data":{"toStatus":"LOADED","actorType":"DRIVER","actualPickupAt":"2026-08-11T08:00:00Z"}
	}`)
	processOutboxPayload(t, ctEnv, payload, tenantID, shipmentID, 1)
	projection, err := ctEnv.repo.GetProjection(context.Background(), tenantID, shipmentID)
	if err != nil || projection == nil || projection.ActualPickupAt == nil {
		t.Fatalf("projection from shipment-style envelope: err=%v projection=%v", err, projection)
	}
	if !projection.ActualPickupAt.Equal(actual) {
		t.Fatalf("actual=%v want %v", projection.ActualPickupAt, actual)
	}
}
