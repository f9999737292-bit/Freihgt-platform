package repository

import "strings"
import "testing"

func TestNLO03CMigration000082Text(t *testing.T) {
	up := readMigration(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql")
	down := readMigration(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.down.sql")
	for _, required := range []string{
		"CREATE TABLE transport.shipment_cargo_execution_evidence",
		"shipment_id uuid NOT NULL REFERENCES transport.shipments (id)",
		"cargo_id uuid NOT NULL REFERENCES transport.cargoes (id)",
		"CONFIRMED_ONBOARD",
		"UNLOADED",
		"state_version > 0",
		"shipment_version > 0",
		"UNIQUE (shipment_id, cargo_id, state_version)",
		"append-only",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("NLO03C_001 up missing %s", required)
		}
	}
	if strings.Contains(strings.ToUpper(up), "INSERT INTO") {
		t.Fatal("NLO03C_004 historical backfill")
	}
	if strings.Contains(down, "consolidation_") || strings.Contains(down, "000081") {
		t.Fatal("NLO03C_002 down touches 000081")
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS transport.shipment_cargo_execution_evidence") {
		t.Fatal("NLO03C_002 down missing drop")
	}
}
