//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO04B_Migration000085(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	for _, name := range []string{
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
		"000081_nlo_pairwise_consolidation_v0_3b.up.sql",
		"000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql",
		"000083_nlo_current_trip_fill_v0_3d.up.sql",
		"000084_nlo_bounded_n_member_search_v0_3e.up.sql",
		"000085_nlo_route_plan_bounded_planner_v0_4b.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("up %s: %v", name, err)
		}
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000085_nlo_route_plan_bounded_planner_v0_4b.down.sql")); err != nil {
		t.Fatalf("down clean: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000085_nlo_route_plan_bounded_planner_v0_4b.up.sql")); err != nil {
		t.Fatalf("up down up: %v", err)
	}
	planID := uuid.New()
	tenant := uuid.New()
	capacity := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plans (
			id, tenant_id, version, status, planning_mode, result_status,
			capacity_id, capacity_version, context_fingerprint, evaluation_fingerprint,
			algorithm_policy_version, routing_policy_version, execution_supported, reason_codes, created_at
		) VALUES ($1,$2,1,'EVALUATED','DEPOT_START','INDETERMINATE_PLAN_FOUND',$3,1,'ctx','eval','alg','route',false,'{}',now())`,
		planID, tenant, capacity); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, mustRead(t, "000085_nlo_route_plan_bounded_planner_v0_4b.down.sql"))
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("down with rows: %v", err)
	}
	var still bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM network_optimizer.route_plans WHERE id=$1)`, planID).Scan(&still); err != nil || !still {
		t.Fatalf("plan survived fail-closed down: %v exists=%v", err, still)
	}
}
