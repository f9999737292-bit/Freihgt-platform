//go:build integration

package chainorch_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestI2B_E2E_DurableExecuteAndTerminalize(t *testing.T) {
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

	t.Run("I2B-E2E-01 claim execute DB COMPLETED", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-01", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("outcome=%s status=%s engine=%s fail=%v reasons=%v", out.Outcome, out.Search.Status, out.EngineResult.Status, out.Search.FailureCode, out.EngineResult.ReasonCounts)
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchCompleted)
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if !got.SearchComplete || got.GlobalOptimum || got.ExecutionSupported {
			t.Fatalf("flags complete=%v opt=%v exec=%v", got.SearchComplete, got.GlobalOptimum, got.ExecutionSupported)
		}
		if got.CompletedAt == nil {
			t.Fatal("completed_at must be DB-owned non-nil")
		}
	})

	t.Run("I2B-E2E-02..06 real engine shortlist persisted", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-02", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("outcome=%s engine=%s fail=%v calls=%d", out.Outcome, out.EngineResult.Status, out.Search.FailureCode, out.EngineResult.ProviderCalls)
		}
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted) == 0 {
			t.Fatal("expected durable shortlist rows from real engine")
		}
		if len(persisted) != len(out.EngineResult.Candidates) {
			t.Fatalf("persisted=%d engine=%d", len(persisted), len(out.EngineResult.Candidates))
		}
		seen := map[uuid.UUID]struct{}{}
		for i, row := range persisted {
			eng := out.EngineResult.Candidates[i]
			if row.Ordinal != i+1 {
				t.Fatalf("E2E-03 ordinal=%d want %d", row.Ordinal, i+1)
			}
			if row.CandidateID != eng.CandidateID {
				t.Fatalf("E2E-04 id mismatch ordinal=%d", i+1)
			}
			if _, dup := seen[row.CandidateID]; dup {
				t.Fatalf("E2E-04 duplicate candidate_id %s", row.CandidateID)
			}
			seen[row.CandidateID] = struct{}{}
			if !(row.FirstLoadID.String() < row.SecondLoadID.String()) {
				t.Fatalf("E2E-05 canonical pair broken %s >= %s", row.FirstLoadID, row.SecondLoadID)
			}
			if row.FirstLoadVersion < 1 || row.SecondLoadVersion < 1 {
				t.Fatalf("E2E-05 versions %d/%d", row.FirstLoadVersion, row.SecondLoadVersion)
			}
			if eng.Feasible && row.FeasibilityStatus != repository.ChainFeasible {
				t.Fatalf("E2E-06 feasible mapping got %s", row.FeasibilityStatus)
			}
			if row.RoutePlanID != nil {
				t.Fatal("route_plan_id must be nil without durable route-plan")
			}
		}
	})

	t.Run("I2B-E2E-07 UNFINISHED remains UNFINISHED", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-07", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		a, b := uuid.New(), uuid.New()
		if b.String() < a.String() {
			a, b = b, a
		}
		controlled := chain.Result{Candidates: []chain.ProjectedCandidate{{
			CandidateID: uuid.New(),
			Loads:       []chain.ProjectedLoad{{LoadID: a, Version: 1}, {LoadID: b, Version: 1}},
			Feasible:    false, ReasonCodes: []string{chain.ReasonUnfinished},
		}}}
		projected, err := chainorch.ProjectDurableCandidates(res.Search, controlled)
		if err != nil {
			t.Fatal(err)
		}
		orch.AfterProjectCandidates = func([]repository.ChainCandidate) []repository.ChainCandidate {
			return projected
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("outcome=%s engine=%s", out.Outcome, out.EngineResult.Status)
		}
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted) != 1 || persisted[0].FeasibilityStatus != repository.ChainUnfinished {
			t.Fatalf("want UNFINISHED got %+v", persisted)
		}
		if persisted[0].SelectedOrder != nil {
			t.Fatal("UNFINISHED must not set selected_order")
		}
	})

	t.Run("I2B-E2E-08 INFEASIBLE remains INFEASIBLE", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-08", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		a, b := uuid.New(), uuid.New()
		if b.String() < a.String() {
			a, b = b, a
		}
		controlled := chain.Result{Candidates: []chain.ProjectedCandidate{{
			CandidateID: uuid.New(),
			Loads:       []chain.ProjectedLoad{{LoadID: a, Version: 1}, {LoadID: b, Version: 1}},
			Feasible:    false, ReasonCodes: []string{chain.ReasonNotFeasible, chain.ReasonStackingUnknown},
		}}}
		projected, err := chainorch.ProjectDurableCandidates(res.Search, controlled)
		if err != nil {
			t.Fatal(err)
		}
		if projected[0].FeasibilityStatus != repository.ChainInfeasible {
			t.Fatalf("projection must keep INFEASIBLE got %s", projected[0].FeasibilityStatus)
		}
		orch.AfterProjectCandidates = func([]repository.ChainCandidate) []repository.ChainCandidate { return projected }
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("outcome=%s engine=%s", out.Outcome, out.EngineResult.Status)
		}
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted) != 1 || persisted[0].FeasibilityStatus != repository.ChainInfeasible {
			t.Fatalf("want INFEASIBLE got statuses=%v", statuses(persisted))
		}
		if persisted[0].FeasibilityStatus == repository.ChainUnfinished {
			t.Fatal("must not collapse INFEASIBLE into UNFINISHED")
		}
	})

	t.Run("I2B-E2E-09 known metrics preserved unknowns NULL", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-09", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		a, b := uuid.New(), uuid.New()
		if b.String() < a.String() {
			a, b = b, a
		}
		util := 0.42
		target := 900
		rev := int64(100)
		cur := "EUR"
		empty := 50
		dur := 120
		controlled := chain.Result{Candidates: []chain.ProjectedCandidate{
			{
				CandidateID: uuid.New(),
				Loads:       []chain.ProjectedLoad{{LoadID: a, Version: 1}, {LoadID: b, Version: 1}},
				Feasible:    true, SelectedOrder: chain.OrderLowerFirst,
				EmptyRoadMeters: &empty, DurationSeconds: &dur,
				Utilization: &util, TargetMeters: &target,
				RevenueAmount: &rev, RevenueCurrency: &cur,
			},
			{
				CandidateID: uuid.New(),
				Loads: []chain.ProjectedLoad{
					{LoadID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Version: 1},
					{LoadID: uuid.MustParse("10000000-0000-0000-0000-000000000002"), Version: 1},
				},
				Feasible: true, SelectedOrder: chain.OrderHigherFirst,
				EmptyRoadMeters: intPtr(1), DurationSeconds: intPtr(2),
				OmittedComponents: []string{chain.OmitUtilization, chain.OmitTarget, chain.OmitRevenue},
			},
		}}
		projected, err := chainorch.ProjectDurableCandidates(res.Search, controlled)
		if err != nil {
			t.Fatal(err)
		}
		orch.AfterProjectCandidates = func([]repository.ChainCandidate) []repository.ChainCandidate { return projected }
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil || out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("err=%v outcome=%s engine=%s", err, out.Outcome, out.EngineResult.Status)
		}
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted[0].Utilization == nil || *persisted[0].Utilization != util {
			t.Fatalf("known utilization lost %+v", persisted[0].Utilization)
		}
		if persisted[0].TargetDistanceM == nil || *persisted[0].TargetDistanceM != float64(target) {
			t.Fatalf("known target lost %+v", persisted[0].TargetDistanceM)
		}
		if persisted[0].RevenueAmount == nil || *persisted[0].RevenueAmount != rev {
			t.Fatalf("known revenue lost %+v", persisted[0].RevenueAmount)
		}
		if persisted[1].Utilization != nil || persisted[1].TargetDistanceM != nil || persisted[1].RevenueAmount != nil {
			t.Fatalf("unknown metrics must stay NULL got util=%v target=%v rev=%v",
				persisted[1].Utilization, persisted[1].TargetDistanceM, persisted[1].RevenueAmount)
		}
	})

	t.Run("I2B-E2E-10 candidate fingerprint deterministic", func(t *testing.T) {
		run := repository.ChainSearchRun{
			ID: uuid.New(), TenantID: uuid.New(),
			RequestFingerprint: "fp-e2e-10", AlgorithmVersion: chainorch.AlgorithmVersion, PolicyVersion: "pol",
		}
		a, b := uuid.New(), uuid.New()
		if b.String() < a.String() {
			a, b = b, a
		}
		cand := chain.ProjectedCandidate{
			CandidateID: uuid.New(),
			Loads:       []chain.ProjectedLoad{{LoadID: a, Version: 1}, {LoadID: b, Version: 1}},
			Feasible:    true, SelectedOrder: chain.OrderLowerFirst,
		}
		r1, err := chainorch.ProjectDurableCandidates(run, chain.Result{Candidates: []chain.ProjectedCandidate{cand}})
		if err != nil {
			t.Fatal(err)
		}
		r2, err := chainorch.ProjectDurableCandidates(run, chain.Result{Candidates: []chain.ProjectedCandidate{cand}})
		if err != nil {
			t.Fatal(err)
		}
		if r1[0].CandidateFingerprint == "" || r1[0].CandidateFingerprint != r2[0].CandidateFingerprint {
			t.Fatalf("fingerprint not deterministic %q vs %q", r1[0].CandidateFingerprint, r2[0].CandidateFingerprint)
		}
	})

	t.Run("I2B-E2E-11 run counters equal engine result", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-11", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("outcome=%s engine=%s", out.Outcome, out.EngineResult.Status)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CandidateCount != len(persisted) {
			t.Fatalf("candidate_count=%d rows=%d", got.CandidateCount, len(persisted))
		}
		if got.EvaluatedPairCount != out.EngineResult.Evaluations {
			t.Fatalf("evaluated_pair_count=%d evaluations=%d", got.EvaluatedPairCount, out.EngineResult.Evaluations)
		}
		if got.ProviderCallCount != out.EngineResult.ProviderCalls {
			t.Fatalf("provider_call_count=%d provider_calls=%d", got.ProviderCallCount, out.EngineResult.ProviderCalls)
		}
	})

	t.Run("I2B-E2E-12 budget exhaustion -> FAILED CHAIN_BUDGET_EXHAUSTED", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-12", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res.Search.ID, 2*time.Second)
		block := &blockingProvider{blockFor: 8 * time.Second}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, block, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeFailed {
			t.Fatalf("outcome=%s status=%s", out.Outcome, out.Search.Status)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchFailed {
			t.Fatalf("status=%s", got.Status)
		}
		if got.FailureCode == nil || *got.FailureCode != chainorch.FailureBudgetExhausted {
			t.Fatalf("failure_code=%v", got.FailureCode)
		}
		if got.SearchComplete {
			t.Fatal("search_complete must be false")
		}
		assertStatus(t, ctx, store, tenant, res.Search.ID, repository.ChainSearchFailed)
		assertTerminalCountersMatchEngine(t, ctx, store, tenant, res.Search.ID, out.EngineResult, 0)
	})

	t.Run("I2B-E2E-13 context cancel -> CANCELLED via cleanup", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-13", i2bFP(tenant)))
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
			t.Fatalf("outcome=%s status=%s engine=%s", out.Outcome, out.Search.Status, out.EngineResult.Status)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchCancelled {
			t.Fatalf("status=%s", got.Status)
		}
		if got.CancelReasonClass == nil || *got.CancelReasonClass != chainorch.CancelContextCancelled {
			t.Fatalf("cancel_reason=%v", got.CancelReasonClass)
		}
		if got.CancelledAt == nil {
			t.Fatal("cancelled_at must be set")
		}
		if got.SearchComplete {
			t.Fatal("search_complete must be false")
		}
		assertTerminalCountersMatchEngine(t, ctx, store, tenant, res.Search.ID, out.EngineResult, 0)
	})

	t.Run("I2B-E2E-14/15 atomic shortlist terminalization", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-15", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		orch.AfterProjectCandidates = func(rows []repository.ChainCandidate) []repository.ChainCandidate {
			if len(rows) == 0 {
				a, b := uuid.New(), uuid.New()
				if b.String() < a.String() {
					a, b = b, a
				}
				rows = []repository.ChainCandidate{{
					CandidateID: uuid.New(), SearchID: res.Search.ID, TenantID: tenant,
					Ordinal: 1, FirstLoadID: a, FirstLoadVersion: 1, SecondLoadID: b, SecondLoadVersion: 1,
					FeasibilityStatus: repository.ChainUnfinished, CandidateFingerprint: "ok",
					OmittedComponents: []string{}, ReasonCodes: []string{chain.ReasonUnfinished},
				}}
			}
			poisoned := append([]repository.ChainCandidate{}, rows...)
			// Force CHECK failure: ordinal outside 1..4.
			poisoned[0].Ordinal = 0
			return poisoned
		}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		_, err = orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err == nil {
			t.Fatal("expected candidate persistence failure")
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status == repository.ChainSearchCompleted {
			t.Fatal("E2E-15 COMPLETED must not persist on candidate failure")
		}
		if got.Status != repository.ChainSearchRunning {
			t.Fatalf("E2E-15 expect RUNNING for cleanup got %s", got.Status)
		}
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted) != 0 {
			t.Fatalf("E2E-15 partial shortlist committed rows=%d", len(persisted))
		}
	})

	t.Run("I2B-E2E-16/17 terminal replay same shortlist zero recompute", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		fp := i2bFP(tenant)
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-16", fp))
		if err != nil {
			t.Fatal(err)
		}
		prov := &countingProvider{}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, prov, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil || out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("err=%v outcome=%s engine=%s", err, out.Outcome, out.EngineResult.Status)
		}
		firstCalls := atomic.LoadInt32(&prov.calls)
		firstRows, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-16", fp))
		if err != nil || replay.Outcome != chainorch.OutcomeReplay {
			t.Fatalf("replay outcome=%s err=%v", replay.Outcome, err)
		}
		if replay.Search.ID != res.Search.ID {
			t.Fatal("replay must return same durable search id")
		}
		replayRows, err := listCands(t, ctx, store, tenant, replay.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(replayRows) != len(firstRows) {
			t.Fatalf("shortlist len %d vs %d", len(replayRows), len(firstRows))
		}
		for i := range firstRows {
			if replayRows[i].CandidateID != firstRows[i].CandidateID || replayRows[i].CandidateFingerprint != firstRows[i].CandidateFingerprint {
				t.Fatalf("shortlist drift at %d", i)
			}
		}
		prov2 := &countingProvider{}
		base2 := durableExecRequest(tenant, prov2, clock)
		base2.Loads = twoStackableLoads(tenant, clock)
		_, err = orch.ExecuteOwnedDurableChainSearch(ctx, tenant, replay.Search.ID, res.OwnerToken, chain.Engine{}, base2, chainorch.ExecuteOwnedOptions{})
		if !errors.Is(err, chainorch.ErrSearchNotRunnable) {
			t.Fatalf("terminal execute want ErrSearchNotRunnable got %v", err)
		}
		if atomic.LoadInt32(&prov2.calls) != 0 {
			t.Fatalf("E2E-17 provider calls=%d want 0", prov2.calls)
		}
		if atomic.LoadInt32(&prov.calls) != firstCalls {
			t.Fatalf("original provider mutated after replay")
		}
	})

	t.Run("I2B-E2E-18 private fields absent from durable shortlist", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-18", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		clock := time.Now().UTC()
		loads := twoStackableLoads(tenant, clock)
		for i := range loads {
			loads[i].Private.ShipperName = "SECRET_SHIPPER_ACME"
			loads[i].Private.CustomerName = "SECRET_CUSTOMER_BETA"
			loads[i].Private.ContractRate = "SECRET_CONTRACT_99"
			loads[i].Private.ExactAddress = "SECRET_ADDR_1"
		}
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = loads
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil || out.Outcome != chainorch.ExecuteOutcomeCompleted {
			t.Fatalf("err=%v outcome=%s engine=%s", err, out.Outcome, out.EngineResult.Status)
		}
		persisted, err := listCands(t, ctx, store, tenant, res.Search.ID)
		if err != nil {
			t.Fatal(err)
		}
		blob, err := json.Marshal(persisted)
		if err != nil {
			t.Fatal(err)
		}
		s := string(blob)
		for _, secret := range []string{"SECRET_SHIPPER_ACME", "SECRET_CUSTOMER_BETA", "SECRET_CONTRACT_99", "SECRET_ADDR_1"} {
			if strings.Contains(s, secret) {
				t.Fatalf("private field leaked into durable shortlist: %s", secret)
			}
		}
		for _, row := range persisted {
			if strings.Contains(row.CandidateFingerprint, "SECRET_") {
				t.Fatal("fingerprint must not contain private text")
			}
			for _, code := range row.ReasonCodes {
				if strings.Contains(code, "SECRET_") {
					t.Fatal("reason_codes must not contain private text")
				}
			}
		}
	})

	t.Run("I2B-E2E-19 budget after real provider work retains counters", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-19", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res.Search.ID, 2*time.Second)
		block := &blockingProvider{blockFor: 8 * time.Second}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, block, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeFailed {
			t.Fatalf("outcome=%s", out.Outcome)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchFailed {
			t.Fatalf("status=%s", got.Status)
		}
		if got.FailureCode == nil || *got.FailureCode != chainorch.FailureBudgetExhausted {
			t.Fatalf("failure_code=%v", got.FailureCode)
		}
		if out.EngineResult.ProviderCalls == 0 {
			t.Fatal("expected real provider work before budget terminalization")
		}
		assertTerminalCountersMatchEngine(t, ctx, store, tenant, res.Search.ID, out.EngineResult, 0)
		if !reasonCountsEqual(got.ReasonCounts, out.EngineResult.ReasonCounts) {
			t.Fatalf("reason_counts got=%v engine=%v", got.ReasonCounts, out.EngineResult.ReasonCounts)
		}
	})

	t.Run("I2B-E2E-20 cancel after real provider call retains counters", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-20", i2bFP(tenant)))
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
			t.Fatalf("outcome=%s engine=%s", out.Outcome, out.EngineResult.Status)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchCancelled {
			t.Fatalf("status=%s", got.Status)
		}
		if out.EngineResult.ProviderCalls == 0 || got.ProviderCallCount == 0 {
			t.Fatalf("expected provider work calls engine=%d db=%d", out.EngineResult.ProviderCalls, got.ProviderCallCount)
		}
		assertTerminalCountersMatchEngine(t, ctx, store, tenant, res.Search.ID, out.EngineResult, 0)
		if !reasonCountsEqual(got.ReasonCounts, out.EngineResult.ReasonCounts) {
			t.Fatalf("reason_counts must be preserved got=%v engine=%v", got.ReasonCounts, out.EngineResult.ReasonCounts)
		}
		if len(got.ReasonCounts) == 0 && len(out.EngineResult.ReasonCounts) > 0 {
			t.Fatal("cancelled path must not drop engine reason_counts")
		}
	})

	t.Run("I2B-E2E-21 unexpected failure after measurable work keeps counters", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-21", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		orch.RewriteEngineResultForTest = func(r chain.Result) chain.Result {
			if r.ProviderCalls == 0 {
				r.ProviderCalls = 3
			}
			if r.Evaluations == 0 {
				r.Evaluations = 1
			}
			if r.ReasonCounts == nil {
				r.ReasonCounts = map[string]int{}
			}
			r.ReasonCounts["MEASURABLE_WORK"] = 1
			r.Status = "NOT_A_REAL_ENGINE_STATUS"
			r.SearchComplete = false
			return r
		}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, &countingProvider{}, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res.Search.ID, res.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != chainorch.ExecuteOutcomeFailed {
			t.Fatalf("outcome=%s", out.Outcome)
		}
		got := getRun(t, ctx, store, tenant, res.Search.ID)
		if got.Status != repository.ChainSearchFailed {
			t.Fatalf("status=%s", got.Status)
		}
		if got.FailureCode == nil || *got.FailureCode != chainorch.FailureUnexpectedEngineStatus {
			t.Fatalf("failure_code=%v", got.FailureCode)
		}
		if got.ProviderCallCount == 0 || got.EvaluatedPairCount == 0 {
			t.Fatalf("counters reset to zero provider=%d eval=%d", got.ProviderCallCount, got.EvaluatedPairCount)
		}
		assertTerminalCountersMatchEngine(t, ctx, store, tenant, res.Search.ID, out.EngineResult, 0)
		if got.ReasonCounts["UNEXPECTED_STATUS"] < 1 {
			t.Fatalf("expected classification reason got=%v", got.ReasonCounts)
		}
		if got.ReasonCounts["MEASURABLE_WORK"] != 1 {
			t.Fatalf("engine reason_counts not preserved got=%v", got.ReasonCounts)
		}
	})

	t.Run("I2B-E2E-22 canonical terminalization ignores arbitrary caller counters", func(t *testing.T) {
		orch := baseOrch()
		tenant := uuid.New()
		res, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-22", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		// Low-level FailSearch can write arbitrary stats — not the canonical path.
		if _, err := orch.BeginRunning(ctx, tenant, res.Search.ID, res.OwnerToken); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := orch.FailSearch(ctx, tenant, res.Search.ID, res.OwnerToken, repository.ChainSearchRunning, "TEST_ARBITRARY", chainorch.TerminalStats{
			CandidateCount: 0, EvaluatedPairCount: 99, ProviderCallCount: 99,
			ReasonCounts: map[string]int{"ARBITRARY": 1},
		}); err != nil || !ok {
			t.Fatalf("low-level fail ok err=%v", err)
		}
		poisoned := getRun(t, ctx, store, tenant, res.Search.ID)
		if poisoned.ProviderCallCount != 99 {
			t.Fatalf("setup: low-level path should accept explicit stats got=%d", poisoned.ProviderCallCount)
		}

		// Canonical ExecuteOwned derives counters only from the engine result.
		res2, err := orch.Claim(ctx, i2bClaim(tenant, "e2e-22b", i2bFP(tenant)))
		if err != nil {
			t.Fatal(err)
		}
		setDeadlineIn(t, ctx, pool, res2.Search.ID, 2*time.Second)
		block := &blockingProvider{blockFor: 8 * time.Second}
		clock := time.Now().UTC()
		base := durableExecRequest(tenant, block, clock)
		base.Loads = twoStackableLoads(tenant, clock)
		out, err := orch.ExecuteOwnedDurableChainSearch(ctx, tenant, res2.Search.ID, res2.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		got := getRun(t, ctx, store, tenant, res2.Search.ID)
		if got.ProviderCallCount != out.EngineResult.ProviderCalls || got.EvaluatedPairCount != out.EngineResult.Evaluations {
			t.Fatalf("canonical counters must equal engine got provider=%d/%d eval=%d/%d",
				got.ProviderCallCount, out.EngineResult.ProviderCalls, got.EvaluatedPairCount, out.EngineResult.Evaluations)
		}
		if got.ProviderCallCount == 99 || got.EvaluatedPairCount == 99 {
			t.Fatal("canonical path must not use arbitrary caller counters")
		}
		_ = chainorch.ExecuteOwnedOptions{} // no counter fields exist for callers to inject
	})
}

func listCands(t *testing.T, ctx context.Context, store *repository.Postgres, tenant, searchID uuid.UUID) ([]repository.ChainCandidate, error) {
	t.Helper()
	var out []repository.ChainCandidate
	err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var listErr error
		out, listErr = tx.ListChainCandidates(ctx, tenant, searchID)
		return listErr
	})
	return out, err
}

func assertTerminalCountersMatchEngine(t *testing.T, ctx context.Context, store *repository.Postgres, tenant, searchID uuid.UUID, engine chain.Result, wantCandidateCount int) {
	t.Helper()
	got := getRun(t, ctx, store, tenant, searchID)
	rows, err := listCands(t, ctx, store, tenant, searchID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CandidateCount != wantCandidateCount || got.CandidateCount != len(rows) {
		t.Fatalf("candidate_count=%d want=%d rows=%d", got.CandidateCount, wantCandidateCount, len(rows))
	}
	if got.EvaluatedPairCount != engine.Evaluations {
		t.Fatalf("evaluated_pair_count=%d evaluations=%d", got.EvaluatedPairCount, engine.Evaluations)
	}
	if got.ProviderCallCount != engine.ProviderCalls {
		t.Fatalf("provider_call_count=%d provider_calls=%d", got.ProviderCallCount, engine.ProviderCalls)
	}
}

func reasonCountsEqual(a, b map[string]int) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func statuses(rows []repository.ChainCandidate) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.FeasibilityStatus
	}
	return out
}

func intPtr(v int) *int { return &v }

func twoStackableLoads(tenant uuid.UUID, clock time.Time) []chain.VisibleLoad {
	stackable := true
	loads := make([]chain.VisibleLoad, 2)
	for i := 0; i < 2; i++ {
		loads[i] = visibleLoad(tenant, clock, i+1)
		loads[i].Stackable = &stackable
	}
	return loads
}
