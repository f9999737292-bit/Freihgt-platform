package service

import (
	"sort"
	"testing"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// nlo05B0Accounting is the current SearchNextLoad provider shape for N survivors
// that each have a pickup and a delivery. It does not call a routing vendor.
func nlo05B0Accounting(n int, ellipse bool) (matrixCalls, routeCalls, cells int) {
	if n <= 0 {
		return 0, 0, 0
	}
	limit := routing.SyncMatrixLimit
	deadhead := batchCount(n)
	direction := batchCount(n * 2)
	matrixCalls = deadhead + direction
	cells = n + n*2
	if ellipse {
		loadedCalls := batchCount(n)
		matrixCalls += loadedCalls
		full := n / limit
		rest := n % limit
		cells += full*limit*limit + rest*rest
	}
	routeCalls = 2
	return matrixCalls, routeCalls, cells
}

func TestNLO05B0ProviderCallAccounting(t *testing.T) {
	if routing.SyncMatrixLimit != 25 {
		t.Fatalf("sync matrix limit = %d", routing.SyncMatrixLimit)
	}
	pools := []int{10, 25, 50, 100, 250, 500, 1000}
	for _, n := range pools {
		for _, ellipse := range []bool{false, true} {
			matrix, routes, cells := nlo05B0Accounting(n, ellipse)
			if matrix < 1 || routes != 2 || cells < n {
				t.Fatalf("n=%d ellipse=%v matrix=%d routes=%d cells=%d", n, ellipse, matrix, routes, cells)
			}
			t.Logf("n=%d ellipse=%v matrix=%d route=%d total=%d cells=%d timeout_ceiling_ms=%d",
				n, ellipse, matrix, routes, matrix+routes, cells, (matrix+routes)*5000)
		}
	}
	matrix, routes, cells := nlo05B0Accounting(25, true)
	if matrix != 4 || routes != 2 || cells != 700 {
		t.Fatalf("native page ellipse matrix=%d routes=%d cells=%d", matrix, routes, cells)
	}
	matrix, routes, cells = nlo05B0Accounting(1000, true)
	if matrix != 160 || routes != 2 || cells != 28000 {
		t.Fatalf("unbounded ellipse matrix=%d routes=%d cells=%d", matrix, routes, cells)
	}
}

func TestNLO05B0LocalPrefilterTiming(t *testing.T) {
	pools := []int{10, 25, 50, 100, 250, 500, 1000}
	profiles := []struct {
		name string
		lat  float64
		lon  float64
		span float64
	}{
		{name: "dense_city", lat: 10, lon: 10, span: 0.05},
		{name: "metro", lat: 20, lon: 20, span: 0.30},
		{name: "regional_highway", lat: 30, lon: 30, span: 2},
		{name: "sparse_long_haul", lat: 40, lon: 40, span: 10},
	}
	const rounds = 31
	for _, profile := range profiles {
		line := [][]float64{{profile.lon, profile.lat}, {profile.lon + profile.span, profile.lat + profile.span}}
		for _, n := range pools {
			points := syntheticPoints(n, profile.lat, profile.lon, profile.span)
			samples := make([]time.Duration, 0, rounds)
			for round := 0; round < rounds; round++ {
				start := time.Now()
				kept := 0
				for _, point := range points {
					straight := routing.StraightLineKm(profile.lat, profile.lon, point[0], point[1])
					projection, err := domain.ProjectPointOnRoute(point[0], point[1], line)
					if err != nil {
						continue
					}
					if straight >= 0 && projection.ForwardProgressKm >= 0 {
						kept++
					}
				}
				samples = append(samples, time.Since(start))
				if kept < 0 {
					t.Fatal("kept")
				}
			}
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			t.Logf("profile=%s n=%d p50_us=%d p95_us=%d p99_us=%d",
				profile.name, n, samples[len(samples)/2].Microseconds(), quantile(samples, 0.95).Microseconds(), quantile(samples, 0.99).Microseconds())
		}
	}
}

func syntheticPoints(n int, lat, lon, span float64) [][2]float64 {
	points := make([][2]float64, n)
	for i := 0; i < n; i++ {
		step := span / float64(n+1)
		points[i] = [2]float64{lat + step*float64(i+1), lon + step*float64((i*3)%(n+1)+1)}
	}
	return points
}

func quantile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(float64(len(sorted)-1) * q)
	return sorted[index]
}
