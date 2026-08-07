package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMigration(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(content)
}

func TestMigration000012UpCreatesStatusHistoryTable(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000012_create_shipment_status_history_v0.1.up.sql"))
	if !strings.Contains(up, "create table transport.shipment_status_history") {
		t.Fatal("up migration must create transport.shipment_status_history")
	}
	if !strings.Contains(up, "uq_shipment_status_history_version") {
		t.Fatal("up migration must define unique (shipment_id, shipment_version)")
	}
	if !strings.Contains(up, "idx_shipment_status_history_tenant_shipment_time") {
		t.Fatal("up migration must create tenant/shipment/time index")
	}
	if !strings.Contains(up, "fk_shipment_status_history_shipment") {
		t.Fatal("up migration must define foreign key to shipments")
	}
	if strings.Contains(up, "insert into transport.shipment_status_history") {
		t.Fatal("up migration must not backfill status history")
	}
	if strings.Contains(up, "migration") && strings.Contains(up, "updated_at") {
		t.Fatal("up migration must not synthesize history from updated_at")
	}
}

func TestMigration000012DownDropsStatusHistorySafely(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000012_create_shipment_status_history_v0.1.down.sql"))
	if !strings.Contains(down, "drop index") || !strings.Contains(down, "idx_shipment_status_history_tenant_shipment_time") {
		t.Fatal("down migration must drop index before table")
	}
	if !strings.Contains(down, "drop table") || !strings.Contains(down, "transport.shipment_status_history") {
		t.Fatal("down migration must drop status history table")
	}
}

func TestMigration000012UpDoesNotModifyShipments(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000012_create_shipment_status_history_v0.1.up.sql"))
	if strings.Contains(up, "alter table transport.shipments") {
		t.Fatal("migration must not alter existing shipments table")
	}
}
