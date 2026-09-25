//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	assertBNO251To253TenantIntegrity(t, ctx, pool)
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

func assertBNO251To253TenantIntegrity(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	carrier := uuid.New()
	other := uuid.New()
	shipper := uuid.New()
	owned := uuid.New()
	sibling := uuid.New()
	foreign := uuid.New()
	loadID := uuid.New()
	insertCapacity := func(id, tenant uuid.UUID) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.capacities (
				id, owner_tenant_id, location_label, available_from, available_until,
				source, visibility_scope, status, version, created_at, updated_at
			) VALUES ($1,$2,'', now(), now() + interval '2 hours', 'MANUAL', 'PRIVATE', 'AVAILABLE', 1, now(), now())`,
			id, tenant); err != nil {
			t.Fatal(err)
		}
	}
	insertCapacity(owned, carrier)
	insertCapacity(sibling, carrier)
	insertCapacity(foreign, other)
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.load_opportunities (
			id, owner_tenant_id, source_type, source_id, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'TRANSPORT_ORDER',$3,'MARKETPLACE','PUBLISHED',1,now(),now())`,
		loadID, shipper, uuid.New()); err != nil {
		t.Fatal(err)
	}

	t.Run("BNO251_SEARCH_RUN_FOREIGN_CAPACITY_DB_REJECT", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.next_load_search_runs (
				id, tenant_id, capacity_id, capacity_version, effective_policy_fingerprint,
				started_at, completed_at, routing_provider, status
			) VALUES ($1,$2,$3,1,'fp', now(), now(), 'UNSPECIFIED', 'COMPLETED')`,
			uuid.New(), other, owned)
		if !fkViolation(err) {
			t.Fatalf("foreign capacity %v", err)
		}
	})

	runID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.next_load_search_runs (
			id, tenant_id, capacity_id, capacity_version, effective_policy_fingerprint,
			started_at, completed_at, routing_provider, status
		) VALUES ($1,$2,$3,1,'fp', now(), now(), 'UNSPECIFIED', 'COMPLETED')`,
		runID, carrier, owned); err != nil {
		t.Fatal(err)
	}

	t.Run("BNO252_CANDIDATE_WRONG_RUN_CAPACITY_DB_REJECT", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.match_candidates (
				id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
				eligibility, compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at
			) VALUES ($1,$2,$3,$4,$5,1,'ELIGIBLE','COMPATIBLE','fp','fp', now())`,
			uuid.New(), runID, carrier, sibling, loadID)
		if !fkViolation(err) {
			t.Fatalf("wrong capacity %v", err)
		}
	})

	t.Run("BNO253_VALID_CROSS_TENANT_VISIBLE_LOAD_REFERENCE_ALLOWED", func(t *testing.T) {
		candidateID := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.match_candidates (
				id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
				eligibility, compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at
			) VALUES ($1,$2,$3,$4,$5,1,'ELIGIBLE','COMPATIBLE','fp','fp', now())`,
			candidateID, runID, carrier, owned, loadID); err != nil {
			t.Fatal(err)
		}
		var loadOwner, runTenant uuid.UUID
		if err := pool.QueryRow(ctx, `
			SELECT l.owner_tenant_id, r.tenant_id
			FROM network_optimizer.match_candidates c
			JOIN network_optimizer.load_opportunities l ON l.id = c.load_opportunity_id
			JOIN network_optimizer.next_load_search_runs r ON r.id = c.search_run_id
			WHERE c.id = $1`, candidateID).Scan(&loadOwner, &runTenant); err != nil {
			t.Fatal(err)
		}
		if loadOwner == runTenant || loadOwner != shipper || runTenant != carrier {
			t.Fatalf("load owner %s run tenant %s", loadOwner, runTenant)
		}
	})
}

func fkViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
