package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestNLO05B1BoundedCandidateDiscovery(t *testing.T) {
	if domain.CandidateDiscoveryCap != 1000 {
		t.Fatalf("CANDIDATE_DISCOVERY_CAP=%d", domain.CandidateDiscoveryCap)
	}

	t.Run("B1-01_pool_below_cap_unchanged", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, 5, 0, 0.02)
		pruned := counterDelta(bnometrics.CandidatePruned)
		returned := counterDelta(bnometrics.CandidateReturned)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if len(loads) != 5 || pruned() != 0 || returned() != 5 {
			t.Fatalf("len=%d pruned=%v returned=%v", len(loads), pruned(), returned())
		}
	})

	t.Run("B1-02_exactly_cap_retained", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, domain.CandidateDiscoveryCap, 0, 0.02)
		pruned := counterDelta(bnometrics.CandidatePruned)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if len(loads) != domain.CandidateDiscoveryCap || pruned() != 0 {
			t.Fatalf("len=%d pruned=%v", len(loads), pruned())
		}
	})

	t.Run("B1-03_above_cap_pruned_to_cap", func(t *testing.T) {
		w := newWorld(t)
		const extra = 250
		inserted := insertVisibleLoads(t, w, domain.CandidateDiscoveryCap+extra, 0, 0.02)
		want := map[uuid.UUID]struct{}{}
		for _, load := range inserted[extra:] {
			want[load.ID] = struct{}{}
		}
		pruned := counterDelta(bnometrics.CandidatePruned)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if len(loads) != domain.CandidateDiscoveryCap || pruned() != float64(extra) {
			t.Fatalf("len=%d pruned=%v", len(loads), pruned())
		}
		for _, load := range loads {
			if _, ok := want[load.ID]; !ok {
				t.Fatalf("kept an older load %s", load.ID)
			}
		}
	})

	t.Run("B1-04_deterministic_repeated_ordering", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, 30, 0, 0.02)
		first, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		second, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if discoveryKey(first) != discoveryKey(second) {
			t.Fatalf("discovery order changed")
		}
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		left := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		right := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if idKey(candidateIDs(left)) != idKey(candidateIDs(right)) || len(left.Candidates) != domain.CandidateRoutingCap {
			t.Fatalf("search order %s %s", idKey(candidateIDs(left)), idKey(candidateIDs(right)))
		}
	})

	t.Run("B1-05_tenant_isolation", func(t *testing.T) {
		w := newWorld(t)
		own := w.loadOwned(w.carrier, domain.VisMarketplace, nil, place(0, 0.02, "Own"), place(0, 1, "Drop"), nil)
		foreignPrivate := w.loadOwned(w.shipper, domain.VisPrivate, nil, place(0, 0.02, "Hidden"), place(0, 1, "Drop"), nil)
		foreignDraft := w.loadOwned(uuid.New(), domain.LoadDraft, nil, place(0, 0.02, "Draft"), place(0, 1, "Drop"), nil)
		foreignVisible := w.load(domain.VisMarketplace, place(0, 0.03, "Visible"), place(0, 1, "Drop"), nil)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		ids := loadIDs(loads)
		if len(ids) != 1 || ids[0] != foreignVisible.ID || containsID(ids, own.ID) || containsID(ids, foreignPrivate.ID) || containsID(ids, foreignDraft.ID) {
			t.Fatalf("visible %s", idKey(ids))
		}
	})

	t.Run("B1-06_no_duplicate_candidate", func(t *testing.T) {
		id := uuid.New()
		other := uuid.New()
		out := boundDiscoveryCandidates([]domain.LoadOpportunity{{ID: id}, {ID: other}, {ID: id}})
		if len(out) != 2 || out[0].ID != id || out[1].ID != other {
			t.Fatalf("%v", loadIDs(out))
		}
		w := newWorld(t)
		insertVisibleLoads(t, w, 20, 0, 0.02)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		seen := map[uuid.UUID]struct{}{}
		for _, load := range loads {
			if _, ok := seen[load.ID]; ok {
				t.Fatalf("duplicate %s", load.ID)
			}
			seen[load.ID] = struct{}{}
		}
	})

	t.Run("B1-07_corridor_prefilter_on_bounded_set", func(t *testing.T) {
		w := newWorld(t)
		w.svc.UseRouting(nil)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		inside := w.load(domain.VisMarketplace, place(0, 0.2, "On"), place(0, 1, "Drop"), nil)
		outside := w.load(domain.VisMarketplace, place(8, 0.2, "Off"), place(0, 1, "Drop"), nil)
		discovered, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if len(discovered) != 2 {
			t.Fatalf("bounded set %d", len(discovered))
		}
		forward, lateral := 2000.0, 20.0
		policy := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchDirectionalCorridor, ForwardSearchKm: &forward, CorridorDeviationKm: &lateral,
			ObjectiveProfile: "MIN_DEADHEAD",
		}
		passed := counterDelta(bnometrics.CandidatePrefilterPass)
		rows := w.svc.evaluateLoads(context.Background(), w.carrier, cap, nil, w.at, policy, routing.Point{Latitude: 0, Longitude: 1}, [][]float64{{0, 0}, {1, 0}}, []domain.LoadOpportunity{inside, outside})
		if passed() != 1 {
			t.Fatalf("prefilter %v", passed())
		}
		if w.routes.calls != 0 || w.routes.routeCalls != 0 {
			t.Fatalf("provider calls route=%d matrix=%d", w.routes.routeCalls, w.routes.calls)
		}
		byID := map[uuid.UUID]evaluatedLoad{}
		for _, row := range rows {
			byID[row.load.ID] = row
		}
		if containsString(strings.Join(byID[inside.ID].reasons, ","), domain.ReasonLateralExceeded) {
			t.Fatalf("inside rejected %v", byID[inside.ID].reasons)
		}
		if !containsString(strings.Join(byID[outside.ID].reasons, ","), domain.ReasonLateralExceeded) {
			t.Fatalf("outside reasons %v", byID[outside.ID].reasons)
		}
	})

	t.Run("B1-08_zero_routing_provider_calls", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, 12, 0, 0.02)
		if _, err := w.svc.visibleLoads(context.Background(), w.actor()); err != nil {
			t.Fatal(err)
		}
		if w.routes.calls != 0 || w.routes.routeCalls != 0 {
			t.Fatalf("provider calls route=%d matrix=%d", w.routes.routeCalls, w.routes.calls)
		}
	})

	t.Run("B1-09_context_cancellation", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, 4, 0, 0.02)
		returned := counterDelta(bnometrics.CandidateReturned)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := w.svc.visibleLoads(ctx, w.actor()); !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		if returned() != 0 {
			t.Fatalf("canceled discovery recorded %v", returned())
		}
	})

	t.Run("B1-10_metric_counters", func(t *testing.T) {
		w := newWorld(t)
		w.svc.UseRouting(nil)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, 0.01, "Near"), place(0, 1, "Drop"), nil)
		w.load(domain.VisMarketplace, place(0, 0.02, "Near"), place(0, 1, "Drop"), nil)
		w.load(domain.VisMarketplace, place(20, 20, "Far"), place(21, 21, "Drop"), nil)
		discovered := counterDelta(bnometrics.CandidateDiscovered)
		visible := counterDelta(bnometrics.CandidateVisibilityPass)
		pruned := counterDelta(bnometrics.CandidatePruned)
		returned := counterDelta(bnometrics.CandidateReturned)
		passed := counterDelta(bnometrics.CandidatePrefilterPass)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(50, 0))
		if discovered() != 3 || visible() != 3 || pruned() != 0 || returned() != 3 || passed() != 2 {
			t.Fatalf("discovered=%v visibility=%v pruned=%v returned=%v prefilter=%v", discovered(), visible(), pruned(), returned(), passed())
		}
		if doc.ReturnedCandidateCount != 0 {
			t.Fatalf("unknown road still returned %d", doc.ReturnedCandidateCount)
		}
		for _, counter := range []prometheus.Counter{
			bnometrics.CandidateDiscovered, bnometrics.CandidateVisibilityPass, bnometrics.CandidatePrefilterPass,
			bnometrics.CandidatePruned, bnometrics.CandidateReturned,
		} {
			var metric dto.Metric
			if err := counter.Write(&metric); err != nil {
				t.Fatal(err)
			}
			if len(metric.Label) != 0 {
				t.Fatalf("metric labels %+v", metric.Label)
			}
		}
	})

	t.Run("B1-11_small_pool_regression", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		first := w.load(domain.VisMarketplace, place(0, 0.02, "One"), place(0, 1, "Drop"), nil)
		second := w.load(domain.VisMarketplace, place(0, 0.03, "Two"), place(0, 1, "Drop"), nil)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		returned := counterDelta(bnometrics.CandidateReturned)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		ids := candidateIDs(doc)
		if returned() != 2 || len(ids) != 2 || !containsID(ids, first.ID) || !containsID(ids, second.ID) {
			t.Fatalf("returned=%v ids=%s", returned(), idKey(ids))
		}
	})

	t.Run("B1-12_no_sensitive_candidate_metadata", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		loc := uuid.New()
		pickup := place(12.345678, 34.567891, "City")
		pickup.LocationID = &loc
		pickup.Label = "SECRET-CUSTOMER"
		w.load(domain.VisAnonymized, pickup, place(12.9, 34.9, "Far"), nil)
		w.routes.defaultM = 64000
		w.routes.defaultSec = 3600
		result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(500, 0)})
		if err != nil {
			t.Fatal(err)
		}
		body := string(result.Body)
		for _, leaked := range []string{w.shipper.String(), loc.String(), "12.345678", "34.567891", "SECRET-CUSTOMER", "latitude", "longitude", "location_id"} {
			if strings.Contains(body, leaked) {
				t.Fatalf("leaked %s", leaked)
			}
		}
	})

	t.Run("client_limit_cannot_raise_discovery_cap", func(t *testing.T) {
		w := newWorld(t)
		w.svc.UseRouting(nil)
		insertVisibleLoads(t, w, domain.CandidateDiscoveryCap+100, 0, 0.02)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if len(loads) != domain.CandidateDiscoveryCap {
			t.Fatalf("discovery %d", len(loads))
		}
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		limit := domain.CandidateDiscoveryCap + 500
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(500, 0), CandidateLimit: &limit})
		if err != nil {
			t.Fatal(err)
		}
		if doc.ReturnedCandidateCount > domain.CandidateDiscoveryCap || strings.Count(string(result.Body), "load_opportunity_id") > domain.CandidateDiscoveryCap {
			t.Fatalf("client limit raised the result %d", doc.ReturnedCandidateCount)
		}
	})
}

func TestNLO05B1DiscoveryLatency(t *testing.T) {
	for _, n := range []int{100, 1000, 5000, 10000} {
		w := newWorld(t)
		insertVisibleLoads(t, w, n, 0, 0.02)
		started := time.Now()
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		elapsed := time.Since(started)
		if err != nil {
			t.Fatal(err)
		}
		want := n
		if want > domain.CandidateDiscoveryCap {
			want = domain.CandidateDiscoveryCap
		}
		if len(loads) != want {
			t.Fatalf("n=%d len=%d", n, len(loads))
		}
		t.Logf("BENCHMARK_%d=%s source_rows=%d returned=%d", n, elapsed, n, len(loads))
	}
}

func insertVisibleLoads(t *testing.T, w *world, n int, lat, lon float64) []domain.LoadOpportunity {
	t.Helper()
	loads := make([]domain.LoadOpportunity, n)
	err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		for i := 0; i < n; i++ {
			created := w.at.Add(time.Duration(i) * time.Millisecond)
			load := domain.LoadOpportunity{
				ID: uuid.New(), OwnerTenantID: w.shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
				Pickup: place(lat, lon, "P"), Delivery: place(lat, lon+1, "D"),
				VisibilityScope: domain.VisMarketplace, Status: domain.LoadPublished, Version: 1,
				CreatedAt: created, UpdatedAt: created,
			}
			if err := tx.InsertLoad(context.Background(), load); err != nil {
				return err
			}
			loads[i] = load
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return loads
}

func counterDelta(counter prometheus.Counter) func() float64 {
	before := testutil.ToFloat64(counter)
	return func() float64 { return testutil.ToFloat64(counter) - before }
}

func discoveryKey(loads []domain.LoadOpportunity) string {
	return idKey(loadIDs(loads))
}

func loadIDs(loads []domain.LoadOpportunity) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(loads))
	for _, load := range loads {
		ids = append(ids, load.ID)
	}
	return ids
}

func idKey(ids []uuid.UUID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id.String())
	}
	return strings.Join(parts, ",")
}
