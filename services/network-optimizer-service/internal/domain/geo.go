package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	SearchRadius                  = "RADIUS"
	SearchDirectionalCorridor     = "DIRECTIONAL_CORRIDOR"
	SearchRouteEllipse            = "ROUTE_ELLIPSE"
	PolicyCarrierDefault          = "CARRIER_DEFAULT"
	PolicyCapacityOverride        = "CAPACITY_OVERRIDE"
	PolicySearchRequest           = "SEARCH_REQUEST"
	HaversineCanonicalRoad        = false
	ReasonBacktrackRejected       = "BACKTRACK_DIRECTION_REJECTED"
	ReasonForwardExceeded         = "FORWARD_SEARCH_EXCEEDED"
	ReasonLateralExceeded         = "CORRIDOR_DEVIATION_EXCEEDED"
	ReasonDeadheadExceeded        = "MAX_DEADHEAD_EXCEEDED"
	ReasonRoadDistanceUnknown     = "ROAD_DISTANCE_UNKNOWN"
	ReasonPickupWindowMissed      = "PICKUP_WINDOW_MISSED"
	ReasonDirectionTarget         = "DIRECTION_TARGET_REQUIRED"
	ReasonDirectionTargetGeo      = "DIRECTION_TARGET_GEO_UNKNOWN"
	ReasonRouteIncreaseExceeded   = "ROUTE_INCREASE_EXCEEDED"
	ReasonDeliveryNotTowardTarget = "DELIVERY_NOT_TOWARD_TARGET"
	ReasonDeadheadTimeExceeded    = "MAX_DEADHEAD_TIME_EXCEEDED"
	ReasonPayloadExceeded         = "PAYLOAD_EXCEEDED"
	ReasonVolumeExceeded          = "VOLUME_EXCEEDED"
	ReasonOutsideRadius           = "OUTSIDE_RADIUS"
	ReasonCargoIncompatible       = "CARGO_EQUIPMENT_INCOMPATIBLE"
	ReasonCargoIndeterminate      = "CARGO_EQUIPMENT_INDETERMINATE"
	ReasonCapacityFactUnknown     = "CAPACITY_FACT_UNKNOWN"
)

var (
	ErrBacktrackRejected     = errors.New(ReasonBacktrackRejected)
	ErrForwardExceeded       = errors.New(ReasonForwardExceeded)
	ErrLateralExceeded       = errors.New(ReasonLateralExceeded)
	ErrDeadheadExceeded      = errors.New(ReasonDeadheadExceeded)
	ErrRoadDistanceUnknown   = errors.New(ReasonRoadDistanceUnknown)
	ErrPickupWindowMissed    = errors.New(ReasonPickupWindowMissed)
	ErrDirectionTarget       = errors.New(ReasonDirectionTarget)
	ErrDirectionTargetGeo    = errors.New(ReasonDirectionTargetGeo)
	ErrRouteIncreaseExceeded = errors.New(ReasonRouteIncreaseExceeded)
)

// LocationSnapshot is the safe planning copy taken from transport.locations.
// It does not carry address lines, contacts, or operational notes.
type LocationSnapshot struct {
	ID          uuid.UUID
	CountryCode string
	Region      string
	City        string
	Latitude    *float64
	Longitude   *float64
	Timezone    string
	Status      string
	Version     int
}

func ApplyPlaceSnapshot(place Place, snap LocationSnapshot) Place {
	id := snap.ID
	place.LocationID = &id
	place.CountryCode = snap.CountryCode
	place.Region = snap.Region
	place.City = snap.City
	place.Latitude = snap.Latitude
	place.Longitude = snap.Longitude
	return place
}

func ApplyCapacitySnapshot(cap *Capacity, snap LocationSnapshot) {
	id := snap.ID
	cap.LocationID = &id
	cap.CountryCode = snap.CountryCode
	cap.Region = snap.Region
	cap.City = snap.City
	cap.Latitude = snap.Latitude
	cap.Longitude = snap.Longitude
}

// CoarseDisplay is the anonymized DisplayGeo. Exact identifiers stay off the view.
func (p Place) CoarseDisplay() Place {
	return Place{CountryCode: p.CountryCode, Region: p.Region, City: p.City}
}

func (p Place) HasCoarse() bool {
	return p.CountryCode != "" || p.Region != "" || p.City != ""
}

// SearchGeo is the server-side exact place. It is not a public anonymized view.
func (p Place) SearchGeo() Place { return p }

// LegDistances keeps each distance meaning on its own field.
type LegDistances struct {
	StraightLineKm            *float64
	CorridorLateralKm         *float64
	CorridorForwardProgressKm *float64
	RoadDeadheadKm            *float64
	RoadDeadheadMinutes       *float64
	LoadedRoadDistanceKm      *float64
	RouteIncreaseKm           *float64
}

type NextLoadSearchPolicy struct {
	SearchMode               string     `json:"search_mode"`
	TargetLocationID         *uuid.UUID `json:"target_location_id,omitempty"`
	ForwardSearchKm          *float64   `json:"forward_search_km,omitempty"`
	CorridorDeviationKm      *float64   `json:"corridor_deviation_km,omitempty"`
	MaxDeadheadKm            *float64   `json:"max_deadhead_km,omitempty"`
	PreferredDeadheadKm      *float64   `json:"preferred_deadhead_km,omitempty"`
	MaxDeadheadMinutes       *float64   `json:"max_deadhead_minutes,omitempty"`
	MinLoadedDistanceKm      *float64   `json:"min_loaded_distance_km,omitempty"`
	MaxRouteIncreaseKm       *float64   `json:"max_route_increase_km,omitempty"`
	RadiusKm                 *float64   `json:"radius_km,omitempty"`
	ObjectiveProfile         string     `json:"objective_profile,omitempty"`
	RankingCurrency          string     `json:"ranking_currency,omitempty"`
	AllowUnknownRoadDistance bool       `json:"allow_unknown_road_distance"`
}

func (p NextLoadSearchPolicy) Validate() error {
	switch p.SearchMode {
	case SearchRadius, SearchDirectionalCorridor, SearchRouteEllipse:
	default:
		return fmt.Errorf("unsupported search_mode")
	}
	if !KnownObjectiveProfile(p.ObjectiveProfile) {
		return fmt.Errorf("objective_profile is not recognized")
	}
	if !ObjectiveExecutable(p.ObjectiveProfile) {
		return ErrObjectiveNotExecutable
	}
	if !ValidRankingCurrency(p.RankingCurrency) {
		return fmt.Errorf("ranking_currency must be empty or three uppercase letters")
	}
	if p.ObjectiveProfile == ProfileMaxRevenue && p.RankingCurrency == "" {
		return ErrRankingCurrencyRequired
	}
	if p.ObjectiveProfile == ProfileReturnHome && p.TargetLocationID == nil {
		return ErrTargetRequiredForObjective
	}
	if p.SearchMode == SearchRadius && (p.RadiusKm == nil || *p.RadiusKm <= 0) {
		return fmt.Errorf("radius_km is required")
	}
	if p.SearchMode == SearchDirectionalCorridor && (p.ForwardSearchKm == nil || p.CorridorDeviationKm == nil) {
		return fmt.Errorf("forward_search_km and corridor_deviation_km are required")
	}
	if p.SearchMode == SearchRouteEllipse && p.MaxRouteIncreaseKm == nil {
		return fmt.Errorf("max_route_increase_km is required")
	}
	if p.SearchMode != SearchRadius && p.TargetLocationID == nil {
		return ErrDirectionTarget
	}
	checks := []struct {
		name string
		v    *float64
	}{
		{"forward_search_km", p.ForwardSearchKm},
		{"corridor_deviation_km", p.CorridorDeviationKm},
		{"max_deadhead_km", p.MaxDeadheadKm},
		{"preferred_deadhead_km", p.PreferredDeadheadKm},
		{"max_deadhead_minutes", p.MaxDeadheadMinutes},
		{"min_loaded_distance_km", p.MinLoadedDistanceKm},
		{"max_route_increase_km", p.MaxRouteIncreaseKm},
		{"radius_km", p.RadiusKm},
	}
	for _, check := range checks {
		if check.name == "radius_km" {
			if check.v != nil && *check.v <= 0 {
				return fmt.Errorf("radius_km must be positive")
			}
			continue
		}
		if check.v != nil && *check.v < 0 {
			return fmt.Errorf("%s must be zero or greater", check.name)
		}
	}
	return nil
}

func (p NextLoadSearchPolicy) ValidateOverride() error {
	if p.SearchMode != "" {
		switch p.SearchMode {
		case SearchRadius, SearchDirectionalCorridor, SearchRouteEllipse:
		default:
			return fmt.Errorf("unsupported search_mode")
		}
	}
	checks := []struct {
		name string
		v    *float64
	}{
		{"forward_search_km", p.ForwardSearchKm},
		{"corridor_deviation_km", p.CorridorDeviationKm},
		{"max_deadhead_km", p.MaxDeadheadKm},
		{"preferred_deadhead_km", p.PreferredDeadheadKm},
		{"max_deadhead_minutes", p.MaxDeadheadMinutes},
		{"min_loaded_distance_km", p.MinLoadedDistanceKm},
		{"max_route_increase_km", p.MaxRouteIncreaseKm},
		{"radius_km", p.RadiusKm},
	}
	for _, check := range checks {
		if check.name == "radius_km" {
			if check.v != nil && *check.v <= 0 {
				return fmt.Errorf("radius_km must be positive")
			}
			continue
		}
		if check.v != nil && *check.v < 0 {
			return fmt.Errorf("%s must be zero or greater", check.name)
		}
	}
	return nil
}

func RequireDirectionTarget(mode string, target *uuid.UUID, hasCoordinates bool) error {
	if mode == SearchRadius {
		return nil
	}
	if target == nil || *target == uuid.Nil {
		return ErrDirectionTarget
	}
	if !hasCoordinates {
		return ErrDirectionTargetGeo
	}
	return nil
}

// MinHardLimit is the strictest applicable hard maximum.
// A downstream request that is larger is ignored.
func MinHardLimit(limits ...*float64) *float64 {
	var min *float64
	for _, limit := range limits {
		if limit == nil {
			continue
		}
		if min == nil || *limit < *min {
			value := *limit
			min = &value
		}
	}
	return min
}

type CorridorPlacement struct {
	ForwardProgressKm float64
	LateralKm         float64
	RoadDeadheadKm    float64
}

// CheckCorridorPlacement applies three separate constraints.
// It does not search or rank loads.
func CheckCorridorPlacement(place CorridorPlacement, forwardSearchKm, corridorDeviationKm, maxDeadheadKm float64) error {
	if place.ForwardProgressKm < 0 {
		return ErrBacktrackRejected
	}
	if place.ForwardProgressKm > forwardSearchKm {
		return ErrForwardExceeded
	}
	if place.LateralKm > corridorDeviationKm {
		return ErrLateralExceeded
	}
	if place.RoadDeadheadKm > maxDeadheadKm {
		return ErrDeadheadExceeded
	}
	return nil
}

// RouteIncreaseKm is candidate road distance minus the direct road baseline.
// The arguments are road kilometres. Straight-line geometry is not an input.
func RouteIncreaseKm(baselineRoadKm, releaseToPickupRoadKm, pickupToTargetRoadKm float64) float64 {
	return releaseToPickupRoadKm + pickupToTargetRoadKm - baselineRoadKm
}

// NextLoadInsertionIncreaseKm is the full load insertion path minus the direct road baseline.
// It is road kilometres only: release to pickup, pickup to delivery, and delivery to target.
func NextLoadInsertionIncreaseKm(baselineRoadKm, releaseToPickupRoadKm, pickupToDeliveryRoadKm, deliveryToTargetRoadKm float64) float64 {
	return releaseToPickupRoadKm + pickupToDeliveryRoadKm + deliveryToTargetRoadKm - baselineRoadKm
}

func CheckRouteIncrease(increaseKm, maxRouteIncreaseKm float64) error {
	if increaseKm > maxRouteIncreaseKm {
		return ErrRouteIncreaseExceeded
	}
	return nil
}

// RequireRoadDeadhead rejects a missing routing result.
// Unknown is not zero and is not a straight-line substitute.
func RequireRoadDeadhead(roadKm *float64, allowUnknown bool) error {
	if roadKm != nil {
		return nil
	}
	// allowUnknown does not turn a missing route into 0 km or a straight-line value.
	_ = allowUnknown
	return ErrRoadDistanceUnknown
}

func ArrivalAtPickup(available time.Time, roadDeadheadMinutes float64) time.Time {
	return available.Add(time.Duration(roadDeadheadMinutes * float64(time.Minute)))
}

func EvaluatePickupWindow(arrival time.Time, window TimeWindow) (waiting time.Duration, err error) {
	if window.End != nil && arrival.After(*window.End) {
		return 0, ErrPickupWindowMissed
	}
	if window.Start != nil && arrival.Before(*window.Start) {
		return window.Start.Sub(arrival), nil
	}
	return 0, nil
}
