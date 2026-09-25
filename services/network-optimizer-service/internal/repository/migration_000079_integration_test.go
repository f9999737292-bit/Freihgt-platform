//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigration000079UpDownUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")); err != nil {
		t.Fatalf("000077 up: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000078_bno_geography_routing_foundation_v0_1c0.up.sql")); err != nil {
		t.Fatalf("000078 up: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000079_bno_next_load_candidate_search_v0_1c1.up.sql")); err != nil {
		t.Fatalf("up: %v", err)
	}
	assertBNO079Present(t, ctx, pool, true)
	assertBNO078Present(t, ctx, pool, true)

	if _, err := pool.Exec(ctx, mustRead(t, "000079_bno_next_load_candidate_search_v0_1c1.down.sql")); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertBNO079Present(t, ctx, pool, false)
	assertBNO078Present(t, ctx, pool, true)

	if _, err := pool.Exec(ctx, mustRead(t, "000079_bno_next_load_candidate_search_v0_1c1.up.sql")); err != nil {
		t.Fatalf("up again: %v", err)
	}
	assertBNO079Present(t, ctx, pool, true)
}

func assertBNO079Present(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want bool) {
	t.Helper()
	checks := []string{
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='carrier_search_policies' AND column_name='radius_km')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='capacity_search_policies' AND column_name='radius_km')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='next_load_search_runs')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='match_candidates')`,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='network_optimizer' AND indexname='next_load_search_runs_tenant_capacity_idx')`,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='network_optimizer' AND indexname='match_candidates_run_idx')`,
	}
	for _, query := range checks {
		var exists bool
		if err := pool.QueryRow(ctx, query).Scan(&exists); err != nil || exists != want {
			t.Fatalf("%s exists=%v want %v err=%v", query, exists, want, err)
		}
	}
	var locations bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='locations')`).Scan(&locations); err != nil || locations {
		t.Fatalf("second location master exists=%v err=%v", locations, err)
	}
}
