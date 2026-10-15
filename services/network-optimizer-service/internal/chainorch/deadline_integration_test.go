//go:build integration

package chainorch_test

import (
	"context"
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

type countingProvider struct {
	calls  int32
	lastTO atomic.Value // time.Duration
}

func (p *countingProvider) Route(ctx context.Context, _ routing.RouteRequest) (routing.RouteResult, error) {
	atomic.AddInt32(&p.calls, 1)
	if dl, ok := ctx.Deadline(); ok {
		p.lastTO.Store(time.Until(dl))
	}
	return routing.RouteResult{DistanceM: 1000, DurationSeconds: 60}, nil
}

func (p *countingProvider) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func TestI2B_DeadlineContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	orch := &chainorch.Orchestrator{
		Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
	}

	t.Run("I2B-DEADLINE-01 remaining durable budget after delay", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dl-01", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		run := res.Search
		delayedStart := run.CreatedAt.Add(8 * time.Second)
		remaining := chainorch.RemainingComputeBudget(run, delayedStart)
		if remaining <= 0 || remaining > 12*time.Second {
			t.Fatalf("remaining budget=%v want ~(12s)", remaining)
		}
		prepared, err := chainorch.PrepareDurableChainRequest(run, chain.Request{})
		if err != nil {
			t.Fatal(err)
		}
		if prepared.ComputeDeadline == nil || !prepared.ComputeDeadline.Equal(run.DeadlineAt) {
			t.Fatalf("engine must receive persisted deadline_at got=%v want=%v", prepared.ComputeDeadline, run.DeadlineAt)
		}
		freshWindow := delayedStart.Add(chain.WallClockBudget)
		if !prepared.ComputeDeadline.Before(freshWindow) {
			t.Fatal("persisted deadline must be earlier than a fresh 20s window from delayed start")
		}
	})

	t.Run("I2B-DEADLINE-02 zero provider calls at/after deadline", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dl-02", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		run := res.Search
		expireToPastDeadline(t, ctx, pool, run.ID)
		run = getRun(t, ctx, store, tenant, run.ID)
		prov := &countingProvider{}
		clock := run.DeadlineAt.Add(time.Millisecond)
		base := durableChainRequest(tenant, prov, func() time.Time { return clock }, clock)
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, base)
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 || out.ProviderCalls != 0 {
			t.Fatalf("provider calls=%d result=%d want 0", prov.calls, out.ProviderCalls)
		}
	})

	t.Run("I2B-DEADLINE-03 provider timeout capped by remaining durable deadline", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dl-03", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		run := res.Search
		setDeadlineIn(t, ctx, pool, run.ID, 2*time.Second)
		run = getRun(t, ctx, store, tenant, run.ID)
		prov := &countingProvider{}
		now := time.Now().UTC()
		base := durableChainRequest(tenant, prov, func() time.Time { return time.Now().UTC() }, now)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, now, 1)}
		_, _ = chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, base)
		if atomic.LoadInt32(&prov.calls) == 0 {
			t.Fatal("expected at least one provider call to observe timeout cap")
		}
		raw := prov.lastTO.Load()
		if raw == nil {
			t.Fatal("missing observed timeout")
		}
		to := raw.(time.Duration)
		if to > 2*time.Second+500*time.Millisecond {
			t.Fatalf("provider timeout %v exceeds remaining durable budget ~2s", to)
		}
	})

	t.Run("I2B-DEADLINE-04 route-plan evaluation cannot start after durable deadline", func(t *testing.T) {
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dl-04", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		run := res.Search
		expireToPastDeadline(t, ctx, pool, run.ID)
		run = getRun(t, ctx, store, tenant, run.ID)
		if err := orch.EnsureComputeAllowed(run); err == nil {
			t.Fatal("compute must be blocked after deadline")
		}
		prov := &countingProvider{}
		clock := run.DeadlineAt.Add(10 * time.Millisecond)
		base := durableChainRequest(tenant, prov, func() time.Time { return clock }, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1), visibleLoad(tenant, clock, 2)}
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, base)
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 || out.ProviderCalls != 0 {
			t.Fatalf("no compute after deadline calls=%d result=%d", prov.calls, out.ProviderCalls)
		}
	})

	t.Run("I2B-DEADLINE-05 grace allows terminal persistence but zero new compute", func(t *testing.T) {
		tenant := uuid.New()
		id := uuid.New()
		owner := uuid.New()
		insertTimedSearch(t, ctx, pool, tenant, id, owner, repository.ChainSearchRunning, -22*time.Second)
		run := getRun(t, ctx, store, tenant, id)
		if err := orch.EnsureComputeAllowed(run); err == nil {
			t.Fatal("grace must not extend compute")
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableChainRequest(tenant, prov, func() time.Time { return clock }, clock)
		base.Loads = []chain.VisibleLoad{visibleLoad(tenant, clock, 1)}
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, run, base)
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&prov.calls) != 0 || out.ProviderCalls != 0 {
			t.Fatal("grace window must allow zero new compute")
		}
		ok, err := ownerPersist(ctx, store, repository.ChainSearchRun{
			ID: id, TenantID: tenant, Status: repository.ChainSearchCompleted,
			SearchComplete: true, ReasonCounts: map[string]int{},
		}, owner, repository.ChainSearchRunning)
		if err != nil || !ok {
			t.Fatalf("terminal persistence in grace ok=%v err=%v", ok, err)
		}
	})
}

func durableChainRequest(tenant uuid.UUID, route routing.Provider, now func() time.Time, clock time.Time) chain.Request {
	loc := uuid.New()
	return chain.Request{
		TenantID: tenant,
		Seed: chain.Seed{
			Source: chain.SeedManualAvailable,
			Start: routeplan.Point{
				Kind: routeplan.PointCanonical, LocationID: &loc, Latitude: 1, Longitude: 2, Source: routeplan.SourceCanonical,
			},
			Capacity: chain.Capacity{
				Payload: chain.Dimension{Known: true, Value: 1000},
			},
		},
		VehicleProfile: routing.VehicleProfile{},
		Now:            now,
		Route:          route,
	}
}

func visibleLoad(tenant uuid.UUID, clock time.Time, n int) chain.VisibleLoad {
	pid := uuid.New()
	did := uuid.New()
	return chain.VisibleLoad{
		ID: uuid.New(), Version: 1, OwnerTenantID: tenant, ConsolidationAllowed: true,
		Pickup:      routeplan.Point{Kind: routeplan.PointCanonical, LocationID: &pid, Latitude: float64(n), Longitude: 10, Source: routeplan.SourceCanonical},
		Delivery:    routeplan.Point{Kind: routeplan.PointCanonical, LocationID: &did, Latitude: float64(n) + 1, Longitude: 11, Source: routeplan.SourceCanonical},
		PickupStart: clock.Add(time.Hour), PickupEnd: clock.Add(10 * time.Hour),
		DeliveryStart: clock.Add(2 * time.Hour), DeliveryEnd: clock.Add(30 * time.Hour),
		WeightKg: 1, PublicationLabel: "VISIBLE_AREA",
	}
}

func expireToPastDeadline(t *testing.T, ctx context.Context, pool *pgxpool.Pool, searchID uuid.UUID) {
	t.Helper()
	// deadline past, claim_expires still in the near future (grace).
	tag, err := pool.Exec(ctx, `
		UPDATE network_optimizer.chain_search_runs
		SET created_at = now() - interval '22 seconds',
		    deadline_at = now() - interval '2 seconds',
		    claim_expires_at = now() + interval '3 seconds',
		    started_at = CASE WHEN status = 'RUNNING' THEN now() - interval '22 seconds' ELSE started_at END
		WHERE id = $1`, searchID)
	if err != nil {
		t.Fatalf("expire deadline: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("expire deadline rows=%d", tag.RowsAffected())
	}
}

func setDeadlineIn(t *testing.T, ctx context.Context, pool *pgxpool.Pool, searchID uuid.UUID, in time.Duration) {
	t.Helper()
	sec := int(in / time.Second)
	if sec < 1 {
		sec = 1
	}
	tag, err := pool.Exec(ctx, `
		UPDATE network_optimizer.chain_search_runs
		SET created_at = now() - make_interval(secs => 20 - $2),
		    deadline_at = now() + make_interval(secs => $2),
		    claim_expires_at = now() + make_interval(secs => $2 + 5)
		WHERE id = $1`, searchID, sec)
	if err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("set deadline rows=%d", tag.RowsAffected())
	}
}
