package repository

import (
	"strings"
	"testing"
)

func TestMigration000014UpCreatesOutboxTable(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000014_create_shipment_event_outbox_v0.1.up.sql"))
	if !strings.Contains(up, "create table transport.shipment_event_outbox") {
		t.Fatal("up migration must create transport.shipment_event_outbox")
	}
	if !strings.Contains(up, "uq_shipment_event_outbox_source_event") {
		t.Fatal("up migration must define unique source_event_id")
	}
	if !strings.Contains(up, "idx_shipment_event_outbox_pending") {
		t.Fatal("up migration must create pending index")
	}
	if !strings.Contains(up, "fk_shipment_event_outbox_history") {
		t.Fatal("up migration must define FK to status history")
	}
	if !strings.Contains(up, "chk_shipment_event_outbox_status") {
		t.Fatal("up migration must define status check constraint")
	}
	if !strings.Contains(up, "chk_shipment_event_outbox_attempts") {
		t.Fatal("up migration must define attempts check constraint")
	}
	if strings.Contains(up, "insert into transport.shipment_event_outbox") {
		t.Fatal("up migration must not backfill outbox")
	}
	if strings.Contains(up, "kafka") || strings.Contains(up, "nats") || strings.Contains(up, "rabbit") {
		t.Fatal("migration must not contain broker-specific SQL")
	}
}

func TestMigration000014DownDropsOutboxSafely(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000014_create_shipment_event_outbox_v0.1.down.sql"))
	if !strings.Contains(down, "drop index") {
		t.Fatal("down migration must drop indexes")
	}
	if !strings.Contains(down, "drop table") || !strings.Contains(down, "transport.shipment_event_outbox") {
		t.Fatal("down migration must drop outbox table")
	}
}

func TestMigration000014UpDoesNotModifyExistingTables(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000014_create_shipment_event_outbox_v0.1.up.sql"))
	if strings.Contains(up, "alter table transport.shipments") {
		t.Fatal("migration must not alter shipments table")
	}
	if strings.Contains(up, "alter table transport.shipment_status_history") {
		t.Fatal("migration must not alter status history table")
	}
}
