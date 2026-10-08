package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestNLO05B3OneLoadSearchFeasibility(t *testing.T) {
	t.Run("corridor_equal_road_distance_rejected", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10), Status: "ACTIVE"}
		w.routes.line = [][]float64{{0, 0}, {10, 0}}
		w.routes.baselineM = 1800000
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Latitude: kmDeg(10), Longitude: kmDeg(180)}
		delivery := routing.Point{Longitude: 4}
		target := routing.Point{Longitude: 10}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 40000, sec: 1800}
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 500000, sec: 1}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 500000, sec: 1}
		doc := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchDirectionalCorridor, TargetLocationID: &targetID,
			ForwardSearchKm: f64(500), CorridorDeviationKm: f64(50), MaxDeadheadKm: f64(80),
			ObjectiveProfile: "MIN_DEADHEAD",
		})
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonDeliveryNotTowardTarget] != 1 {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		if containsID(candidateIDs(doc), load.ID) {
			t.Fatal("equal-distance load leaked")
		}
		if w.routes.calls > domain.MaxMatrixCallsPerSearch || w.routes.routeCalls > domain.MaxRouteCallsPerSearch || w.routes.calls+w.routes.routeCalls > domain.MaxProviderCallsTotal {
			t.Fatalf("provider calls matrix=%d route=%d", w.routes.calls, w.routes.routeCalls)
		}
	})

	t.Run("ellipse_missing_loaded_leg_is_unknown", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
		w.routes.baselineM = 1000000
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Longitude: 1}
		delivery := routing.Point{Longitude: 2}
		target := routing.Point{Longitude: 10}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 100000, sec: 600}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 800000, sec: 600}
		doc := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchRouteEllipse, TargetLocationID: &targetID, MaxRouteIncreaseKm: f64(200),
			ObjectiveProfile: "MIN_DEADHEAD",
		})
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		if containsID(candidateIDs(doc), load.ID) {
			t.Fatal("unknown load leaked")
		}
		if w.routes.calls > domain.MaxMatrixCallsPerSearch || w.routes.routeCalls > domain.MaxRouteCallsPerSearch || w.routes.calls+w.routes.routeCalls > domain.MaxProviderCallsTotal {
			t.Fatalf("provider calls matrix=%d route=%d", w.routes.calls, w.routes.routeCalls)
		}
	})

	t.Run("two_loads_are_not_a_chain", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
		w.routes.baselineM = 1000000
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		target := routing.Point{Longitude: 10}
		passPickup := routing.Point{Longitude: 1}
		passDelivery := routing.Point{Longitude: 2}
		missPickup := routing.Point{Longitude: 3}
		missDelivery := routing.Point{Longitude: 4}
		pass := w.load(domain.VisMarketplace, pointPlace(passPickup, "Pass"), pointPlace(passDelivery, "PassD"), nil)
		miss := w.load(domain.VisMarketplace, pointPlace(missPickup, "Miss"), pointPlace(missDelivery, "MissD"), nil)
		release := routing.Point{}
		w.routes.roads[roadKey(release, passPickup)] = roadCell{m: 100000, sec: 600}
		w.routes.roads[roadKey(passPickup, passDelivery)] = roadCell{m: 200000, sec: 600}
		w.routes.roads[roadKey(passDelivery, target)] = roadCell{m: 800000, sec: 600}
		w.routes.roads[roadKey(passPickup, target)] = roadCell{m: 900000, sec: 600}
		w.routes.roads[roadKey(release, missPickup)] = roadCell{m: 100000, sec: 600}
		w.routes.roads[roadKey(missDelivery, target)] = roadCell{m: 800000, sec: 600}
		doc := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchRouteEllipse, TargetLocationID: &targetID, MaxRouteIncreaseKm: f64(150),
			ObjectiveProfile: "MIN_DEADHEAD",
		})
		if doc.EligibleCandidateCount != 1 || len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != pass.ID {
			t.Fatalf("eligible %+v counts %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		if doc.Candidates[0].RouteIncreaseKm == nil || *doc.Candidates[0].RouteIncreaseKm != 100 {
			t.Fatalf("increase %+v", doc.Candidates[0])
		}
		if containsID(candidateIDs(doc), miss.ID) || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 1 {
			t.Fatalf("independent %+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("radius_does_not_apply_backhaul_rules", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Longitude: kmDeg(4)}
		w.load(domain.VisMarketplace, pointPlace(pickup, "P"), place(0, 1, "D"), nil)
		w.routes.defaultM = 30000
		w.routes.defaultSec = 1800
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if doc.EligibleCandidateCount != 1 || doc.RejectionCountsByReason[domain.ReasonDeliveryNotTowardTarget] != 0 || doc.RejectionCountsByReason[domain.ReasonRouteIncreaseExceeded] != 0 {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})
}

func TestNLO05B3CorridorFeasibility(t *testing.T) {
	t.Run("delivery_toward_target_eligible", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		pickup := routing.Point{Latitude: kmDeg(22), Longitude: kmDeg(240)}
		delivery := routing.Point{Longitude: 8}
		target := routing.Point{Longitude: 10}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 64000, sec: 3600}
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 800000, sec: 1}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 400000, sec: 1}
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if doc.EligibleCandidateCount != 1 || len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		raw, _ := json.Marshal(doc)
		if strings.Contains(string(raw), `"execution_supported":true`) {
			t.Fatal("search became executable")
		}
		assertProviderCeiling(t, w)
	})

	t.Run("delivery_farther_rejected", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		pickup := routing.Point{Latitude: kmDeg(10), Longitude: kmDeg(180)}
		delivery := routing.Point{Longitude: 1}
		target := routing.Point{Longitude: 10}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 40000, sec: 1800}
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 500000, sec: 1}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 900000, sec: 1}
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonDeliveryNotTowardTarget] != 1 || containsID(candidateIDs(doc), load.ID) {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})

	t.Run("backtrack_rejected", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		load := w.load(domain.VisMarketplace, place(0, -kmDeg(20), "Back"), place(0, 1, "D"), nil)
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if doc.RejectionCountsByReason[domain.ReasonBacktrackRejected] != 1 || containsID(candidateIDs(doc), load.ID) {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("lateral_deviation_rejected", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		load := w.load(domain.VisMarketplace, place(kmDeg(80), kmDeg(100), "Side"), place(0, 1, "D"), nil)
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if doc.RejectionCountsByReason[domain.ReasonLateralExceeded] != 1 || containsID(candidateIDs(doc), load.ID) {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("forward_search_rejected", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(600), "Past"), place(0, 1, "D"), nil)
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if doc.RejectionCountsByReason[domain.ReasonForwardExceeded] != 1 || containsID(candidateIDs(doc), load.ID) {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})
}

func TestNLO05B3EllipseFeasibility(t *testing.T) {
	t.Run("valid_increase_includes_baseline", func(t *testing.T) {
		w, targetID, cap, load := ellipseLoad(t)
		doc := w.search(w.actor(), cap.ID, ellipsePolicy(targetID, 150))
		if doc.EligibleCandidateCount != 1 || len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		got := doc.Candidates[0].RouteIncreaseKm
		withoutBaseline := 100.0 + 200.0 + 800.0
		if got == nil || *got != 100 || *got == withoutBaseline {
			t.Fatalf("increase %v", got)
		}
		assertProviderCeiling(t, w)
	})

	t.Run("excessive_increase_rejected", func(t *testing.T) {
		w, targetID, cap, load := ellipseLoad(t)
		doc := w.search(w.actor(), cap.ID, ellipsePolicy(targetID, 50))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonRouteIncreaseExceeded] != 1 || containsID(candidateIDs(doc), load.ID) {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})

	t.Run("error_cell_zero_is_not_a_road_distance", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
		w.routes.baselineM = 1000000
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Longitude: 1}
		delivery := routing.Point{Longitude: 2}
		target := routing.Point{Longitude: 10}
		w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 100000, sec: 600}
		w.routes.roads[roadKey(pickup, delivery)] = roadCell{m: 200000, sec: 600}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 800000, sec: 600}
		w.routes.cellErr = map[string]error{roadKey(pickup, delivery): routing.ErrRouteNotFound}
		doc := w.search(w.actor(), cap.ID, ellipsePolicy(targetID, 200))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})
}

func TestNLO05B3TimeFeasibility(t *testing.T) {
	t.Run("early_arrival_records_waiting", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		pickup := routing.Point{Latitude: kmDeg(10), Longitude: kmDeg(120)}
		delivery := routing.Point{Longitude: 8}
		target := routing.Point{Longitude: 10}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), window(w.at.Add(90*time.Minute), w.at.Add(4*time.Hour)))
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 40000, sec: 1800}
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 800000, sec: 1}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 400000, sec: 1}
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID || doc.Candidates[0].WaitingMinutes == nil || *doc.Candidates[0].WaitingMinutes != 60 {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})

	t.Run("missed_pickup_window", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		pickup := routing.Point{Latitude: kmDeg(10), Longitude: kmDeg(120)}
		delivery := routing.Point{Longitude: 8}
		target := routing.Point{Longitude: 10}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), window(w.at.Add(-2*time.Hour), w.at.Add(20*time.Minute)))
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 40000, sec: 1800}
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 800000, sec: 1}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 400000, sec: 1}
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonPickupWindowMissed] != 1 || containsID(candidateIDs(doc), load.ID) {
			t.Fatalf("%+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
	})
}

func TestNLO05B3BoundsAndSafety(t *testing.T) {
	if domain.CandidateDiscoveryCap != 1000 || domain.CandidateRoutingCap != 25 || domain.CandidateFinalEvaluationCap != 25 {
		t.Fatalf("caps %d %d %d", domain.CandidateDiscoveryCap, domain.CandidateRoutingCap, domain.CandidateFinalEvaluationCap)
	}
	if domain.MaxMatrixCallsPerSearch != 4 || domain.MaxRouteCallsPerSearch != 2 || domain.MaxProviderCallsTotal != 6 {
		t.Fatal("provider caps")
	}
	if domain.SearchHardBudget != 30*time.Second || domain.ProviderRequestTimeout != 5*time.Second || domain.ProviderRetryCount != 0 || domain.HaversineCanonicalRoad {
		t.Fatal("budget or haversine")
	}

	t.Run("own_and_private_loads_excluded", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		own := w.loadOwned(w.carrier, domain.VisMarketplace, nil, place(0, kmDeg(80), "Own"), place(0, 8, "D"), nil)
		hidden := w.loadOwned(w.shipper, domain.VisPrivate, nil, place(0, kmDeg(90), "Hidden"), place(0, 8, "D"), nil)
		pickup := routing.Point{Latitude: kmDeg(5), Longitude: kmDeg(100)}
		visible := w.load(domain.VisMarketplace, pointPlace(pickup, "Visible"), pointPlace(routing.Point{Longitude: 8}, "D"), nil)
		target := routing.Point{Longitude: 10}
		w.routes.defaultM = 40000
		w.routes.defaultSec = 600
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 800000, sec: 1}
		w.routes.roads[roadKey(routing.Point{Longitude: 8}, target)] = roadCell{m: 200000, sec: 1}
		doc := w.search(w.actor(), cap.ID, corridorPolicy(targetID))
		ids := candidateIDs(doc)
		if !containsID(ids, visible.ID) || containsID(ids, own.ID) || containsID(ids, hidden.ID) {
			t.Fatalf("ids %v counts %+v", ids, doc.RejectionCountsByReason)
		}
		other := serviceActor(uuid.New(), nil)
		if _, err := w.svc.SearchNextLoad(context.Background(), other, SearchCommand{CapacityID: cap.ID, Policy: corridorPolicy(targetID)}); !notFound(err) {
			t.Fatalf("tenant %v", err)
		}
	})

	t.Run("anonymized_exact_geo_hidden", func(t *testing.T) {
		w, targetID, cap := newCorridor(t)
		loc := uuid.New()
		pickup := place(0, kmDeg(100), "City")
		pickup.LocationID = &loc
		pickup.Label = "SECRET-CUSTOMER"
		delivery := place(0, 8, "Drop")
		w.load(domain.VisAnonymized, pickup, delivery, nil)
		target := routing.Point{Longitude: 10}
		w.routes.roads[roadKey(routing.Point{}, routing.Point{Latitude: 0, Longitude: kmDeg(100)})] = roadCell{m: 40000, sec: 1800}
		w.routes.roads[roadKey(routing.Point{Latitude: 0, Longitude: kmDeg(100)}, target)] = roadCell{m: 800000, sec: 1}
		w.routes.roads[roadKey(routing.Point{Latitude: 0, Longitude: 8}, target)] = roadCell{m: 400000, sec: 1}
		result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: corridorPolicy(targetID)})
		if err != nil {
			t.Fatal(err)
		}
		body := string(result.Body)
		for _, leaked := range []string{loc.String(), "SECRET-CUSTOMER", "latitude", "longitude"} {
			if strings.Contains(body, leaked) {
				t.Fatalf("leaked %s", leaked)
			}
		}
		if !strings.Contains(body, targetID.String()) {
			t.Fatal("corridor target missing")
		}
		if !strings.Contains(body, "deadhead_bucket") {
			t.Fatalf("missing bucket %s", body)
		}
	})
}

func newCorridor(t *testing.T) (*world, uuid.UUID, domain.Capacity) {
	t.Helper()
	w := newWorld(t)
	targetID := uuid.New()
	w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10), Status: "ACTIVE"}
	w.routes.line = [][]float64{{0, 0}, {10, 0}}
	w.routes.baselineM = 1800000
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	return w, targetID, cap
}

func corridorPolicy(targetID uuid.UUID) domain.NextLoadSearchPolicy {
	return domain.NextLoadSearchPolicy{
		SearchMode: domain.SearchDirectionalCorridor, TargetLocationID: &targetID,
		ForwardSearchKm: f64(500), CorridorDeviationKm: f64(50), MaxDeadheadKm: f64(80),
		ObjectiveProfile: "MIN_DEADHEAD",
	}
}

func ellipsePolicy(targetID uuid.UUID, maxIncrease float64) domain.NextLoadSearchPolicy {
	return domain.NextLoadSearchPolicy{
		SearchMode: domain.SearchRouteEllipse, TargetLocationID: &targetID, MaxRouteIncreaseKm: f64(maxIncrease),
		ObjectiveProfile: "MIN_DEADHEAD",
	}
}

func ellipseLoad(t *testing.T) (*world, uuid.UUID, domain.Capacity, domain.LoadOpportunity) {
	t.Helper()
	w := newWorld(t)
	targetID := uuid.New()
	w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
	w.routes.baselineM = 1000000
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	pickup := routing.Point{Longitude: 1}
	delivery := routing.Point{Longitude: 2}
	target := routing.Point{Longitude: 10}
	load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
	w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 100000, sec: 600}
	w.routes.roads[roadKey(pickup, delivery)] = roadCell{m: 200000, sec: 600}
	w.routes.roads[roadKey(delivery, target)] = roadCell{m: 800000, sec: 600}
	w.routes.roads[roadKey(pickup, target)] = roadCell{m: 900000, sec: 600}
	return w, targetID, cap, load
}

func assertProviderCeiling(t *testing.T, w *world) {
	t.Helper()
	if w.routes.calls > domain.MaxMatrixCallsPerSearch || w.routes.routeCalls > domain.MaxRouteCallsPerSearch || w.routes.calls+w.routes.routeCalls > domain.MaxProviderCallsTotal {
		t.Fatalf("provider calls matrix=%d route=%d", w.routes.calls, w.routes.routeCalls)
	}
}
