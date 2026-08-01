package repository

import (
	"strings"
	"testing"
)

func TestMigration000015UpCreatesControlTowerSchema(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.up.sql"))
	if !strings.Contains(up, "create schema if not exists control_tower") {
		t.Fatal("up migration must create control_tower schema")
	}
}

func TestMigration000015UpCreatesInboxTable(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.up.sql"))
	if !strings.Contains(up, "create table control_tower.shipment_status_event_inbox") {
		t.Fatal("up migration must create shipment_status_event_inbox")
	}
	if !strings.Contains(up, "event_id uuid primary key") {
		t.Fatal("up migration must define event_id primary key")
	}
	if !strings.Contains(up, "uq_shipment_status_event_inbox_source_event") {
		t.Fatal("up migration must define unique source_event_id")
	}
	if !strings.Contains(up, "uq_shipment_status_event_inbox_position") {
		t.Fatal("up migration must define unique kafka position")
	}
	if !strings.Contains(up, "chk_shipment_status_event_inbox_version") {
		t.Fatal("up migration must define aggregate version check")
	}
	if !strings.Contains(up, "chk_shipment_status_event_inbox_schema_version") {
		t.Fatal("up migration must define schema version check")
	}
	if !strings.Contains(up, "chk_shipment_status_event_inbox_outcome") {
		t.Fatal("up migration must define outcome check")
	}
}

func TestMigration000015UpCreatesProjectionTable(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.up.sql"))
	if !strings.Contains(up, "create table control_tower.shipment_status_projection") {
		t.Fatal("up migration must create shipment_status_projection")
	}
	if !strings.Contains(up, "primary key (tenant_id, shipment_id)") {
		t.Fatal("up migration must define tenant_id + shipment_id primary key")
	}
	if !strings.Contains(up, "chk_shipment_status_projection_version") {
		t.Fatal("up migration must define shipment version check")
	}
	if !strings.Contains(up, "chk_shipment_status_projection_gap") {
		t.Fatal("up migration must define gap check constraint")
	}
	if !strings.Contains(up, "idx_shipment_status_projection_tenant_status") {
		t.Fatal("up migration must create tenant+status index")
	}
	if !strings.Contains(up, "idx_shipment_status_projection_tenant_updated") {
		t.Fatal("up migration must create tenant+updated index")
	}
}

func TestMigration000015UpCreatesDeadLetterTable(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.up.sql"))
	if !strings.Contains(up, "create table control_tower.shipment_status_event_dead_letter") {
		t.Fatal("up migration must create shipment_status_event_dead_letter")
	}
	if !strings.Contains(up, "uq_shipment_status_event_dead_letter_position") {
		t.Fatal("up migration must define dead-letter unique kafka position")
	}
	if !strings.Contains(up, "payload_sha256") {
		t.Fatal("up migration must store payload_sha256")
	}
	if strings.Contains(up, "payload bytea") || strings.Contains(up, "raw_payload") {
		t.Fatal("up migration must not store raw payload column")
	}
}

func TestMigration000015UpDoesNotBackfill(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.up.sql"))
	if strings.Contains(up, "insert into control_tower.") {
		t.Fatal("up migration must not backfill projection data")
	}
}

func TestMigration000015UpDoesNotModifyShipmentTables(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.up.sql"))
	if strings.Contains(up, "alter table transport.shipments") {
		t.Fatal("migration must not alter transport.shipments")
	}
	if strings.Contains(up, "alter table transport.shipment_status_history") {
		t.Fatal("migration must not alter shipment_status_history")
	}
	if strings.Contains(up, "alter table transport.shipment_event_outbox") {
		t.Fatal("migration must not alter shipment_event_outbox")
	}
}

func TestMigration000015DownDropsReadModelObjects(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.down.sql"))
	if !strings.Contains(down, "drop index") {
		t.Fatal("down migration must drop indexes")
	}
	if !strings.Contains(down, "drop table if exists control_tower.shipment_status_event_dead_letter") {
		t.Fatal("down migration must drop dead-letter table")
	}
	if !strings.Contains(down, "drop table if exists control_tower.shipment_status_projection") {
		t.Fatal("down migration must drop projection table")
	}
	if !strings.Contains(down, "drop table if exists control_tower.shipment_status_event_inbox") {
		t.Fatal("down migration must drop inbox table")
	}
	if !strings.Contains(down, "drop schema if exists control_tower") {
		t.Fatal("down migration must drop control_tower schema")
	}
}

func TestMigration000015DownDoesNotModifyShipmentTables(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000015_create_control_tower_shipment_status_projection_v0.1.down.sql"))
	if strings.Contains(down, "transport.shipments") {
		t.Fatal("down migration must not touch shipment tables")
	}
}
