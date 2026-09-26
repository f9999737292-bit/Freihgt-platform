//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestNLO03B_089_099_PostgresPoolAndRunIntegrity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startConsolidationPostgres(t, ctx)
	for _, name := range []string{
		"000001_create_schemas.up.sql",
		"000003_create_transport_tables.up.sql",
		"000075_bno_capacity_marketplace_foundation_v0_1a.up.sql",
		"000076_bno_predictive_capacity_v0_1b.up.sql",
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
		"000081_nlo_pairwise_consolidation_v0_3b.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	store := repository.NewPostgres(pool)
	carrier := uuid.New()
	shipper := uuid.New()
	company := uuid.New()
	otherCompany := uuid.New()
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	originA, originB := uuid.New(), uuid.New()
	destA, destB := uuid.New(), uuid.New()
	if originB.String() < originA.String() {
		originA, originB = originB, originA
	}
	payload := 1000.0
	capA := domain.Capacity{
		ID: uuid.New(), OwnerTenantID: carrier, LocationLabel: "yard", AvailableFrom: now, AvailableUntil: now.Add(2 * time.Hour),
		Source: domain.SourceManual, VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
		PayloadRemainingKg: &payload, CreatedAt: now, UpdatedAt: now,
	}
	capB := capA
	capB.ID = uuid.New()
	if err := store.Within(ctx, func(tx repository.Tx) error {
		if err := tx.InsertCapacity(ctx, capA); err != nil {
			return err
		}
		return tx.InsertCapacity(ctx, capB)
	}); err != nil {
		t.Fatal(err)
	}
	insert := func(visibility string, invited []uuid.UUID, general, cross bool, origin, dest *uuid.UUID) uuid.UUID {
		t.Helper()
		load := domain.LoadOpportunity{
			ID: uuid.New(), OwnerTenantID: shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
			Pickup: domain.Place{LocationID: origin, City: "A", CountryCode: "RU"}, Delivery: domain.Place{LocationID: dest, City: "B", CountryCode: "RU"},
			VisibilityScope: visibility, InvitedCarrierCompanyIDs: invited, Status: domain.LoadPublished, Version: 1,
			ConsolidationAllowed: general, CrossShipperConsolidationAllowed: cross, CreatedAt: now, UpdatedAt: now,
		}
		if err := store.Within(ctx, func(tx repository.Tx) error { return tx.InsertLoad(ctx, load) }); err != nil {
			t.Fatal(err)
		}
		return load.ID
	}
	marketplace := insert(domain.VisMarketplace, nil, true, false, &originA, &destA)
	anonymized := insert(domain.VisAnonymized, nil, false, true, &originB, &destB)
	invited := insert(domain.VisInvited, []uuid.UUID{company}, true, false, &originA, &destB)
	insert(domain.VisInvited, []uuid.UUID{otherCompany}, true, false, &originB, &destA)
	insert(domain.VisPrivate, nil, true, true, &originA, &destA)
	insert(domain.VisNetworkOnly, nil, true, true, &originA, &destA)
	insert(domain.VisMarketplace, nil, false, false, &originA, &destA)
	unproven := insert(domain.VisMarketplace, nil, true, false, nil, nil)

	t.Run("NLO03B_089_DEDICATED_CONSOLIDATION_POOL_POSTGRES", func(t *testing.T) {
		var rows []domain.LoadOpportunity
		started := time.Now()
		err := store.Within(ctx, func(tx repository.Tx) error {
			listed, err := tx.ListPublicConsolidationPool(ctx, carrier, &company)
			rows = listed
			return err
		})
		elapsed := time.Since(started)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("POSTGRES_POOL_QUERY %s rows %d", elapsed, len(rows))
		got := map[uuid.UUID]struct{}{}
		var previous string
		for _, row := range rows {
			got[row.ID] = struct{}{}
			key := consolidationLocationKey(row)
			if previous > key {
				t.Fatalf("order %s then %s", previous, key)
			}
			previous = key
		}
		for _, id := range []uuid.UUID{marketplace, anonymized, invited, unproven} {
			if _, ok := got[id]; !ok {
				t.Fatalf("missing %s", id)
			}
		}
		if len(rows) != 4 {
			t.Fatalf("pool %d", len(rows))
		}
		var plan string
		if err := pool.QueryRow(ctx, `
			EXPLAIN SELECT id FROM network_optimizer.load_opportunities
			WHERE status='PUBLISHED'
			  AND owner_tenant_id <> $1
			  AND (consolidation_allowed OR cross_shipper_consolidation_allowed)
			  AND (
				visibility_scope IN ('MARKETPLACE', 'ANONYMIZED_MARKETPLACE')
				OR (visibility_scope='INVITED_CARRIERS' AND $2::uuid IS NOT NULL AND $2 = ANY(invited_carrier_company_ids))
			  )
			ORDER BY pickup_location_id, delivery_location_id, id`, carrier, company).Scan(&plan); err != nil {
			t.Fatal(err)
		}
		t.Logf("POSTGRES_POOL_QUERY_PLAN %s", plan)
		svc := New(store, nil)
		result, err := svc.SearchConsolidation(ctx, Actor{TenantID: carrier, UserID: uuid.New(), CompanyID: &company}, ConsolidationCommand{
			CapacityID: capA.ID, Pattern: PatternSameOriginDestination,
		})
		if err != nil {
			t.Fatal(err)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(result.Body, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.PoolLoadCount != len(rows) {
			t.Fatalf("search pool %d dedicated query %d", doc.PoolLoadCount, len(rows))
		}
	})

	t.Run("NLO03B_093_CANDIDATE_CAPACITY_MUST_MATCH_RUN", func(t *testing.T) {
		runID := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at, status,
				pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,$3,1,'SAME_ORIGIN_SAME_DESTINATION', now(), now(), 'COMPLETED', 0, 0, now())`,
			runID, carrier, capA.ID); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_candidates (
				id, search_run_id, tenant_id, capacity_id, pattern, status, execution_supported,
				compatibility_status, compatibility_fingerprint, candidate_fingerprint, placement_check, created_at
			) VALUES ($1,$2,$3,$4,'SAME_ORIGIN_SAME_DESTINATION','INDETERMINATE', false, 'INDETERMINATE', 'fp', $5, 'NOT_EVALUATED', now())`,
			uuid.New(), runID, carrier, capB.ID, "capacity-mismatch-"+uuid.NewString())
		if !consolidationFK(err) {
			t.Fatalf("expected capacity mismatch reject, got %v", err)
		}
	})

	t.Run("NLO03B_094_CANDIDATE_PATTERN_MUST_MATCH_RUN", func(t *testing.T) {
		runID := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at, status,
				pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,$3,1,'SAME_ORIGIN_SAME_DESTINATION', now(), now(), 'COMPLETED', 0, 0, now())`,
			runID, carrier, capA.ID); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_candidates (
				id, search_run_id, tenant_id, capacity_id, pattern, status, execution_supported,
				compatibility_status, compatibility_fingerprint, candidate_fingerprint, placement_check, created_at
			) VALUES ($1,$2,$3,$4,'CURRENT_TRIP_FILL','INDETERMINATE', false, 'INDETERMINATE', 'fp', $5, 'NOT_EVALUATED', now())`,
			uuid.New(), runID, carrier, capA.ID, "pattern-mismatch-"+uuid.NewString())
		if !consolidationFK(err) {
			t.Fatalf("expected pattern mismatch reject, got %v", err)
		}
	})
}

func consolidationLocationKey(load domain.LoadOpportunity) string {
	origin, dest := "", ""
	if load.Pickup.LocationID != nil {
		origin = load.Pickup.LocationID.String()
	}
	if load.Delivery.LocationID != nil {
		dest = load.Delivery.LocationID.String()
	}
	return origin + "|" + dest + "|" + load.ID.String()
}

func consolidationFK(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func startConsolidationPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
		DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("bno_test").
		Username("freight").Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/bno_test?sslmode=disable", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
