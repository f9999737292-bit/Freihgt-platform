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

func TestI2B_ClaimRecoveryOrchestration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	orch := &chainorch.Orchestrator{
		Store:            store,
		DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
	}

	t.Run("I2B-01 atomic search + idempotency commit", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-01"
		res, err := orch.Claim(ctx, i2bClaim(tenant, key, i2bFP(tenant)))
		if err != nil || res.Outcome != chainorch.OutcomeCreated {
			t.Fatalf("claim outcome=%s err=%v", res.Outcome, err)
		}
		var mapping repository.ChainSearchIdempotency
		if err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			var getErr error
			mapping, getErr = tx.GetChainSearchIdempotency(ctx, tenant, key)
			return getErr
		}); err != nil {
			t.Fatal(err)
		}
		if mapping.SearchID != res.Search.ID || mapping.RequestFingerprint != res.Search.RequestFingerprint {
			t.Fatalf("idempotency mapping mismatch %+v search=%s", mapping, res.Search.ID)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchPending || got.OwnerToken == uuid.Nil {
			t.Fatalf("pending claim status=%s owner=%s", got.Status, got.OwnerToken)
		}
	})

	t.Run("I2B-02 claim rollback leaves neither row", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		now := time.Now().UTC()
		one := 1
		capID := uuid.New()
		err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			if _, err := tx.InsertChainSearchRun(ctx, repository.ChainSearchRun{
				ID: id, TenantID: tenant, RequestFingerprint: "fp-rollback",
				PolicyVersion: chain.PolicyVersion, AlgorithmVersion: chainorch.AlgorithmVersion,
				CatalogFingerprint: "c", RoutingProviderVersion: "UNKNOWN",
				ServiceDurationSnapshot: repository.ServiceDurationSnapshot{State: repository.ServiceDurationUnknown},
				HorizonStart:            now, HorizonEnd: now.Add(48 * time.Hour),
				SeedSource: repository.ChainSeedManualAvailableCapacity, SeedCapacityID: &capID, SeedCapacityVersion: &one,
				ReasonCounts: map[string]int{},
			}); err != nil {
				return err
			}
			return errors.New("forced rollback before idempotency")
		})
		if err == nil {
			t.Fatal("expected rollback error")
		}
		err = store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			_, getErr := tx.GetChainSearchRun(ctx, tenant, id)
			if !errors.Is(getErr, repository.ErrNotFound) {
				return getErr
			}
			_, getErr = tx.GetChainSearchIdempotency(ctx, tenant, "unused-key")
			if !errors.Is(getErr, repository.ErrNotFound) {
				return getErr
			}
			return nil
		})
		if err != nil {
			t.Fatalf("orphan rows remain: %v", err)
		}
	})

	t.Run("I2B-03 concurrent same-key creates one search", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-03"
		fp := i2bFP(tenant)
		var created int32
		var ids sync.Map
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
				if err != nil && !errors.Is(err, chainorch.ErrInProgress) {
					t.Errorf("claim err=%v", err)
					return
				}
				ids.Store(res.Search.ID, true)
				if res.Outcome == chainorch.OutcomeCreated {
					atomic.AddInt32(&created, 1)
				}
			}()
		}
		wg.Wait()
		if created != 1 {
			t.Fatalf("created=%d want 1", created)
		}
		count := 0
		ids.Range(func(_, _ any) bool { count++; return true })
		if count != 1 {
			t.Fatalf("distinct search ids=%d want 1", count)
		}
	})

	t.Run("I2B-04 same key/same fingerprint terminal replay", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-04"
		fp := i2bFP(tenant)
		first, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil {
			t.Fatal(err)
		}
		run, err := orch.BeginRunning(ctx, tenant, first.Search.ID, first.OwnerToken)
		if err != nil {
			t.Fatal(err)
		}
		completed, ok, err := orch.CompleteSearch(ctx, first.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{
				ID: run.ID, TenantID: tenant, Status: repository.ChainSearchCompleted,
				SearchComplete: true, ReasonCounts: map[string]int{},
			},
			Candidates: []repository.ChainCandidate{i2bCandidate(tenant, run.ID, 1)},
		})
		if err != nil || !ok || completed.Status != repository.ChainSearchCompleted {
			t.Fatalf("complete ok=%v status=%s err=%v", ok, completed.Status, err)
		}
		replay, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil || replay.Outcome != chainorch.OutcomeReplay || replay.Search.ID != first.Search.ID {
			t.Fatalf("replay outcome=%s id=%s err=%v", replay.Outcome, replay.Search.ID, err)
		}
	})

	t.Run("I2B-05 same key/different fingerprint conflict", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-05"
		fp1 := i2bFP(tenant)
		if _, err := orch.Claim(ctx, i2bClaim(tenant, key, fp1)); err != nil {
			t.Fatal(err)
		}
		fp2 := fp1
		fp2.CatalogFingerprint = "other-catalog"
		res, err := orch.Claim(ctx, i2bClaim(tenant, key, fp2))
		if !errors.Is(err, chainorch.ErrFingerprintConflict) || res.Outcome != chainorch.OutcomeFingerprintConflict {
			t.Fatalf("want fingerprint conflict got outcome=%s err=%v", res.Outcome, err)
		}
	})

	t.Run("I2B-06 same key/in-flight returns in-progress", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-06"
		fp := i2bFP(tenant)
		first, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil {
			t.Fatal(err)
		}
		second, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if !errors.Is(err, chainorch.ErrInProgress) || second.Outcome != chainorch.OutcomeInProgress {
			t.Fatalf("want in-progress got outcome=%s err=%v", second.Outcome, err)
		}
		if second.Search.ID != first.Search.ID {
			t.Fatalf("in-progress search_id mismatch")
		}
		if second.Created || second.OwnerToken != uuid.Nil {
			t.Fatal("loser must not receive owner token / create")
		}
	})

	t.Run("I2B-07 same key/expired recovers interrupted", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-07"
		fp := i2bFP(tenant)
		first, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil {
			t.Fatal(err)
		}
		expireClaimWindow(t, ctx, pool, first.Search.ID)
		res, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil {
			t.Fatal(err)
		}
		if res.Outcome != chainorch.OutcomeInterruptedRecovered {
			t.Fatalf("outcome=%s want INTERRUPTED_RECOVERED", res.Outcome)
		}
		if res.Search.Status != repository.ChainSearchFailed || res.Search.FailureCode == nil || *res.Search.FailureCode != repository.ChainInterruptedFailureCode {
			t.Fatalf("want interrupted got status=%s code=%v", res.Search.Status, res.Search.FailureCode)
		}
		if res.Search.SearchComplete {
			t.Fatal("interrupted must have search_complete=false")
		}
		if res.Search.ID != first.Search.ID {
			t.Fatal("recovery must replay same search_id")
		}
	})

	t.Run("I2B-08 same key different tenants independent", func(t *testing.T) {
		key := "i2b-08-shared"
		a := uuid.New()
		b := uuid.New()
		ra, err := orch.Claim(ctx, i2bClaim(a, key, i2bFP(a)))
		if err != nil {
			t.Fatal(err)
		}
		rb, err := orch.Claim(ctx, i2bClaim(b, key, i2bFP(b)))
		if err != nil {
			t.Fatal(err)
		}
		if ra.Search.ID == rb.Search.ID {
			t.Fatal("cross-tenant keys must be independent")
		}
	})

	t.Run("I2B-09 only one owner reaches RUNNING", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-09"
		fp := i2bFP(tenant)
		first, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil {
			t.Fatal(err)
		}
		var runningOK int32
		var wg sync.WaitGroup
		tokens := []uuid.UUID{first.OwnerToken, uuid.New(), uuid.New()}
		for _, tok := range tokens {
			wg.Add(1)
			go func(tok uuid.UUID) {
				defer wg.Done()
				_, err := orch.BeginRunning(ctx, tenant, first.Search.ID, tok)
				if err == nil {
					atomic.AddInt32(&runningOK, 1)
				}
			}(tok)
		}
		wg.Wait()
		if runningOK != 1 {
			t.Fatalf("running winners=%d want 1", runningOK)
		}
		assertStatus(t, ctx, store, tenant, first.Search.ID, repository.ChainSearchRunning)
	})

	t.Run("I2B-10 deadline stops new compute", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -21*time.Second)
		run := getRun(t, ctx, store, tenant, id)
		if err := orch.EnsureComputeAllowed(run); err == nil {
			t.Fatal("compute must stop after deadline_at")
		}
		ok, err := markRunning(ctx, store, tenant, id, owner)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("MarkChainSearchRunning must not succeed after deadline")
		}
	})

	t.Run("I2B-11 5s grace allows persistence only", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -22*time.Second)
		run := getRun(t, ctx, store, tenant, id)
		if err := orch.EnsureComputeAllowed(run); err == nil {
			t.Fatal("grace must not extend compute")
		}
		ok, err := ownerPersist(ctx, store, repository.ChainSearchRun{
			ID: id, TenantID: tenant, Status: repository.ChainSearchCompleted,
			SearchComplete: true, ReasonCounts: map[string]int{},
		}, owner, repository.ChainSearchRunning)
		if err != nil || !ok {
			t.Fatalf("persistence in grace ok=%v err=%v", ok, err)
		}
	})

	t.Run("I2B-12 late owner output rejected", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -26*time.Second)
		_, ok, err := orch.CompleteSearch(ctx, owner, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{ID: id, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("late owner complete must be rejected after claim expiry")
		}
		assertStatus(t, ctx, store, tenant, id, repository.ChainSearchRunning)
	})

	t.Run("I2B-13 duration snapshot used by chain execution", func(t *testing.T) {
		policyID := uuid.New()
		snap := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: policyID, Version: 3, PickupSeconds: 45, DeliverySeconds: 45,
		})
		tenant := uuid.New()
		local := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: snap},
		}
		res, err := local.Claim(ctx, i2bClaim(tenant, "i2b-13", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.ServiceDurationSnapshot.PolicyVersion == nil || *got.ServiceDurationSnapshot.PolicyVersion != 3 {
			t.Fatalf("stored snapshot missing policy version: %+v", got.ServiceDurationSnapshot)
		}
		var req chain.Request
		if err := chainorch.ApplyStoredDurationToChainRequest(&req, got.ServiceDurationSnapshot); err != nil {
			t.Fatal(err)
		}
		if req.ActionServiceSeconds["PICKUP"] != 45 {
			t.Fatalf("chain must consume stored snapshot actions=%v", req.ActionServiceSeconds)
		}
		live := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: policyID, Version: 4, PickupSeconds: 99, DeliverySeconds: 99,
		})
		if chainorch.SnapshotsEquivalent(got.ServiceDurationSnapshot, live) {
			t.Fatal("live policy must differ from stored snapshot")
		}
	})

	t.Run("I2B-14 duration policy change changes fingerprint", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-14"
		a := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: uuid.New(), Version: 1, PickupSeconds: 10, DeliverySeconds: 10})
		b := a
		ver := 2
		b.PolicyVersion = &ver
		first := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: a}}
		if _, err := first.Claim(ctx, i2bClaim(tenant, key, i2bFP(tenant))); err != nil {
			t.Fatal(err)
		}
		second := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: b}}
		res, err := second.Claim(ctx, i2bClaim(tenant, key, i2bFP(tenant)))
		if !errors.Is(err, chainorch.ErrFingerprintConflict) {
			t.Fatalf("want conflict got outcome=%s err=%v", res.Outcome, err)
		}
	})

	t.Run("I2B-15 KNOWN_ZERO != UNKNOWN", func(t *testing.T) {
		tenant := uuid.New()
		zero := chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)
		unknown := chainorch.UnknownDurationSnapshot()
		r1, err := (&chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: zero}}).
			Claim(ctx, i2bClaim(tenant, "i2b-15a", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		r2, err := (&chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: unknown}}).
			Claim(ctx, i2bClaim(tenant, "i2b-15b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if r1.Search.RequestFingerprint == r2.Search.RequestFingerprint {
			t.Fatal("KNOWN_ZERO and UNKNOWN must produce different request fingerprints")
		}
		if r1.Search.ServiceDurationSnapshot.State != repository.ServiceDurationKnown || r2.Search.ServiceDurationSnapshot.State != repository.ServiceDurationUnknown {
			t.Fatal("snapshot states not preserved")
		}
	})

	t.Run("I2B-16 cancellation persists with cleanup context", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i2b-16", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		cancelledParent, cancelFn := context.WithCancel(ctx)
		cancelFn()
		out, ok, err := orch.CancelSearch(cancelledParent, tenant, res.Search.ID, res.OwnerToken, repository.ChainSearchPending, chainorch.CancelClientCancelled, chainorch.TerminalStats{})
		if err != nil || !ok || out.Status != repository.ChainSearchCancelled {
			t.Fatalf("cancel ok=%v status=%s err=%v", ok, out.Status, err)
		}
		if out.CancelReasonClass == nil || *out.CancelReasonClass != chainorch.CancelClientCancelled {
			t.Fatalf("cancel reason=%v", out.CancelReasonClass)
		}
		if out.CancelledAt == nil || out.SearchComplete {
			t.Fatal("cancelled_at/search_complete invariant")
		}
	})

	t.Run("I2B-17 crash-before-cancel recovers interrupted", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -26*time.Second)
		ok, err := orch.RecoverExpired(ctx, tenant, id)
		if err != nil || !ok {
			t.Fatalf("recover ok=%v err=%v", ok, err)
		}
		got := getRun(t, ctx, store, tenant, id)
		if got.Status != repository.ChainSearchFailed || got.CancelReasonClass != nil {
			t.Fatalf("status=%s cancel=%v", got.Status, got.CancelReasonClass)
		}
		if got.FailureCode == nil || *got.FailureCode != repository.ChainInterruptedFailureCode {
			t.Fatalf("failure_code=%v", got.FailureCode)
		}
	})

	t.Run("I2B-18 recovery idempotent", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchPending, -26*time.Second)
		ok1, err := orch.RecoverExpired(ctx, tenant, id)
		if err != nil || !ok1 {
			t.Fatalf("first recover ok=%v err=%v", ok1, err)
		}
		ok2, err := orch.RecoverExpired(ctx, tenant, id)
		if err != nil {
			t.Fatal(err)
		}
		if ok2 {
			t.Fatal("second recover must be no-op")
		}
		assertStatus(t, ctx, store, tenant, id, repository.ChainSearchFailed)
	})

	t.Run("I2B-19 completion vs cancellation single winner", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i2b-19", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
			t.Fatal(err)
		}
		var completeOK, cancelOK int32
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, ok, err := orch.CompleteSearch(ctx, res.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
				Run: repository.ChainSearchRun{ID: res.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
			})
			if err != nil {
				t.Errorf("complete err=%v", err)
				return
			}
			if ok {
				atomic.AddInt32(&completeOK, 1)
			}
		}()
		go func() {
			defer wg.Done()
			_, ok, err := orch.CancelSearch(ctx, tenant, res.Search.ID, res.OwnerToken, repository.ChainSearchRunning, chainorch.CancelContextCancelled, chainorch.TerminalStats{})
			if err != nil {
				t.Errorf("cancel err=%v", err)
				return
			}
			if ok {
				atomic.AddInt32(&cancelOK, 1)
			}
		}()
		wg.Wait()
		if completeOK+cancelOK != 1 {
			t.Fatalf("winners complete=%d cancel=%d", completeOK, cancelOK)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if !chainorch.IsTerminal(got.Status) {
			t.Fatalf("nonterminal %s", got.Status)
		}
	})

	t.Run("I2B-20 completion vs recovery single winner", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -26*time.Second)
		var completeOK, recoverOK int32
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, ok, err := orch.CompleteSearch(ctx, owner, repository.ChainSearchRunning, chainorch.TerminalWrite{
				Run: repository.ChainSearchRun{ID: id, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
			})
			if err != nil {
				t.Errorf("complete err=%v", err)
				return
			}
			if ok {
				atomic.AddInt32(&completeOK, 1)
			}
		}()
		go func() {
			defer wg.Done()
			ok, err := orch.RecoverExpired(ctx, tenant, id)
			if err != nil {
				t.Errorf("recover err=%v", err)
				return
			}
			if ok {
				atomic.AddInt32(&recoverOK, 1)
			}
		}()
		wg.Wait()
		if completeOK+recoverOK != 1 {
			t.Fatalf("winners complete=%d recover=%d", completeOK, recoverOK)
		}
		got := getRun(t, ctx, store, tenant, id)
		if !chainorch.IsTerminal(got.Status) {
			t.Fatalf("nonterminal %s", got.Status)
		}
	})

	t.Run("I2B-21 cancellation vs recovery single winner", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -26*time.Second)
		var cancelOK, recoverOK int32
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, ok, err := orch.CancelSearch(ctx, tenant, id, owner, repository.ChainSearchRunning, chainorch.CancelGatewayCancelled, chainorch.TerminalStats{})
			if err != nil {
				t.Errorf("cancel err=%v", err)
				return
			}
			if ok {
				atomic.AddInt32(&cancelOK, 1)
			}
		}()
		go func() {
			defer wg.Done()
			ok, err := orch.RecoverExpired(ctx, tenant, id)
			if err != nil {
				t.Errorf("recover err=%v", err)
				return
			}
			if ok {
				atomic.AddInt32(&recoverOK, 1)
			}
		}()
		wg.Wait()
		if cancelOK+recoverOK != 1 {
			t.Fatalf("winners cancel=%d recover=%d", cancelOK, recoverOK)
		}
	})

	t.Run("I2B-22 no partial terminal shortlist visibility", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i2b-22", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
			t.Fatal(err)
		}
		cand := i2bCandidate(tenant, res.Search.ID, 1)
		_, ok, err := orch.CompleteSearch(ctx, res.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{
				ID: res.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted,
				SearchComplete: true, CandidateCount: 1, ReasonCounts: map[string]int{},
			},
			Candidates: []repository.ChainCandidate{cand},
		})
		if err != nil || !ok {
			t.Fatalf("complete ok=%v err=%v", ok, err)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		list, err := orch.ListCandidates(ctx, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != repository.ChainSearchCompleted || len(list) != 1 {
			t.Fatalf("terminal+shortlist inconsistent status=%s cands=%d", got.Status, len(list))
		}
	})

	t.Run("I2B-23 route-plan linkage owner/tenant fenced", func(t *testing.T) {
		tenant := uuid.New()
		other := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i2b-23", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
			t.Fatal(err)
		}
		planID := uuid.New()
		insertMinimalRoutePlan(t, ctx, pool, tenant, planID)
		foreignPlan := uuid.New()
		insertMinimalRoutePlan(t, ctx, pool, other, foreignPlan)

		err = store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			if err := tx.LockChainSearchRun(ctx, tenant, res.Search.ID); err != nil {
				return err
			}
			c := i2bCandidate(tenant, res.Search.ID, 1)
			c.RoutePlanID = &foreignPlan
			c.FeasibilityStatus = repository.ChainFeasible
			order := repository.ChainOrderL1BeforeL2
			c.SelectedOrder = &order
			return tx.InsertChainCandidate(ctx, c)
		})
		if err == nil {
			t.Fatal("cross-tenant route_plan FK must fail")
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchRunning)

		_, ok, err := orch.CompleteSearch(ctx, uuid.New(), repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{ID: res.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
			Candidates: []repository.ChainCandidate{func() repository.ChainCandidate {
				c := i2bCandidate(tenant, res.Search.ID, 1)
				c.RoutePlanID = &planID
				c.FeasibilityStatus = repository.ChainFeasible
				order := repository.ChainOrderL1BeforeL2
				c.SelectedOrder = &order
				return c
			}()},
		})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("wrong owner must not complete")
		}
	})

	t.Run("I2B-24 no recompute after terminal replay", func(t *testing.T) {
		tenant := uuid.New()
		key := "i2b-24"
		fp := i2bFP(tenant)
		first, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orch.BeginRunning(ctx, tenant, first.Search.ID, first.OwnerToken); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := orch.CompleteSearch(ctx, first.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{ID: first.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
		}); err != nil || !ok {
			t.Fatalf("complete ok=%v err=%v", ok, err)
		}
		replay, err := orch.Claim(ctx, i2bClaim(tenant, key, fp))
		if err != nil || replay.Outcome != chainorch.OutcomeReplay {
			t.Fatalf("outcome=%s err=%v", replay.Outcome, err)
		}
		if replay.Created || replay.OwnerToken != uuid.Nil {
			t.Fatal("replay must not grant compute ownership")
		}
		_, err = orch.BeginRunning(ctx, tenant, replay.Search.ID, first.OwnerToken)
		if err == nil {
			t.Fatal("terminal search must not re-enter RUNNING / recompute")
		}
	})
}

func i2bUnknownDuration() repository.ServiceDurationSnapshot {
	return chainorch.UnknownDurationSnapshot()
}

func i2bFP(tenant uuid.UUID) chainorch.FingerprintInput {
	one := 1
	capID := uuid.New()
	return chainorch.FingerprintInput{
		TenantID:               tenant,
		PolicyVersion:          chain.PolicyVersion,
		AlgorithmVersion:       chainorch.AlgorithmVersion,
		CatalogFingerprint:     "catalog-i2b",
		RoutingProviderVersion: "UNKNOWN",
		SeedSource:             repository.ChainSeedManualAvailableCapacity,
		SeedCapacityID:         &capID,
		SeedCapacityVersion:    &one,
		HorizonAnchorPolicy:    chainorch.HorizonAnchorSearchClock,
		PublicationVisibility:  "tenant-pool",
		FilterFingerprint:      "filter-default",
	}
}

func i2bClaim(tenant uuid.UUID, key string, fp chainorch.FingerprintInput) chainorch.ClaimRequest {
	return chainorch.ClaimRequest{
		TenantID:       tenant,
		IdempotencyKey: key,
		Fingerprint:    fp,
	}
}

func i2bCandidate(tenant, search uuid.UUID, ordinal int) repository.ChainCandidate {
	a, b := uuid.New(), uuid.New()
	first, second := a, b
	if a.String() > b.String() {
		first, second = b, a
	}
	return repository.ChainCandidate{
		CandidateID:          uuid.New(),
		SearchID:             search,
		TenantID:             tenant,
		Ordinal:              ordinal,
		FirstLoadID:          first,
		FirstLoadVersion:     1,
		SecondLoadID:         second,
		SecondLoadVersion:    1,
		FeasibilityStatus:    repository.ChainUnfinished,
		OmittedComponents:    []string{},
		ReasonCodes:          []string{"EVALUATION_UNFINISHED"},
		CandidateFingerprint: "cand-fp",
		CreatedAt:            time.Now().UTC(),
	}
}
