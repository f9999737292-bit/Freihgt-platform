package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestNLO05B2BoundedRoutingBudget(t *testing.T) {
	if domain.CandidateDiscoveryCap != 1000 || domain.CandidateRoutingCap != 25 {
		t.Fatalf("caps discovery=%d routing=%d", domain.CandidateDiscoveryCap, domain.CandidateRoutingCap)
	}
	if domain.MatrixDimensionLimit != routing.SyncMatrixLimit || domain.MaxMatrixCallsPerSearch != 4 || domain.MaxRouteCallsPerSearch != 2 || domain.MaxProviderCallsTotal != 6 {
		t.Fatalf("provider bounds %+v", domain.MaxProviderCallsTotal)
	}
	if domain.ProviderRequestTimeout != 5*time.Second || domain.SearchHardBudget != 30*time.Second || domain.ProviderRetryCount != 0 {
		t.Fatal("timeout policy")
	}
	if domain.CandidateFinalEvaluationCap != 25 {
		t.Fatal("final evaluation cap")
	}

	t.Run("B2-01_prefilter_survivors_below_25_unchanged", func(t *testing.T) {
		w, cap := radiusWorld(t, 8)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if len(doc.Candidates) != 8 || w.routes.maxDests != 8 || w.routes.calls != 1 {
			t.Fatalf("candidates %d dests %d calls %d", len(doc.Candidates), w.routes.maxDests, w.routes.calls)
		}
	})

	t.Run("B2-02_exactly_25_survivors_all_routed", func(t *testing.T) {
		w, cap := radiusWorld(t, 25)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if len(doc.Candidates) != 25 || w.routes.maxDests != 25 || w.routes.calls != 1 {
			t.Fatalf("candidates %d dests %d calls %d", len(doc.Candidates), w.routes.maxDests, w.routes.calls)
		}
	})

	t.Run("B2-03_more_than_25_survivors_routed_at_25", func(t *testing.T) {
		w, cap := radiusWorld(t, 40)
		pruned := counterDelta(bnometrics.RoutingCandidatesPruned)
		selected := counterDelta(bnometrics.RoutingCandidatesSelected)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if len(doc.Candidates) != 25 || w.routes.maxDests != 25 || pruned() != 15 || selected() != 25 {
			t.Fatalf("candidates %d dests %d pruned %v selected %v", len(doc.Candidates), w.routes.maxDests, pruned(), selected())
		}
	})

	t.Run("B2-04_discovery_1000_does_not_route_1000", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, 1000, 0, 0.01)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if len(doc.Candidates) != 25 || w.routes.calls > domain.MaxProviderCallsTotal || w.routes.maxDests > 25 || w.routes.maxOrigins > 25 {
			t.Fatalf("candidates %d matrix %d route %d dests %d origins %d", len(doc.Candidates), w.routes.calls, w.routes.routeCalls, w.routes.maxDests, w.routes.maxOrigins)
		}
	})

	t.Run("B2-05_cheap_rejects_do_not_consume_routing_cap", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		for i := 0; i < 30; i++ {
			w.load(domain.VisMarketplace, place(20, 20, "Far"), place(21, 21, "D"), nil)
		}
		for i := 0; i < 3; i++ {
			w.load(domain.VisMarketplace, place(0, 0.01+float64(i)/1000, "Near"), place(0, 1, "D"), nil)
		}
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(50, 0))
		if len(doc.Candidates) != 3 || w.routes.maxDests != 3 || w.routes.calls != 1 {
			t.Fatalf("candidates %d dests %d calls %d", len(doc.Candidates), w.routes.maxDests, w.routes.calls)
		}
	})

	t.Run("B2-06_routing_selection_deterministic", func(t *testing.T) {
		w, cap, ids := radiusIDs(t, 30)
		left := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		right := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if idKey(candidateIDs(left)) != idKey(candidateIDs(right)) || len(left.Candidates) != 25 {
			t.Fatalf("order %s", idKey(candidateIDs(left)))
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		if idKey(candidateIDs(left)) != idKey(ids[:25]) {
			t.Fatal("selection is not the load-id order")
		}
	})

	t.Run("B2-07_client_candidate_limit_cannot_raise_routing_cap", func(t *testing.T) {
		w, cap := radiusWorld(t, 40)
		limit := 100
		if _, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(500, 0), CandidateLimit: &limit}); err != nil {
			t.Fatal(err)
		}
		if w.routes.maxDests != 25 || w.routes.calls != 1 {
			t.Fatalf("dests %d calls %d", w.routes.maxDests, w.routes.calls)
		}
	})

	t.Run("B2-08_09_10_11_ellipse_call_ceiling", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
		w.routes.line = [][]float64{{0, 0}, {10, 0}}
		w.routes.baselineM = 1000000
		w.routes.defaultM = 100000
		w.routes.defaultSec = 600
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		for i := 0; i < 25; i++ {
			w.load(domain.VisMarketplace, place(0, 0.1+float64(i)/1000, "P"), place(0, 0.2, "D"), nil)
		}
		policy := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchRouteEllipse, TargetLocationID: &targetID, MaxRouteIncreaseKm: f64(100000),
			ObjectiveProfile: "MIN_DEADHEAD",
		}
		if _, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy}); err != nil {
			t.Fatal(err)
		}
		total := w.routes.calls + w.routes.routeCalls
		t.Logf("ELLIPSE_25_MATRIX_CALLS=%d ELLIPSE_25_ROUTE_CALLS=%d ELLIPSE_25_TOTAL_PROVIDER_CALLS=%d origins=%d dests=%d", w.routes.calls, w.routes.routeCalls, total, w.routes.maxOrigins, w.routes.maxDests)
		if w.routes.calls != 4 || w.routes.routeCalls != 2 || total != 6 || w.routes.maxOrigins > 25 || w.routes.maxDests > 25 {
			t.Fatalf("matrix %d route %d origins %d dests %d", w.routes.calls, w.routes.routeCalls, w.routes.maxOrigins, w.routes.maxDests)
		}
	})

	t.Run("B2-12_13_provider_timeout_no_retry_and_stops", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(1)}
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, 0.01, "P"), place(0, 1, "D"), nil)
		w.routes.failAfter = 1
		w.routes.failErr = routing.ErrTimeout
		policy := radiusPolicy(500, 0)
		policy.TargetLocationID = &targetID
		doc := w.search(w.actor(), cap.ID, policy)
		if w.routes.attempts != 1 || w.routes.calls != 1 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 1 {
			t.Fatalf("attempts %d calls %d counts %+v", w.routes.attempts, w.routes.calls, doc.RejectionCountsByReason)
		}
	})

	t.Run("B2-14_provider_unavailable_fail_closed", func(t *testing.T) {
		w, cap := radiusWorld(t, 2)
		w.routes.fail = true
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if w.routes.attempts != 1 || len(doc.Candidates) != 0 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 2 {
			t.Fatalf("attempts %d %+v", w.routes.attempts, doc.RejectionCountsByReason)
		}
	})

	t.Run("B2-15_invalid_response_fail_closed", func(t *testing.T) {
		w, cap := radiusWorld(t, 2)
		w.routes.failAfter = 1
		w.routes.failErr = routing.ErrInvalidResponse
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if w.routes.attempts != 1 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 2 {
			t.Fatalf("attempts %d %+v", w.routes.attempts, doc.RejectionCountsByReason)
		}
	})

	t.Run("B2-16_17_partial_matrix_success_preserved", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		good := w.load(domain.VisMarketplace, place(0, 0.01, "Good"), place(0, 1, "D"), nil)
		bad := w.load(domain.VisMarketplace, place(0, 0.02, "Bad"), place(0, 1, "D"), nil)
		w.routes.defaultM = 12000
		w.routes.defaultSec = 600
		w.routes.cellErr = map[string]error{roadKey(routing.Point{}, routing.Point{Latitude: 0, Longitude: 0.02}): routing.ErrRouteNotFound}
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != good.ID || doc.Candidates[0].RoadDeadheadKm == nil || *doc.Candidates[0].RoadDeadheadKm == 0 {
			t.Fatalf("good %+v", doc.Candidates)
		}
		_, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.LoadOpportunityID == bad.ID && (row.RoadDeadheadKm != nil || !containsString(strings.Join(row.RejectReasons, ","), domain.ReasonRoadDistanceUnknown)) {
				t.Fatalf("bad row %+v", row)
			}
		}
	})

	t.Run("B2-18_no_haversine_road_fallback", func(t *testing.T) {
		w, cap := radiusWorld(t, 1)
		w.routes.fail = true
		straight := routing.StraightLineKm(0, 0, 0, 0.01)
		result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(500, 0)})
		if err != nil {
			t.Fatal(err)
		}
		if w.routes.sawHaversineSubstitute || strings.Contains(string(result.Body), "road_deadhead_km") {
			t.Fatalf("fallback body contained a road distance")
		}
		if straight <= 0 {
			t.Fatal("fixture")
		}
	})

	t.Run("B2-19_hard_watchdog_stops_new_provider_calls", func(t *testing.T) {
		w, cap := radiusWorld(t, 2)
		base := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
		step := 0
		w.svc.SetClock(func() time.Time {
			step++
			if step == 1 {
				return base
			}
			return base.Add(31 * time.Second)
		})
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if w.routes.calls != 0 || w.routes.routeCalls != 0 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 2 {
			t.Fatalf("calls %d route %d counts %+v steps %d", w.routes.calls, w.routes.routeCalls, doc.RejectionCountsByReason, step)
		}
	})

	t.Run("B2-20_context_cancellation", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(1)}
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, 0.2, "P"), place(0, 1, "D"), nil)
		w.routes.defaultM = 10000
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		policy := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchDirectionalCorridor, TargetLocationID: &targetID,
			ForwardSearchKm: f64(500), CorridorDeviationKm: f64(50), ObjectiveProfile: "MIN_DEADHEAD",
		}
		_, err := w.svc.SearchNextLoad(ctx, w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
		if !errors.Is(err, context.Canceled) || w.routes.routeCalls != 0 || w.routes.calls != 0 {
			t.Fatalf("err %v route %d matrix %d", err, w.routes.routeCalls, w.routes.calls)
		}
	})

	t.Run("B2-21_discovery_cap_1000_regression", func(t *testing.T) {
		w := newWorld(t)
		insertVisibleLoads(t, w, domain.CandidateDiscoveryCap+1, 0, 0.01)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		if len(loads) != 1000 {
			t.Fatalf("discovery %d", len(loads))
		}
	})

	t.Run("B2-22_tenant_visibility_regression", func(t *testing.T) {
		w := newWorld(t)
		own := w.loadOwned(w.carrier, domain.VisMarketplace, nil, place(0, 0.01, "Own"), place(0, 1, "D"), nil)
		hidden := w.loadOwned(w.shipper, domain.VisPrivate, nil, place(0, 0.01, "Hidden"), place(0, 1, "D"), nil)
		visible := w.load(domain.VisMarketplace, place(0, 0.02, "Visible"), place(0, 1, "D"), nil)
		loads, err := w.svc.visibleLoads(context.Background(), w.actor())
		if err != nil {
			t.Fatal(err)
		}
		ids := loadIDs(loads)
		if len(ids) != 1 || ids[0] != visible.ID || containsID(ids, own.ID) || containsID(ids, hidden.ID) {
			t.Fatalf("%s", idKey(ids))
		}
	})

	t.Run("B2-23_anonymized_privacy_regression", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		loc := uuid.New()
		pickup := place(12.345678, 34.567891, "City")
		pickup.LocationID = &loc
		pickup.Label = "SECRET-CUSTOMER"
		w.load(domain.VisAnonymized, pickup, place(12.9, 34.9, "Far"), nil)
		w.routes.defaultM = 64000
		w.routes.defaultSec = 3600
		result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(5000, 0)})
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

	t.Run("B2-24_duplicate_load_regression", func(t *testing.T) {
		id := uuid.New()
		out := boundDiscoveryCandidates([]domain.LoadOpportunity{{ID: id}, {ID: id}})
		if len(out) != 1 {
			t.Fatalf("%d", len(out))
		}
	})

	t.Run("B2-25_small_pool_result_regression", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		first := w.load(domain.VisMarketplace, place(0, 0.02, "One"), place(0, 1, "D"), nil)
		second := w.load(domain.VisMarketplace, place(0, 0.03, "Two"), place(0, 1, "D"), nil)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		ids := candidateIDs(doc)
		if len(ids) != 2 || !containsID(ids, first.ID) || !containsID(ids, second.ID) {
			t.Fatalf("%s", idKey(ids))
		}
	})

	t.Run("route_not_found_fail_closed", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(1)}
		w.routes.line = [][]float64{{0, 0}, {1, 0}}
		w.routes.failAfter = 1
		w.routes.failErr = routing.ErrRouteNotFound
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, 0.2, "P"), place(0, 1, "D"), nil)
		policy := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchDirectionalCorridor, TargetLocationID: &targetID,
			ForwardSearchKm: f64(500), CorridorDeviationKm: f64(80), ObjectiveProfile: "MIN_DEADHEAD",
		}
		doc := w.search(w.actor(), cap.ID, policy)
		if w.routes.routeCalls != 1 || w.routes.calls != 0 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 1 {
			t.Fatalf("route %d matrix %d %+v", w.routes.routeCalls, w.routes.calls, doc.RejectionCountsByReason)
		}
	})

	t.Run("later_provider_failure_keeps_earlier_road_fact", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(1)}
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		load := w.load(domain.VisMarketplace, place(0, 0.01, "P"), place(0, 1, "D"), nil)
		w.routes.defaultM = 15000
		w.routes.defaultSec = 600
		w.routes.failAfter = 2
		w.routes.failErr = routing.ErrProviderUnavailable
		policy := radiusPolicy(500, 0)
		policy.TargetLocationID = &targetID
		doc := w.search(w.actor(), cap.ID, policy)
		if w.routes.attempts != 2 || len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID || doc.Candidates[0].RoadDeadheadKm == nil || *doc.Candidates[0].RoadDeadheadKm <= 0 {
			t.Fatalf("attempts %d %+v", w.routes.attempts, doc.Candidates)
		}
	})

	t.Run("metric_labels_stay_bounded", func(t *testing.T) {
		var metric dto.Metric
		if err := bnometrics.ProviderBudgetExhausted.WithLabelValues("deadline").Write(&metric); err != nil {
			t.Fatal(err)
		}
		for _, label := range metric.Label {
			if label.GetName() != "reason" || strings.Contains(label.GetValue(), ".") {
				t.Fatalf("label %+v", label)
			}
		}
	})
}

func TestNLO05B2BoundedDiscoveryLatency(t *testing.T) {
	for _, n := range []int{100, 1000, 5000, 10000} {
		w := newWorld(t)
		insertVisibleLoads(t, w, n, 0, 0.01)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		started := time.Now()
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		elapsed := time.Since(started)
		if len(doc.Candidates) > domain.CandidateRoutingCap || w.routes.calls+w.routes.routeCalls > domain.MaxProviderCallsTotal || w.routes.maxDests > domain.MatrixDimensionLimit {
			t.Fatalf("n=%d candidates %d calls %d dests %d", n, len(doc.Candidates), w.routes.calls, w.routes.maxDests)
		}
		t.Logf("BENCHMARK_%d=%s candidates=%d provider_calls=%d", n, elapsed, len(doc.Candidates), w.routes.calls+w.routes.routeCalls)
	}
}

func radiusWorld(t *testing.T, n int) (*world, domain.Capacity) {
	t.Helper()
	w, cap, _ := radiusIDs(t, n)
	return w, cap
}

func radiusIDs(t *testing.T, n int) (*world, domain.Capacity, []uuid.UUID) {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		load := w.load(domain.VisMarketplace, place(0, 0.01+float64(i)/100000, "P"), place(0, 1, "D"), nil)
		ids = append(ids, load.ID)
	}
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	return w, cap, ids
}

func TestProviderBudgetCounterHasNoCoordinateLabel(t *testing.T) {
	counter := bnometrics.ProviderMatrixCalls
	before := testutil.ToFloat64(counter)
	bnometrics.ProviderMatrixCall()
	if testutil.ToFloat64(counter)-before != 1 {
		t.Fatal("matrix counter")
	}
	var metric dto.Metric
	if err := counter.(prometheus.Metric).Write(&metric); err != nil {
		t.Fatal(err)
	}
	if len(metric.Label) != 0 {
		t.Fatalf("%+v", metric.Label)
	}
}
