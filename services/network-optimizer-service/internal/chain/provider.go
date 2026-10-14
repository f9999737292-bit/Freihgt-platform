package chain

import (
	"context"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// limitedProvider enforces the chain call cap, the remaining deadline, and no retry.
type limitedProvider struct {
	base      routing.Provider
	now       func() time.Time
	deadline  time.Time
	calls     int
	exhausted bool
	timeouts  []time.Duration
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
	remaining := p.deadline.Sub(current)
	timeout := ProviderCallTimeout
	if remaining < timeout {
		timeout = remaining
	}
	p.calls++
	p.timeouts = append(p.timeouts, timeout)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
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
