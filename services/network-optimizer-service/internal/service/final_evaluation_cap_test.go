package service

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestNLO05B4FinalEvaluationCapInvariant(t *testing.T) {
	if domain.CandidateDiscoveryCap != 1000 || domain.CandidateRoutingCap != 25 || domain.CandidateFinalEvaluationCap != 25 {
		t.Fatalf("caps discovery=%d routing=%d final=%d", domain.CandidateDiscoveryCap, domain.CandidateRoutingCap, domain.CandidateFinalEvaluationCap)
	}
	if domain.MatrixDimensionLimit != 25 || domain.MaxMatrixCallsPerSearch != 4 || domain.MaxRouteCallsPerSearch != 2 || domain.MaxProviderCallsTotal != 6 || domain.ProviderRetryCount != 0 {
		t.Fatal("provider budget")
	}

	t.Run("B4-01_RADIUS", func(t *testing.T) {
		w := newWorld(t)
		loads := insertVisibleLoads(t, w, domain.CandidateDiscoveryCap, 0, 0.01)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		assertFinalEvaluationCap(t, w, cap, radiusPolicy(500, 0), loads)
	})

	t.Run("B4-02_DIRECTIONAL_CORRIDOR", func(t *testing.T) {
		w, targetID, cap, loads := b4DirectionWorld(t)
		assertFinalEvaluationCap(t, w, cap, corridorPolicy(targetID), loads)
	})

	t.Run("B4-03_ROUTE_ELLIPSE", func(t *testing.T) {
		w, targetID, cap, loads := b4DirectionWorld(t)
		w.routes.baselineM = 1000000
		assertFinalEvaluationCap(t, w, cap, ellipsePolicy(targetID, 100000), loads)
	})
}

func b4DirectionWorld(t *testing.T) (*world, uuid.UUID, domain.Capacity, []domain.LoadOpportunity) {
	t.Helper()
	w := newWorld(t)
	targetID := uuid.New()
	w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10), Status: "ACTIVE"}
	w.routes.line = [][]float64{{0, 0}, {10, 0}}
	w.routes.baselineM = 1000000
	loads := insertVisibleLoads(t, w, domain.CandidateDiscoveryCap, 0, 0.1)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	release := routing.Point{}
	target := routing.Point{Longitude: 10}
	pickup := routing.Point{Longitude: 0.1}
	delivery := routing.Point{Longitude: 1.1}
	w.routes.roads[roadKey(release, pickup)] = roadCell{m: 10000, sec: 600}
	w.routes.roads[roadKey(pickup, target)] = roadCell{m: 100000, sec: 600}
	w.routes.roads[roadKey(delivery, target)] = roadCell{m: 40000, sec: 600}
	w.routes.roads[roadKey(pickup, delivery)] = roadCell{m: 20000, sec: 600}
	return w, targetID, cap, loads
}

func assertFinalEvaluationCap(t *testing.T, w *world, cap domain.Capacity, policy domain.NextLoadSearchPolicy, loads []domain.LoadOpportunity) {
	t.Helper()
	if len(loads) != domain.CandidateDiscoveryCap {
		t.Fatalf("discovered %d", len(loads))
	}
	routed, tail := b4Split(loads)
	if len(routed) != domain.CandidateRoutingCap || len(tail) <= 0 {
		t.Fatalf("split routed %d tail %d", len(routed), len(tail))
	}

	resetRouteCounters(w)
	doc := w.search(w.actor(), cap.ID, policy)
	assertB4Response(t, w, doc, routed, tail)

	resetRouteCounters(w)
	again := w.search(w.actor(), cap.ID, policy)
	if idKey(candidateIDs(doc)) != idKey(candidateIDs(again)) || idKey(candidateIDs(again)) != idKey(routed) {
		t.Fatalf("selection changed or left load-id order: %s", idKey(candidateIDs(again)))
	}

	limit := domain.CandidateDiscoveryCap
	resetRouteCounters(w)
	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{
		CapacityID: cap.ID, Policy: policy, CandidateLimit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	var limited SearchResponse
	if err := json.Unmarshal(result.Body, &limited); err != nil {
		t.Fatal(err)
	}
	if limited.EligibleCandidateCount > domain.CandidateFinalEvaluationCap || len(limited.Candidates) > domain.CandidateFinalEvaluationCap || w.routes.maxDests > domain.CandidateRoutingCap {
		t.Fatalf("client limit raised cap eligible=%d returned=%d dests=%d", limited.EligibleCandidateCount, len(limited.Candidates), w.routes.maxDests)
	}
	if idKey(candidateIDs(limited)) != idKey(routed) {
		t.Fatal("client limit changed the eligible set")
	}
}

func assertB4Response(t *testing.T, w *world, doc SearchResponse, routed, tail []uuid.UUID) {
	t.Helper()
	if w.routes.maxDests > domain.CandidateRoutingCap || w.routes.maxOrigins > domain.MatrixDimensionLimit {
		t.Fatalf("dests %d origins %d", w.routes.maxDests, w.routes.maxOrigins)
	}
	if w.routes.calls > domain.MaxMatrixCallsPerSearch || w.routes.routeCalls > domain.MaxRouteCallsPerSearch || w.routes.calls+w.routes.routeCalls > domain.MaxProviderCallsTotal {
		t.Fatalf("matrix %d route %d", w.routes.calls, w.routes.routeCalls)
	}
	if w.routes.attempts != w.routes.calls+w.routes.routeCalls {
		t.Fatalf("attempts %d calls %d", w.routes.attempts, w.routes.calls+w.routes.routeCalls)
	}
	if doc.EligibleCandidateCount != domain.CandidateRoutingCap || doc.RankedCandidateCount+doc.UnrankedEligibleCount != doc.EligibleCandidateCount {
		t.Fatalf("eligible %d ranked %d unranked %d", doc.EligibleCandidateCount, doc.RankedCandidateCount, doc.UnrankedEligibleCount)
	}
	if doc.RankedCandidateCount > domain.CandidateFinalEvaluationCap || doc.UnrankedEligibleCount > domain.CandidateFinalEvaluationCap || len(doc.Candidates) != doc.EligibleCandidateCount {
		t.Fatalf("ranked %d unranked %d returned %d", doc.RankedCandidateCount, doc.UnrankedEligibleCount, len(doc.Candidates))
	}
	if doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != len(tail) {
		t.Fatalf("unknown %d tail %d counts %+v", doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown], len(tail), doc.RejectionCountsByReason)
	}
	returned := map[uuid.UUID]struct{}{}
	for _, id := range candidateIDs(doc) {
		returned[id] = struct{}{}
		if _, ok := routedSet(tail)[id]; ok {
			t.Fatalf("tail candidate %s", id)
		}
	}
	if idKey(candidateIDs(doc)) != idKey(routed) {
		t.Fatal("eligible set is not the load-id routing prefix")
	}

	_, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != domain.CandidateDiscoveryCap {
		t.Fatalf("persisted %d", len(rows))
	}
	scored := 0
	tailSeen := 0
	for _, row := range rows {
		_, isTail := routedSet(tail)[row.LoadOpportunityID]
		_, isRouted := routedSet(routed)[row.LoadOpportunityID]
		switch {
		case isTail:
			tailSeen++
			if row.Eligibility != "REJECTED" || row.ScoreStatus != domain.ScoreNotApplicable || row.Rank != nil || row.ScoreTotal != nil || row.RoadDeadheadKm != nil || !containsIDReason(row.RejectReasons, domain.ReasonRoadDistanceUnknown) {
				t.Fatalf("tail row %+v", row)
			}
			if _, ok := returned[row.LoadOpportunityID]; ok {
				t.Fatalf("tail returned %s", row.LoadOpportunityID)
			}
		case isRouted:
			if row.Eligibility != "ELIGIBLE" || (row.ScoreStatus != domain.ScoreRanked && row.ScoreStatus != domain.ScoreUnranked) || row.RoadDeadheadKm == nil {
				t.Fatalf("routed row %+v", row)
			}
			scored++
		default:
			t.Fatalf("unexpected persisted load %s", row.LoadOpportunityID)
		}
	}
	if tailSeen != len(tail) || scored != domain.CandidateRoutingCap {
		t.Fatalf("tail seen %d scored %d", tailSeen, scored)
	}
}

func b4Split(loads []domain.LoadOpportunity) (routed, tail []uuid.UUID) {
	ids := loadIDs(loads)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids[:domain.CandidateRoutingCap], ids[domain.CandidateRoutingCap:]
}

func routedSet(ids []uuid.UUID) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func containsIDReason(reasons []string, reason string) bool {
	for _, item := range reasons {
		if item == reason {
			return true
		}
	}
	return false
}

func resetRouteCounters(w *world) {
	w.routes.calls = 0
	w.routes.routeCalls = 0
	w.routes.maxDests = 0
	w.routes.maxOrigins = 0
	w.routes.attempts = 0
}
