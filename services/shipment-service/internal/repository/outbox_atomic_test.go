package repository

import (
	"os"
	"strings"
	"testing"
)

func TestInsertStatusHistoryReturningQueryPresent(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(insertStatusHistoryReturningQuery)
	if !strings.Contains(q, "returning id") {
		t.Fatal("history insert must return id for outbox source_event_id")
	}
}

func TestInsertOutboxEventQueryPresent(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(insertOutboxEventQuery)
	if !strings.Contains(q, "insert into transport.shipment_event_outbox") {
		t.Fatal("outbox insert query must target outbox table")
	}
	if !strings.Contains(q, "source_event_id") {
		t.Fatal("outbox insert must persist source_event_id")
	}
}

func TestClaimPendingOutboxUsesSkipLocked(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(claimPendingOutboxQuery)
	if !strings.Contains(q, "for update skip locked") {
		t.Fatal("claim query must use FOR UPDATE SKIP LOCKED")
	}
	if !strings.Contains(q, "status = 'pending'") {
		t.Fatal("claim query must filter pending rows")
	}
	if !strings.Contains(q, "available_at <=") {
		t.Fatal("claim query must respect available_at")
	}
	if !strings.Contains(q, "locked_at is null or locked_at <") {
		t.Fatal("claim query must recover stale locks")
	}
	if !strings.Contains(q, "order by created_at asc, id asc") {
		t.Fatal("claim query must use stable ordering")
	}
}

func TestMarkOutboxPublishedRequiresWorkerLock(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(markOutboxPublishedQuery)
	if !strings.Contains(q, "locked_by = $3") {
		t.Fatal("mark published must require current worker lock")
	}
	if !strings.Contains(q, "status = 'pending'") {
		t.Fatal("mark published must only update pending rows")
	}
}

func TestShipmentRepositoryUsesInsertStatusHistoryAndOutbox(t *testing.T) {
	t.Parallel()
	content := readRepoSource(t, "shipment_repository.go")
	lower := strings.ToLower(content)
	if strings.Count(lower, "insertstatushistoryandoutbox") < 4 {
		t.Fatal("status mutation paths must call insertStatusHistoryAndOutbox")
	}
	if strings.Contains(lower, "insertstatushistoryrow(") {
		t.Fatal("repository must not call legacy insertStatusHistoryRow directly")
	}
}

func readRepoSource(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read repo source %s: %v", name, err)
	}
	return string(content)
}
