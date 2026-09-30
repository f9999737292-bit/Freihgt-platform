package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestExecutionStopDedupKeyUsesStopIdentity(t *testing.T) {
	stop := uuid.New()
	when := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	observed := when.Add(-time.Minute)
	eventID := "evt-1"
	first := BuildExecutionStopETADedupKey("generic", "execution_stop", stop, when, observed, &eventID)
	second := BuildExecutionStopETADedupKey("generic", "execution_stop", stop, when, observed, &eventID)
	if first != second {
		t.Fatal("same execution stop eta did not dedup")
	}
	otherStop := uuid.New()
	if first == BuildExecutionStopETADedupKey("generic", "execution_stop", otherStop, when, observed, &eventID) {
		t.Fatal("different stops shared a dedup key")
	}
	shipmentKey := BuildETADedupKey("generic", "pickup", uuid.New(), when, observed, &eventID)
	if first == shipmentKey {
		t.Fatal("execution stop key collided with a pickup key")
	}
}
