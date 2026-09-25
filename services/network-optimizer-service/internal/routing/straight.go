package routing

import "math"

// StraightLineKm is a candidate prefilter only.
// It must never be stored as road_deadhead_km, road duration, or route increase.
func StraightLineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthKm = 6371.0
	r1 := lat1 * math.Pi / 180
	r2 := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(r1)*math.Cos(r2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
