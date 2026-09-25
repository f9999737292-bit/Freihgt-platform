package repository

import (
	"strings"
	"testing"
)

func TestMigration000080Text(t *testing.T) {
	up := readMigration(t, "000080_bno_match_score_topn_v0_1c2.up.sql")
	down := readMigration(t, "000080_bno_match_score_topn_v0_1c2.down.sql")
	for _, required := range []string{
		"score_profiles",
		"score_profile_components",
		"MIN_DEADHEAD",
		"MAX_CONTRIBUTION",
		"RESERVED",
		"match_candidates_score_total_chk",
		"match_candidates_search_rank_uidx",
		"SCORE_NOT_RECORDED",
		"ranking_currency",
		"bno-score-0.1c2.1",
		"weight_bps <= 10000",
		"score_profile_components_ordinal_uidx",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS network_optimizer.score_profiles") || !strings.Contains(down, "DROP COLUMN IF EXISTS ranking_currency") {
		t.Fatal("down migration incomplete")
	}
}
