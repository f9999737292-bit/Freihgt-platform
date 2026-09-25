package routing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const (
	TrafficStatic = "STATIC"
	TrafficLive   = "LIVE"
	RouteFastest  = "FASTEST"
)

var (
	ErrProviderUnavailable = errors.New("ROUTING_PROVIDER_UNAVAILABLE")
	ErrRouteNotFound       = errors.New("ROUTE_NOT_FOUND")
	ErrTimeout             = errors.New("ROUTING_TIMEOUT")
	ErrInvalidResponse     = errors.New("ROUTING_INVALID_RESPONSE")
)

type Point struct {
	Latitude  float64
	Longitude float64
}

type VehicleProfile struct {
	GrossWeightKg  *float64
	HeightM        *float64
	WidthM         *float64
	LengthM        *float64
	AxleLoadKg     *float64
	DangerousCargo *bool
}

func (p VehicleProfile) Hash() string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (p VehicleProfile) Complete() bool {
	return p.GrossWeightKg != nil && p.HeightM != nil && p.WidthM != nil && p.LengthM != nil && p.AxleLoadKg != nil && p.DangerousCargo != nil
}

type RouteRequest struct {
	Origin         Point
	Destination    Point
	DepartureAt    *time.Time
	VehicleProfile VehicleProfile
	RouteMode      string
	TrafficMode    string
}

type Geometry struct {
	Type        string
	Coordinates [][]float64
}

type RouteResult struct {
	Provider            string
	ProviderRouteID     *string
	DistanceM           int
	DurationSeconds     int
	Geometry            Geometry
	CalculatedAt        time.Time
	TrafficMode         string
	RouteMode           string
	RequestFingerprint  string
	ProviderDefaultUsed bool
	ExpiresAt           time.Time
}

type MatrixRequest struct {
	Origins        []Point
	Destinations   []Point
	DepartureAt    *time.Time
	VehicleProfile VehicleProfile
	RouteMode      string
	TrafficMode    string
}

type MatrixCell struct {
	OriginIndex      int
	DestinationIndex int
	DistanceM        int
	DurationSeconds  int
	Err              error
}

type MatrixResult struct {
	Provider     string
	Cells        []MatrixCell
	CalculatedAt time.Time
	TrafficMode  string
	RouteMode    string
}

type Provider interface {
	Route(ctx context.Context, req RouteRequest) (RouteResult, error)
	Matrix(ctx context.Context, req MatrixRequest) (MatrixResult, error)
}

func Fingerprint(provider string, req RouteRequest) string {
	body := struct {
		Provider  string
		Origin    Point
		Dest      Point
		Profile   string
		RouteMode string
		Traffic   string
		Departure string
	}{
		Provider: provider, Origin: req.Origin, Dest: req.Destination,
		Profile: req.VehicleProfile.Hash(), RouteMode: req.RouteMode, Traffic: req.TrafficMode,
	}
	if req.TrafficMode == TrafficLive && req.DepartureAt != nil {
		body.Departure = req.DepartureAt.UTC().Truncate(15 * time.Minute).Format(time.RFC3339)
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
