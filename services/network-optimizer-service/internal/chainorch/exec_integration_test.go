//go:build integration

package chainorch_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

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
		run := res.Search
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		base.IdempotencyKey = "must-be-cleared"
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, base)
		if err != nil {
			t.Fatal(err)
		}
		if out.SearchID != run.ID {
			t.Fatalf("SearchID=%s want persisted %s", out.SearchID, run.ID)
		}
	})

	t.Run("I2B-EXEC-02 cancelled/budget/rejected also uses run.ID", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "exec-02", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		run := res.Search
		expireToPastDeadline(t, ctx, pool, run.ID)
		run = getRun(t, ctx, store, tenant, run.ID)
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
		_, err = chainorch.RunDurableChainSearch(ctx, engine, res.Search, base)
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
		run := res.Search
		wantStart, wantEnd := run.HorizonStart, run.HorizonEnd
		// Delayed engine clock would derive a different horizon if not pinned.
		delayed := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
		base := durableExecRequest(tenant, &countingProvider{}, delayed)
		base.Now = func() time.Time { return delayed } // overwritten by PrepareDurable
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, base)
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
		// Structural: ClaimRequest has no HorizonStart/End fields after R2.
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
		if _, err := orch.BeginRunning(ctx, tenant, first.Search.ID, first.OwnerToken); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := orch.CompleteSearch(ctx, first.OwnerToken, repository.ChainSearchRunning, chainorch.TerminalWrite{
			Run: repository.ChainSearchRun{ID: first.Search.ID, TenantID: tenant, Status: repository.ChainSearchCompleted, SearchComplete: true, ReasonCounts: map[string]int{}},
		}); err != nil || !ok {
			t.Fatalf("complete ok=%v err=%v", ok, err)
		}
		// Replay under a different claim clock must keep original horizon.
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
