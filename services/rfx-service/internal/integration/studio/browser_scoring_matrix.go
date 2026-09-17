package studio

// MatrixIterationPlan describes which browser suites may run in one cold-start iteration.
type MatrixIterationPlan struct {
	RunReadiness  bool
	RunAcceptance bool
}

// PlanMatrixIterationAfterReadiness enforces stop-on-first-failure orchestration.
func PlanMatrixIterationAfterReadiness(readinessExitCode int) MatrixIterationPlan {
	if readinessExitCode != 0 {
		return MatrixIterationPlan{RunReadiness: true, RunAcceptance: false}
	}
	return MatrixIterationPlan{RunReadiness: true, RunAcceptance: true}
}

// MatrixShouldContinue reports whether another iteration may start.
func MatrixShouldContinue(readinessExitCode, acceptanceExitCode int) bool {
	return readinessExitCode == 0 && acceptanceExitCode == 0
}

// MatrixIterationResult captures one iteration outcome for reporting.
type MatrixIterationResult struct {
	ReadinessExitCode  int
	AcceptanceExitCode int
	AcceptanceStarted  bool
}

func EvaluateMatrixIteration(readinessExit, acceptanceExit int, acceptanceStarted bool) MatrixIterationResult {
	return MatrixIterationResult{
		ReadinessExitCode:  readinessExit,
		AcceptanceExitCode: acceptanceExit,
		AcceptanceStarted:  acceptanceStarted,
	}
}

func (r MatrixIterationResult) Passed() bool {
	if r.ReadinessExitCode != 0 {
		return false
	}
	if r.AcceptanceStarted && r.AcceptanceExitCode != 0 {
		return false
	}
	return true
}

func (r MatrixIterationResult) MatrixExitCode() int {
	if r.ReadinessExitCode != 0 {
		return r.ReadinessExitCode
	}
	if r.AcceptanceStarted && r.AcceptanceExitCode != 0 {
		return r.AcceptanceExitCode
	}
	return 0
}
