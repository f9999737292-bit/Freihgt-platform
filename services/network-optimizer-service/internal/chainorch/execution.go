package chainorch

import (
	"context"
	"errors"
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

// RunOwnedDurableChainSearch is the low-level I2B compute primitive.
// It commits PENDING -> RUNNING under tenant/search/owner fencing before any
// provider/engine/route-plan work. No takeover and no automatic retry.
// Production callers must use ExecuteOwnedDurableChainSearch, which terminalizes
// durable state after compute. This method alone leaves RUNNING.
func (o *Orchestrator) RunOwnedDurableChainSearch(
	ctx context.Context,
	tenant, searchID, ownerToken uuid.UUID,
	engine chain.Engine,
	base chain.Request,
) (chain.Result, error) {
	if o == nil || o.Store == nil {
		return chain.Result{}, errors.New("orchestrator store required")
	}
	if tenant == uuid.Nil || searchID == uuid.Nil || ownerToken == uuid.Nil {
		return chain.Result{}, ErrNotOwner
	}

	var running repository.ChainSearchRun
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, searchID); err != nil {
			return err
		}
		row, err := tx.GetChainSearchRun(ctx, tenant, searchID)
		if err != nil {
			return err
		}
		if err := classifyComputeStart(row, ownerToken, o.now()); err != nil {
			return err
		}
		ok, err := tx.MarkChainSearchRunning(ctx, tenant, searchID, ownerToken)
		if err != nil {
			return err
		}
		if !ok {
			// Concurrent start, deadline race, or fencing loss — re-read to classify.
			row, err = tx.GetChainSearchRun(ctx, tenant, searchID)
			if err != nil {
				return err
			}
			if err := classifyComputeStart(row, ownerToken, o.now()); err != nil {
				return err
			}
			return ErrNotOwner
		}
		running, err = tx.GetChainSearchRun(ctx, tenant, searchID)
		if err != nil {
			return err
		}
		if running.Status != repository.ChainSearchRunning {
			return ErrSearchNotRunnable
		}
		return nil
	})
	if err != nil {
		return chain.Result{}, err
	}

	// Compute only after RUNNING commit — never inside the start transaction.
	return runDurableChainSearch(ctx, engine, running, base)
}

func classifyComputeStart(row repository.ChainSearchRun, ownerToken uuid.UUID, now time.Time) error {
	if IsTerminal(row.Status) {
		return ErrSearchNotRunnable
	}
	if row.Status == repository.ChainSearchRunning {
		return ErrInProgress
	}
	if row.Status != repository.ChainSearchPending {
		return ErrSearchNotRunnable
	}
	if row.OwnerToken != ownerToken {
		return ErrNotOwner
	}
	if !now.Before(row.DeadlineAt) {
		return ErrComputeDeadlinePassed
	}
	return nil
}

// RunDurableChainSearch is a low-level helper for an already-verified RUNNING row.
// Production orchestration must use RunOwnedDurableChainSearch. This entry point
// rejects any non-RUNNING status so PENDING/forged rows cannot bypass the gate.
func RunDurableChainSearch(ctx context.Context, engine chain.Engine, run repository.ChainSearchRun, base chain.Request) (chain.Result, error) {
	return runDurableChainSearch(ctx, engine, run, base)
}

// runDurableChainSearch executes the in-memory chain engine under a RUNNING
// search's persisted identity, horizon, absolute deadline, and stored duration.
// SearchID is pinned via DurableSearchID / result postcondition only — Engine.NewID
// remains free for candidate ID generation.
func runDurableChainSearch(ctx context.Context, engine chain.Engine, run repository.ChainSearchRun, base chain.Request) (chain.Result, error) {
	if run.Status != repository.ChainSearchRunning {
		return chain.Result{}, ErrSearchNotRunnable
	}
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
	execCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	durableEngine := engine
	durableEngine.Repository = nil
	// Do NOT pin NewID to run.ID — that would collide CandidateIDs with SearchID.

	result, err := durableEngine.Search(execCtx, prepared)
	result.SearchID = run.ID
	result.HorizonStart = run.HorizonStart
	result.HorizonEnd = run.HorizonEnd
	return result, err
}
