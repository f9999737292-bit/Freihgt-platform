package chainorch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestProjectDurableCandidates_FeasibilityAndOrdinals(t *testing.T) {
	run := repository.ChainSearchRun{
		ID: uuid.New(), TenantID: uuid.New(),
		RequestFingerprint: "req-fp", AlgorithmVersion: AlgorithmVersion, PolicyVersion: "pol",
	}
	low := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	high := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	util := 0.5
	target := 100
	rev := int64(42)
	cur := "EUR"
	result := chain.Result{
		Candidates: []chain.ProjectedCandidate{
			{
				CandidateID: uuid.New(),
				Loads: []chain.ProjectedLoad{
					{LoadID: high, Version: 2}, {LoadID: low, Version: 1},
				},
				Feasible: true, SelectedOrder: chain.OrderLowerFirst,
				EmptyRoadMeters: intPtr(10), DurationSeconds: intPtr(20),
				Utilization: &util, TargetMeters: &target,
				RevenueAmount: &rev, RevenueCurrency: &cur,
			},
			{
				CandidateID: uuid.New(),
				Loads: []chain.ProjectedLoad{
					{LoadID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Version: 1},
					{LoadID: uuid.MustParse("00000000-0000-0000-0000-000000000004"), Version: 1},
				},
				Feasible: false, ReasonCodes: []string{chain.ReasonUnfinished},
			},
			{
				CandidateID: uuid.New(),
				Loads: []chain.ProjectedLoad{
					{LoadID: uuid.MustParse("00000000-0000-0000-0000-000000000005"), Version: 1},
					{LoadID: uuid.MustParse("00000000-0000-0000-0000-000000000006"), Version: 1},
				},
				Feasible: false, ReasonCodes: []string{chain.ReasonNotFeasible},
			},
		},
	}
	rows, err := ProjectDurableCandidates(run, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].Ordinal != 1 || rows[1].Ordinal != 2 || rows[2].Ordinal != 3 {
		t.Fatalf("ordinals %+v", []int{rows[0].Ordinal, rows[1].Ordinal, rows[2].Ordinal})
	}
	if rows[0].FirstLoadID != low || rows[0].SecondLoadID != high {
		t.Fatalf("canonical pair first=%s second=%s", rows[0].FirstLoadID, rows[0].SecondLoadID)
	}
	if rows[0].FirstLoadVersion != 1 || rows[0].SecondLoadVersion != 2 {
		t.Fatalf("versions first=%d second=%d", rows[0].FirstLoadVersion, rows[0].SecondLoadVersion)
	}
	if rows[0].FeasibilityStatus != repository.ChainFeasible {
		t.Fatalf("feasible=%s", rows[0].FeasibilityStatus)
	}
	if rows[0].SelectedOrder == nil || *rows[0].SelectedOrder != chain.OrderLowerFirst {
		t.Fatalf("selected_order=%v", rows[0].SelectedOrder)
	}
	if rows[0].Utilization == nil || *rows[0].Utilization != util {
		t.Fatalf("utilization=%v", rows[0].Utilization)
	}
	if rows[0].TargetDistanceM == nil || *rows[0].TargetDistanceM != 100 {
		t.Fatalf("target=%v", rows[0].TargetDistanceM)
	}
	if rows[0].RoutePlanID != nil {
		t.Fatal("route_plan_id must stay nil")
	}
	if rows[1].FeasibilityStatus != repository.ChainUnfinished {
		t.Fatalf("unfinished=%s", rows[1].FeasibilityStatus)
	}
	if rows[1].SelectedOrder != nil {
		t.Fatal("unfinished must not have selected_order")
	}
	if rows[2].FeasibilityStatus != repository.ChainInfeasible {
		t.Fatalf("infeasible=%s", rows[2].FeasibilityStatus)
	}
	fp1 := rows[0].CandidateFingerprint
	fp2 := ComputeCandidateFingerprint(CandidateFingerprintInput{
		RequestFingerprint: run.RequestFingerprint,
		AlgorithmVersion:   run.AlgorithmVersion,
		PolicyVersion:      run.PolicyVersion,
		CandidateID:        rows[0].CandidateID,
		FirstLoadID:        rows[0].FirstLoadID,
		FirstLoadVersion:   rows[0].FirstLoadVersion,
		SecondLoadID:       rows[0].SecondLoadID,
		SecondLoadVersion:  rows[0].SecondLoadVersion,
		FeasibilityStatus:  rows[0].FeasibilityStatus,
		SelectedOrder:      rows[0].SelectedOrder,
		OrdersEvaluated:    result.Candidates[0].OrdersEvaluated,
	})
	if fp1 != fp2 || fp1 == "" {
		t.Fatalf("fingerprint not deterministic %s vs %s", fp1, fp2)
	}
}

func TestProjectDurableCandidates_UnknownMetricsStayNil(t *testing.T) {
	run := repository.ChainSearchRun{
		ID: uuid.New(), TenantID: uuid.New(),
		RequestFingerprint: "fp", AlgorithmVersion: "a", PolicyVersion: "p",
	}
	a, b := uuid.New(), uuid.New()
	if b.String() < a.String() {
		a, b = b, a
	}
	result := chain.Result{Candidates: []chain.ProjectedCandidate{{
		CandidateID: uuid.New(),
		Loads:       []chain.ProjectedLoad{{LoadID: a, Version: 1}, {LoadID: b, Version: 1}},
		Feasible:    true, SelectedOrder: chain.OrderLowerFirst,
		EmptyRoadMeters: intPtr(1), DurationSeconds: intPtr(2),
		OmittedComponents: []string{chain.OmitUtilization, chain.OmitTarget, chain.OmitRevenue},
	}}}
	rows, err := ProjectDurableCandidates(run, result)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Utilization != nil || rows[0].TargetDistanceM != nil || rows[0].RevenueAmount != nil || rows[0].RevenueCurrency != nil {
		t.Fatalf("unknown metrics must stay nil got %+v", rows[0])
	}
}

func TestClassifyDurableTerminal_BudgetAuthoritative(t *testing.T) {
	run := repository.ChainSearchRun{DeadlineAt: mustParseTime("2020-01-01T00:00:00Z")}
	finished := mustParseTime("2020-01-01T00:00:01Z")
	d := classifyDurableTerminal(contextBackground(), run, chain.Result{Status: chain.StatusCancelled}, context.Canceled, finished)
	if d.kind != terminalBudget || d.failureCode != FailureBudgetExhausted {
		t.Fatalf("want budget got %+v", d)
	}
}

func TestClassifyDurableTerminal_ComputeBeforeDeadlinePersistInGrace(t *testing.T) {
	run := repository.ChainSearchRun{DeadlineAt: mustParseTime("2020-01-01T00:00:00Z")}
	finishedBefore := mustParseTime("2019-12-31T23:59:59Z")
	d := classifyDurableTerminal(contextBackground(), run, chain.Result{Status: chain.StatusSearchComplete}, nil, finishedBefore)
	if d.kind != terminalComplete {
		t.Fatalf("compute before deadline must allow COMPLETED got %+v", d)
	}
}

func TestClassifyDurableTerminal_ComputeAfterDeadlineCannotComplete(t *testing.T) {
	run := repository.ChainSearchRun{DeadlineAt: mustParseTime("2020-01-01T00:00:00Z")}
	finishedAfter := mustParseTime("2020-01-01T00:00:03Z") // would be persistence grace if misused as finish time
	d := classifyDurableTerminal(contextBackground(), run, chain.Result{Status: chain.StatusSearchComplete}, nil, finishedAfter)
	if d.kind != terminalBudget || d.failureCode != FailureBudgetExhausted {
		t.Fatalf("SEARCH_COMPLETE after deadline must budget-fail got %+v", d)
	}
}

func TestClassifyDurableTerminal_CancelBeatsUncommittedComplete(t *testing.T) {
	run := repository.ChainSearchRun{DeadlineAt: mustParseTime("2099-01-01T00:00:00Z")}
	finished := mustParseTime("2020-01-01T00:00:00Z")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := classifyDurableTerminal(ctx, run, chain.Result{Status: chain.StatusSearchComplete}, nil, finished)
	if d.kind != terminalCancel {
		t.Fatalf("cancelled ctx must beat SEARCH_COMPLETE got %+v", d)
	}
}

func TestClassifyDurableTerminal_RejectNotCompleted(t *testing.T) {
	run := repository.ChainSearchRun{DeadlineAt: mustParseTime("2099-01-01T00:00:00Z")}
	d := classifyDurableTerminal(contextBackground(), run, chain.Result{Status: chain.StatusRejected}, nil, mustParseTime("2020-01-01T00:00:00Z"))
	if d.kind != terminalFail || d.failureCode != FailureUnexpectedEngineStatus {
		t.Fatalf("REJECTED must fail closed got %+v", d)
	}
}

func TestTerminalStatsFromEngine_PreservesWork(t *testing.T) {
	result := chain.Result{
		Evaluations: 2, ProviderCalls: 7,
		ReasonCounts: map[string]int{"ROAD_UNKNOWN": 1},
	}
	classified := map[string]int{"ROAD_UNKNOWN": 1, "UNEXPECTED_STATUS": 1}
	stats := TerminalStatsFromEngine(result, 0, classified)
	if stats.CandidateCount != 0 || stats.EvaluatedPairCount != 2 || stats.ProviderCallCount != 7 {
		t.Fatalf("stats=%+v", stats)
	}
	if stats.ReasonCounts["UNEXPECTED_STATUS"] != 1 || stats.ReasonCounts["ROAD_UNKNOWN"] != 1 {
		t.Fatalf("reasons=%v", stats.ReasonCounts)
	}
}

func intPtr(v int) *int { return &v }

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func contextBackground() context.Context { return context.Background() }
