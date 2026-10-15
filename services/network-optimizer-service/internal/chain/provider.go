package chain

import (
	"context"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// limitedProvider enforces the chain call cap, the remaining deadline, and no retry.
type limitedProvider struct {
	base            routing.Provider
	now             func() time.Time
	deadline        time.Time
	durableAbsolute bool // when true, wall clock binds call timeouts to persisted deadline_at
	calls           int
	exhausted       bool
	timeouts        []time.Duration
}

func (p *limitedProvider) Route(ctx context.Context, req routing.RouteRequest) (routing.RouteResult, error) {
	if err := ctx.Err(); err != nil {
		return routing.RouteResult{}, err
	}
	current := p.now()
	if !current.Before(p.deadline) || p.calls >= ProviderCallsMax {
		p.exhausted = true
		return routing.RouteResult{}, routing.ErrSearchBudget
	}
	timeout := ProviderCallTimeout
	if remaining := p.deadline.Sub(current); remaining > 0 && remaining < timeout {
		timeout = remaining
	}

	var callCtx context.Context
	var cancel context.CancelFunc
	if p.durableAbsolute {
		// Durable path: absolute wall clock so a stale/fake Request.Now cannot
		// reopen timeouts past persisted deadline_at.
		wallNow := time.Now().UTC()
		absCap := p.deadline.UTC()
		if !wallNow.Before(absCap) {
			p.exhausted = true
			return routing.RouteResult{}, routing.ErrSearchBudget
		}
		if remaining := absCap.Sub(wallNow); remaining < timeout {
			timeout = remaining
		}
		callDeadline := wallNow.Add(timeout)
		if callDeadline.After(absCap) {
			callDeadline = absCap
		}
		callCtx, cancel = context.WithDeadline(ctx, callDeadline)
	} else {
		// Non-durable I0 keeps the Request.Now seam for budget math.
		callCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	p.calls++
	p.timeouts = append(p.timeouts, timeout)
	result, err := p.base.Route(callCtx, req)
	if err != nil {
		return routing.RouteResult{}, err
	}
	return result, nil
}

func (p *limitedProvider) Matrix(ctx context.Context, req routing.MatrixRequest) (routing.MatrixResult, error) {
	if _, err := p.Route(ctx, routing.RouteRequest{}); err != nil {
		return routing.MatrixResult{}, err
	}
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}
