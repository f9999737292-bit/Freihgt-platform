//go:build nlo03ediscovery

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// BENCHMARK_PROVIDER is a deterministic in-process routing double.
// NOT_PRODUCTION_LATENCY: its duration is not 2GIS latency.
const benchmarkProviderName = "BENCHMARK_PROVIDER"

type benchCatalog struct{ calls int }

func (c *benchCatalog) Evaluation(context.Context, uuid.UUID) (compat.Context, error) {
	c.calls++
	return compat.Context{}, nil
}

type benchRoute struct{ calls int }

func (b *benchRoute) ProviderName() string { return benchmarkProviderName }

func (b *benchRoute) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	b.calls++
	return routing.RouteResult{
		Provider: benchmarkProviderName, DistanceM: 1000, DurationSeconds: 1,
		RequestFingerprint: "BENCHMARK_PROVIDER|NOT_PRODUCTION_LATENCY",
		Geometry:           routing.Geometry{Type: "LineString", Coordinates: [][]float64{{37, 55}, {38, 56}}},
	}, nil
}

func (b *benchRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrProviderUnavailable
}

type benchSample struct {
	Pool              int    `json:"pool"`
	Eligible          int    `json:"eligible"`
	PairsOrCandidates int    `json:"pairs_or_candidates"`
	GroupageCalls     int    `json:"groupage_calls"`
	CatalogLoads      int    `json:"catalog_loads"`
	RoutingCalls      int    `json:"routing_calls"`
	DBReads           int    `json:"db_reads_logical"`
	DBWrites          int    `json:"db_writes_logical"`
	RowsPersisted     int    `json:"rows_persisted"`
	ResponseBytes     int    `json:"response_bytes"`
	AllocBytes        uint64 `json:"alloc_bytes"`
	HeapAlloc         uint64 `json:"heap_alloc"`
	P50Millis         int64  `json:"p50_ms"`
	P95Millis         int64  `json:"p95_ms"`
	Repeats           int    `json:"repeats"`
	Fingerprint       string `json:"fingerprint"`
	Scenario          string `json:"scenario"`
	Provider          string `json:"provider"`
}

func TestNLO03EDiscoveryBenchmark(t *testing.T) {
	t.Logf("ENV go=%s os=%s arch=%s cpus=%d provider=%s NOT_PRODUCTION_LATENCY", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), benchmarkProviderName)
	var pairwise []benchSample
	for _, n := range []int{10, 50, 100, 500, 1000} {
		repeats := 5
		if n >= 500 {
			repeats = 3
		}
		if n == 1000 {
			repeats = 1
		}
		sample := measurePairwise(t, n, repeats)
		pairwise = append(pairwise, sample)
		raw, _ := json.Marshal(sample)
		t.Logf("PAIRWISE %s", raw)
	}
	var fills []benchSample
	for _, scenario := range []string{"FEASIBLE", "HARD_REJECT_STILL_ROUTES", "STALE_POSITION_SKIPS_ROUTING", "INDETERMINATE", "MIXED"} {
		for _, n := range []int{10, 50, 100, 500} {
			sample := measureFill(t, n, scenario, 5)
			fills = append(fills, sample)
			raw, _ := json.Marshal(sample)
			t.Logf("FILL %s", raw)
		}
	}
	if len(pairwise) == 0 || len(fills) == 0 {
		t.Fatal("no samples")
	}
}

func measurePairwise(t *testing.T, n, repeats int) benchSample {
	t.Helper()
	var durations []time.Duration
	var first benchSample
	var firstHash string
	for run := 0; run < repeats; run++ {
		w := newWorld(t)
		cat := &benchCatalog{}
		w.svc.UseCatalog(cat)
		cap := benchCapacity(t, w)
		origin, dest := benchID(0x21, 1), benchID(0x22, 1)
		win := span(w.at, w.at.Add(4*time.Hour))
		for i := 1; i <= n; i++ {
			benchLoad(t, w, benchID(0x11, i), w.shipper, origin, dest, f64(1), win, win)
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		started := time.Now()
		result, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestination})
		elapsed := time.Since(started)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(result.Body, &doc); err != nil {
			t.Fatal(err)
		}
		durations = append(durations, elapsed)
		if doc.PoolLoadCount != n || doc.EvaluatedPairCount != n*(n-1)/2 {
			t.Fatalf("pool %d pairs %d want %d", doc.PoolLoadCount, doc.EvaluatedPairCount, n*(n-1)/2)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		members := 0
		prints := make([]string, 0, len(rows))
		for _, row := range rows {
			members += len(row.Members)
			prints = append(prints, row.CandidateFingerprint)
		}
		sort.Strings(prints)
		sum := sha256.Sum256([]byte(fmt.Sprint(prints)))
		hash := hex.EncodeToString(sum[:])
		if run == 0 {
			firstHash = hash
			first = benchSample{
				Pool: doc.PoolLoadCount, Eligible: doc.EvaluatedPairCount, PairsOrCandidates: doc.EvaluatedPairCount,
				GroupageCalls: cat.calls, CatalogLoads: cat.calls, RoutingCalls: 0,
				DBReads: 2, DBWrites: 1 + len(rows) + members, RowsPersisted: 1 + len(rows) + members,
				ResponseBytes: len(result.Body), AllocBytes: after.TotalAlloc - before.TotalAlloc,
				HeapAlloc: after.HeapAlloc, Fingerprint: hash, Scenario: "SAME_OD_ALL_ELIGIBLE",
				Provider: "NONE", Repeats: repeats,
			}
			if cat.calls != doc.EvaluatedPairCount*2 {
				t.Fatalf("catalog %d pairs %d", cat.calls, doc.EvaluatedPairCount)
			}
		} else if hash != firstHash {
			t.Fatalf("pairwise fingerprint changed at n=%d", n)
		}
		_ = doc
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	first.P50Millis = durations[len(durations)/2].Milliseconds()
	first.P95Millis = durations[len(durations)-1].Milliseconds()
	return first
}

func measureFill(t *testing.T, n int, scenario string, repeats int) benchSample {
	t.Helper()
	var durations []time.Duration
	var first benchSample
	var firstHash string
	for run := 0; run < repeats; run++ {
		w, src := benchTrip(t, scenario)
		cat := &benchCatalog{}
		routes := &benchRoute{}
		w.svc.UseCatalog(cat)
		w.svc.UseRouting(routes)
		origin, dest := benchID(0x31, 1), benchID(0x32, 1)
		win := span(w.at.Add(-time.Hour), w.at.Add(48*time.Hour))
		for i := 1; i <= n; i++ {
			weight := f64(100)
			switch scenario {
			case "HARD_REJECT_STILL_ROUTES", "STALE_POSITION_SKIPS_ROUTING":
				weight = f64(50000)
			case "INDETERMINATE":
				weight = nil
			case "MIXED":
				switch i % 4 {
				case 0:
					weight = f64(50000)
				case 1:
					weight = nil
				default:
					weight = f64(100)
				}
			}
			load := benchLoad(t, w, benchID(0x41, i), w.shipper, origin, dest, weight, win, win)
			_ = load
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		started := time.Now()
		result, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{ShipmentID: src.execution.ShipmentID, Pattern: PatternCurrentTripFill})
		elapsed := time.Since(started)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(result.Body, &doc); err != nil {
			t.Fatal(err)
		}
		durations = append(durations, elapsed)
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		members := 0
		prints := make([]string, 0, len(rows))
		for _, row := range rows {
			members += len(row.Members)
			prints = append(prints, row.CandidateFingerprint)
		}
		sort.Strings(prints)
		sum := sha256.Sum256([]byte(fmt.Sprint(prints)))
		hash := hex.EncodeToString(sum[:])
		if run == 0 {
			firstHash = hash
			if scenario == "FEASIBLE" && (doc.FeasibleCandidateCount != n || routes.calls != n*4) {
				t.Fatalf("feasible %d routing %d indeterminate %d hard %d pool %d excluded %+v", doc.FeasibleCandidateCount, routes.calls, doc.IndeterminateCandidateCount, doc.HardRejectCandidateCount, doc.PoolLoadCount, doc.ExcludedCountsByReason)
			}
			first = benchSample{
				Pool: doc.PoolLoadCount, Eligible: doc.EvaluatedPairCount, PairsOrCandidates: doc.EvaluatedPairCount,
				GroupageCalls: cat.calls, CatalogLoads: cat.calls, RoutingCalls: routes.calls,
				DBReads: 1 + 6, DBWrites: 1 + len(rows) + members, RowsPersisted: 1 + len(rows) + members,
				ResponseBytes: len(result.Body), AllocBytes: after.TotalAlloc - before.TotalAlloc,
				HeapAlloc: after.HeapAlloc, Fingerprint: hash, Scenario: scenario,
				Provider: benchmarkProviderName + "|NOT_PRODUCTION_LATENCY", Repeats: repeats,
			}
		} else if hash != firstHash {
			t.Fatalf("fill fingerprint changed n=%d scenario=%s", n, scenario)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	first.P50Millis = durations[len(durations)/2].Milliseconds()
	first.P95Millis = durations[len(durations)-1].Milliseconds()
	return first
}

func benchTrip(t *testing.T, scenario string) (*world, *fillSources) {
	t.Helper()
	w := newWorld(t)
	shipment := benchID(0x51, 1)
	vehicle := benchID(0x52, 1)
	cargo := benchID(0x53, 1)
	dest := benchID(0x54, 1)
	lat, lon := 55.0, 37.0
	recorded := w.at
	arrival := w.at.Add(4 * time.Hour)
	fresh := "FRESH"
	if scenario == "STALE_POSITION_SKIPS_ROUTING" {
		fresh = "STALE"
	}
	src := &fillSources{
		execution: currenttrip.ShipmentExecution{
			ShipmentID: shipment, TenantID: w.carrier, ShipmentVersion: 3, ShipmentStatus: "IN_TRANSIT",
			VehicleID: &vehicle, OriginLocationID: benchID(0x55, 1), DestinationLocationID: dest,
		},
		onboard: currenttrip.OnboardCargo{ShipmentID: shipment, ShipmentVersion: 3, Resolution: currenttrip.EvidenceConfirmedOnboard, Items: []currenttrip.OnboardEvidenceItem{{
			CargoID: cargo, State: currenttrip.EvidenceConfirmedOnboard, StateVersion: 2, OccurredAt: w.at, Source: "DRIVER_OPERATION",
		}}},
		profiles: map[uuid.UUID]currenttrip.CargoProfile{cargo: {ID: cargo, Version: 4, WeightKg: f64(1000), VolumeM3: f64(2)}},
		vehicle: currenttrip.VehicleCapability{
			ID: vehicle, TenantID: w.carrier, Version: 5, CapacityWeight: f64(20000), CapacityVolume: f64(80),
			InternalHeightMM: intPtr(2700), LoadingAccess: []string{"SIDE"}, UnloadingAccess: []string{"REAR"},
			PalletPositions: intPtr(33),
		},
		position: currenttrip.TrackingPosition{Freshness: fresh, Latitude: &lat, Longitude: &lon, RecordedAt: &recorded},
		eta:      currenttrip.ETA{FreshnessStatus: "FRESH", EstimatedArrivalAt: &arrival, SourceObservedAt: &recorded},
	}
	w.dir.snaps[dest] = domain.LocationSnapshot{ID: dest, Latitude: f64(56), Longitude: f64(38), CountryCode: "RU", City: "Dest"}
	w.svc.ConfigureCurrentTrip(currenttrip.NewProvider(src, src, src, src, src, src))
	return w, src
}

func benchCapacity(t *testing.T, w *world) domain.Capacity {
	t.Helper()
	cap := domain.Capacity{
		ID: benchID(0x61, 1), OwnerTenantID: w.carrier, Latitude: f64(55), Longitude: f64(37),
		AvailableFrom: w.at, AvailableUntil: w.at.Add(24 * time.Hour), Source: domain.SourceManual,
		VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
		PayloadRemainingKg: f64(20000), VolumeRemainingM3: f64(80),
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertCapacity(context.Background(), cap)
	}); err != nil {
		t.Fatal(err)
	}
	return cap
}

func benchLoad(t *testing.T, w *world, id, owner, origin, dest uuid.UUID, weight *float64, pickup, delivery domain.TimeWindow) domain.LoadOpportunity {
	t.Helper()
	load := domain.LoadOpportunity{
		ID: id, OwnerTenantID: owner, SourceType: domain.SourceTransportOrder, SourceID: id,
		Pickup: benchPlace(origin, "Origin", 55.1, 37.1), Delivery: benchPlace(dest, "Dest", 55.4, 37.4),
		PickupWindow: pickup, DeliveryWindow: delivery, VisibilityScope: domain.VisMarketplace,
		Status: domain.LoadPublished, Version: 1, ConsolidationAllowed: true, CrossShipperConsolidationAllowed: true,
		WeightKg: weight, VolumeM3: f64(1),
		CreatedAt: w.at, UpdatedAt: w.at,
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertLoad(context.Background(), load)
	}); err != nil {
		t.Fatal(err)
	}
	return load
}

func benchID(prefix byte, n int) uuid.UUID {
	var id uuid.UUID
	id[0] = prefix
	id[14] = byte(n >> 8)
	id[15] = byte(n)
	return id
}

func benchPlace(id uuid.UUID, label string, lat, lon float64) domain.Place {
	place := located(id, label)
	place.Latitude = f64(lat)
	place.Longitude = f64(lon)
	return place
}
