package chainorch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// Execute outcomes for the canonical claim→RUNNING→compute→terminal path.
const (
	ExecuteOutcomeCompleted = "COMPLETED"
	ExecuteOutcomeFailed    = "FAILED"
	ExecuteOutcomeCancelled = "CANCELLED"
)

// ExecuteOwnedOptions controls optional cancel-reason provenance for I2C.
// When CancelReasonClass is empty, CONTEXT_CANCELLED is used.
type ExecuteOwnedOptions struct {
	CancelReasonClass string
}

// ExecuteOwnedResult is the durable terminal outcome of ExecuteOwnedDurableChainSearch.
type ExecuteOwnedResult struct {
	Outcome      string
	Search       repository.ChainSearchRun
	EngineResult chain.Result
	Candidates   []repository.ChainCandidate
}

// AfterProjectCandidates is an optional test seam applied after projection and before
// CompleteSearch. Production leaves it nil. Used to force candidate-insert failures
// without inventing a second persistence path.
type AfterProjectCandidates func([]repository.ChainCandidate) []repository.ChainCandidate

// trustedWallClock is the durable compute-finish proof clock. Tests must not replace
// this with Orchestrator.Now or Request.Now.
var trustedWallClock = func() time.Time { return time.Now().UTC() }

// ExecuteOwnedDurableChainSearch is the canonical I2B production entry point:
// owner-fenced RUNNING → real chain compute → durable terminal state (+ shortlist).
// Callers must not interpret chain.Result statuses or choose persistence primitives.
//
// RunOwnedDurableChainSearch remains the low-level compute primitive and is not
// the final production service entry point.
func (o *Orchestrator) ExecuteOwnedDurableChainSearch(
	ctx context.Context,
	tenant, searchID, ownerToken uuid.UUID,
	engine chain.Engine,
	base chain.Request,
	opts ExecuteOwnedOptions,
) (ExecuteOwnedResult, error) {
	if o == nil || o.Store == nil {
		return ExecuteOwnedResult{}, errors.New("orchestrator store required")
	}
	if err := validateCancelReasonOption(opts); err != nil {
		return ExecuteOwnedResult{}, err
	}

	engineResult, computeErr := o.RunOwnedDurableChainSearch(ctx, tenant, searchID, ownerToken, engine, base)
	// Trusted server wall clock — not Orchestrator.Now / Request.Now.
	computeFinishedAt := trustedWallClock()
	if o.AfterComputeBeforeTerminalizeForTest != nil {
		o.AfterComputeBeforeTerminalizeForTest()
	}

	// Parent cancel/deadline before or during the PENDING→RUNNING start must not
	// leave a PENDING row. A parent deadline is cancellation, not durable budget.
	if requestContextStopped(ctx, computeErr) {
		handled, out, err := o.cancelOwnedBeforeCompute(ctx, tenant, searchID, ownerToken, opts)
		if err != nil {
			return ExecuteOwnedResult{}, err
		}
		if handled {
			return out, nil
		}
	}

	if computeErr != nil {
		// Pre-compute fencing failures: do not terminalize a PENDING/foreign row.
		if errors.Is(computeErr, ErrNotOwner) ||
			errors.Is(computeErr, ErrInProgress) ||
			errors.Is(computeErr, ErrSearchNotRunnable) ||
			errors.Is(computeErr, ErrComputeDeadlinePassed) {
			return ExecuteOwnedResult{}, computeErr
		}
		return o.terminalizeAfterCompute(ctx, tenant, searchID, ownerToken, engineResult, computeErr, opts, computeFinishedAt)
	}
	return o.terminalizeAfterCompute(ctx, tenant, searchID, ownerToken, engineResult, nil, opts, computeFinishedAt)
}

func requestContextStopped(ctx context.Context, computeErr error) bool {
	return errors.Is(ctx.Err(), context.Canceled) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded) ||
		errors.Is(computeErr, context.Canceled) ||
		errors.Is(computeErr, context.DeadlineExceeded)
}

// cancelOwnedBeforeCompute persists CANCELLED when the request stopped before a
// durable RUNNING compute, or returns handled=false when the row is already RUNNING
// so the caller can use the post-compute terminal path (with engine counters).
// Wrong owners are not cancelled. Already-terminal rows are left unchanged.
func (o *Orchestrator) cancelOwnedBeforeCompute(
	parent context.Context,
	tenant, searchID, ownerToken uuid.UUID,
	opts ExecuteOwnedOptions,
) (bool, ExecuteOwnedResult, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(parent), o.cleanupTimeout())
	defer cancel()

	run, err := o.getRun(cleanup, tenant, searchID)
	if err != nil {
		return false, ExecuteOwnedResult{}, err
	}
	if run.OwnerToken != ownerToken {
		return true, ExecuteOwnedResult{}, ErrNotOwner
	}
	if IsTerminal(run.Status) {
		cands, listErr := o.listCandidates(cleanup, tenant, searchID)
		return true, ExecuteOwnedResult{
			Outcome:    run.Status,
			Search:     run,
			Candidates: cands,
		}, listErr
	}
	if run.Status == repository.ChainSearchRunning {
		return false, ExecuteOwnedResult{}, nil
	}
	if run.Status != repository.ChainSearchPending {
		return true, ExecuteOwnedResult{Search: run}, ErrSearchNotRunnable
	}
	reason := opts.CancelReasonClass
	if reason == "" {
		reason = CancelContextCancelled
	}
	out, ok, cancelErr := o.CancelSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchPending, reason, TerminalStats{})
	if cancelErr != nil {
		return false, ExecuteOwnedResult{}, cancelErr
	}
	if !ok {
		fresh, getErr := o.getRun(cleanup, tenant, searchID)
		if getErr != nil {
			return false, ExecuteOwnedResult{}, getErr
		}
		if fresh.OwnerToken != ownerToken {
			return true, ExecuteOwnedResult{}, ErrNotOwner
		}
		if IsTerminal(fresh.Status) {
			return true, ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh}, nil
		}
		if fresh.Status == repository.ChainSearchRunning {
			return false, ExecuteOwnedResult{}, nil
		}
		return true, ExecuteOwnedResult{Search: fresh}, ErrSearchNotRunnable
	}
	return true, ExecuteOwnedResult{Outcome: ExecuteOutcomeCancelled, Search: out}, nil
}

func validateCancelReasonOption(opts ExecuteOwnedOptions) error {
	if opts.CancelReasonClass == "" {
		return nil
	}
	if !ValidCancelReason(opts.CancelReasonClass) {
		return ErrInvalidCancelReason
	}
	return nil
}

func (o *Orchestrator) terminalizeAfterCompute(
	ctx context.Context,
	tenant, searchID, ownerToken uuid.UUID,
	engineResult chain.Result,
	computeErr error,
	opts ExecuteOwnedOptions,
	computeFinishedAt time.Time,
) (ExecuteOwnedResult, error) {
	// Persistence must not depend on a cancelled/deadline-exceeded request ctx.
	cleanup, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), o.cleanupTimeout())
	defer cleanupCancel()

	run, err := o.getRun(cleanup, tenant, searchID)
	if err != nil {
		return ExecuteOwnedResult{EngineResult: engineResult}, err
	}
	if IsTerminal(run.Status) {
		cands, listErr := o.listCandidates(cleanup, tenant, searchID)
		return ExecuteOwnedResult{
			Outcome:      run.Status,
			Search:       run,
			EngineResult: engineResult,
			Candidates:   cands,
		}, listErr
	}
	if run.Status != repository.ChainSearchRunning {
		return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, ErrSearchNotRunnable
	}

	if o.RewriteEngineResultForTest != nil {
		engineResult = o.RewriteEngineResultForTest(engineResult)
	}
	decision := classifyDurableTerminal(ctx, run, engineResult, computeErr, computeFinishedAt)
	// No partial candidate rows on fail/cancel paths today — candidate_count stays 0
	// unless rows are actually committed (COMPLETED shortlist only).
	stats := TerminalStatsFromEngine(engineResult, 0, decision.reasonCounts)
	switch decision.kind {
	case terminalBudget:
		out, ok, failErr := o.FailSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, FailureBudgetExhausted, stats)
		if failErr != nil {
			return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, failErr
		}
		if !ok {
			fresh, getErr := o.getRun(cleanup, tenant, searchID)
			return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult}, getErr
		}
		return ExecuteOwnedResult{Outcome: ExecuteOutcomeFailed, Search: out, EngineResult: engineResult}, nil

	case terminalCancel:
		reason := opts.CancelReasonClass
		if reason == "" {
			reason = CancelContextCancelled
		}
		out, ok, cancelErr := o.CancelSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, reason, stats)
		if cancelErr != nil {
			return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, cancelErr
		}
		if !ok {
			fresh, getErr := o.getRun(cleanup, tenant, searchID)
			return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult}, getErr
		}
		return ExecuteOwnedResult{Outcome: ExecuteOutcomeCancelled, Search: out, EngineResult: engineResult}, nil

	case terminalFail:
		out, ok, failErr := o.FailSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, decision.failureCode, stats)
		if failErr != nil {
			return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, failErr
		}
		if !ok {
			fresh, getErr := o.getRun(cleanup, tenant, searchID)
			return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult}, getErr
		}
		return ExecuteOwnedResult{Outcome: ExecuteOutcomeFailed, Search: out, EngineResult: engineResult}, nil

	case terminalComplete:
		// Re-check request cancellation before committing COMPLETED.
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason := opts.CancelReasonClass
			if reason == "" {
				reason = CancelContextCancelled
			}
			out, ok, cancelErr := o.CancelSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, reason, stats)
			if cancelErr != nil {
				return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, cancelErr
			}
			if !ok {
				fresh, getErr := o.getRun(cleanup, tenant, searchID)
				return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult}, getErr
			}
			return ExecuteOwnedResult{Outcome: ExecuteOutcomeCancelled, Search: out, EngineResult: engineResult}, nil
		}
		candidates, projErr := ProjectDurableCandidates(run, engineResult)
		if projErr != nil {
			failReasons := map[string]int{}
			for k, v := range decision.reasonCounts {
				failReasons[k] = v
			}
			failReasons["PROJECT_ERROR"] = failReasons["PROJECT_ERROR"] + 1
			failStats := TerminalStatsFromEngine(engineResult, 0, failReasons)
			out, ok, failErr := o.FailSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, FailureUnexpectedEngineStatus, failStats)
			if failErr != nil {
				return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, failErr
			}
			if !ok {
				fresh, _ := o.getRun(cleanup, tenant, searchID)
				return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult}, fmt.Errorf("project: %w", projErr)
			}
			return ExecuteOwnedResult{Outcome: ExecuteOutcomeFailed, Search: out, EngineResult: engineResult}, fmt.Errorf("project: %w", projErr)
		}
		if o.AfterProjectCandidates != nil {
			candidates = o.AfterProjectCandidates(candidates)
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason := opts.CancelReasonClass
			if reason == "" {
				reason = CancelContextCancelled
			}
			out, ok, cancelErr := o.CancelSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, reason, stats)
			if cancelErr != nil {
				return ExecuteOwnedResult{EngineResult: engineResult, Search: run, Candidates: candidates}, cancelErr
			}
			if !ok {
				fresh, getErr := o.getRun(cleanup, tenant, searchID)
				return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult, Candidates: candidates}, getErr
			}
			return ExecuteOwnedResult{Outcome: ExecuteOutcomeCancelled, Search: out, EngineResult: engineResult, Candidates: candidates}, nil
		}
		completeStats := TerminalStatsFromEngine(engineResult, len(candidates), decision.reasonCounts)
		write := TerminalWrite{
			Run: repository.ChainSearchRun{
				ID:                 run.ID,
				TenantID:           run.TenantID,
				Status:             repository.ChainSearchCompleted,
				CandidateCount:     completeStats.CandidateCount,
				EvaluatedPairCount: completeStats.EvaluatedPairCount,
				ProviderCallCount:  completeStats.ProviderCallCount,
				ReasonCounts:       completeStats.ReasonCounts,
			},
			Candidates: candidates,
		}
		out, ok, completeErr := o.CompleteSearch(cleanup, ownerToken, repository.ChainSearchRunning, write)
		if completeErr != nil {
			// Atomic TX rolled back — search remains RUNNING for cleanup/failure handling.
			return ExecuteOwnedResult{EngineResult: engineResult, Search: run, Candidates: candidates}, completeErr
		}
		if !ok {
			fresh, getErr := o.getRun(cleanup, tenant, searchID)
			return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult, Candidates: candidates}, getErr
		}
		return ExecuteOwnedResult{
			Outcome:      ExecuteOutcomeCompleted,
			Search:       out,
			EngineResult: engineResult,
			Candidates:   candidates,
		}, nil

	default:
		out, ok, failErr := o.FailSearch(cleanup, tenant, searchID, ownerToken, repository.ChainSearchRunning, FailureUnexpectedEngineStatus, stats)
		if failErr != nil {
			return ExecuteOwnedResult{EngineResult: engineResult, Search: run}, failErr
		}
		if !ok {
			fresh, getErr := o.getRun(cleanup, tenant, searchID)
			return ExecuteOwnedResult{Outcome: fresh.Status, Search: fresh, EngineResult: engineResult}, getErr
		}
		return ExecuteOwnedResult{Outcome: ExecuteOutcomeFailed, Search: out, EngineResult: engineResult}, nil
	}
}

type terminalKind int

const (
	terminalComplete terminalKind = iota
	terminalBudget
	terminalCancel
	terminalFail
)

type terminalDecision struct {
	kind         terminalKind
	failureCode  string
	reasonCounts map[string]int
}

// classifyDurableTerminal decides durable terminalization from the engine result.
//
// Priority:
//  1. computeFinishedAt >= deadline_at (or engine budget) → FAILED / CHAIN_BUDGET_EXHAUSTED
//  2. request cancellation observed before durable completion → CANCELLED
//  3. SEARCH_COMPLETE with computeFinishedAt < deadline_at → COMPLETED
//  4. engine error / unexpected → FAILED
//
// Persistence time is irrelevant: only computeFinishedAt proves deadline respect.
// Parent context DeadlineExceeded before the durable deadline is cancellation, not budget.
func classifyDurableTerminal(
	ctx context.Context,
	run repository.ChainSearchRun,
	result chain.Result,
	computeErr error,
	computeFinishedAt time.Time,
) terminalDecision {
	reasons := map[string]int{}
	for k, v := range result.ReasonCounts {
		reasons[k] = v
	}

	finishedAtOrAfterDeadline := !computeFinishedAt.UTC().Before(run.DeadlineAt.UTC())

	// 1) Durable compute deadline is authoritative — even if engine reported SEARCH_COMPLETE.
	if finishedAtOrAfterDeadline || result.Status == chain.StatusBudgetExhausted {
		return terminalDecision{kind: terminalBudget, failureCode: FailureBudgetExhausted, reasonCounts: reasons}
	}

	ctxCanceled := errors.Is(ctx.Err(), context.Canceled)
	ctxDeadline := errors.Is(ctx.Err(), context.DeadlineExceeded)
	errCanceled := errors.Is(computeErr, context.Canceled)
	errDeadline := errors.Is(computeErr, context.DeadlineExceeded)

	// 2) Cancellation observed before durable completion commit.
	if result.Status == chain.StatusCancelled || ctxCanceled || errCanceled || ctxDeadline || errDeadline {
		return terminalDecision{kind: terminalCancel, reasonCounts: reasons}
	}

	if computeErr != nil {
		reasons["ENGINE_ERROR"] = reasons["ENGINE_ERROR"] + 1
		return terminalDecision{kind: terminalFail, failureCode: FailureEngineError, reasonCounts: reasons}
	}

	// 3) Valid SEARCH_COMPLETE only when compute finished strictly before deadline_at.
	if result.Status == chain.StatusSearchComplete {
		return terminalDecision{kind: terminalComplete, reasonCounts: reasons}
	}

	switch result.Status {
	case chain.StatusRejected:
		reasons["ENGINE_REJECTED"] = reasons["ENGINE_REJECTED"] + 1
		return terminalDecision{kind: terminalFail, failureCode: FailureUnexpectedEngineStatus, reasonCounts: reasons}
	default:
		reasons["UNEXPECTED_STATUS"] = reasons["UNEXPECTED_STATUS"] + 1
		return terminalDecision{kind: terminalFail, failureCode: FailureUnexpectedEngineStatus, reasonCounts: reasons}
	}
}

func (o *Orchestrator) getRun(ctx context.Context, tenant, searchID uuid.UUID) (repository.ChainSearchRun, error) {
	var out repository.ChainSearchRun
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var getErr error
		out, getErr = tx.GetChainSearchRun(ctx, tenant, searchID)
		return getErr
	})
	return out, err
}

func (o *Orchestrator) listCandidates(ctx context.Context, tenant, searchID uuid.UUID) ([]repository.ChainCandidate, error) {
	var out []repository.ChainCandidate
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var listErr error
		out, listErr = tx.ListChainCandidates(ctx, tenant, searchID)
		return listErr
	})
	return out, err
}
