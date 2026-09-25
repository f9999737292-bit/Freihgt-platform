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
	// TrafficCurrent is current road conditions at the time of the request.
	// It is a provider-neutral mode. Adapters translate it; it is not sent as a provider enum.
	TrafficCurrent = "CURRENT"
	// TrafficStatistical is time-based statistical planning for DepartureAt.
	// A departure time is part of the request only when this mode is selected and DepartureAt is set.
	TrafficStatistical = "STATISTICAL"
	RouteFastest       = "FASTEST"
	RouteShortest      = "SHORTEST"
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
	Provider            string
	Cells               []MatrixCell
	CalculatedAt        time.Time
	TrafficMode         string
	RouteMode           string
	ProviderDefaultUsed bool
	RequestFingerprint  string
	ExpiresAt           time.Time
}

type Provider interface {
	Route(ctx context.Context, req RouteRequest) (RouteResult, error)
	Matrix(ctx context.Context, req MatrixRequest) (MatrixResult, error)
}

// IdentifiedProvider exposes a normalized vendor name such as 2GIS.
// A provider that does not implement it has no persisted vendor identity.
type IdentifiedProvider interface {
	ProviderName() string
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
		Departure: departureBucket(req.TrafficMode, req.DepartureAt),
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func MatrixFingerprint(provider string, req MatrixRequest) string {
	body := struct {
		Provider     string
		Origins      []Point
		Destinations []Point
		Profile      string
		RouteMode    string
		Traffic      string
		Departure    string
	}{
		Provider: provider, Origins: req.Origins, Destinations: req.Destinations,
		Profile: req.VehicleProfile.Hash(), RouteMode: req.RouteMode, Traffic: req.TrafficMode,
		Departure: departureBucket(req.TrafficMode, req.DepartureAt),
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func departureBucket(mode string, at *time.Time) string {
	if mode != TrafficStatistical || at == nil {
		return ""
	}
	return at.UTC().Truncate(15 * time.Minute).Format(time.RFC3339)
}
