package domain

import "math"

const earthRadiusKm = 6371.0

// RouteProjection is candidate geometry on a route polyline.
// Forward progress and lateral distance are not road distance.
type RouteProjection struct {
	ForwardProgressKm float64
	LateralDistanceKm float64
}

// ProjectPointOnRoute projects a point onto a LineString using spherical cross-track distance.
// Coordinates are [longitude, latitude]. Forward progress is negative when the point is behind the route start.
func ProjectPointOnRoute(latitude, longitude float64, line [][]float64) (RouteProjection, error) {
	if len(line) < 2 {
		return RouteProjection{}, ErrDirectionTargetGeo
	}
	best := scored{score: math.MaxFloat64}
	cumulative := 0.0
	for i := 0; i < len(line)-1; i++ {
		startLon, startLat := line[i][0], line[i][1]
		endLon, endLat := line[i+1][0], line[i+1][1]
		segment := haversineKm(startLat, startLon, endLat, endLon)
		along, cross := crossTrackKm(startLat, startLon, endLat, endLon, latitude, longitude)
		last := i == len(line)-2
		switch {
		case along < 0 && i == 0:
			consider(&best, along, math.Abs(cross))
		case along > segment && last:
			consider(&best, cumulative+along, math.Abs(cross))
		case along >= 0 && along <= segment:
			consider(&best, cumulative+along, math.Abs(cross))
		}
		cumulative += segment
	}
	if !best.ok {
		return RouteProjection{}, ErrDirectionTargetGeo
	}
	return RouteProjection{ForwardProgressKm: best.forward, LateralDistanceKm: best.lateral}, nil
}

type scored struct {
	forward float64
	lateral float64
	score   float64
	ok      bool
}

func consider(best *scored, forward, lateral float64) {
	score := math.Abs(lateral)
	if !best.ok || score < best.score-1e-6 || (math.Abs(score-best.score) <= 1e-6 && forward < best.forward) {
		*best = scored{forward: forward, lateral: lateral, score: score, ok: true}
	}
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	r1 := lat1 * math.Pi / 180
	r2 := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(r1)*math.Cos(r2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func crossTrackKm(lat1, lon1, lat2, lon2, lat3, lon3 float64) (alongKm, crossKm float64) {
	distance := haversineKm(lat1, lon1, lat3, lon3)
	if distance == 0 {
		return 0, 0
	}
	θ13 := bearingRad(lat1, lon1, lat3, lon3)
	θ12 := bearingRad(lat1, lon1, lat2, lon2)
	delta := θ13 - θ12
	angular := distance / earthRadiusKm
	cross := math.Asin(clamp(math.Sin(angular)*math.Sin(delta), -1, 1)) * earthRadiusKm
	cosCross := math.Cos(cross / earthRadiusKm)
	if math.Abs(cosCross) < 1e-12 {
		return 0, cross
	}
	along := math.Acos(clamp(math.Cos(angular)/cosCross, -1, 1)) * earthRadiusKm
	if math.Cos(delta) < 0 {
		along = -along
	}
	return along, cross
}

func bearingRad(lat1, lon1, lat2, lon2 float64) float64 {
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(φ2)
	x := math.Cos(φ1)*math.Sin(φ2) - math.Sin(φ1)*math.Cos(φ2)*math.Cos(dLon)
	return math.Atan2(y, x)
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
