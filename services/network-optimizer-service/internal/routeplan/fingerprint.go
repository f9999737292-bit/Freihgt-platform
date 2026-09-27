package routeplan

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Point struct {
	Kind       string
	LocationID *uuid.UUID
	Latitude   float64
	Longitude  float64
	Source     string
	ObservedAt *time.Time
}

func (p Point) Fingerprint() string {
	parts := []string{
		"point_kind=" + p.Kind,
		"latitude=" + strconv.FormatFloat(p.Latitude, 'g', -1, 64),
		"longitude=" + strconv.FormatFloat(p.Longitude, 'g', -1, 64),
		"point_source=" + p.Source,
	}
	if p.LocationID != nil {
		parts = append(parts, "location_id="+p.LocationID.String())
	}
	if p.Kind == PointAnchor && p.ObservedAt != nil {
		parts = append(parts, "observed_at="+p.ObservedAt.UTC().Format(time.RFC3339Nano))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func LegKey(fromFP, toFP, vehicleProfileHash, routeMode, trafficMode, departureBucket string) string {
	return strings.Join([]string{
		"from=" + fromFP,
		"to=" + toFP,
		"vehicle=" + vehicleProfileHash,
		"route=" + routeMode,
		"traffic=" + trafficMode,
		"departure=" + departureBucket,
	}, "|")
}

func DepartureBucket(trafficMode string, at *time.Time) string {
	if trafficMode != "STATISTICAL" || at == nil {
		return ""
	}
	return at.UTC().Truncate(15 * time.Minute).Format(time.RFC3339)
}

func ProofFingerprint(leg Leg) string {
	used := "0"
	if leg.ProviderDefaultUsed {
		used = "1"
	}
	raw := strings.Join([]string{
		leg.RequestFingerprint,
		leg.Provider,
		leg.ProviderRouteID,
		strconv.Itoa(leg.DistanceM),
		strconv.Itoa(leg.DurationSeconds),
		leg.RouteMode,
		leg.TrafficMode,
		used,
		leg.CalculatedAt.UTC().Format(time.RFC3339Nano),
		leg.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
