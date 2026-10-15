//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestI2A_TX_OwnerAndDeadlineFencing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := NewPostgres(pool)

	t.Run("I2A-TX-01 wrong owner cannot PENDING to RUNNING", func(t *testing.T) {
		tenant, id, owner := insertRepoPending(t, ctx, store)
		ok, err := markRunning(ctx, store, tenant, id, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("wrong owner must not mark running")
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchPending)
		_ = owner
	})

	t.Run("I2A-TX-02 nil owner rejected", func(t *testing.T) {
		tenant, id, _ := insertRepoPending(t, ctx, store)
		ok, err := markRunning(ctx, store, tenant, id, uuid.Nil)
		if err == nil || ok {
			t.Fatalf("nil owner must fail ok=%v err=%v", ok, err)
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchPending)
	})

	t.Run("I2A-TX-03 PENDING to RUNNING before deadline", func(t *testing.T) {
		tenant, id, owner := insertRepoPending(t, ctx, store)
		ok, err := markRunning(ctx, store, tenant, id, owner)
		if err != nil || !ok {
			t.Fatalf("mark running before deadline ok=%v err=%v", ok, err)
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchRunning)
	})

	t.Run("I2A-TX-04 PENDING to RUNNING fails after deadline before claim expiry", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		// created_at = now()-21s => deadline=now()-1s, claim_expires=now()+4s
		insertTimedSearch(t, ctx, pool, tenant, id, owner, ChainSearchPending, -21*time.Second)
		ok, err := markRunning(ctx, store, tenant, id, owner)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("mark running after deadline must fail while claim grace remains")
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchPending)
	})

	t.Run("I2A-TX-05 owner terminal write after deadline before claim expiry", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		// created_at = now()-22s => deadline=now()-2s, claim_expires=now()+3s
		insertTimedSearch(t, ctx, pool, tenant, id, owner, ChainSearchRunning, -22*time.Second)
		completed := time.Now().UTC()
		ok, err := ownerPersist(ctx, store, ChainSearchRun{
			ID:             id,
			TenantID:       tenant,
			Status:         ChainSearchCompleted,
			CompletedAt:    &completed,
			SearchComplete: true,
			ReasonCounts:   map[string]int{},
		}, owner, ChainSearchRunning)
		if err != nil || !ok {
			t.Fatalf("owner terminal in grace window ok=%v err=%v", ok, err)
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchCompleted)
	})

	t.Run("I2A-TX-06 owner write fails after claim expiry", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		// created_at = now()-26s => deadline=now()-6s, claim_expires=now()-1s
		insertTimedSearch(t, ctx, pool, tenant, id, owner, ChainSearchRunning, -26*time.Second)
		completed := time.Now().UTC()
		ok, err := ownerPersist(ctx, store, ChainSearchRun{
			ID:             id,
			TenantID:       tenant,
			Status:         ChainSearchCompleted,
			CompletedAt:    &completed,
			SearchComplete: true,
			ReasonCounts:   map[string]int{},
		}, owner, ChainSearchRunning)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("owner write after claim expiry must fail")
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchRunning)
	})

	t.Run("I2A-TX-07 terminal immutable via owner primitive", func(t *testing.T) {
		tenant, id, owner := insertRepoPending(t, ctx, store)
		if ok, err := markRunning(ctx, store, tenant, id, owner); err != nil || !ok {
			t.Fatalf("setup running ok=%v err=%v", ok, err)
		}
		completed := time.Now().UTC()
		if ok, err := ownerPersist(ctx, store, ChainSearchRun{
			ID: id, TenantID: tenant, Status: ChainSearchCompleted, CompletedAt: &completed,
			SearchComplete: true, ReasonCounts: map[string]int{},
		}, owner, ChainSearchRunning); err != nil || !ok {
			t.Fatalf("complete ok=%v err=%v", ok, err)
		}
		failedCode := "CHAIN_BUDGET_EXHAUSTED"
		ok, err := ownerPersist(ctx, store, ChainSearchRun{
			ID: id, TenantID: tenant, Status: ChainSearchFailed, FailureCode: &failedCode,
			SearchComplete: false, ReasonCounts: map[string]int{},
		}, owner, ChainSearchCompleted)
		if err == nil && ok {
			t.Fatal("terminal row must not move via owner primitive")
		}
		assertStatus(t, ctx, store, tenant, id, ChainSearchCompleted)
	})
}

func insertRepoPending(t *testing.T, ctx context.Context, store *Postgres) (tenant, id, owner uuid.UUID) {
	t.Helper()
	tenant = uuid.New()
	id = uuid.New()
	owner = uuid.New()
	now := time.Now().UTC()
	one := 1
	capID := uuid.New()
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		_, err := tx.InsertChainSearchRun(ctx, ChainSearchRun{
			ID:                     id,
			TenantID:               tenant,
			Status:                 ChainSearchPending,
			RequestFingerprint:     "fp-" + id.String(),
			PolicyVersion:          "NLO-0.9C-I0/BOUNDED_TWO_LOAD",
			AlgorithmVersion:       "BOUNDED_TWO_ORDER_COMPARISON",
			CatalogFingerprint:     "catalog",
			RoutingProviderVersion: "UNKNOWN",
			OwnerToken:             owner,
			ServiceDurationSnapshot: ServiceDurationSnapshot{
				State: ServiceDurationUnknown, PickupKnown: false, DeliveryKnown: false,
			},
			HorizonStart:        now,
			HorizonEnd:          now.Add(48 * time.Hour),
			SeedSource:          ChainSeedManualAvailableCapacity,
			SeedCapacityID:      &capID,
			SeedCapacityVersion: &one,
			ReasonCounts:        map[string]int{},
		})
		return err
	}); err != nil {
		t.Fatalf("insert pending: %v", err)
	}
	return tenant, id, owner
}

func insertTimedSearch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant, id, owner uuid.UUID, status string, createdOffset time.Duration) {
	t.Helper()
	one := 1
	capID := uuid.New()
	searchComplete := status == ChainSearchCompleted
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.chain_search_runs (
			id, tenant_id, status, created_at, started_at,
			request_fingerprint, policy_version, algorithm_version, catalog_fingerprint,
			routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_snapshot_json, horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version,
			candidate_count, evaluated_pair_count, provider_call_count,
			search_complete, global_optimum, execution_supported, reason_counts_json
		) VALUES (
			$1,$2,$3,
			now() + $4::interval,
			CASE WHEN $3 = 'RUNNING' THEN now() + $4::interval ELSE NULL END,
			$5,'NLO-0.9C-I0/BOUNDED_TWO_LOAD','BOUNDED_TWO_ORDER_COMPARISON','catalog',
			'UNKNOWN',
			(now() + $4::interval) + interval '20 seconds',
			$6,
			(now() + $4::interval) + interval '25 seconds',
			'{"state":"UNKNOWN","pickup_known":false,"delivery_known":false}'::jsonb,
			now(), now() + interval '48 hours', 'MANUAL_AVAILABLE_CAPACITY',
			$7,$8,
			0,0,0,
			$9,false,false,'{}'::jsonb
		)`,
		id, tenant, status, createdOffset.String(), "fp-"+id.String(), owner, capID, one, searchComplete,
	); err != nil {
		t.Fatalf("insert timed search: %v", err)
	}
}

func markRunning(ctx context.Context, store *Postgres, tenant, id, owner uuid.UUID) (bool, error) {
	var ok bool
	err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		ok, err = tx.MarkChainSearchRunning(ctx, tenant, id, owner)
		return err
	})
	return ok, err
}

func ownerPersist(ctx context.Context, store *Postgres, run ChainSearchRun, owner uuid.UUID, expected string) (bool, error) {
	var ok bool
	err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		ok, err = tx.OwnerPersistChainSearchRun(ctx, run, owner, expected)
		return err
	})
	return ok, err
}

func assertStatus(t *testing.T, ctx context.Context, store *Postgres, tenant, id uuid.UUID, want string) {
	t.Helper()
	var got ChainSearchRun
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		got, err = tx.GetChainSearchRun(ctx, tenant, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.Status != want {
		t.Fatalf("status=%s want %s", got.Status, want)
	}
}
