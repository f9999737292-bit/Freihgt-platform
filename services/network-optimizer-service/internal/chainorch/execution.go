package chainorch

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// PrepareDurableChainRequest binds a claimed search's persisted identity, horizon,
// absolute deadline, and immutable duration snapshot into a chain.Request.
//
// Durable orchestration owns idempotency: legacy IdempotencyKey / engine.Repository
// paths are cleared. ClientChainID is stripped. Request.Now is replaced with the
// server wall clock so a stale caller clock cannot extend compute.
func PrepareDurableChainRequest(run repository.ChainSearchRun, base chain.Request) (chain.Request, error) {
	out := base
	out.TenantID = run.TenantID
	out.IdempotencyKey = ""
	out.ClientChainID = nil
	id := run.ID
	out.DurableSearchID = &id
	hs := run.HorizonStart.UTC()
	he := run.HorizonEnd.UTC()
	out.PersistedHorizonStart = &hs
	out.PersistedHorizonEnd = &he
	deadline := run.DeadlineAt.UTC()
	out.ComputeDeadline = &deadline
	// Server-owned wall clock for all durable engine/provider deadline checks.
	out.Now = time.Now
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
// search's persisted identity, horizon, absolute deadline, and stored duration.
// The engine copy has Repository=nil so legacy Claim/Commit cannot run.
func RunDurableChainSearch(ctx context.Context, engine chain.Engine, run repository.ChainSearchRun, base chain.Request) (chain.Result, error) {
	prepared, err := PrepareDurableChainRequest(run, base)
	if err != nil {
		return chain.Result{}, err
	}
	deadline := run.DeadlineAt.UTC()
	wallNow := time.Now().UTC()
	if !wallNow.Before(deadline) {
		return chain.Result{
			SearchID:       run.ID,
			Status:         chain.StatusBudgetExhausted,
			SearchComplete: false,
			NoRetry:        true,
			HorizonStart:   run.HorizonStart,
			HorizonEnd:     run.HorizonEnd,
			ReasonCounts:   map[string]int{},
		}, nil
	}
	// Absolute context deadline so provider work cannot outlive persisted deadline_at.
	execCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	durableEngine := engine
	durableEngine.Repository = nil
	if durableEngine.NewID == nil {
		// Defensive: durable path must never allocate a different public identity.
		pinned := run.ID
		durableEngine.NewID = func() uuid.UUID { return pinned }
	}

	result, err := durableEngine.Search(execCtx, prepared)
	// Pin durable resource identity and horizon regardless of engine internals.
	result.SearchID = run.ID
	result.HorizonStart = run.HorizonStart
	result.HorizonEnd = run.HorizonEnd
	return result, err
}
