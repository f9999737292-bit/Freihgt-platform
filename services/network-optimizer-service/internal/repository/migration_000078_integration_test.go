//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestMigration000078UpDownUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	if _, err := pool.Exec(ctx, mustRead(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")); err != nil {
		t.Fatalf("000077 up: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000078_bno_geography_routing_foundation_v0_1c0.up.sql")); err != nil {
		t.Fatalf("up: %v", err)
	}
	assertBNO078Present(t, ctx, pool, true)
	assertBNO077Present(t, ctx, pool, true)
	assertCapacityPolicyTenantBoundary(t, ctx, pool)

	if _, err := pool.Exec(ctx, mustRead(t, "000078_bno_geography_routing_foundation_v0_1c0.down.sql")); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertBNO078Present(t, ctx, pool, false)
	assertBNO077Present(t, ctx, pool, true)

	if _, err := pool.Exec(ctx, mustRead(t, "000078_bno_geography_routing_foundation_v0_1c0.up.sql")); err != nil {
		t.Fatalf("up again: %v", err)
	}
	assertBNO078Present(t, ctx, pool, true)
	assertBNO077Present(t, ctx, pool, true)
}

func assertBNO078Present(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want bool) {
	t.Helper()
	checks := []string{
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='capacities' AND column_name='location_id')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='capacities' AND column_name='country_code')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='capacities' AND column_name='region')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='capacities' AND column_name='city')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='load_opportunities' AND column_name='pickup_country_code')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='network_optimizer' AND table_name='load_opportunities' AND column_name='delivery_city')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='carrier_search_policies')`,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='capacity_search_policies')`,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='network_optimizer' AND indexname='capacities_id_owner_uidx')`,
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

func assertCapacityPolicyTenantBoundary(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	store := NewPostgres(pool)
	owner := uuid.New()
	other := uuid.New()
	capacityID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (
			id, owner_tenant_id, location_label, available_from, available_until,
			source, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'', now(), now() + interval '2 hours', 'MANUAL', 'PRIVATE', 'AVAILABLE', 1, now(), now())`,
		capacityID, owner); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCapacityPolicy(ctx, other, capacityID, domain.NextLoadSearchPolicy{}); err != ErrNotFound {
		t.Fatalf("BNO201 postgres takeover %v", err)
	}
	if err := store.UpsertCapacityPolicy(ctx, owner, capacityID, domain.NextLoadSearchPolicy{}); err != nil {
		t.Fatal(err)
	}
	var storedOwner uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT owner_tenant_id FROM network_optimizer.capacity_search_policies WHERE capacity_id=$1`, capacityID).Scan(&storedOwner); err != nil || storedOwner != owner {
		t.Fatalf("policy owner %s %v", storedOwner, err)
	}
	km := 25.0
	if err := store.UpsertCapacityPolicy(ctx, other, capacityID, domain.NextLoadSearchPolicy{MaxDeadheadKm: &km}); err != ErrNotFound {
		t.Fatalf("BNO201 postgres rewrite %v", err)
	}
	var storedKm *float64
	if err := pool.QueryRow(ctx, `SELECT max_deadhead_km FROM network_optimizer.capacity_search_policies WHERE capacity_id=$1`, capacityID).Scan(&storedKm); err != nil || storedKm != nil {
		t.Fatalf("policy rewritten %+v %v", storedKm, err)
	}
}
