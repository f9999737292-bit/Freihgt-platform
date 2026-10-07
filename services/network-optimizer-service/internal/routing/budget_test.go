package routing

import (
	"context"
	"testing"
	"time"
)

type countingProvider struct {
	calls    int
	deadline time.Duration
	err      error
	cells    MatrixResult
}

func (p *countingProvider) Route(ctx context.Context, _ RouteRequest) (RouteResult, error) {
	p.calls++
	if deadline, ok := ctx.Deadline(); ok {
		p.deadline = time.Until(deadline)
	}
	if p.err != nil {
		return RouteResult{}, p.err
	}
	return RouteResult{DistanceM: 1, DurationSeconds: 1}, nil
}

func (p *countingProvider) Matrix(ctx context.Context, req MatrixRequest) (MatrixResult, error) {
	p.calls++
	if deadline, ok := ctx.Deadline(); ok {
		p.deadline = time.Until(deadline)
	}
	if p.err != nil {
		return MatrixResult{}, p.err
	}
	if len(p.cells.Cells) > 0 {
		return p.cells, nil
	}
	cells := make([]MatrixCell, 0, len(req.Origins)*len(req.Destinations))
	for i := range req.Origins {
		for j := range req.Destinations {
			cells = append(cells, MatrixCell{OriginIndex: i, DestinationIndex: j, DistanceM: 1000, DurationSeconds: 60})
		}
	}
	return MatrixResult{Cells: cells}, nil
}

func TestSearchBudgetCallAllowance(t *testing.T) {
	timeout, ok := callAllowance(0, 30*time.Second, 5*time.Second)
	if !ok || timeout != 5*time.Second {
		t.Fatalf("fresh %s %v", timeout, ok)
	}
	timeout, ok = callAllowance(28*time.Second, 30*time.Second, 5*time.Second)
	if !ok || timeout != 2*time.Second {
		t.Fatalf("remaining %s %v", timeout, ok)
	}
	if _, ok = callAllowance(30*time.Second, 30*time.Second, 5*time.Second); ok {
		t.Fatal("expired allowance")
	}
}

func TestSearchBudgetStopsAtDeadlineWithoutCallingProvider(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	now := start.Add(30 * time.Second)
	inner := &countingProvider{}
	budget := NewSearchBudget(inner, start, func() time.Time { return now }, SearchBudgetPolicy{})
	if _, err := budget.Route(context.Background(), RouteRequest{}); err != ErrSearchBudget || inner.calls != 0 {
		t.Fatalf("calls %d err %v", inner.calls, err)
	}
}

func TestSearchBudgetUsesRemainingDeadline(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	now := start.Add(28 * time.Second)
	inner := &countingProvider{}
	budget := NewSearchBudget(inner, start, func() time.Time { return now }, SearchBudgetPolicy{})
	if _, err := budget.Route(context.Background(), RouteRequest{}); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 || inner.deadline < time.Second || inner.deadline > 3*time.Second {
		t.Fatalf("deadline %s calls %d", inner.deadline, inner.calls)
	}
}

func TestSearchBudgetTimeoutDoesNotRetry(t *testing.T) {
	inner := &countingProvider{err: ErrTimeout}
	budget := NewSearchBudget(inner, time.Unix(0, 0), func() time.Time { return time.Unix(0, 0) }, SearchBudgetPolicy{})
	if _, err := budget.Matrix(context.Background(), MatrixRequest{Origins: []Point{{}}, Destinations: []Point{{}}}); err != ErrTimeout {
		t.Fatal(err)
	}
	if _, err := budget.Route(context.Background(), RouteRequest{}); err != ErrSearchBudget || inner.calls != 1 {
		t.Fatalf("calls %d err %v", inner.calls, err)
	}
}

func TestSearchBudgetRejectsOversizedMatrixDimension(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	inner := &countingProvider{}
	budget := NewSearchBudget(inner, now, func() time.Time { return now }, SearchBudgetPolicy{})
	origins := make([]Point, SyncMatrixLimit+1)
	if _, err := budget.Matrix(context.Background(), MatrixRequest{Origins: origins, Destinations: []Point{{}}}); err != ErrInvalidResponse || inner.calls != 0 {
		t.Fatalf("calls %d err %v", inner.calls, err)
	}
	page := make([]Point, SyncMatrixLimit)
	square := &countingProvider{}
	okBudget := NewSearchBudget(square, now, func() time.Time { return now }, SearchBudgetPolicy{})
	if _, err := okBudget.Matrix(context.Background(), MatrixRequest{Origins: page, Destinations: page}); err != nil || square.calls != 1 {
		t.Fatalf("square page calls %d err %v", square.calls, err)
	}
}

func TestSearchBudgetPreservesPartialMatrixCells(t *testing.T) {
	inner := &countingProvider{cells: MatrixResult{Cells: []MatrixCell{
		{OriginIndex: 0, DestinationIndex: 0, DistanceM: 1500, DurationSeconds: 90},
		{OriginIndex: 0, DestinationIndex: 1, Err: ErrRouteNotFound},
	}}}
	now := time.Unix(1_700_000_000, 0)
	budget := NewSearchBudget(inner, now, func() time.Time { return now }, SearchBudgetPolicy{})
	result, err := budget.Matrix(context.Background(), MatrixRequest{Origins: []Point{{}}, Destinations: []Point{{}, {}}})
	if err != nil || len(result.Cells) != 2 || result.Cells[0].DistanceM != 1500 || result.Cells[1].Err != ErrRouteNotFound {
		t.Fatalf("%+v %v", result.Cells, err)
	}
}
