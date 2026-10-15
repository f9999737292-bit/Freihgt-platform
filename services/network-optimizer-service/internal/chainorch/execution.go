package chainorch

import (
	"context"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// PrepareDurableChainRequest binds a claimed search's persisted deadline and
// immutable duration snapshot into a chain.Request for real engine execution.
//
// ComputeDeadline is the absolute persisted deadline_at (never restarted as
// engine_start+20s). Duration comes only from the stored snapshot.
func PrepareDurableChainRequest(run repository.ChainSearchRun, base chain.Request) (chain.Request, error) {
	out := base
	out.TenantID = run.TenantID
	deadline := run.DeadlineAt.UTC()
	out.ComputeDeadline = &deadline
	if err := ApplyStoredDurationToChainRequest(&out, run.ServiceDurationSnapshot); err != nil {
		return chain.Request{}, err
	}
	return out, nil
}

// EnsureComputeAllowed fails when wall clock is at/after deadline_at.
// Persistence grace (deadline..claim_expires) must not extend compute.
func (o *Orchestrator) EnsureComputeAllowed(run repository.ChainSearchRun) error {
	if !o.now().Before(run.DeadlineAt) {
		return ErrComputeDeadlinePassed
	}
	return nil
}

// RemainingComputeBudget returns time left until persisted deadline_at.
func RemainingComputeBudget(run repository.ChainSearchRun, now time.Time) time.Duration {
	remaining := run.DeadlineAt.UTC().Sub(now.UTC())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RunDurableChainSearch executes the in-memory chain engine under the claimed
// search's persisted absolute deadline and stored duration snapshot.
func RunDurableChainSearch(ctx context.Context, engine chain.Engine, run repository.ChainSearchRun, base chain.Request) (chain.Result, error) {
	prepared, err := PrepareDurableChainRequest(run, base)
	if err != nil {
		return chain.Result{}, err
	}
	if prepared.ComputeDeadline != nil && !time.Now().UTC().Before(prepared.ComputeDeadline.UTC()) {
		// Fail closed before engine work when already past durable deadline.
		res := chain.Result{
			SearchID:       run.ID,
			Status:         chain.StatusBudgetExhausted,
			SearchComplete: false,
			NoRetry:        true,
			ReasonCounts:   map[string]int{},
		}
		return res, nil
	}
	return engine.Search(ctx, prepared)
}
