//go:build integration

package chainorch_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestI270_AgentI_DeadlineAndCancelRaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	baseOrch := func() *chainorch.Orchestrator {
		return &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)},
		}
	}

	t.Run("I270-001-A compute after deadline cannot COMPLETED", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-001a", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res.Search.ID, 2*time.Second)
		var calls int32
		// Deterministic internal path: groupage blocks past durable deadline without
		// relying solely on provider ctx cancellation. Search returns after deadline.
		groupage := func([]compat.GroupageItem) compat.Result {
			n := atomic.AddInt32(&calls, 1)
			if n == 1 {
				time.Sleep(3 * time.Second)
			}
			return compat.Result{Status: compat.StatusCompatible}
		}
		clock := time.Now().UTC()
		prov := &countingProvider{}
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		base.Groupage = groupage
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome == chainorch.ExecuteOutcomeCompleted {
			t.Fatal("compute finishing after deadline must never COMPLETED")
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchFailed {
			t.Fatalf("status=%s want FAILED", got.Status)
		}
		if got.FailureCode == nil || *got.FailureCode != chainorch.FailureBudgetExhausted {
			t.Fatalf("failure_code=%v", got.FailureCode)
		}
		if got.SearchComplete {
			t.Fatal("search_complete must be false")
		}
		cands, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(cands) != 0 {
			t.Fatalf("budget fail must not expose COMPLETED shortlist rows=%d", len(cands))
		}
	})

	t.Run("I270-001-B compute before deadline may persist in grace", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-001b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res.Search.ID, 2*time.Second)
		// After trusted finish capture, delay terminal write into +5s grace.
		orch.AfterComputeBeforeTerminalizeForTest = func() {
			time.Sleep(3 * time.Second)
		}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("outcome=%s engine=%s fail=%v", out.Outcome, out.EngineResult.Status, out.Search.FailureCode)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchCompleted)
	})

	t.Run("I270-002-A cancel after compute before terminal commit", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002a", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		execCtx, execCancel := context.WithCancel(ctx)
		orch.AfterComputeBeforeTerminalizeForTest = func() {
			execCancel()
		}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("outcome=%s want CANCELLED", out.Outcome)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchCancelled {
			t.Fatalf("status=%s", got.Status)
		}
		if got.CancelReasonClass == nil || *got.CancelReasonClass != chainorch.CancelContextCancelled {
			t.Fatalf("cancel_reason=%v", got.CancelReasonClass)
		}
		cands, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(cands) != 0 {
			t.Fatalf("must not expose COMPLETED shortlist after cancel rows=%d", len(cands))
		}
	})

	t.Run("I270-002-B cancel during provider work still CANCELLED", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		block := &blockingProvider{blockFor: 30 * time.Second}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, block, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		execCtx, execCancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(200 * time.Millisecond)
			execCancel()
		}()
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("outcome=%s", out.Outcome)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchCancelled)
	})

	t.Run("I270-002-C cancel beats later completion overwrite", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002c", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		execCtx, execCancel := context.WithCancel(ctx)
		orch.AfterComputeBeforeTerminalizeForTest = func() { execCancel() }
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil || out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("err=%v outcome=%s", err, out.Outcome)
		}
		// Subsequent completion attempt must not overwrite CANCELLED.
		_, ok, err := orch.CompleteSearch(ctx, res.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{
				ID: res.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted,
				SearchComplete: true, ReasonCounts: map[string]int{},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("completion must not overwrite CANCELLED")
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchCancelled)
	})

	t.Run("I270-002-D cancelled terminal replay zero recompute", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		fp := i2bFP(tenant)
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002d", fp))
		if err != nil {
			t.Fatal(err)
		}
		execCtx, execCancel := context.WithCancel(ctx)
		orch.AfterComputeBeforeTerminalizeForTest = func() { execCancel() }
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil || out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("err=%v outcome=%s", err, out.Outcome)
		}
		replay, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002d", fp))
		if err != nil || replay.Outcome != chainorch.OutcomeReplay {
			t.Fatalf("replay outcome=%s err=%v", replay.Outcome, err)
		}
		if replay.Search.Status != repository.ChainSearchCancelled {
			t.Fatalf("replay status=%s", replay.Search.Status)
		}
		prov := &countingProvider{}
		base2 := durableExecRequest(tenant, prov, clock)
		base2.Loads = twoStackableLoads(tenant, clock)
		_, err = orch.ExecuteOwnedDurableChainSearch(ctx, tenant, replay.Search.ID, res.OwnerToken, chain.Engine{}, base2, chainorch.ExecuteOwnedOptions{})
		if !errors.Is(err, chainorch.ErrSearchNotRunnable) {
			t.Fatalf("want ErrSearchNotRunnable got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("recompute calls=%d", prov.calls)
		}
	})

	t.Run("I270-002-E invalid cancel reason fails before compute", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002e", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		_, err = orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{
			CancelReasonClass: "NOT_A_VALID_REASON",
		})
		if !errors.Is(err, chainorch.ErrInvalidCancelReason) {
			t.Fatalf("want ErrInvalidCancelReason got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("provider calls=%d want 0", prov.calls)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
	})
}
