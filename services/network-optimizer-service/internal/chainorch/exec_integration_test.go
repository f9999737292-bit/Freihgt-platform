//go:build integration

package chainorch_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

type spyRepo struct {
	claims  int32
	commits int32
}

func (s *spyRepo) Claim(context.Context, uuid.UUID, string, string) (chain.ClaimResult, error) {
	atomic.AddInt32(&s.claims, 1)
	return chain.ClaimResult{}, nil
}

func (s *spyRepo) Commit(context.Context, uuid.UUID, string, string, chain.Result) error {
	atomic.AddInt32(&s.commits, 1)
	return nil
}

func TestI2B_DurableExecutionIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	orch := &chainorch.Orchestrator{
		Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)},
	}

	t.Run("I2B-EXEC-01 successful durable result SearchID == run.ID", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-01", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		base.IdempotencyKey = "must-be-cleared"
		out, err := orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
		if err != nil {
			t.Fatal(err)
		}
		if out.SearchID != res.Search.ID {
			t.Fatalf("SearchID=%s want persisted %s", out.SearchID, res.Search.ID)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchRunning)
	})

	t.Run("I2B-EXEC-02 budget exhausted on RUNNING still uses run.ID", func(t *testing.T) {
		// Low-level: already RUNNING, deadline passed — SearchID postcondition only.
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-02", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
			t.Fatal(err)
		}
		expireToPastDeadline(t, ctx, pool, res.Search.ID)
		run := getRun(t, ctx, store, tenant, res.Search.ID)
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, durableExecRequest(tenant, &countingProvider{}, time.Now().UTC()))
		if err != nil {
			t.Fatal(err)
		}
		if out.SearchID != run.ID {
			t.Fatalf("budget SearchID=%s want %s", out.SearchID, run.ID)
		}
		if out.Status != chain.StatusBudgetExhausted {
			t.Fatalf("status=%s", out.Status)
		}
	})

	t.Run("I2B-EXEC-03/04 legacy Repository Claim/Commit not invoked", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-03", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		spy := &spyRepo{}
		engine := chain.Engine{Repository: spy}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		base.IdempotencyKey = "legacy-key"
		_, err = orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, engine, base)
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&spy.claims) != 0 || atomic.LoadInt32(&spy.commits) != 0 {
			t.Fatalf("legacy claim=%d commit=%d", spy.claims, spy.commits)
		}
	})

	t.Run("I2B-EXEC-05 IdempotencyKey cannot create second domain", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-05", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := chainorch.PrepareDurableChainRequest(res.Search, chain.Request{IdempotencyKey: "other-domain"})
		if err != nil {
			t.Fatal(err)
		}
		if prepared.IdempotencyKey != "" {
			t.Fatalf("IdempotencyKey must be cleared got=%q", prepared.IdempotencyKey)
		}
		if prepared.ClientChainID != nil {
			t.Fatal("ClientChainID must be nil")
		}
		if prepared.DurableSearchID == nil || *prepared.DurableSearchID != res.Search.ID {
			t.Fatal("DurableSearchID must pin run.ID")
		}
	})
}

func TestI2B_OwnedComputeLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	orch := &chainorch.Orchestrator{
		Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)},
	}

	t.Run("I2B-EXEC-06 RUNNING committed before first provider call", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-06", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
		prov := &statusAtCallProvider{pool: pool, tenant: tenant, searchID: res.Search.ID}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		_, err = orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&prov.calls) == 0 {
			t.Fatal("expected provider work after RUNNING")
		}
		if !prov.firstWasRunning {
			t.Fatal("DB status must already be RUNNING at first provider invocation")
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchRunning)
	})

	t.Run("I2B-EXEC-07 wrong owner remains PENDING with zero provider calls", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-07", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		_, err = orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, uuid.New(), chain.Engine{}, base)
		if !errors.Is(err, chainorch.ErrNotOwner) {
			t.Fatalf("want ErrNotOwner got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("wrong owner must not compute calls=%d", prov.calls)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
	})

	t.Run("I2B-EXEC-08 concurrent start: one RUNNING winner, one compute", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-08", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		provA, provB := &countingProvider{}, &countingProvider{}
		clock := time.Now().UTC()
		baseA := durableExecRequest(tenant, provA, clock)
		baseA.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		baseB := durableExecRequest(tenant, provB, clock)
		baseB.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}

		var wg sync.WaitGroup
		var win, lose int32
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, baseA)
			if err == nil {
				atomic.AddInt32(&win, 1)
			} else if errors.Is(err, chainorch.ErrInProgress) {
				atomic.AddInt32(&lose, 1)
			} else {
				t.Errorf("unexpected A err=%v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_, err := orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, baseB)
			if err == nil {
				atomic.AddInt32(&win, 1)
			} else if errors.Is(err, chainorch.ErrInProgress) {
				atomic.AddInt32(&lose, 1)
			} else {
				t.Errorf("unexpected B err=%v", err)
			}
		}()
		wg.Wait()
		if atomic.LoadInt32(&win) != 1 || atomic.LoadInt32(&lose) != 1 {
			t.Fatalf("want 1 winner 1 loser got win=%d lose=%d", win, lose)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchRunning)
		totalCalls := atomic.LoadInt32(&provA.calls) + atomic.LoadInt32(&provB.calls)
		if totalCalls == 0 {
			t.Fatal("winner must perform compute")
		}
		// Loser must not have started provider work.
		if atomic.LoadInt32(&provA.calls) > 0 && atomic.LoadInt32(&provB.calls) > 0 {
			t.Fatalf("both providers ran A=%d B=%d — loser must not compute", provA.calls, provB.calls)
		}
	})

	t.Run("I2B-EXEC-09 deadline passed while PENDING: no RUNNING, zero providers", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-09", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		expireToPastDeadline(t, ctx, pool, res.Search.ID)
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		_, err = orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
		if !errors.Is(err, chainorch.ErrComputeDeadlinePassed) {
			t.Fatalf("want ErrComputeDeadlinePassed got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("calls=%d", prov.calls)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
	})

	t.Run("I2B-EXEC-10 terminal search cannot start compute", func(t *testing.T) {
		for _, status := range []string{
			repository.ChainSearchCompleted, repository.ChainSearchCancelled, repository.ChainSearchFailed,
		} {
			tenant := uuid.New()
			res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-10-"+status, i2bFP(tenant)))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
				t.Fatal(err)
			}
			switch status {
			case repository.ChainSearchCompleted:
				if _, ok, err := orch.CompleteSearch(ctx, res.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
					Run: repository.ChainSearchRun{ID: res.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
				}); err != nil || !ok {
					t.Fatalf("complete ok=%v err=%v", ok, err)
				}
			case repository.ChainSearchCancelled:
				if _, ok, err := orch.CancelSearch(ctx, tenant, res.Search.ID, res.OwnerToken, repository.ChainSearchRunning, chainorch.CancelClientCancelled); err != nil || !ok {
					t.Fatalf("cancel ok=%v err=%v", ok, err)
				}
			default:
				if _, ok, err := orch.FailSearch(ctx, tenant, res.Search.ID, res.OwnerToken, repository.ChainSearchRunning, "TEST_FAIL", map[string]int{}); err != nil || !ok {
					t.Fatalf("fail ok=%v err=%v", ok, err)
				}
			}
			prov := &countingProvider{}
			clock := time.Now().UTC()
			base := durableExecRequest(tenant, prov, clock)
			base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
			_, err = orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
			if !errors.Is(err, chainorch.ErrSearchNotRunnable) {
				t.Fatalf("status=%s want ErrSearchNotRunnable got %v", status, err)
			}
			if atomic.LoadInt32(&prov.calls) != 0 {
				t.Fatalf("terminal %s must not compute", status)
			}
		}
	})

	t.Run("I2B-EXEC-11 already RUNNING cannot start second compute", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-11", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		_, err = orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
		if !errors.Is(err, chainorch.ErrInProgress) {
			t.Fatalf("want ErrInProgress got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatalf("second start must not compute calls=%d", prov.calls)
		}
	})

	t.Run("I2B-EXEC-12 candidate IDs unique and distinct from SearchID", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-12", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = multiCandidateLoads(tenant, clock)
		out, err := orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
		if err != nil {
			t.Fatal(err)
		}
		if out.SearchID != res.Search.ID {
			t.Fatalf("SearchID=%s want %s", out.SearchID, res.Search.ID)
		}
		if len(out.Candidates) < 2 {
			t.Fatalf("need >=2 candidates got %d (pairs=%d)", len(out.Candidates), out.PairsRetained)
		}
		seen := map[uuid.UUID]bool{}
		for _, c := range out.Candidates {
			if c.CandidateID == uuid.Nil {
				t.Fatal("CandidateID must not be nil")
			}
			if c.CandidateID == out.SearchID {
				t.Fatalf("CandidateID must not equal SearchID %s", out.SearchID)
			}
			if seen[c.CandidateID] {
				t.Fatalf("duplicate CandidateID %s", c.CandidateID)
			}
			seen[c.CandidateID] = true
		}
	})

	t.Run("I2B-EXEC-13 custom NewID does not alter durable SearchID", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-13", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		engine := chain.Engine{NewID: func() uuid.UUID {
			n++
			return uuid.MustParse(fmt.Sprintf("aaaaaaaa-bbbb-cccc-dddd-%012d", n))
		}}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = multiCandidateLoads(tenant, clock)
		out, err := orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, engine, base)
		if err != nil {
			t.Fatal(err)
		}
		if out.SearchID != res.Search.ID {
			t.Fatalf("SearchID must stay run.ID got=%s want=%s", out.SearchID, res.Search.ID)
		}
		if len(out.Candidates) == 0 {
			t.Fatal("expected candidates from custom NewID generator")
		}
		for _, c := range out.Candidates {
			if c.CandidateID == res.Search.ID {
				t.Fatal("custom NewID candidates must not be forced to run.ID")
			}
			if c.CandidateID.String()[:8] != "aaaaaaaa" {
				t.Fatalf("candidate should use custom NewID got=%s", c.CandidateID)
			}
		}
	})

	t.Run("low-level RunDurableChainSearch rejects PENDING", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-ll", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		_, err = chainorch.RunDurableChainSearch(ctx, chain.Engine{}, res.Search, durableExecRequest(tenant, prov, time.Now().UTC()))
		if !errors.Is(err, chainorch.ErrSearchNotRunnable) {
			t.Fatalf("want ErrSearchNotRunnable got %v", err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 {
			t.Fatal("PENDING bypass must not compute")
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchPending)
	})
}

func TestI2B_PersistedHorizon(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)

	t.Run("I2B-HORIZON-01/02 delayed execution keeps persisted horizon", func(t *testing.T) {
		tenant := uuid.New()
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
			Now: func() time.Time { return time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC) },
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "hz-01", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		wantStart, wantEnd := res.Search.HorizonStart, res.Search.HorizonEnd
		// Reset claim clock to wall time so MarkRunning deadline check can pass.
		orch.Now = nil
		delayed := time.Now().UTC().Add(4 * time.Hour)
		base := durableExecRequest(tenant, &countingProvider{}, delayed)
		base.Now = func() time.Time { return delayed }
		out, err := orch.RunOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base)
		if err != nil {
			t.Fatal(err)
		}
		if !out.HorizonStart.Equal(wantStart) || !out.HorizonEnd.Equal(wantEnd) {
			t.Fatalf("horizon drifted start=%v/%v end=%v/%v", out.HorizonStart, wantStart, out.HorizonEnd, wantEnd)
		}
	})

	t.Run("I2B-HORIZON-03 manual available_from/until derive exact bounds", func(t *testing.T) {
		tenant := uuid.New()
		from := time.Date(2026, 2, 1, 6, 0, 0, 0, time.UTC)
		until := from.Add(12 * time.Hour)
		fp := i2bFP(tenant)
		fp.HorizonAnchorPolicy = ""
		fp.AvailableFrom = &from
		fp.AvailableUntil = &until
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "hz-03", fp))
		if err != nil {
			t.Fatal(err)
		}
		if !res.Search.HorizonStart.Equal(from) || !res.Search.HorizonEnd.Equal(until) {
			t.Fatalf("manual horizon start=%v end=%v want %v..%v", res.Search.HorizonStart, res.Search.HorizonEnd, from, until)
		}
	})

	t.Run("I2B-HORIZON-04 caller cannot inject independent HorizonStart/End", func(t *testing.T) {
		var req chainorch.ClaimRequest
		_ = req
		tenant := uuid.New()
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
			Now: func() time.Time { return time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) },
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "hz-04", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		wantStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		if !res.Search.HorizonStart.Equal(wantStart) {
			t.Fatalf("horizon must come from server clock got=%v", res.Search.HorizonStart)
		}
	})

	t.Run("I2B-HORIZON-05 same-key replay returns original persisted horizon", func(t *testing.T) {
		tenant := uuid.New()
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
			Now: func() time.Time { return time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC) },
		}
		fp := i2bFP(tenant)
		first, err := orch.Claim(ctx, i2bClaim(tenant, "hz-05", fp))
		if err != nil {
			t.Fatal(err)
		}
		orch.Now = nil
		if _, err := orch.BeginRunning(ctx, tenant, first.Search.ID, first.OwnerToken); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := orch.CompleteSearch(ctx, first.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{ID: first.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
		}); err != nil || !ok {
			t.Fatalf("complete ok=%v err=%v", ok, err)
		}
		orch.Now = func() time.Time { return time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC) }
		replay, err := orch.Claim(ctx, i2bClaim(tenant, "hz-05", fp))
		if err != nil || replay.Outcome != chainorch.OutcomeReplay {
			t.Fatalf("replay outcome=%s err=%v", replay.Outcome, err)
		}
		if !replay.Search.HorizonStart.Equal(first.Search.HorizonStart) || !replay.Search.HorizonEnd.Equal(first.Search.HorizonEnd) {
			t.Fatal("replay horizon must match original persisted bounds")
		}
	})
}

type statusAtCallProvider struct {
	pool            *pgxpool.Pool
	tenant, searchID uuid.UUID
	calls           int32
	firstWasRunning bool
}

func (p *statusAtCallProvider) Route(ctx context.Context, _ routing.RouteRequest) (routing.RouteResult, error) {
	n := atomic.AddInt32(&p.calls, 1)
	if n == 1 {
		var status string
		if err := p.pool.QueryRow(ctx, `
			SELECT status FROM network_optimizer.chain_search_runs
			WHERE tenant_id=$1 AND id=$2`, p.tenant, p.searchID).Scan(&status); err == nil && status == repository.ChainSearchRunning {
			p.firstWasRunning = true
		}
	}
	return routing.RouteResult{
		Provider: "script", DistanceM: 1000, DurationSeconds: 60,
		CalculatedAt: time.Unix(1, 0).UTC(), ExpiresAt: time.Unix(1, 0).UTC().Add(time.Hour),
	}, nil
}

func (p *statusAtCallProvider) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func multiCandidateLoads(tenant uuid.UUID, clock time.Time) []chain.VisibleLoad {
	stackable := true
	loads := make([]chain.VisibleLoad, 3)
	for i := 0; i < 3; i++ {
		loads[i] = visibleLoad(tenant, clock, i+1)
		loads[i].Stackable = &stackable
	}
	return loads
}

func durableExecRequest(tenant uuid.UUID, route routing.Provider, clock time.Time) chain.Request {
	observed := time.Unix(50, 0).UTC()
	gross, height, width, length, axle := 18000.0, 4.0, 2.55, 16.5, 8000.0
	danger := false
	dim := chain.Dimension{Known: true, Value: 1000}
	return chain.Request{
		TenantID: tenant,
		Seed: chain.Seed{
			Source: chain.SeedManualAvailable,
			Start: routeplan.Point{
				Kind: routeplan.PointAnchor, Latitude: 0, Longitude: 10,
				Source: routeplan.SourceTracking, ObservedAt: &observed,
			},
			Capacity: chain.Capacity{
				Payload: dim, Volume: dim, Pallets: dim, Linear: dim, Height: dim,
			},
		},
		VehicleProfile: routing.VehicleProfile{
			GrossWeightKg: &gross, HeightM: &height, WidthM: &width, LengthM: &length, AxleLoadKg: &axle, DangerousCargo: &danger,
		},
		Now:   func() time.Time { return clock },
		Route: route,
	}
}
