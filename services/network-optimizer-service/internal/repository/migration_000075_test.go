package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigration000075Text(t *testing.T) {
	up := readMigration(t, "000075_bno_capacity_marketplace_foundation_v0_1a.up.sql")
	down := readMigration(t, "000075_bno_capacity_marketplace_foundation_v0_1a.down.sql")
	if !strings.Contains(up, "CREATE SCHEMA IF NOT EXISTS network_optimizer") {
		t.Fatal("up migration must create the network_optimizer schema")
	}
	for _, required := range []string{
		"CREATE TABLE network_optimizer.load_opportunities",
		"CREATE TABLE network_optimizer.capacities",
		"CREATE TABLE network_optimizer.audit_events",
		"CREATE TABLE network_optimizer.outbox",
		"CREATE TABLE network_optimizer.idempotency_keys",
		"load_opportunities_owner_status_idx",
		"load_opportunities_visibility_status_idx",
		"load_opportunities_active_source_uidx",
		"capacities_owner_status_idx",
		"capacities_visibility_status_idx",
		"CONSTRAINT capacities_source_chk CHECK (source = 'MANUAL')",
		"CONSTRAINT capacities_status_chk CHECK (status IN ('AVAILABLE', 'WITHDRAWN'))",
		"weight_kg IS NULL OR weight_kg > 0",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	for _, absent := range []string{"pallet_count", "linear_metre", "linear_meter", "trailer_id", "ALTER TABLE transport.shipments", "ALTER TABLE transport.transport_orders"} {
		if strings.Contains(strings.ToLower(up), absent) {
			t.Fatalf("up migration must not contain %s", absent)
		}
	}
	if strings.TrimSpace(down) != "DROP SCHEMA IF EXISTS network_optimizer CASCADE;" {
		t.Fatalf("down migration = %q", down)
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
