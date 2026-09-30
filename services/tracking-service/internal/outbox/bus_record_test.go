package outbox

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/repository"
)

func TestBuildBusRecordMatchesShipmentContract(t *testing.T) {
	eventID := uuid.New()
	aggregateID := uuid.New()
	record, err := BuildBusRecord("shipment.status.v1", repository.TrackingOutboxEvent{
		AggregateID:   aggregateID,
		EventType:     "tracking.stop.approaching",
		SchemaVersion: 1,
		SourceEventID: eventID,
		Payload:       []byte(`{"eventType":"tracking.stop.approaching"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Topic != "shipment.status.v1" || record.Key != aggregateID.String() {
		t.Fatalf("record topic/key = %s %s", record.Topic, record.Key)
	}
	got := map[string]string{}
	for _, header := range record.Headers {
		got[header.Key] = header.Value
	}
	for _, key := range []string{"event_type", "schema_version", "source_event_id", "content_type"} {
		if got[key] == "" {
			t.Fatalf("missing header %s", key)
		}
	}
	if got["event_type"] != "tracking.stop.approaching" || got["content_type"] != "application/json" || got["source_event_id"] != eventID.String() {
		t.Fatalf("headers %+v", got)
	}
}
