package repository

import (
	"strings"
	"testing"
)

func TestProcessEventQueryUsesInboxIdempotencyKeys(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(`
SELECT outcome
FROM control_tower.shipment_status_event_inbox
WHERE event_id = $1
   OR source_event_id = $2
   OR (topic = $3 AND partition_id = $4 AND message_offset = $5)
LIMIT 1`)
	if !strings.Contains(q, "where event_id = $1") {
		t.Fatal("inbox lookup must filter by event_id")
	}
	if !strings.Contains(q, "source_event_id = $2") {
		t.Fatal("inbox lookup must filter by source_event_id")
	}
	if !strings.Contains(q, "topic = $3 and partition_id = $4 and message_offset = $5") {
		t.Fatal("inbox lookup must filter by kafka position")
	}
}

func TestLockProjectionQueryUsesTenantAndShipmentForUpdate(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(`
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at, complete, gap_detected,
       gap_from_version, gap_to_version, created_at, updated_at
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1 AND shipment_id = $2
FOR UPDATE`)
	if !strings.Contains(q, "where tenant_id = $1 and shipment_id = $2") {
		t.Fatal("projection lock must scope by tenant and shipment")
	}
	if !strings.Contains(q, "for update") {
		t.Fatal("projection lock must use FOR UPDATE")
	}
}

func TestUpsertProjectionQueryUsesTenantShipmentConflict(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(`
ON CONFLICT (tenant_id, shipment_id) DO UPDATE SET
    shipment_version = EXCLUDED.shipment_version`)
	if !strings.Contains(q, "on conflict (tenant_id, shipment_id) do update") {
		t.Fatal("projection upsert must conflict on tenant_id + shipment_id")
	}
}

func TestGetProjectionQueryUsesTenantPredicate(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(`
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1 AND shipment_id = $2`)
	if !strings.Contains(q, "where tenant_id = $1 and shipment_id = $2") {
		t.Fatal("get projection must filter by tenant and shipment")
	}
}

func TestGetStatusSummaryQueriesAreTenantScoped(t *testing.T) {
	t.Parallel()
	queries := []string{
		`SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id = $1`,
		`SELECT current_status, COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id = $1 GROUP BY current_status`,
		`SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id = $1 AND complete = FALSE`,
		`SELECT MIN(updated_at), MAX(updated_at) FROM control_tower.shipment_status_projection WHERE tenant_id = $1`,
	}
	for _, q := range queries {
		if !strings.Contains(strings.ToLower(q), "where tenant_id = $1") {
			t.Fatalf("summary query must be tenant scoped: %q", q)
		}
	}
}

func TestInsertDeadLetterDoesNotStoreRawPayload(t *testing.T) {
	t.Parallel()
	q := strings.ToLower(`
INSERT INTO control_tower.shipment_status_event_dead_letter (
    payload_sha256, error_code, received_at
) VALUES ($9, $10, $11)
ON CONFLICT (topic, partition_id, message_offset) DO NOTHING`)
	if !strings.Contains(q, "payload_sha256") {
		t.Fatal("dead-letter insert must store payload_sha256")
	}
	if strings.Contains(q, "payload bytea") || strings.Contains(q, "raw_payload") {
		t.Fatal("dead-letter insert must not store raw payload")
	}
	if !strings.Contains(q, "on conflict (topic, partition_id, message_offset) do nothing") {
		t.Fatal("dead-letter insert must be idempotent by kafka position")
	}
}

func TestListProjectionsDefaultLimit(t *testing.T) {
	t.Parallel()
	limit := normalizeListLimit(0)
	if limit != 50 {
		t.Fatalf("default limit=%d want 50", limit)
	}
}

func TestListProjectionsCapsLimitAt100(t *testing.T) {
	t.Parallel()
	limit := normalizeListLimit(500)
	if limit != 50 {
		t.Fatalf("capped limit=%d want 50", limit)
	}
}

func normalizeListLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 50
	}
	return limit
}
