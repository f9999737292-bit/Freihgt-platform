package studio

import "testing"

func TestPlanMatrixIterationAfterReadiness_blocksAcceptanceOnFailure(t *testing.T) {
	plan := PlanMatrixIterationAfterReadiness(1)
	if plan.RunAcceptance {
		t.Fatalf("acceptance must not run after readiness failure")
	}
	if !plan.RunReadiness {
		t.Fatalf("readiness should remain planned")
	}
}

func TestPlanMatrixIterationAfterReadiness_allowsAcceptanceOnSuccess(t *testing.T) {
	plan := PlanMatrixIterationAfterReadiness(0)
	if !plan.RunAcceptance || !plan.RunReadiness {
		t.Fatalf("expected readiness and acceptance, got %+v", plan)
	}
}

func TestMatrixShouldContinue_stopsAfterReadinessFailure(t *testing.T) {
	if MatrixShouldContinue(1, 0) {
		t.Fatalf("matrix must stop after readiness failure")
	}
}

func TestMatrixIterationResult_preservesReadinessExitCode(t *testing.T) {
	result := EvaluateMatrixIteration(1, 0, false)
	if result.Passed() || result.MatrixExitCode() != 1 {
		t.Fatalf("expected readiness failure to preserve exit code 1, got %+v", result)
	}
	if result.AcceptanceStarted {
		t.Fatalf("acceptance must not start after readiness failure")
	}
}

func TestMatrixIterationResult_failsWhenAcceptanceFails(t *testing.T) {
	result := EvaluateMatrixIteration(0, 2, true)
	if result.Passed() || result.MatrixExitCode() != 2 {
		t.Fatalf("expected acceptance exit code 2, got %+v", result)
	}
}
