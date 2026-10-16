//go:build integration

package chainorch_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestI2B_AtomicClaimRollback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	orch := &chainorch.Orchestrator{
		Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
	}

	t.Run("I2B-ATOMIC-01 second-step idempotency failure rolls back search insert", func(t *testing.T) {
		tenant := uuid.New()
		key := "atomic-01-winner"
		winner, err := orch.Claim(ctx, i2bClaim(tenant, key, i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		losingID := uuid.New()
		now := time.Now().UTC()
		one := 1
		capID := uuid.New()
		// Insert a NEW search then fail idempotency via fingerprint FK mismatch.
		err = store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			inserted, err := tx.InsertChainSearchRun(ctx, repository.ChainSearchRun{
				ID: losingID, TenantID: tenant, RequestFingerprint: "fp-loser-unique",
				PolicyVersion: chain.PolicyVersion, AlgorithmVersion: chainorch.AlgorithmVersion,
				CatalogFingerprint: "c", RoutingProviderVersion: "UNKNOWN",
				ServiceDurationSnapshot: repository.ServiceDurationSnapshot{State: repository.ServiceDurationUnknown},
				HorizonStart:            now, HorizonEnd: now.Add(48 * time.Hour),
				SeedSource: repository.ChainSeedManualAvailableCapacity, SeedCapacityID: &capID, SeedCapacityVersion: &one,
				ReasonCounts: map[string]int{},
			})
			if err != nil {
				return err
			}
			// Second step: idempotency row with different fingerprint than search -> FK fail.
			return tx.InsertChainSearchIdempotency(ctx, repository.ChainSearchIdempotency{
				TenantID: tenant, IdempotencyKey: "atomic-01-orphan-key",
				RequestFingerprint: "fp-does-not-match-search",
				SearchID:           inserted.ID,
				CreatedAt:          inserted.CreatedAt,
			})
		})
		if err == nil {
			t.Fatal("expected second-step failure")
		}
		// Losing search must not exist after rollback.
		err = store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			_, getErr := tx.GetChainSearchRun(ctx, tenant, losingID)
			if !errors.Is(getErr, repository.ErrNotFound) {
				return getErr
			}
			_, getErr = tx.GetChainSearchIdempotency(ctx, tenant, "atomic-01-orphan-key")
			if !errors.Is(getErr, repository.ErrNotFound) {
				return getErr
			}
			return nil
		})
		if err != nil {
			t.Fatalf("orphan rows remain: %v", err)
		}
		// Winner mapping unchanged.
		var mapping repository.ChainSearchIdempotency
		if err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			var getErr error
			mapping, getErr = tx.GetChainSearchIdempotency(ctx, tenant, key)
			return getErr
		}); err != nil {
			t.Fatal(err)
		}
		if mapping.SearchID != winner.Search.ID {
			t.Fatalf("winner mapping changed got=%s want=%s", mapping.SearchID, winner.Search.ID)
		}
		var loserCount int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.chain_search_runs
			WHERE tenant_id=$1 AND id=$2`, tenant, losingID).Scan(&loserCount); err != nil || loserCount != 0 {
			t.Fatalf("losing search_id still present count=%d err=%v", loserCount, err)
		}
	})

	t.Run("I2B-ATOMIC-02 concurrent same-key leaves exactly one run and one mapping", func(t *testing.T) {
		tenant := uuid.New()
		key := "atomic-02"
		fp := i2bFP(tenant)
		var created int32
		var wg sync.WaitGroup
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
				if err != nil && !errors.Is(err, chainorch.ErrInProgress) {
					t.Errorf("claim err=%v", err)
					return
				}
				if res.Outcome == chainorch.OutcomeCreated {
					atomic.AddInt32(&created, 1)
				}
			}()
		}
		wg.Wait()
		if created != 1 {
			t.Fatalf("created=%d want 1", created)
		}
		var runCount, mapCount, orphanCount int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.chain_search_runs WHERE tenant_id=$1`, tenant).Scan(&runCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.chain_search_idempotency
			WHERE tenant_id=$1 AND idempotency_key=$2`, tenant, key).Scan(&mapCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.chain_search_runs r
			WHERE r.tenant_id=$1
			  AND NOT EXISTS (
			    SELECT 1 FROM network_optimizer.chain_search_idempotency i
			    WHERE i.tenant_id=r.tenant_id AND i.search_id=r.id
			  )`, tenant).Scan(&orphanCount); err != nil {
			t.Fatal(err)
		}
		if runCount != 1 || mapCount != 1 || orphanCount != 0 {
			t.Fatalf("runs=%d maps=%d orphans=%d want 1/1/0", runCount, mapCount, orphanCount)
		}
	})

	t.Run("I2B-02 strengthened claim rollback leaves neither row", func(t *testing.T) {
		// Alias coverage retained via ATOMIC-01; keep original label for gate report.
		tenant := uuid.New()
		losingID := uuid.New()
		now := time.Now().UTC()
		one := 1
		capID := uuid.New()
		err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			inserted, err := tx.InsertChainSearchRun(ctx, repository.ChainSearchRun{
				ID: losingID, TenantID: tenant, RequestFingerprint: "fp-i2b02",
				PolicyVersion: chain.PolicyVersion, AlgorithmVersion: chainorch.AlgorithmVersion,
				CatalogFingerprint: "c", RoutingProviderVersion: "UNKNOWN",
				ServiceDurationSnapshot: repository.ServiceDurationSnapshot{State: repository.ServiceDurationUnknown},
				HorizonStart:            now, HorizonEnd: now.Add(48 * time.Hour),
				SeedSource: repository.ChainSeedManualAvailableCapacity, SeedCapacityID: &capID, SeedCapacityVersion: &one,
				ReasonCounts: map[string]int{},
			})
			if err != nil {
				return err
			}
			return tx.InsertChainSearchIdempotency(ctx, repository.ChainSearchIdempotency{
				TenantID: tenant, IdempotencyKey: "i2b-02-key",
				RequestFingerprint: "fp-mismatch", SearchID: inserted.ID, CreatedAt: inserted.CreatedAt,
			})
		})
		if err == nil {
			t.Fatal("expected rollback")
		}
		var runs, maps int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.chain_search_runs WHERE tenant_id=$1`, tenant).Scan(&runs)
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.chain_search_idempotency WHERE tenant_id=$1`, tenant).Scan(&maps)
		if runs != 0 || maps != 0 {
			t.Fatalf("orphan after rollback runs=%d maps=%d", runs, maps)
		}
	})
}
