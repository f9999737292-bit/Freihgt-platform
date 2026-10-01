//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNLO04D_Migration000094(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000086(t, ctx, pool)
	up := mustRead(t, "000094_nlo_service_duration_policy_v0_4d.up.sql")
	down := mustRead(t, "000094_nlo_service_duration_policy_v0_4d.down.sql")
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("down clean: %v", err)
	}
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	tenant := uuid.New()
	first := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policies (id, tenant_id, version, status, created_at)
		VALUES ($1,$2,1,'DRAFT',now())`, first, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policy_entries (policy_id, action_type, duration_seconds)
		VALUES ($1,'PICKUP',900), ($1,'DELIVERY',1200)`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, down); err == nil || !strings.Contains(err.Error(), "nlo-0.4d down migration refused") {
		t.Fatalf("down with rows: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.service_duration_policy_entries`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.service_duration_policies`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("down after delete: %v", err)
	}
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("up down up: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policies (id, tenant_id, version, status, created_at)
		VALUES ($1,$2,1,'DRAFT',now())`, first, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policy_entries (policy_id, action_type, duration_seconds)
		VALUES ($1,'PICKUP',900), ($1,'DELIVERY',1200)`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.service_duration_policies
		SET status='ACTIVE', published_at=now()
		WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.service_duration_policy_entries
		SET duration_seconds=1 WHERE policy_id=$1 AND action_type='PICKUP'`, first); err == nil || !strings.Contains(err.Error(), "SERVICE_DURATION_POLICY_IMMUTABLE") {
		t.Fatalf("active entry update: %v", err)
	}
	second := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policies (id, tenant_id, version, status, created_at)
		VALUES ($1,$2,2,'DRAFT',now())`, second, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policy_entries (policy_id, action_type, duration_seconds)
		VALUES ($1,'PICKUP',30), ($1,'DELIVERY',40)`, second); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.service_duration_policies
		SET status='ACTIVE', published_at=now()
		WHERE id=$1`, second); err == nil {
		t.Fatal("second active policy inserted beside the first")
	}
	planID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plans (
			id, tenant_id, version, status, planning_mode, result_status,
			shipment_id, shipment_version, context_fingerprint, evaluation_fingerprint,
			algorithm_policy_version, routing_policy_version, execution_supported, reason_codes, created_at
		) VALUES ($1,$2,1,'EVALUATED','CURRENT_TRIP','FEASIBLE_PLAN_FOUND',$3,1,'ctx','eval','alg','route',false,'{}',now())`,
		planID, tenant, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_dependencies (
			id, route_plan_id, dependency_kind, subject_id, subject_version, fingerprint
		) VALUES ($1,$2,'SERVICE_DURATION_POLICY',$3,1,'PICKUP=900|DELIVERY=1200')`,
		uuid.New(), planID, first); err != nil {
		t.Fatal(err)
	}
}

func applyThrough000086(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
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
		"000086_nlo_route_plan_accept_activate_v0_4c.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("up %s: %v", name, err)
		}
	}
}
