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

	t.Run("I270-001-A real compute crosses durable deadline ends FAILED budget", func(t *testing.T) {
		// Real execution proof: groupage is invoked and wall clock passes deadline_at.
		// Durable row is FAILED/CHAIN_BUDGET_EXHAUSTED. Actual engine status is logged
		// and not relabeled. SEARCH_COMPLETE after deadline is the unit classifier test
		// TestClassifyDurableTerminal_ComputeAfterDeadlineCannotComplete.
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-001a", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res.Search.ID, 2*time.Second)
		runBefore := getRun(t, ctx, store, tenant, res.Search.ID)
		var calls int32
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
		finishedWall := time.Now().UTC()
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&calls) == 0 {
			t.Fatal("slow groupage path was not invoked")
		}
		if out.EngineResult.PlannerInvocations == 0 {
			t.Fatalf("expected planner work engine=%s", out.EngineResult.Status)
		}
		if finishedWall.Before(runBefore.DeadlineAt) {
			t.Fatalf("wall %v did not cross deadline %v", finishedWall, runBefore.DeadlineAt)
		}
		if out.Outcome == chainorch.ExecuteOutcomeCompleted {
			t.Fatal("compute finishing after deadline must never COMPLETED")
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchFailed {
			t.Fatalf("status=%s want FAILED", got.Status)
		}
		if got.FailureCode == nil || *got.FailureCode != chainorch.FailureBudgetExhausted {
			t.Fatalf("failure_code=%v engine=%s", got.FailureCode, out.EngineResult.Status)
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
		t.Logf("I270-001-A actual engine status=%s planner=%d groupage=%d", out.EngineResult.Status, out.EngineResult.PlannerInvocations, calls)
	})

	t.Run("I270-001-B compute before deadline may persist in grace", func(t *testing.T) {
		// Real execution proof: observed finish is before deadline_at; observed terminal
		// write is after deadline_at and before claim_expires_at.
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-001b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res.Search.ID, 2*time.Second)
		runBefore := getRun(t, ctx, store, tenant, res.Search.ID)
		var finishedAt, persistAt time.Time
		orch.AfterComputeBeforeTerminalizeForTest = func() {
			finishedAt = time.Now().UTC()
			time.Sleep(3 * time.Second)
			persistAt = time.Now().UTC()
		}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if finishedAt.IsZero() || !finishedAt.Before(runBefore.DeadlineAt) {
			t.Fatalf("compute finish %v must be before deadline %v", finishedAt, runBefore.DeadlineAt)
		}
		if persistAt.IsZero() || !persistAt.After(runBefore.DeadlineAt) || !persistAt.Before(runBefore.ClaimExpiresAt) {
			t.Fatalf("persist %v must be inside grace (%v, %v)", persistAt, runBefore.DeadlineAt, runBefore.ClaimExpiresAt)
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

	t.Run("I270-002-F context already cancelled before ExecuteOwned", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002f", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		execCtx, execCancel := context.WithCancel(ctx)
		execCancel()
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("outcome=%s", out.Outcome)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchCancelled || got.StartedAt != nil {
			t.Fatalf("status=%s started=%v", got.Status, got.StartedAt)
		}
		if got.SearchComplete || got.CancelledAt == nil {
			t.Fatalf("complete=%v cancelled_at=%v", got.SearchComplete, got.CancelledAt)
		}
		if got.CancelReasonClass == nil || *got.CancelReasonClass != chainorch.CancelContextCancelled {
			t.Fatalf("reason=%v", got.CancelReasonClass)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("provider calls=%d", prov.calls)
		}
	})

	t.Run("I270-002-G parent deadline already exceeded durable deadline still future", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002g", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		run := getRun(t, ctx, store, tenant, res.Search.ID)
		if !time.Now().Before(run.DeadlineAt) {
			t.Fatal("durable deadline must still be in the future")
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		execCtx, execCancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
		defer execCancel()
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("outcome=%s want CANCELLED not budget", out.Outcome)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchCancelled {
			t.Fatalf("status=%s", got.Status)
		}
		if got.FailureCode != nil {
			t.Fatalf("must not be FAILED code=%v", got.FailureCode)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("provider calls=%d", prov.calls)
		}
	})

	t.Run("I270-002-H cancel races PENDING to RUNNING", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002h", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		block := &blockingProvider{blockFor: 30 * time.Second}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, block, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		execCtx, execCancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			execCancel()
		}()
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		wg.Wait()
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("outcome=%s", out.Outcome)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchCancelled {
			t.Fatalf("status=%s must not remain PENDING", got.Status)
		}
		// Second execute must not recompute.
		prov := &countingProvider{}
		base2 := durableExecRequest(tenant, prov, clock)
		base2.Loads = twoStackableLoads(tenant, clock)
		_, err = orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base2, chainorch.ExecuteOwnedOptions{})
		if !errors.Is(err, chainorch.ErrSearchNotRunnable) {
			t.Fatalf("replay want ErrSearchNotRunnable got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("recompute calls=%d", prov.calls)
		}
	})

	t.Run("I270-002-I wrong owner pre-cancelled context", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002i", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		execCtx, execCancel := context.WithCancel(ctx)
		execCancel()
		_, err = orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, uuid.New(), chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if !errors.Is(err, chainorch.ErrNotOwner) {
			t.Fatalf("want ErrNotOwner got %v", err)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("provider calls=%d", prov.calls)
		}
	})

	t.Run("I270-002-J pre-running CANCELLED replay zero compute", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		fp := i2bFP(tenant)
		res, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002j", fp))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		execCtx, execCancel := context.WithCancel(ctx)
		execCancel()
		out, err := orch.ExecuteOwnedDurableChainSearch(execCtx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil || out.Outcome != chainorch.ExecuteOutcomeCancelled {
			t.Fatalf("err=%v outcome=%s", err, out.Outcome)
		}
		replay, err := orch.Claim(ctx, i2bClaim(tenant, "i270-002j", fp))
		if err != nil || replay.Outcome != chainorch.OutcomeReplay || replay.Search.ID != res.Search.ID {
			t.Fatalf("replay outcome=%s id=%s err=%v", replay.Outcome, replay.Search.ID, err)
		}
		if replay.Search.Status != repository.ChainSearchCancelled {
			t.Fatalf("replay status=%s", replay.Search.Status)
		}
		prov2 := &countingProvider{}
		base2 := durableExecRequest(tenant, prov2, clock)
		base2.Loads = twoStackableLoads(tenant, clock)
		_, err = orch.ExecuteOwnedDurableChainSearch(ctx, tenant, replay.Search.ID, res.OwnerToken, chain.Engine{}, base2, chainorch.ExecuteOwnedOptions{})
		if !errors.Is(err, chainorch.ErrSearchNotRunnable) {
			t.Fatalf("want ErrSearchNotRunnable got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 || atomic.LoadInt32(&prov2.calls) != 0 {
			t.Fatalf("calls first=%d replay=%d", prov.calls, prov2.calls)
		}
	})
}
