package repository

import (
	"strings"
	"testing"
)

func TestMigration000079Text(t *testing.T) {
	up := readMigration(t, "000079_bno_next_load_candidate_search_v0_1c1.up.sql")
	down := readMigration(t, "000079_bno_next_load_candidate_search_v0_1c1.down.sql")
	for _, required := range []string{
		"ADD COLUMN radius_km double precision",
		"carrier_search_radius_chk",
		"capacity_search_radius_chk",
		"CREATE TABLE network_optimizer.next_load_search_runs",
		"CREATE TABLE network_optimizer.match_candidates",
		"reject_reasons text[]",
		"match_candidates_run_fk",
		"ELIGIBLE",
		"REJECTED",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	if strings.Contains(strings.ToLower(up), "create table network_optimizer.locations") {
		t.Fatal("migration created a second location master")
	}
	for _, required := range []string{
		"DROP TABLE IF EXISTS network_optimizer.match_candidates",
		"DROP TABLE IF EXISTS network_optimizer.next_load_search_runs",
		"DROP COLUMN IF EXISTS radius_km",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %s", required)
		}
	}
}
