package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

func TestParseExecutionEventAcceptsJsonbSpacedEmptyActionID(t *testing.T) {
	t.Parallel()
	operating := uuid.New()
	executionID := uuid.New()
	revisionID := uuid.New()
	stopID := uuid.New()
	eventID := uuid.New()
	payload := []byte(`{
		"event_id": "` + eventID.String() + `",
		"event_type": "shipment.route_stop.current",
		"operating_tenant_id": "` + operating.String() + `",
		"execution_id": "` + executionID.String() + `",
		"revision_id": "` + revisionID.String() + `",
		"event_sequence": 2,
		"stop_id": "` + stopID.String() + `",
		"action_id": "",
		"occurred_at": "2026-10-03T09:31:47.444508Z"
	}`)

	event, err := parseExecutionEvent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if event.EventID != eventID || event.ExecutionID != executionID || event.Sequence != 2 {
		t.Fatalf("identity %+v", event)
	}
	if event.ActionID != nil {
		t.Fatal("blank action_id must decode as absent")
	}
	if event.StopID == nil || *event.StopID != stopID {
		t.Fatal("stop id lost")
	}
	if event.EventType != domain.EventRouteStopCurrent {
		t.Fatal(event.EventType)
	}
	if event.OccurredAt.IsZero() || event.OccurredAt.UTC() != time.Date(2026, 10, 3, 9, 31, 47, 444508000, time.UTC) {
		t.Fatal(event.OccurredAt)
	}
	if strings.Contains(string(normalizeEmptyUUIDStrings(payload)), `"action_id":""`) {
		t.Fatal("blank action_id remained an empty string")
	}
}
