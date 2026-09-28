//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO04C_Migration000086(t *testing.T) {
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
		"000086_nlo_route_plan_accept_activate_v0_4c.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("up %s: %v", name, err)
		}
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000086_nlo_route_plan_accept_activate_v0_4c.down.sql")); err != nil {
		t.Fatalf("down clean: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000086_nlo_route_plan_accept_activate_v0_4c.up.sql")); err != nil {
		t.Fatalf("up down up: %v", err)
	}
	planID := uuid.New()
	tenant := uuid.New()
	shipment := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plans (
			id, tenant_id, version, status, planning_mode, result_status,
			shipment_id, shipment_version, context_fingerprint, evaluation_fingerprint,
			algorithm_policy_version, routing_policy_version, execution_supported, reason_codes, created_at
		) VALUES ($1,$2,1,'ACCEPTED','CURRENT_TRIP','FEASIBLE_PLAN_FOUND',$3,1,'ctx','eval','alg','route',false,'{}',now())`,
		planID, tenant, shipment); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_activations (
			id, tenant_id, route_plan_id, plan_version, idempotency_key,
			execution_shipment_id, status, created_at
		) VALUES ($1,$2,$3,1,'pending-ok',$4,'PENDING_EXECUTION',now())`,
		uuid.New(), tenant, planID, shipment); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.route_plan_activations WHERE route_plan_id=$1`, planID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_activations (
			id, tenant_id, route_plan_id, plan_version, idempotency_key,
			execution_shipment_id, execution_id, status, created_at
		) VALUES ($1,$2,$3,1,'pending-bad',$4,$5,'PENDING_EXECUTION',now())`,
		uuid.New(), tenant, planID, shipment, uuid.New()); err == nil {
		t.Fatal("pending activation with an execution id inserted")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_activations (
			id, tenant_id, route_plan_id, plan_version, idempotency_key,
			execution_shipment_id, effective_shipment_id, status, created_at
		) VALUES ($1,$2,$3,1,'linked-missing',$4,$4,'EXECUTION_LINKED',now())`,
		uuid.New(), tenant, planID, shipment); err == nil {
		t.Fatal("execution-linked activation without execution ids inserted")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_activations (
			id, tenant_id, route_plan_id, plan_version, idempotency_key,
			execution_shipment_id, execution_id, status, created_at
		) VALUES ($1,$2,$3,1,'partial',$4,$5,'EXECUTION_LINKED',now())`,
		uuid.New(), tenant, planID, shipment, uuid.New()); err == nil {
		t.Fatal("partial execution linkage inserted")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_activations (
			id, tenant_id, route_plan_id, plan_version, idempotency_key,
			execution_shipment_id, effective_shipment_id, execution_id, execution_revision_id, status, created_at
		) VALUES ($1,$2,$3,1,'key-a',$4,$4,$5,$6,'EXECUTION_LINKED',now())`,
		uuid.New(), tenant, planID, shipment, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	otherPlan := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plans (
			id, tenant_id, version, status, planning_mode, result_status,
			shipment_id, shipment_version, context_fingerprint, evaluation_fingerprint,
			algorithm_policy_version, routing_policy_version, execution_supported, reason_codes, created_at
		) VALUES ($1,$2,1,'ACCEPTED','CURRENT_TRIP','FEASIBLE_PLAN_FOUND',$3,1,'ctx','eval','alg','route',false,'{}',now())`,
		otherPlan, tenant, shipment); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_activations (
			id, tenant_id, route_plan_id, plan_version, idempotency_key,
			execution_shipment_id, effective_shipment_id, execution_id, execution_revision_id, status, created_at
		) VALUES ($1,$2,$3,1,'key-b',$4,$4,$5,$6,'EXECUTION_LINKED',now())`,
		uuid.New(), tenant, otherPlan, shipment, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("second effective activation inserted")
	}
	_, err = pool.Exec(ctx, mustRead(t, "000086_nlo_route_plan_accept_activate_v0_4c.down.sql"))
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("down with rows: %v", err)
	}
	var still int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.route_plan_activations WHERE route_plan_id=$1`, planID).Scan(&still); err != nil || still != 1 {
		t.Fatalf("activation survived fail-closed down: %v count=%d", err, still)
	}
}
