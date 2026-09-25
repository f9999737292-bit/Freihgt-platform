package routing

import (
	"context"
	"testing"
)

type limitProvider struct {
	calls    int
	maxDests int
	maxOrigs int
}

func (p *limitProvider) Route(context.Context, RouteRequest) (RouteResult, error) {
	return RouteResult{}, nil
}

func (p *limitProvider) Matrix(_ context.Context, req MatrixRequest) (MatrixResult, error) {
	p.calls++
	if len(req.Origins) > p.maxOrigs {
		p.maxOrigs = len(req.Origins)
	}
	if len(req.Destinations) > p.maxDests {
		p.maxDests = len(req.Destinations)
	}
	if len(req.Origins) > SyncMatrixLimit || len(req.Destinations) > SyncMatrixLimit {
		return MatrixResult{}, ErrInvalidResponse
	}
	cells := make([]MatrixCell, 0, len(req.Origins)*len(req.Destinations))
	for i := range req.Origins {
		for j, dest := range req.Destinations {
			cells = append(cells, MatrixCell{OriginIndex: i, DestinationIndex: j, DistanceM: int(dest.Longitude * 1000), DurationSeconds: 60})
		}
	}
	return MatrixResult{Cells: cells}, nil
}

func TestBNO232AndBNO233MatrixBatchLimitAndIdentity(t *testing.T) {
	provider := &limitProvider{}
	destinations := make([]Point, 30)
	for i := range destinations {
		destinations[i] = Point{Longitude: float64(i + 1)}
	}
	result, err := BatchMatrix(context.Background(), provider, MatrixRequest{
		Origins: []Point{{Latitude: 1}}, Destinations: destinations,
	}, SyncMatrixLimit)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || provider.maxDests > SyncMatrixLimit || provider.maxOrigs > SyncMatrixLimit {
		t.Fatalf("BNO232 calls=%d dests=%d origins=%d", provider.calls, provider.maxDests, provider.maxOrigs)
	}
	if len(result.Cells) != 30 {
		t.Fatalf("BNO233 cells %d", len(result.Cells))
	}
	seen := map[int]int{}
	for _, cell := range result.Cells {
		seen[cell.DestinationIndex] = cell.DistanceM
	}
	if seen[0] != 1000 || seen[25] != 26000 || seen[29] != 30000 {
		t.Fatalf("BNO233 identity %+v", seen)
	}
}
