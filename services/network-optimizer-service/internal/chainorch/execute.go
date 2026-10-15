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

	engineResult, computeErr := o.RunOwnedDurableChainSearch(ctx, tenant, searchID, ownerToken, engine, base)
	if computeErr != nil {
		// Pre-compute fencing failures: do not terminalize a PENDING/foreign row.
		if errors.Is(computeErr, ErrNotOwner) ||
			errors.Is(computeErr, ErrInProgress) ||
			errors.Is(computeErr, ErrSearchNotRunnable) ||
			errors.Is(computeErr, ErrComputeDeadlinePassed) {
			return ExecuteOwnedResult{}, computeErr
		}
		return o.terminalizeAfterCompute(ctx, tenant, searchID, ownerToken, engineResult, computeErr, opts)
	}
	return o.terminalizeAfterCompute(ctx, tenant, searchID, ownerToken, engineResult, nil, opts)
}

func (o *Orchestrator) terminalizeAfterCompute(
	ctx context.Context,
	tenant, searchID, ownerToken uuid.UUID,
	engineResult chain.Result,
	computeErr error,
	opts ExecuteOwnedOptions,
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
	decision := classifyDurableTerminal(ctx, run, engineResult, computeErr, o.now())
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
// The persisted durable deadline is authoritative over an underlying provider
// cancel/road-unavailable appearance when wall clock is at/after deadline_at.
func classifyDurableTerminal(
	ctx context.Context,
	run repository.ChainSearchRun,
	result chain.Result,
	computeErr error,
	wallNow time.Time,
) terminalDecision {
	reasons := map[string]int{}
	for k, v := range result.ReasonCounts {
		reasons[k] = v
	}

	deadlinePassed := !wallNow.UTC().Before(run.DeadlineAt.UTC())

	// Successful engine completion terminalizes to COMPLETED even if wall clock
	// has entered the +5s persistence grace after deadline_at.
	if computeErr == nil && result.Status == chain.StatusSearchComplete {
		return terminalDecision{kind: terminalComplete, reasonCounts: reasons}
	}

	// Persisted durable deadline is authoritative when compute did not complete:
	// an in-flight provider cancel/deadline must still classify as budget exhaustion.
	if deadlinePassed || result.Status == chain.StatusBudgetExhausted {
		return terminalDecision{kind: terminalBudget, failureCode: FailureBudgetExhausted, reasonCounts: reasons}
	}

	ctxCanceled := errors.Is(ctx.Err(), context.Canceled)
	ctxDeadline := errors.Is(ctx.Err(), context.DeadlineExceeded)
	errCanceled := errors.Is(computeErr, context.Canceled)
	errDeadline := errors.Is(computeErr, context.DeadlineExceeded)

	if result.Status == chain.StatusCancelled || ctxCanceled || errCanceled {
		return terminalDecision{kind: terminalCancel, reasonCounts: reasons}
	}
	if ctxDeadline || errDeadline {
		return terminalDecision{kind: terminalCancel, reasonCounts: reasons}
	}

	if computeErr != nil {
		reasons["ENGINE_ERROR"] = reasons["ENGINE_ERROR"] + 1
		return terminalDecision{kind: terminalFail, failureCode: FailureEngineError, reasonCounts: reasons}
	}

	switch result.Status {
	case chain.StatusRejected:
		// Never map REJECTED to COMPLETED.
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
