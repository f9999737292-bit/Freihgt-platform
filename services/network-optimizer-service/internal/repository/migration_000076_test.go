package repository

import (
	"strings"
	"testing"
)

func TestMigration000076Text(t *testing.T) {
	up := readMigration(t, "000076_bno_predictive_capacity_v0_1b.up.sql")
	down := readMigration(t, "000076_bno_predictive_capacity_v0_1b.down.sql")
	for _, required := range []string{
		"CREATE TABLE network_optimizer.predicted_capacities",
		"predicted_capacities_one_current_uidx",
		"predicted_capacities_owner_idx",
		"CURRENT_SHIPMENT_PREDICTION",
		"'PREDICTED'",
		"confidence >= 0 AND confidence <= 1",
		"loading_access IS NULL",
		"body_type IS NULL",
		"ALTER TABLE transport.vehicles",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	if strings.Contains(strings.ToLower(up), "trailer_id") {
		t.Fatal("migration must not invent a trailer master")
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS network_optimizer.predicted_capacities") || !strings.Contains(down, "source = 'MANUAL'") {
		t.Fatal("down migration must restore the manual capacity constraint")
	}
}
