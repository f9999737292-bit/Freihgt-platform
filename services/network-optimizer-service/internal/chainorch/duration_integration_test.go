//go:build integration

package chainorch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

func TestI2B_DurationProvenance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)

	t.Run("I2B-DUR-01 active policy server-resolved before claim", func(t *testing.T) {
		tenant := uuid.New()
		policy := publishActiveDuration(t, ctx, store, tenant, 100, 200)
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-01", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		snap := res.Search.ServiceDurationSnapshot
		if snap.PolicyID == nil || *snap.PolicyID != policy.ID || snap.PolicyVersion == nil || *snap.PolicyVersion != policy.Version {
			t.Fatalf("server policy not resolved: %+v want %s v%d", snap, policy.ID, policy.Version)
		}
		if snap.PickupSeconds == nil || *snap.PickupSeconds != 100 || snap.DeliverySeconds == nil || *snap.DeliverySeconds != 200 {
			t.Fatalf("effective durations wrong: %+v", snap)
		}
	})

	t.Run("I2B-DUR-02 caller fake duration cannot become canonical", func(t *testing.T) {
		tenant := uuid.New()
		_ = publishActiveDuration(t, ctx, store, tenant, 30, 40)
		fake := chainorch.KnownZeroDurationSnapshot(uuid.New(), 99)
		fp := i2bFP(tenant)
		fp.Duration = fake // must be ignored
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-02", fp))
		if err != nil {
			t.Fatal(err)
		}
		if res.Search.ServiceDurationSnapshot.State != repository.ServiceDurationKnown {
			t.Fatal("expected server KNOWN snapshot")
		}
		if res.Search.ServiceDurationSnapshot.PickupSeconds == nil || *res.Search.ServiceDurationSnapshot.PickupSeconds != 30 {
			t.Fatalf("caller fake zero must not win: %+v", res.Search.ServiceDurationSnapshot)
		}
	})

	t.Run("I2B-DUR-03 persisted snapshot equals fingerprinted snapshot", func(t *testing.T) {
		tenant := uuid.New()
		_ = publishActiveDuration(t, ctx, store, tenant, 50, 60)
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		fpIn := i2bFP(tenant)
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-03", fpIn))
		if err != nil {
			t.Fatal(err)
		}
		fpIn.TenantID = tenant
		fpIn.Duration = res.Search.ServiceDurationSnapshot
		want := chainorch.ComputeRequestFingerprint(fpIn)
		if res.Search.RequestFingerprint != want {
			t.Fatalf("persisted fingerprint mismatch\n got=%s\nwant=%s", res.Search.RequestFingerprint, want)
		}
	})

	t.Run("I2B-DUR-04 policy change after claim does not affect run", func(t *testing.T) {
		tenant := uuid.New()
		_ = publishActiveDuration(t, ctx, store, tenant, 70, 80)
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-04", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		before := res.Search.ServiceDurationSnapshot
		_ = publishActiveDuration(t, ctx, store, tenant, 700, 800)
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if !chainorch.SnapshotsEquivalent(before, got.ServiceDurationSnapshot) {
			t.Fatalf("stored snapshot mutated after policy publish before=%+v after=%+v", before, got.ServiceDurationSnapshot)
		}
		var req chain.Request
		if err := chainorch.ApplyStoredDurationToChainRequest(&req, got.ServiceDurationSnapshot); err != nil {
			t.Fatal(err)
		}
		if req.ActionServiceSeconds[routeplan.ActionPickup] != 70 {
			t.Fatalf("execution must keep claim-time pickup 70 got=%v", req.ActionServiceSeconds)
		}
	})

	t.Run("I2B-DUR-05 real chain/route-plan consumes stored snapshot", func(t *testing.T) {
		tenant := uuid.New()
		snap := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: uuid.New(), Version: 1, PickupSeconds: 15, DeliverySeconds: 25,
		})
		orch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: snap}}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-05", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := chainorch.PrepareDurableChainRequest(res.Search, chain.Request{})
		if err != nil {
			t.Fatal(err)
		}
		if prepared.ActionServiceSeconds[routeplan.ActionPickup] != 15 || prepared.ActionServiceSeconds[routeplan.ActionDelivery] != 25 {
			t.Fatalf("prepared actions=%v", prepared.ActionServiceSeconds)
		}
	})

	t.Run("I2B-DUR-06 asymmetric pickup/delivery consumed", func(t *testing.T) {
		tenant := uuid.New()
		snap := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: uuid.New(), Version: 1, PickupSeconds: 900, DeliverySeconds: 1200,
		})
		orch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: snap}}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-06", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		var req chain.Request
		if err := chainorch.ApplyStoredDurationToChainRequest(&req, res.Search.ServiceDurationSnapshot); err != nil {
			t.Fatalf("asymmetric must not fail closed: %v", err)
		}
		if req.ActionServiceSeconds[routeplan.ActionPickup] != 900 || req.ActionServiceSeconds[routeplan.ActionDelivery] != 1200 {
			t.Fatalf("actions=%v", req.ActionServiceSeconds)
		}
	})

	t.Run("I2B-DUR-07 KNOWN_ZERO distinct through real execution wiring", func(t *testing.T) {
		tenant := uuid.New()
		zero := chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)
		unknown := chainorch.UnknownDurationSnapshot()
		zOrch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: zero}}
		uOrch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: unknown}}
		zr, err := zOrch.Claim(ctx, i2bClaim(tenant, "dur-07z", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		ur, err := uOrch.Claim(ctx, i2bClaim(tenant, "dur-07u", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		zp, err := chainorch.PrepareDurableChainRequest(zr.Search, chain.Request{})
		if err != nil {
			t.Fatal(err)
		}
		up, err := chainorch.PrepareDurableChainRequest(ur.Search, chain.Request{})
		if err != nil {
			t.Fatal(err)
		}
		if zp.ActionServiceSeconds == nil || zp.ActionServiceSeconds[routeplan.ActionPickup] != 0 {
			t.Fatalf("known zero actions=%v", zp.ActionServiceSeconds)
		}
		if up.ActionServiceSeconds != nil {
			t.Fatalf("unknown must leave action map nil got=%v", up.ActionServiceSeconds)
		}
		if zr.Search.RequestFingerprint == ur.Search.RequestFingerprint {
			t.Fatal("fingerprints must differ")
		}
	})

	t.Run("I2B-DUR-08 same key + newly resolved changed policy -> conflict", func(t *testing.T) {
		tenant := uuid.New()
		key := "dur-08"
		_ = publishActiveDuration(t, ctx, store, tenant, 11, 12)
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		if _, err := orch.Claim(ctx, i2bClaim(tenant, key, i2bFP(tenant))); err != nil {
			t.Fatal(err)
		}
		_ = publishActiveDuration(t, ctx, store, tenant, 21, 22)
		res, err := orch.Claim(ctx, i2bClaim(tenant, key, i2bFP(tenant)))
		if !errors.Is(err, chainorch.ErrFingerprintConflict) {
			t.Fatalf("want conflict got outcome=%s err=%v", res.Outcome, err)
		}
	})

	t.Run("I2B-FP-09 caller cannot choose persisted search_id", func(t *testing.T) {
		tenant := uuid.New()
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.UnknownDurationSnapshot()},
		}
		req := i2bClaim(tenant, "fp-09", i2bFP(tenant))
		// ClaimRequest has no SearchID field — structural proof plus two claims get distinct ids.
		a, err := orch.Claim(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		b, err := orch.Claim(ctx, i2bClaim(tenant, "fp-09b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if a.Search.ID == uuid.Nil || b.Search.ID == uuid.Nil || a.Search.ID == b.Search.ID {
			t.Fatalf("search ids must be internally generated distinct a=%s b=%s", a.Search.ID, b.Search.ID)
		}
	})

	t.Run("I2B-DUR-09 nil DurationResolver fails closed and creates no search", func(t *testing.T) {
		tenant := uuid.New()
		orch := &chainorch.Orchestrator{Store: store} // DurationResolver intentionally nil
		_, err := orch.Claim(ctx, i2bClaim(tenant, "dur-09", i2bFP(tenant)))
		if !errors.Is(err, chainorch.ErrDurationResolverRequired) {
			t.Fatalf("want ErrDurationResolverRequired got %v", err)
		}
		var n int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.chain_search_runs WHERE tenant_id=$1`, tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("nil resolver must create no search got count=%d", n)
		}
	})

	t.Run("I2B-DUR-10 Postgres resolver with no active policy returns UNKNOWN", func(t *testing.T) {
		tenant := uuid.New()
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-10", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		if res.Search.ServiceDurationSnapshot.State != repository.ServiceDurationUnknown {
			t.Fatalf("want UNKNOWN got %+v", res.Search.ServiceDurationSnapshot)
		}
		if res.Search.ServiceDurationSnapshot.FingerprintProvenance != "NO_ACTIVE_POLICY" {
			t.Fatalf("provenance=%s", res.Search.ServiceDurationSnapshot.FingerprintProvenance)
		}
	})

	t.Run("I2B-DUR-11 Postgres resolver with active policy never silently UNKNOWN", func(t *testing.T) {
		tenant := uuid.New()
		policy := publishActiveDuration(t, ctx, store, tenant, 111, 222)
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-11", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		snap := res.Search.ServiceDurationSnapshot
		if snap.State != repository.ServiceDurationKnown {
			t.Fatalf("active policy must not yield UNKNOWN: %+v", snap)
		}
		if snap.PolicyID == nil || *snap.PolicyID != policy.ID {
			t.Fatalf("policy id mismatch %+v", snap)
		}
		if snap.PickupSeconds == nil || *snap.PickupSeconds != 111 || snap.DeliverySeconds == nil || *snap.DeliverySeconds != 222 {
			t.Fatalf("durations %+v", snap)
		}
	})
}

func TestI2B_RealEngineDurationProof(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)

	t.Run("I2B-DUR-12 stored durations reach real route-plan evaluation", func(t *testing.T) {
		tenant := uuid.New()
		snap := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: uuid.New(), Version: 1, PickupSeconds: 900, DeliverySeconds: 1200,
		})
		orch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: snap}}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-12", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		clock := time.Now().UTC()
		base := durationExecRequest(tenant, clock)
		base.Loads = durationSensitiveLoads(tenant, clock)
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, res.Search, base)
		if err != nil {
			t.Fatal(err)
		}
		if out.PlannerInvocations == 0 || out.Evaluations == 0 {
			t.Fatalf("expected real planner path invocations=%d evaluations=%d", out.PlannerInvocations, out.Evaluations)
		}
		if durableFeasible(out) {
			t.Fatal("stored 900/1200 must miss tight second-delivery window in real evaluation")
		}
	})

	t.Run("I2B-DUR-13 asymmetric pickup=900 / delivery=1200 honored by real evaluation", func(t *testing.T) {
		tenant := uuid.New()
		asymmetric := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: uuid.New(), Version: 1, PickupSeconds: 900, DeliverySeconds: 1200,
		})
		pickupOnly := chainorch.SnapshotFromPolicy(repository.ServiceDurationPolicy{
			ID: uuid.New(), Version: 1, PickupSeconds: 900, DeliverySeconds: 0,
		})
		clock := time.Now().UTC()
		loads := durationSensitiveLoads(tenant, clock)

		aOrch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: asymmetric}}
		aRes, err := aOrch.Claim(ctx, i2bClaim(tenant, "dur-13a", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		aBase := durationExecRequest(tenant, clock)
		aBase.Loads = loads
		aOut, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, aRes.Search, aBase)
		if err != nil {
			t.Fatal(err)
		}

		pOrch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: pickupOnly}}
		pRes, err := pOrch.Claim(ctx, i2bClaim(tenant, "dur-13b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		pBase := durationExecRequest(tenant, clock)
		pBase.Loads = loads
		pOut, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, pRes.Search, pBase)
		if err != nil {
			t.Fatal(err)
		}

		if durableFeasible(aOut) {
			t.Fatal("900/1200 must miss second-delivery window")
		}
		if !durableFeasible(pOut) {
			t.Fatalf("900/0 must remain feasible — proves delivery=1200 is what fails the plan status=%s planner=%d candidates=%+v reasons=%v aFeasible=%v",
				pOut.Status, pOut.PlannerInvocations, pOut.Candidates, pOut.ReasonCounts, durableFeasible(aOut))
		}
		if aOut.PlannerInvocations == 0 || pOut.PlannerInvocations == 0 {
			t.Fatal("both runs must exercise Planner")
		}
	})

	t.Run("I2B-DUR-14 policy change after claim cannot change real execution values", func(t *testing.T) {
		tenant := uuid.New()
		_ = publishActiveDuration(t, ctx, store, tenant, 900, 1200)
		orch := &chainorch.Orchestrator{
			Store: store, DurationResolver: chainorch.PostgresDurationResolver{Store: store},
		}
		res, err := orch.Claim(ctx, i2bClaim(tenant, "dur-14", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		_ = publishActiveDuration(t, ctx, store, tenant, 0, 0)
		clock := time.Now().UTC()
		base := durationExecRequest(tenant, clock)
		base.Loads = durationSensitiveLoads(tenant, clock)
		out, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, res.Search, base)
		if err != nil {
			t.Fatal(err)
		}
		if durableFeasible(out) {
			t.Fatal("execution must keep claim-time 900/1200 (infeasible), not post-claim zero policy")
		}
		if out.PlannerInvocations == 0 {
			t.Fatal("must reach planner")
		}
	})

	t.Run("I2B-DUR-15 KNOWN_ZERO and UNKNOWN produce distinct real evaluation semantics", func(t *testing.T) {
		tenant := uuid.New()
		zero := chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)
		unknown := chainorch.UnknownDurationSnapshot()
		clock := time.Now().UTC()
		loads := durationSensitiveLoads(tenant, clock)

		zOrch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: zero}}
		zRes, err := zOrch.Claim(ctx, i2bClaim(tenant, "dur-15z", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		zBase := durationExecRequest(tenant, clock)
		zBase.Loads = loads
		zOut, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, zRes.Search, zBase)
		if err != nil {
			t.Fatal(err)
		}

		uOrch := &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: unknown}}
		uRes, err := uOrch.Claim(ctx, i2bClaim(tenant, "dur-15u", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		uBase := durationExecRequest(tenant, clock)
		uBase.Loads = loads
		uOut, err := chainorch.RunDurableChainSearch(ctx, chain.Engine{}, uRes.Search, uBase)
		if err != nil {
			t.Fatal(err)
		}

		if !durableFeasible(zOut) {
			t.Fatalf("KNOWN_ZERO must be feasible on duration-sensitive windows status=%s planner=%d evals=%d providers=%d candidates=%+v reasons=%v",
				zOut.Status, zOut.PlannerInvocations, zOut.Evaluations, zOut.ProviderCalls, zOut.Candidates, zOut.ReasonCounts)
		}
		if durableFeasible(uOut) {
			t.Fatal("UNKNOWN must not score as feasible")
		}
		if !durableHasReason(uOut, routeplan.ReasonServiceDurationUnknown) {
			t.Fatalf("UNKNOWN must surface SERVICE_DURATION_UNKNOWN got candidates=%+v", uOut.Candidates)
		}
		if durableHasReason(zOut, routeplan.ReasonServiceDurationUnknown) {
			t.Fatal("KNOWN_ZERO must not carry SERVICE_DURATION_UNKNOWN")
		}
	})
}

func durationExecRequest(tenant uuid.UUID, clock time.Time) chain.Request {
	return durableExecRequest(tenant, &countingProvider{}, clock)
}

// durationSensitiveLoads builds a two-load set where L2 pickup shares L1 delivery
// location (few Planner sequences under LegsPerOrderMax) and L2 delivery window
// distinguishes asymmetric service on the combined stop:
//   - pickup=900 delivery=0 → combined stop service 900 → L2 delivery still fits
//   - pickup=900 delivery=1200 → combined stop service 2100 → L2 delivery misses
func durationSensitiveLoads(tenant uuid.UUID, clock time.Time) []chain.VisibleLoad {
	anchor := clock.Add(time.Hour)
	stackable := true
	l1 := visibleLoad(tenant, clock, 1)
	l2 := visibleLoad(tenant, clock, 2)
	l1.Stackable = &stackable
	l2.Stackable = &stackable
	l1.PickupStart, l1.PickupEnd = anchor, anchor.Add(2*time.Hour)
	l1.DeliveryStart, l1.DeliveryEnd = anchor, anchor.Add(2*time.Hour)
	// Co-locate L2 pickup with L1 delivery so Planner attaches rather than enumerate-insert.
	l2.Pickup = l1.Delivery
	l2.PickupStart, l2.PickupEnd = anchor, anchor.Add(2*time.Hour)
	// Tight L2 delivery end: ~1920s from anchor with 900 combined service fits;
	// ~3120s with 2100 combined service misses.
	l2.DeliveryStart, l2.DeliveryEnd = anchor, anchor.Add(40*time.Minute)
	return []chain.VisibleLoad{l1, l2}
}

func durableFeasible(result chain.Result) bool {
	for _, c := range result.Candidates {
		if c.Feasible {
			return true
		}
	}
	return false
}

func durableHasReason(result chain.Result, reason string) bool {
	for _, c := range result.Candidates {
		for _, code := range c.ReasonCodes {
			if code == reason {
				return true
			}
		}
	}
	return false
}

func publishActiveDuration(t *testing.T, ctx context.Context, store *repository.Postgres, tenant uuid.UUID, pickup, delivery int) repository.ServiceDurationPolicy {
	t.Helper()
	var published repository.ServiceDurationPolicy
	err := store.Within(ctx, func(tx repository.Tx) error {
		version, err := tx.NextServiceDurationVersion(ctx, tenant)
		if err != nil {
			return err
		}
		draft := repository.ServiceDurationPolicy{
			ID: uuid.New(), TenantID: tenant, Version: version, Status: repository.DurationPolicyDraft,
			PickupSeconds: pickup, DeliverySeconds: delivery, CreatedAt: time.Now().UTC(),
		}
		if err := tx.InsertServiceDurationDraft(ctx, draft); err != nil {
			return err
		}
		published, err = tx.PublishServiceDurationPolicy(ctx, tenant, draft.ID, time.Now().UTC())
		return err
	})
	if err != nil {
		t.Fatalf("publish duration: %v", err)
	}
	return published
}
