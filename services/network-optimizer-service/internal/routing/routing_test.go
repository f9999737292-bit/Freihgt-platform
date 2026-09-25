package routing

import (
	"context"
	"testing"
	"time"
)

type fakeProvider struct{}

func (fakeProvider) Route(context.Context, RouteRequest) (RouteResult, error) {
	return RouteResult{Provider: "FAKE", DistanceM: 80000, DurationSeconds: 4800, Geometry: Geometry{Type: "LineString", Coordinates: [][]float64{{60.6, 56.8}, {37.6, 55.7}}}}, nil
}

func (fakeProvider) Matrix(context.Context, MatrixRequest) (MatrixResult, error) {
	return MatrixResult{Provider: "FAKE", Cells: []MatrixCell{{DistanceM: 80000, DurationSeconds: 4800}}}, nil
}

func TestBNO179RoutingPortAndCache(t *testing.T) {
	var provider Provider = fakeProvider{}
	result, err := provider.Route(context.Background(), RouteRequest{RouteMode: RouteFastest, TrafficMode: TrafficCurrent})
	if err != nil || result.DistanceM != 80000 || result.DurationSeconds != 4800 || result.Geometry.Type != "LineString" {
		t.Fatalf("BNO179 route %+v %v", result, err)
	}
	matrix, err := provider.Matrix(context.Background(), MatrixRequest{})
	if err != nil || len(matrix.Cells) != 1 || matrix.Cells[0].DistanceM != 80000 {
		t.Fatalf("BNO179 matrix %+v %v", matrix, err)
	}
	cache := NewMemoryCache()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	result.CalculatedAt = now
	result.ExpiresAt = Expiry(TrafficCurrent, now)
	result.RequestFingerprint = Fingerprint("FAKE", RouteRequest{TrafficMode: TrafficCurrent, DepartureAt: &now})
	if result.RequestFingerprint == "" {
		t.Fatal("BNO179 fingerprint")
	}
	cache.Put(context.Background(), result.RequestFingerprint, result)
	if _, ok := cache.Get(context.Background(), result.RequestFingerprint, now.Add(LiveRouteTTL)); ok {
		t.Fatal("BNO179 live duration stayed valid past its expiry")
	}
	if got, ok := cache.Get(context.Background(), result.RequestFingerprint, now.Add(time.Minute)); !ok || got.DistanceM != 80000 {
		t.Fatal("BNO179 fresh cache miss")
	}
}

func TestBNO183StraightLinePrefilter(t *testing.T) {
	km := StraightLineKm(56.838, 60.597, 55.7558, 37.6173)
	if km < 1000 || km > 2000 {
		t.Fatalf("BNO183 straight line %v", km)
	}
}
