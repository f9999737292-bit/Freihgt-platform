package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
)

func firstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstUUID(values ...*uuid.UUID) *uuid.UUID {
	for _, value := range values {
		if value != nil && *value != uuid.Nil {
			copy := *value
			return &copy
		}
	}
	return nil
}

func firstFloat(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			copy := *value
			return &copy
		}
	}
	return nil
}

func MaxHardLimit(limits ...*float64) *float64 {
	var max *float64
	for _, limit := range limits {
		if limit == nil {
			continue
		}
		if max == nil || *limit > *max {
			value := *limit
			max = &value
		}
	}
	return max
}

// EffectiveSearchPolicy resolves carrier, capacity, and request policies.
// Ordinary fields use request, then capacity, then carrier.
// Hard maxima use the strictest supplied value. A request cannot widen one.
func EffectiveSearchPolicy(carrier, capacity, request NextLoadSearchPolicy) NextLoadSearchPolicy {
	return NextLoadSearchPolicy{
		SearchMode:               firstText(request.SearchMode, capacity.SearchMode, carrier.SearchMode),
		TargetLocationID:         firstUUID(request.TargetLocationID, capacity.TargetLocationID, carrier.TargetLocationID),
		PreferredDeadheadKm:      firstFloat(request.PreferredDeadheadKm, capacity.PreferredDeadheadKm, carrier.PreferredDeadheadKm),
		ObjectiveProfile:         firstText(request.ObjectiveProfile, capacity.ObjectiveProfile, carrier.ObjectiveProfile),
		ForwardSearchKm:          MinHardLimit(carrier.ForwardSearchKm, capacity.ForwardSearchKm, request.ForwardSearchKm),
		CorridorDeviationKm:      MinHardLimit(carrier.CorridorDeviationKm, capacity.CorridorDeviationKm, request.CorridorDeviationKm),
		MaxDeadheadKm:            MinHardLimit(carrier.MaxDeadheadKm, capacity.MaxDeadheadKm, request.MaxDeadheadKm),
		MaxDeadheadMinutes:       MinHardLimit(carrier.MaxDeadheadMinutes, capacity.MaxDeadheadMinutes, request.MaxDeadheadMinutes),
		MaxRouteIncreaseKm:       MinHardLimit(carrier.MaxRouteIncreaseKm, capacity.MaxRouteIncreaseKm, request.MaxRouteIncreaseKm),
		RadiusKm:                 MinHardLimit(carrier.RadiusKm, capacity.RadiusKm, request.RadiusKm),
		MinLoadedDistanceKm:      MaxHardLimit(carrier.MinLoadedDistanceKm, capacity.MinLoadedDistanceKm, request.MinLoadedDistanceKm),
		AllowUnknownRoadDistance: carrier.AllowUnknownRoadDistance && capacity.AllowUnknownRoadDistance && request.AllowUnknownRoadDistance,
	}
}

func (p NextLoadSearchPolicy) Fingerprint() string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func DeadheadBucket(km float64) string {
	switch {
	case km < 25:
		return "0-25"
	case km < 50:
		return "25-50"
	case km < 100:
		return "50-100"
	case km < 200:
		return "100-200"
	default:
		return "200+"
	}
}
