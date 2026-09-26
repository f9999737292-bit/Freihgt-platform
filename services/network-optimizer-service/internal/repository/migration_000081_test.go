package repository

import (
	"strings"
	"testing"
)

func TestNLO03BMigration000081Text(t *testing.T) {
	up := readMigration(t, "000081_nlo_pairwise_consolidation_v0_3b.up.sql")
	down := readMigration(t, "000081_nlo_pairwise_consolidation_v0_3b.down.sql")
	for _, required := range []string{
		"consolidation_allowed boolean NOT NULL DEFAULT false",
		"cross_shipper_consolidation_allowed boolean NOT NULL DEFAULT false",
		"load_opportunities_published_opt_in_od_idx",
		"consolidation_search_runs",
		"SAME_ORIGIN_SAME_DESTINATION",
		"consolidation_candidates",
		"execution_supported = false",
		"NOT_EVALUATED",
		"consolidation_candidate_members",
		"load_owner_tenant_id",
		"consolidation_candidates_fingerprint_uidx",
		"REFERENCES network_optimizer.capacities (id, owner_tenant_id)",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("NLO03B_077 up migration missing %s", required)
		}
	}
	if strings.Contains(up, "000080") {
		t.Fatal("000081 must not rewrite 000080")
	}
	for _, required := range []string{
		"DROP TABLE IF EXISTS network_optimizer.consolidation_candidate_members",
		"DROP TABLE IF EXISTS network_optimizer.consolidation_candidates",
		"DROP TABLE IF EXISTS network_optimizer.consolidation_search_runs",
		"DROP COLUMN IF EXISTS cross_shipper_consolidation_allowed",
		"DROP COLUMN IF EXISTS consolidation_allowed",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("NLO03B_078 down migration missing %s", required)
		}
	}
}
