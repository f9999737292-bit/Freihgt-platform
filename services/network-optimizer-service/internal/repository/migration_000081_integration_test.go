//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO03B_077_081_Migration000081(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	for _, name := range []string{
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	shipper := uuid.New()
	existing := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.load_opportunities (
			id, owner_tenant_id, source_type, source_id, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'TRANSPORT_ORDER',$3,'MARKETPLACE','PUBLISHED',1,now(),now())`,
		existing, shipper, uuid.New()); err != nil {
		t.Fatal(err)
	}

	t.Run("NLO03B_077_MIGRATION_000081_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000081_nlo_pairwise_consolidation_v0_3b.up.sql")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("NLO03B_080_EXISTING_LOADS_DEFAULT_OPT_OUT", func(t *testing.T) {
		var general, cross bool
		if err := pool.QueryRow(ctx, `SELECT consolidation_allowed, cross_shipper_consolidation_allowed FROM network_optimizer.load_opportunities WHERE id=$1`, existing).Scan(&general, &cross); err != nil || general || cross {
			t.Fatalf("general %v cross %v err %v", general, cross, err)
		}
	})
	t.Run("NLO03B_081_EXISTING_BNO_0_1C_DATA_COMPATIBLE", func(t *testing.T) {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM network_optimizer.load_opportunities WHERE id=$1`, existing).Scan(&status); err != nil || status != "PUBLISHED" {
			t.Fatalf("%s %v", status, err)
		}
		var profiles int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.score_profiles WHERE status='ACTIVE'`).Scan(&profiles); err != nil || profiles != 6 {
			t.Fatalf("profiles %d %v", profiles, err)
		}
	})
	t.Run("NLO03B_078_MIGRATION_000081_DOWN", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000081_nlo_pairwise_consolidation_v0_3b.down.sql")); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='load_opportunities' AND column_name='consolidation_allowed')`).Scan(&exists); err != nil || exists {
			t.Fatalf("column remains %v %v", exists, err)
		}
	})
	t.Run("NLO03B_079_MIGRATION_000081_UP_DOWN_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000081_nlo_pairwise_consolidation_v0_3b.up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000081_nlo_pairwise_consolidation_v0_3b.down.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000081_nlo_pairwise_consolidation_v0_3b.up.sql")); err != nil {
			t.Fatal(err)
		}
		var general bool
		if err := pool.QueryRow(ctx, `SELECT consolidation_allowed FROM network_optimizer.load_opportunities WHERE id=$1`, existing).Scan(&general); err != nil || general {
			t.Fatalf("reapplied %v %v", general, err)
		}
		var indexed bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='network_optimizer' AND indexname='load_opportunities_published_opt_in_od_idx')`).Scan(&indexed); err != nil || !indexed {
			t.Fatalf("index %v %v", indexed, err)
		}
	})
}
