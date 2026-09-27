//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO03D_Migration000083(t *testing.T) {
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
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := uuid.New()
	insertFill := func() error {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,NULL,NULL,'CURRENT_TRIP_FILL',now(),now(),'COMPLETED',0,0,now())`,
			uuid.New(), tenant)
		return err
	}
	t.Run("NLO03D_083_REJECTED_BEFORE_MIGRATION", func(t *testing.T) {
		if err := insertFill(); err == nil {
			t.Fatal("CURRENT_TRIP_FILL accepted before 000083")
		}
	})
	t.Run("NLO03D_083_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000083_nlo_current_trip_fill_v0_3d.up.sql")); err != nil {
			t.Fatal(err)
		}
		if err := insertFill(); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,NULL,0,'CURRENT_TRIP_FILL',now(),now(),'COMPLETED',0,0,now())`, uuid.New(), tenant); err == nil {
			t.Fatal("current-trip capacity_version 0 accepted")
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,NULL,NULL,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',0,0,now())`, uuid.New(), tenant); err == nil {
			t.Fatal("pairwise null capacity accepted")
		}
		capacity := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.capacities (
				id, owner_tenant_id, location_label, available_from, available_until, source,
				visibility_scope, status, version, created_at, updated_at
			) VALUES ($1,$2,'Yard',now(),now() + interval '1 hour','MANUAL','PRIVATE','AVAILABLE',3,now(),now())`, capacity, tenant); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,$3,3,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',0,0,now())`, uuid.New(), tenant, capacity); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("NLO03D_083_DOWN", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000083_nlo_current_trip_fill_v0_3d.down.sql")); err != nil {
			t.Fatal(err)
		}
		if err := insertFill(); err == nil {
			t.Fatal("CURRENT_TRIP_FILL accepted after down")
		}
	})
	t.Run("NLO03D_083_UP_DOWN_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000083_nlo_current_trip_fill_v0_3d.up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000083_nlo_current_trip_fill_v0_3d.down.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000083_nlo_current_trip_fill_v0_3d.up.sql")); err != nil {
			t.Fatal(err)
		}
		if err := insertFill(); err != nil {
			t.Fatal(err)
		}
	})
}
