package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBNO184To193SearchPolicy(t *testing.T) {
	target := uuid.New()
	forward, lateral, deadhead := 500.0, 50.0, 80.0
	preferred, minutes, loaded, increase := 40.0, 90.0, 100.0, 25.0
	policy := NextLoadSearchPolicy{
		SearchMode: SearchDirectionalCorridor, TargetLocationID: &target,
		ForwardSearchKm: &forward, CorridorDeviationKm: &lateral, MaxDeadheadKm: &deadhead,
		PreferredDeadheadKm: &preferred, MaxDeadheadMinutes: &minutes,
		MinLoadedDistanceKm: &loaded, MaxRouteIncreaseKm: &increase,
		ObjectiveProfile: "MIN_EMPTY",
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("BNO184 %v", err)
	}
	if policy.ForwardSearchKm == policy.CorridorDeviationKm || *policy.MaxDeadheadKm == *policy.ForwardSearchKm {
		t.Fatal("BNO188 constraints collapsed")
	}
	carrier, capacity, request := 150.0, 100.0, 120.0
	effective := MinHardLimit(&carrier, &capacity, &request)
	if effective == nil || *effective != 100 {
		t.Fatalf("BNO185 %v", effective)
	}
	wider := 200.0
	if got := MinHardLimit(&carrier, &wider); got == nil || *got != 150 {
		t.Fatalf("BNO186 %v", got)
	}
	if err := RequireDirectionTarget(SearchDirectionalCorridor, nil, true); !errors.Is(err, ErrDirectionTarget) {
		t.Fatalf("BNO187 %v", err)
	}
	if err := RequireDirectionTarget(SearchRouteEllipse, &target, false); !errors.Is(err, ErrDirectionTargetGeo) {
		t.Fatalf("BNO187 geo %v", err)
	}
	if err := CheckCorridorPlacement(CorridorPlacement{ForwardProgressKm: 400, LateralKm: 60, RoadDeadheadKm: 10}, forward, lateral, deadhead); !errors.Is(err, ErrLateralExceeded) {
		t.Fatalf("BNO188 %v", err)
	}
	if err := CheckCorridorPlacement(CorridorPlacement{ForwardProgressKm: -1, LateralKm: 1, RoadDeadheadKm: 1}, forward, lateral, deadhead); !errors.Is(err, ErrBacktrackRejected) {
		t.Fatalf("BNO188 backtrack %v", err)
	}
	if err := CheckCorridorPlacement(CorridorPlacement{ForwardProgressKm: 100, LateralKm: 10, RoadDeadheadKm: 90}, forward, lateral, deadhead); !errors.Is(err, ErrDeadheadExceeded) {
		t.Fatalf("BNO189 %v", err)
	}
	if err := RequireRoadDeadhead(nil, false); !errors.Is(err, ErrRoadDistanceUnknown) {
		t.Fatalf("BNO190 %v", err)
	}
	if err := RequireRoadDeadhead(nil, true); !errors.Is(err, ErrRoadDistanceUnknown) {
		t.Fatalf("BNO190 allow %v", err)
	}
	zero := 0.0
	if err := RequireRoadDeadhead(&zero, false); err != nil {
		t.Fatalf("BNO190 real zero %v", err)
	}
	available := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	arrival := ArrivalAtPickup(available, 90)
	if !arrival.Equal(available.Add(90 * time.Minute)) {
		t.Fatalf("BNO191 %s", arrival)
	}
	start := available.Add(30 * time.Minute)
	end := available.Add(60 * time.Minute)
	if _, err := EvaluatePickupWindow(arrival, TimeWindow{Start: &start, End: &end}); !errors.Is(err, ErrPickupWindowMissed) {
		t.Fatalf("BNO192 %v", err)
	}
	waiting, err := EvaluatePickupWindow(available, TimeWindow{Start: &start, End: &end})
	if err != nil || waiting != 30*time.Minute {
		t.Fatalf("BNO192 wait %v %v", waiting, err)
	}
	increaseKm := RouteIncreaseKm(1000, 80, 980)
	if increaseKm != 60 {
		t.Fatalf("BNO193 %v", increaseKm)
	}
	if err := CheckRouteIncrease(increaseKm, 50); !errors.Is(err, ErrRouteIncreaseExceeded) {
		t.Fatalf("BNO193 limit %v", err)
	}
}

func TestBNO183HaversineIsNotRoadDistance(t *testing.T) {
	if HaversineCanonicalRoad {
		t.Fatal("HAVERSINE_CANONICAL_ROAD_DISTANCE")
	}
	straight := 12.5
	leg := LegDistances{StraightLineKm: &straight}
	if leg.RoadDeadheadKm != nil || leg.RoadDeadheadMinutes != nil || leg.LoadedRoadDistanceKm != nil || leg.RouteIncreaseKm != nil {
		t.Fatal("straight-line value occupied a road field")
	}
}
