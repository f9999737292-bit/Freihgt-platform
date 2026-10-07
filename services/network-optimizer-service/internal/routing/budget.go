package routing

import (
	"context"
	"errors"
	"sync"
	"time"

	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
)

// ErrSearchBudget means a provider call was not started.
// The search may still persist results that already have road facts.
var ErrSearchBudget = errors.New("ROUTING_SEARCH_BUDGET_EXHAUSTED")

// SearchBudgetPolicy is the server-owned ceiling for one search.
// A zero field uses the frozen first-release bound. It does not mean unlimited.
type SearchBudgetPolicy struct {
	HardBudget       time.Duration
	RequestTimeout   time.Duration
	MaxMatrixCalls   int
	MaxRouteCalls    int
	MaxProviderCalls int
	MatrixDimension  int
}

func (p SearchBudgetPolicy) normalized() SearchBudgetPolicy {
	if p.HardBudget <= 0 {
		p.HardBudget = 30 * time.Second
	}
	if p.RequestTimeout <= 0 {
		p.RequestTimeout = 5 * time.Second
	}
	if p.MaxMatrixCalls <= 0 {
		p.MaxMatrixCalls = 4
	}
	if p.MaxRouteCalls <= 0 {
		p.MaxRouteCalls = 2
	}
	if p.MaxProviderCalls <= 0 {
		p.MaxProviderCalls = 6
	}
	if p.MatrixDimension <= 0 {
		p.MatrixDimension = SyncMatrixLimit
	}
	return p
}

// SearchBudget wraps one routing provider for a single search.
// Calls are sequential. A refused call is not retried and does not reach the vendor.
type SearchBudget struct {
	base    Provider
	started time.Time
	now     func() time.Time
	policy  SearchBudgetPolicy
	matrix  int
	route   int
	fatal   error
	mu      sync.Mutex
}

type searchProviderKey struct{}

// WithSearchProvider returns a context that carries the search-scoped provider.
// The returned context is not canceled when the provider budget expires.
func WithSearchProvider(ctx context.Context, provider Provider) context.Context {
	return context.WithValue(ctx, searchProviderKey{}, provider)
}

// SearchProvider returns the provider bound to this search, if one was installed.
func SearchProvider(ctx context.Context) Provider {
	provider, _ := ctx.Value(searchProviderKey{}).(Provider)
	return provider
}

// NewSearchBudget starts the hard watchdog at started.
// now is the search clock. It is not reset for each provider call.
func NewSearchBudget(base Provider, started time.Time, now func() time.Time, policy SearchBudgetPolicy) *SearchBudget {
	if now == nil {
		now = time.Now
	}
	return &SearchBudget{base: base, started: started, now: now, policy: policy.normalized()}
}

func (b *SearchBudget) ProviderName() string {
	named, ok := b.base.(IdentifiedProvider)
	if !ok {
		return ""
	}
	return named.ProviderName()
}

func (b *SearchBudget) Route(ctx context.Context, req RouteRequest) (RouteResult, error) {
	callCtx, cancel, err := b.begin(ctx, false, 0, 0)
	if err != nil {
		return RouteResult{}, err
	}
	defer cancel()
	result, err := b.base.Route(callCtx, req)
	err = b.finish(ctx, err)
	return result, err
}

func (b *SearchBudget) Matrix(ctx context.Context, req MatrixRequest) (MatrixResult, error) {
	callCtx, cancel, err := b.begin(ctx, true, len(req.Origins), len(req.Destinations))
	if err != nil {
		return MatrixResult{}, err
	}
	defer cancel()
	result, err := b.base.Matrix(callCtx, req)
	err = b.finish(ctx, err)
	return result, err
}

func (b *SearchBudget) begin(ctx context.Context, matrix bool, origins, destinations int) (context.Context, context.CancelFunc, error) {
	noop := func() {}
	if err := ctx.Err(); err != nil {
		return nil, noop, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fatal != nil {
		bnometrics.ProviderBudget("provider_error")
		return nil, noop, ErrSearchBudget
	}
	elapsed := b.now().Sub(b.started)
	timeout, ok := callAllowance(elapsed, b.policy.HardBudget, b.policy.RequestTimeout)
	if !ok {
		bnometrics.ProviderBudget("deadline")
		return nil, noop, ErrSearchBudget
	}
	if matrix && (origins > b.policy.MatrixDimension || destinations > b.policy.MatrixDimension) {
		bnometrics.ProviderBudget("dimension")
		b.fatal = ErrInvalidResponse
		return nil, noop, ErrInvalidResponse
	}
	nextMatrix := b.matrix
	nextRoute := b.route
	if matrix {
		nextMatrix++
	} else {
		nextRoute++
	}
	if nextMatrix > b.policy.MaxMatrixCalls {
		bnometrics.ProviderBudget("matrix")
		return nil, noop, ErrSearchBudget
	}
	if nextRoute > b.policy.MaxRouteCalls {
		bnometrics.ProviderBudget("route")
		return nil, noop, ErrSearchBudget
	}
	if nextMatrix+nextRoute > b.policy.MaxProviderCalls {
		bnometrics.ProviderBudget("total")
		return nil, noop, ErrSearchBudget
	}
	b.matrix = nextMatrix
	b.route = nextRoute
	if matrix {
		bnometrics.ProviderMatrixCall()
	} else {
		bnometrics.ProviderRouteCall()
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	return callCtx, cancel, nil
}

func (b *SearchBudget) finish(parent context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(parent.Err(), context.Canceled) {
		return parent.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		err = ErrTimeout
	}
	if fatalProviderError(err) {
		b.mu.Lock()
		if b.fatal == nil {
			b.fatal = err
		}
		b.mu.Unlock()
	}
	return err
}

func fatalProviderError(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrProviderUnavailable) ||
		errors.Is(err, ErrInvalidResponse) || errors.Is(err, ErrRouteNotFound)
}

// callAllowance is the per-call timeout that still fits inside the hard watchdog.
// The hard watchdog is not extended to finish a call.
func callAllowance(elapsed, hard, request time.Duration) (time.Duration, bool) {
	if elapsed >= hard || hard <= 0 || request <= 0 {
		return 0, false
	}
	remaining := hard - elapsed
	if remaining < request {
		return remaining, true
	}
	return request, true
}
