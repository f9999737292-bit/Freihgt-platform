//go:build integration

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestI2A_DB_TenantIntegrity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)

	tenantA := uuid.New()
	tenantB := uuid.New()
	searchA := insertMinimalChainSearch(t, ctx, pool, tenantA, "fp-a")
	searchB := insertMinimalChainSearch(t, ctx, pool, tenantB, "fp-b")
	planA := uuid.New()
	planB := uuid.New()
	insertMinimalRoutePlan(t, ctx, pool, tenantA, planA)
	insertMinimalRoutePlan(t, ctx, pool, tenantB, planB)

	t.Run("I2A-DB-01 same-tenant candidate search FK", func(t *testing.T) {
		if err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 1, uuid.MustParse("00000000-0000-4000-8000-000000000001"), uuid.MustParse("00000000-0000-4000-8000-000000000002"), "UNFINISHED", nil, nil); err != nil {
			t.Fatalf("I2A-DB-01: %v", err)
		}
	})

	t.Run("I2A-DB-02 cross-tenant candidate search FK fails", func(t *testing.T) {
		// Use a distinct ordinal so unique(search_id, ordinal) cannot mask the FK error.
		err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantB, 4, uuid.MustParse("00000000-0000-4000-8000-000000000011"), uuid.MustParse("00000000-0000-4000-8000-000000000012"), "UNFINISHED", nil, nil)
		if err == nil || !isFKViolation(err) {
			t.Fatalf("I2A-DB-02 want FK fail got %v", err)
		}
	})

	t.Run("I2A-DB-03 same-tenant candidate route_plan FK", func(t *testing.T) {
		if err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 2, uuid.MustParse("00000000-0000-4000-8000-000000000021"), uuid.MustParse("00000000-0000-4000-8000-000000000022"), "FEASIBLE", nil, &planA); err != nil {
			t.Fatalf("I2A-DB-03: %v", err)
		}
	})

	t.Run("I2A-DB-04 cross-tenant candidate route_plan FK fails", func(t *testing.T) {
		err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 3, uuid.MustParse("00000000-0000-4000-8000-000000000031"), uuid.MustParse("00000000-0000-4000-8000-000000000032"), "FEASIBLE", nil, &planB)
		if err == nil || !isFKViolation(err) {
			t.Fatalf("I2A-DB-04 want FK fail got %v", err)
		}
	})

	t.Run("I2A-DB-05 idempotency search tenant match", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.chain_search_idempotency
				(tenant_id, idempotency_key, request_fingerprint, search_id, created_at)
			VALUES ($1,'key-ok','fp-a',$2,now())`, tenantA, searchA); err != nil {
			t.Fatalf("I2A-DB-05: %v", err)
		}
	})

	t.Run("I2A-DB-06 idempotency tenant mismatch fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.chain_search_idempotency
				(tenant_id, idempotency_key, request_fingerprint, search_id, created_at)
			VALUES ($1,'key-mismatch','fp-a',$2,now())`, tenantB, searchA)
		if err == nil || !isFKViolation(err) {
			t.Fatalf("I2A-DB-06 want FK fail got %v", err)
		}
	})

	t.Run("I2A-DB-07 idempotency fingerprint mismatch fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.chain_search_idempotency
				(tenant_id, idempotency_key, request_fingerprint, search_id, created_at)
			VALUES ($1,'key-fp','fp-wrong',$2,now())`, tenantA, searchA)
		if err == nil || !isFKViolation(err) {
			t.Fatalf("I2A-DB-07 want FK fail got %v", err)
		}
	})

	t.Run("I2A-DB-08 duplicate tenant idempotency key fails", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.chain_search_idempotency
				(tenant_id, idempotency_key, request_fingerprint, search_id, created_at)
			VALUES ($1,'key-ok','fp-a',$2,now())`, tenantA, searchA)
		if err == nil || !isUniqueViolation(err) {
			t.Fatalf("I2A-DB-08 want unique fail got %v", err)
		}
	})

	t.Run("I2A-DB-09 duplicate canonical pair fails", func(t *testing.T) {
		first := uuid.MustParse("00000000-0000-4000-8000-000000000041")
		second := uuid.MustParse("00000000-0000-4000-8000-000000000042")
		if err := insertRawCandidate(ctx, pool, uuid.New(), searchB, tenantB, 1, first, second, "UNFINISHED", nil, nil); err != nil {
			t.Fatalf("seed pair: %v", err)
		}
		err := insertRawCandidate(ctx, pool, uuid.New(), searchB, tenantB, 2, first, second, "UNFINISHED", nil, nil)
		if err == nil || !isUniqueViolation(err) {
			t.Fatalf("I2A-DB-09 want unique fail got %v", err)
		}
	})

	t.Run("I2A-DB-10 duplicate ordinal fails", func(t *testing.T) {
		err := insertRawCandidate(ctx, pool, uuid.New(), searchB, tenantB, 1,
			uuid.MustParse("00000000-0000-4000-8000-000000000051"),
			uuid.MustParse("00000000-0000-4000-8000-000000000052"), "UNFINISHED", nil, nil)
		if err == nil || !isUniqueViolation(err) {
			t.Fatalf("I2A-DB-10 want unique fail got %v", err)
		}
	})

	t.Run("I2A-DB-11 UNFINISHED with route_plan rejected", func(t *testing.T) {
		err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 4,
			uuid.MustParse("00000000-0000-4000-8000-000000000061"),
			uuid.MustParse("00000000-0000-4000-8000-000000000062"), "UNFINISHED", nil, &planA)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-11 want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-12 INFEASIBLE with route_plan rejected", func(t *testing.T) {
		err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 4,
			uuid.MustParse("00000000-0000-4000-8000-000000000071"),
			uuid.MustParse("00000000-0000-4000-8000-000000000072"), "INFEASIBLE", nil, &planA)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-12 want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-13 non-FEASIBLE with selected_order rejected", func(t *testing.T) {
		order := "L1_BEFORE_L2"
		err := insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 4,
			uuid.MustParse("00000000-0000-4000-8000-000000000081"),
			uuid.MustParse("00000000-0000-4000-8000-000000000082"), "UNFINISHED", &order, nil)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-13 unfinished want check fail got %v", err)
		}
		err = insertRawCandidate(ctx, pool, uuid.New(), searchA, tenantA, 4,
			uuid.MustParse("00000000-0000-4000-8000-000000000083"),
			uuid.MustParse("00000000-0000-4000-8000-000000000084"), "INFEASIBLE", &order, nil)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-13 infeasible want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-14 exact 20s+5s window succeeds", func(t *testing.T) {
		id := insertMinimalChainSearch(t, ctx, pool, uuid.New(), "fp-exact")
		var created, deadline, claim time.Time
		if err := pool.QueryRow(ctx, `
			SELECT created_at, deadline_at, claim_expires_at
			FROM network_optimizer.chain_search_runs WHERE id=$1`, id).Scan(&created, &deadline, &claim); err != nil {
			t.Fatal(err)
		}
		if !deadline.Equal(created.Add(20*time.Second)) || !claim.Equal(deadline.Add(5*time.Second)) {
			t.Fatalf("window created=%v deadline=%v claim=%v", created, deadline, claim)
		}
	})

	t.Run("I2A-DB-15 deadline 19s rejected", func(t *testing.T) {
		err := insertSearchWithOffsets(ctx, pool, uuid.New(), uuid.New(), 19*time.Second, 24*time.Second)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-15 want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-16 deadline 21s rejected", func(t *testing.T) {
		err := insertSearchWithOffsets(ctx, pool, uuid.New(), uuid.New(), 21*time.Second, 26*time.Second)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-16 want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-17 claim expiry deadline+4s rejected", func(t *testing.T) {
		err := insertSearchWithOffsets(ctx, pool, uuid.New(), uuid.New(), 20*time.Second, 24*time.Second)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-17 want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-18 claim expiry deadline+6s rejected", func(t *testing.T) {
		err := insertSearchWithOffsets(ctx, pool, uuid.New(), uuid.New(), 20*time.Second, 26*time.Second)
		if err == nil || !isCheckViolation(err) {
			t.Fatalf("I2A-DB-18 want check fail got %v", err)
		}
	})

	t.Run("I2A-DB-19 cross-tenant same idempotency key isolation", func(t *testing.T) {
		store := NewPostgres(pool)
		key := "shared-idem-key"
		tenantA := uuid.New()
		tenantB := uuid.New()
		tenantC := uuid.New()
		searchA := insertMinimalChainSearch(t, ctx, pool, tenantA, "fp-a-iso")
		searchB := insertMinimalChainSearch(t, ctx, pool, tenantB, "fp-b-iso")
		now := time.Now().UTC()
		if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
			if err := tx.InsertChainSearchIdempotency(ctx, ChainSearchIdempotency{
				TenantID: tenantA, IdempotencyKey: key, RequestFingerprint: "fp-a-iso", SearchID: searchA, CreatedAt: now,
			}); err != nil {
				return err
			}
			return tx.InsertChainSearchIdempotency(ctx, ChainSearchIdempotency{
				TenantID: tenantB, IdempotencyKey: key, RequestFingerprint: "fp-b-iso", SearchID: searchB, CreatedAt: now,
			})
		}); err != nil {
			t.Fatalf("I2A-DB-19 insert both: %v", err)
		}
		var gotA, gotB ChainSearchIdempotency
		if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
			var err error
			gotA, err = tx.GetChainSearchIdempotency(ctx, tenantA, key)
			if err != nil {
				return err
			}
			gotB, err = tx.GetChainSearchIdempotency(ctx, tenantB, key)
			return err
		}); err != nil {
			t.Fatalf("I2A-DB-19 get both: %v", err)
		}
		if gotA.SearchID != searchA || gotA.TenantID != tenantA {
			t.Fatalf("tenant A key must resolve only to A search got=%+v", gotA)
		}
		if gotB.SearchID != searchB || gotB.TenantID != tenantB {
			t.Fatalf("tenant B key must resolve only to B search got=%+v", gotB)
		}
		if gotA.SearchID == gotB.SearchID {
			t.Fatal("A and B must not resolve to the same search")
		}
		if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
			row, err := tx.GetChainSearchIdempotency(ctx, tenantA, key)
			if err != nil {
				return err
			}
			if row.SearchID == searchB {
				t.Fatal("tenant A scope retrieved tenant B mapping")
			}
			row, err = tx.GetChainSearchIdempotency(ctx, tenantB, key)
			if err != nil {
				return err
			}
			if row.SearchID == searchA {
				t.Fatal("tenant B scope retrieved tenant A mapping")
			}
			_, err = tx.GetChainSearchIdempotency(ctx, tenantC, key)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("third tenant want ErrNotFound got %v", err)
			}
			return nil
		}); err != nil {
			t.Fatalf("I2A-DB-19 isolation checks: %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.chain_search_idempotency
			WHERE idempotency_key=$1 AND tenant_id IN ($2,$3)`, key, tenantA, tenantB).Scan(&count); err != nil || count != 2 {
			t.Fatalf("both rows must coexist count=%d err=%v", count, err)
		}
	})
}

func insertMinimalChainSearch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant uuid.UUID, fingerprint string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	one := 1
	if _, err := pool.Exec(ctx, `
		WITH claim_clock AS (SELECT clock_timestamp() AS ts)
		INSERT INTO network_optimizer.chain_search_runs (
			id, tenant_id, status, created_at, request_fingerprint, policy_version, algorithm_version,
			catalog_fingerprint, routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_snapshot_json, horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version,
			candidate_count, evaluated_pair_count, provider_call_count,
			search_complete, global_optimum, execution_supported, reason_counts_json
		)
		SELECT
			$1,$2,'PENDING', claim_clock.ts, $3,'NLO-0.9C-I0/BOUNDED_TWO_LOAD','BOUNDED_TWO_ORDER_COMPARISON',
			'catalog','UNKNOWN', claim_clock.ts + interval '20 seconds', $4, claim_clock.ts + interval '25 seconds',
			$5::jsonb, claim_clock.ts, claim_clock.ts + interval '48 hours','MANUAL_AVAILABLE_CAPACITY',
			$6,$7,
			0,0,0,
			false,false,false,'{}'::jsonb
		FROM claim_clock`,
		id, tenant, fingerprint, uuid.New(),
		`{"state":"UNKNOWN","pickup_known":false,"delivery_known":false}`,
		uuid.New(), &one,
	); err != nil {
		t.Fatalf("insert chain search: %v", err)
	}
	return id
}

func insertSearchWithOffsets(ctx context.Context, pool *pgxpool.Pool, tenant, id uuid.UUID, deadlineOffset, claimOffset time.Duration) error {
	one := 1
	_, err := pool.Exec(ctx, `
		WITH claim_clock AS (SELECT clock_timestamp() AS ts)
		INSERT INTO network_optimizer.chain_search_runs (
			id, tenant_id, status, created_at, request_fingerprint, policy_version, algorithm_version,
			catalog_fingerprint, routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_snapshot_json, horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version,
			candidate_count, evaluated_pair_count, provider_call_count,
			search_complete, global_optimum, execution_supported, reason_counts_json
		)
		SELECT
			$1,$2,'PENDING', claim_clock.ts, $3,'NLO-0.9C-I0/BOUNDED_TWO_LOAD','BOUNDED_TWO_ORDER_COMPARISON',
			'catalog','UNKNOWN', claim_clock.ts + $4::interval, $5, claim_clock.ts + $6::interval,
			'{"state":"UNKNOWN","pickup_known":false,"delivery_known":false}'::jsonb,
			claim_clock.ts, claim_clock.ts + interval '48 hours','MANUAL_AVAILABLE_CAPACITY',
			$7,$8,
			0,0,0,false,false,false,'{}'::jsonb
		FROM claim_clock`,
		id, tenant, "fp-"+id.String(), deadlineOffset.String(), uuid.New(), claimOffset.String(), uuid.New(), one,
	)
	return err
}

func insertRawCandidate(ctx context.Context, pool *pgxpool.Pool, candidateID, searchID, tenant uuid.UUID, ordinal int, first, second uuid.UUID, feasibility string, selectedOrder *string, routePlan *uuid.UUID) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.chain_candidates (
			candidate_id, search_id, tenant_id, ordinal,
			first_load_id, first_load_version, second_load_id, second_load_version,
			feasibility_status, selected_order, omitted_components, reason_codes,
			route_plan_id, candidate_fingerprint, created_at
		) VALUES ($1,$2,$3,$4,$5,1,$6,1,$7,$8,'{}','{}',$9,'fp',now())`,
		candidateID, searchID, tenant, ordinal, first, second, feasibility, selectedOrder, routePlan)
	return err
}

func isFKViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "foreign key") || strings.Contains(msg, "violates foreign key") || strings.Contains(msg, "SQLSTATE 23503")
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}
