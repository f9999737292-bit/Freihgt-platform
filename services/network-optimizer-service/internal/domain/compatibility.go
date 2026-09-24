package domain

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ResultCompatible    = "COMPATIBLE"
	ResultIncompatible  = "INCOMPATIBLE"
	ResultIndeterminate = "INDETERMINATE"

	BodyTent         = "TENT"
	BodyContainer    = "CONTAINER"
	BodyIsothermal   = "ISOTHERMAL"
	BodyRefrigerator = "REFRIGERATOR"
	BodyBox          = "BOX"
	BodyPlatform     = "PLATFORM"
	BodyLowbed       = "LOWBED"
	BodyTank         = "TANK"
	BodyTipper       = "TIPPER"
	BodyCarCarrier   = "CAR_CARRIER"
	BodyTimber       = "TIMBER"
	BodyOther        = "OTHER"

	AccessRear = "REAR"
	AccessSide = "SIDE"
	AccessTop  = "TOP"

	CombinationTruck              = "TRUCK"
	CombinationTractorSemitrailer = "TRACTOR_SEMITRAILER"
	CombinationTruckTrailer       = "TRUCK_TRAILER"
	CombinationRoadTrain          = "ROAD_TRAIN"
	CombinationOther              = "OTHER"

	TemperatureNone    = "NONE"
	TemperaturePassive = "PASSIVE"
	TemperatureActive  = "ACTIVE"

	ReasonBodyTypeIncompatible          = "BODY_TYPE_INCOMPATIBLE"
	ReasonBodyTypeUnknown               = "BODY_TYPE_UNKNOWN"
	ReasonLoadingAccessUnsupported      = "LOADING_ACCESS_UNSUPPORTED"
	ReasonLoadingAccessUnknown          = "LOADING_ACCESS_UNKNOWN"
	ReasonUnloadingAccessUnsupported    = "UNLOADING_ACCESS_UNSUPPORTED"
	ReasonUnloadingAccessUnknown        = "UNLOADING_ACCESS_UNKNOWN"
	ReasonTemperatureControlRequired    = "TEMPERATURE_CONTROL_REQUIRED"
	ReasonTemperatureCapabilityUnknown  = "TEMPERATURE_CAPABILITY_UNKNOWN"
	ReasonTemperatureRangeUnsupported   = "TEMPERATURE_RANGE_UNSUPPORTED"
	ReasonTemperatureRangesIncompatible = "TEMPERATURE_RANGES_INCOMPATIBLE"
	ReasonTemperaturePreconditioning    = "TEMPERATURE_PRECONDITIONING_REQUIRED"
	ReasonMultiZoneRequired             = "MULTI_ZONE_REQUIRED"
	ReasonTemperatureZoneUnavailable    = "TEMPERATURE_ZONE_UNAVAILABLE"

	CapacitySemanticsNextLoad = "NEXT_LOAD_FUTURE_CAPACITY"
)

var canonicalBodies = map[string]struct{}{
	BodyTent: {}, BodyContainer: {}, BodyIsothermal: {}, BodyRefrigerator: {},
	BodyBox: {}, BodyPlatform: {}, BodyLowbed: {}, BodyTank: {}, BodyTipper: {},
	BodyCarCarrier: {}, BodyTimber: {}, BodyOther: {},
}

var canonicalAccess = map[string]struct{}{
	AccessRear: {}, AccessSide: {}, AccessTop: {},
}

type CompatibilityResult struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func compatible() CompatibilityResult {
	return CompatibilityResult{Status: ResultCompatible}
}

func incompatible(reason string) CompatibilityResult {
	return CompatibilityResult{Status: ResultIncompatible, Reason: reason}
}

func indeterminate(reason string) CompatibilityResult {
	return CompatibilityResult{Status: ResultIndeterminate, Reason: reason}
}

func validateBodyTokens(values []string) error {
	return validateTokens("required_body_types", values, canonicalBodies)
}

func validateAccessTokens(field string, values []string) error {
	return validateTokens(field, values, canonicalAccess)
}

func validateTokens(field string, values []string, allowed map[string]struct{}) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return fmt.Errorf("%s contains an unsupported value", field)
		}
		if _, dup := seen[value]; dup {
			return fmt.Errorf("%s contains a duplicate value", field)
		}
		seen[value] = struct{}{}
	}
	return nil
}

// CanonicalBody returns the normalized body.
// An explicit canonical value wins.
// A legacy equipment string is used only when it is exactly a canonical token after trim.
// Any other legacy text stays unknown. UNKNOWN and a nil explicit value are not a body type.
func CanonicalBody(explicit, legacy *string) (*string, error) {
	if explicit != nil {
		token, unknown, err := canonicalOrUnknown(*explicit, canonicalBodies, "body_type")
		if err != nil || unknown {
			return nil, err
		}
		return &token, nil
	}
	if legacy == nil {
		return nil, nil
	}
	token, unknown, err := canonicalOrUnknown(*legacy, canonicalBodies, "equipment_type")
	if err != nil {
		return nil, nil
	}
	if unknown {
		return nil, nil
	}
	return &token, nil
}

func canonicalOrUnknown(raw string, allowed map[string]struct{}, field string) (string, bool, error) {
	token := trim(raw)
	if token == "" || token == "UNKNOWN" {
		return "", true, nil
	}
	if _, ok := allowed[token]; !ok {
		if field == "equipment_type" {
			return "", true, nil
		}
		return "", false, fmt.Errorf("%s is not a canonical value", field)
	}
	return token, false, nil
}

func trim(raw string) string {
	return strings.TrimSpace(raw)
}

// NormalizeAccess keeps nil as unknown and an empty slice as a known empty set.
// Values are validated, de-duplicated, and sorted. Nil in, nil out.
func NormalizeAccess(values []string) ([]string, error) {
	if values == nil {
		return nil, nil
	}
	if err := validateAccessTokens("access", values); err != nil {
		return nil, err
	}
	out := make([]string, len(values))
	copy(out, values)
	sort.Strings(out)
	return out, nil
}

func BodyCompatible(required []string, capacityBody *string) CompatibilityResult {
	if len(required) == 0 {
		return compatible()
	}
	if capacityBody == nil || *capacityBody == "" {
		return indeterminate(ReasonBodyTypeUnknown)
	}
	for _, body := range required {
		if body == *capacityBody {
			return compatible()
		}
	}
	return incompatible(ReasonBodyTypeIncompatible)
}

func LoadingCompatible(required, supported []string) CompatibilityResult {
	return accessCompatible(required, supported, ReasonLoadingAccessUnsupported, ReasonLoadingAccessUnknown)
}

func UnloadingCompatible(required, supported []string) CompatibilityResult {
	return accessCompatible(required, supported, ReasonUnloadingAccessUnsupported, ReasonUnloadingAccessUnknown)
}

func accessCompatible(required, supported []string, unsupported, unknown string) CompatibilityResult {
	if len(required) == 0 {
		return compatible()
	}
	if supported == nil {
		return indeterminate(unknown)
	}
	have := map[string]struct{}{}
	for _, value := range supported {
		have[value] = struct{}{}
	}
	for _, value := range required {
		if _, ok := have[value]; !ok {
			return incompatible(unsupported)
		}
	}
	return compatible()
}

type TemperatureCapability struct {
	ControlMode *string
	MinC        *float64
	MaxC        *float64
	ZoneCount   *int
}

type TemperatureRequirement struct {
	Required  bool
	MinC      *float64
	MaxC      *float64
	SetpointC *float64
}

// SingleCargoTemperature checks one cargo against one vehicle capability.
// ISOTHERMAL with PASSIVE control does not satisfy an active-control requirement.
// An unknown range is indeterminate, not a fabricated default interval.
func SingleCargoTemperature(vehicle TemperatureCapability, cargo TemperatureRequirement) CompatibilityResult {
	if !cargo.Required && cargo.MinC == nil && cargo.MaxC == nil {
		return compatible()
	}
	mode := ""
	if vehicle.ControlMode != nil {
		mode = *vehicle.ControlMode
	}
	if cargo.Required && mode != TemperatureActive {
		return incompatible(ReasonTemperatureControlRequired)
	}
	if vehicle.MinC == nil || vehicle.MaxC == nil {
		return indeterminate(ReasonTemperatureCapabilityUnknown)
	}
	if cargo.MinC == nil || cargo.MaxC == nil {
		return indeterminate(ReasonTemperatureCapabilityUnknown)
	}
	if *vehicle.MinC > *cargo.MinC || *vehicle.MaxC < *cargo.MaxC {
		return incompatible(ReasonTemperatureRangeUnsupported)
	}
	return compatible()
}

// SingleZoneCoLoad requires a non-empty intersection of the two allowed intervals.
// A single zone cannot carry +5°C and -20°C cargo at the same time.
func SingleZoneCoLoad(aMin, aMax, bMin, bMax *float64) CompatibilityResult {
	if aMin == nil || aMax == nil || bMin == nil || bMax == nil {
		return indeterminate(ReasonTemperatureCapabilityUnknown)
	}
	low := *aMin
	if *bMin > low {
		low = *bMin
	}
	high := *aMax
	if *bMax < high {
		high = *bMax
	}
	if low <= high {
		return compatible()
	}
	return incompatible(ReasonTemperatureRangesIncompatible)
}

// MultiZoneOptimization is reserved and not implemented.
// A nil zone count is unknown. It is not treated as one invented zone, and it is not treated as multi-zone capability.
func MultiZoneOptimization(zoneCount *int) CompatibilityResult {
	if zoneCount == nil {
		return indeterminate(ReasonTemperatureZoneUnavailable)
	}
	if *zoneCount > 1 {
		return indeterminate(ReasonMultiZoneRequired)
	}
	return indeterminate(ReasonMultiZoneRequired)
}

// TemperatureTransition does not invent warming, cleaning, or stabilization time.
// When the current setpoint is outside the next cargo interval, or the setpoint is unknown
// while the next cargo has a temperature requirement, the result stays indeterminate.
func TemperatureTransition(currentSetpoint, nextMin, nextMax *float64, nextRequired bool) CompatibilityResult {
	if !nextRequired && nextMin == nil && nextMax == nil {
		return compatible()
	}
	if currentSetpoint == nil || nextMin == nil || nextMax == nil {
		return indeterminate(ReasonTemperaturePreconditioning)
	}
	if *currentSetpoint < *nextMin || *currentSetpoint > *nextMax {
		return indeterminate(ReasonTemperaturePreconditioning)
	}
	return compatible()
}
