package domain

import (
	"math"
	"testing"
)

func TestCorridorProjectionAheadBehindAndLateral(t *testing.T) {
	line := [][]float64{{0, 0}, {10, 0}}
	aheadLat, aheadLon := kmToDegrees(22), kmToDegrees(240)
	ahead, err := ProjectPointOnRoute(aheadLat, aheadLon, line)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(ahead.ForwardProgressKm-240) > 5 || math.Abs(ahead.LateralDistanceKm-22) > 5 {
		t.Fatalf("ahead %+v", ahead)
	}

	behind, err := ProjectPointOnRoute(0, -kmToDegrees(30), line)
	if err != nil || behind.ForwardProgressKm >= 0 {
		t.Fatalf("behind %+v %v", behind, err)
	}

	left, err := ProjectPointOnRoute(kmToDegrees(18), kmToDegrees(120), line)
	if err != nil {
		t.Fatal(err)
	}
	right, err := ProjectPointOnRoute(-kmToDegrees(18), kmToDegrees(120), line)
	if err != nil {
		t.Fatal(err)
	}
	if left.LateralDistanceKm < 10 || right.LateralDistanceKm < 10 {
		t.Fatalf("left/right %+v %+v", left, right)
	}
	if math.Abs(left.LateralDistanceKm-right.LateralDistanceKm) > 2 {
		t.Fatalf("left/right differ %+v %+v", left, right)
	}
	if left.ForwardProgressKm < 0 || right.ForwardProgressKm < 0 {
		t.Fatalf("left/right not ahead %+v %+v", left, right)
	}

	beyond, err := ProjectPointOnRoute(0, kmToDegrees(600), [][]float64{{0, 0}, {kmToDegrees(200), 0}})
	if err != nil || beyond.ForwardProgressKm <= 200 {
		t.Fatalf("beyond %+v %v", beyond, err)
	}

	join := kmToDegrees(200)
	boundary, err := ProjectPointOnRoute(0.001, join, [][]float64{{0, 0}, {join, 0}, {kmToDegrees(400), 0}})
	if err != nil || math.Abs(boundary.ForwardProgressKm-200) > 5 || boundary.LateralDistanceKm > 5 {
		t.Fatalf("boundary %+v %v", boundary, err)
	}
}

func kmToDegrees(km float64) float64 {
	return km / (earthRadiusKm * math.Pi / 180)
}
