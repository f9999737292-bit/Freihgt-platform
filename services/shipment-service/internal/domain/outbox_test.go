package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func sampleHistory(from *string, to string) ShipmentStatusHistory {
	return ShipmentStatusHistory{
		ID:              uuid.New(),
		TenantID:        uuid.New(),
		ShipmentID:      uuid.New(),
		ShipmentVersion: 4,
		FromStatus:      from,
		ToStatus:        to,
		ActorType:       ActorTypeUser,
		OccurredAt:      time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestMapOutboxEventTypeCreated(t *testing.T) {
	t.Parallel()
	if got := MapOutboxEventType(sampleHistory(nil, ShipmentStatusCarrierAssigned)); got != OutboxEventTypeCreated {
		t.Fatalf("got %s want %s", got, OutboxEventTypeCreated)
	}
}

func TestMapOutboxEventTypeGenericChange(t *testing.T) {
	t.Parallel()
	from := ShipmentStatusInTransit
	if got := MapOutboxEventType(sampleHistory(&from, ShipmentStatusDelivered)); got != OutboxEventTypeStatusChanged {
		t.Fatalf("got %s", got)
	}
}

func TestMapOutboxEventTypeCancelled(t *testing.T) {
	t.Parallel()
	from := ShipmentStatusCarrierAssigned
	if got := MapOutboxEventType(sampleHistory(&from, ShipmentStatusCancelled)); got != OutboxEventTypeCancelled {
		t.Fatalf("got %s", got)
	}
}

func TestMapOutboxEventTypeBillingMilestones(t *testing.T) {
	t.Parallel()
	from := ShipmentStatusDeliveryConfirmed
	cases := map[string]string{
		ShipmentStatusReadyForBilling:    OutboxEventTypeReadyForBilling,
		ShipmentStatusDocumentsCompleted: OutboxEventTypeDocumentsCompleted,
		ShipmentStatusFinanciallyClosed:  OutboxEventTypeFinanciallyClosed,
	}
	for to, want := range cases {
		if got := MapOutboxEventType(sampleHistory(&from, to)); got != want {
			t.Fatalf("to=%s got=%s want=%s", to, got, want)
		}
	}
}

func TestBuildOutboxEventFromStatusHistory(t *testing.T) {
	t.Parallel()
	correlation := "req-123"
	history := sampleHistory(nil, ShipmentStatusCarrierAssigned)
	history.CorrelationID = &correlation
	outbox, err := BuildOutboxEventFromStatusHistory(history)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if outbox.SourceEventID != history.ID {
		t.Fatal("source event id must match history id")
	}
	if outbox.SchemaVersion != OutboxSchemaVersion {
		t.Fatalf("schema=%d", outbox.SchemaVersion)
	}
	if outbox.EventType != OutboxEventTypeCreated {
		t.Fatalf("eventType=%s", outbox.EventType)
	}
	if err := ValidateOutboxAgainstHistory(outbox, history); err != nil {
		t.Fatalf("validate: %v", err)
	}
	var envelope ShipmentStatusEventEnvelope
	if err := json.Unmarshal(outbox.Payload, &envelope); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if envelope.SourceEventID != history.ID.String() {
		t.Fatal("envelope sourceEventId mismatch")
	}
	if envelope.CorrelationID == nil || *envelope.CorrelationID != correlation {
		t.Fatal("correlation id must be preserved")
	}
	if !envelope.OccurredAt.Equal(history.OccurredAt) {
		t.Fatal("occurredAt mismatch")
	}
	lower := strings.ToLower(string(outbox.Payload))
	for _, forbidden := range []string{"jwt", "email", "phone", "password", "authorization"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("payload must not contain %s", forbidden)
		}
	}
}

func TestBuildOutboxEventStableSerialization(t *testing.T) {
	t.Parallel()
	history := sampleHistory(nil, ShipmentStatusCarrierAssigned)
	first, err := BuildOutboxEventFromStatusHistory(history)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildOutboxEventFromStatusHistory(history)
	if err != nil {
		t.Fatal(err)
	}
	if first.EventType != second.EventType || first.SchemaVersion != second.SchemaVersion {
		t.Fatal("mapping must be deterministic for same history")
	}
}

func TestValidateOutboxAgainstHistoryRejectsMismatch(t *testing.T) {
	t.Parallel()
	history := sampleHistory(nil, ShipmentStatusCarrierAssigned)
	outbox, err := BuildOutboxEventFromStatusHistory(history)
	if err != nil {
		t.Fatal(err)
	}
	outbox.TenantID = uuid.New()
	if err := ValidateOutboxAgainstHistory(outbox, history); err == nil {
		t.Fatal("tenant mismatch must fail")
	}
}
