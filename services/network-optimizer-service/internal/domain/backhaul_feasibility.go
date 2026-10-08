package domain

// OneLoadBackhaulInput is the road evidence for one published load.
// A second load is a separate call. Nil kilometres are unknown.
// Straight-line distance is not an input and is not treated as zero.
type OneLoadBackhaulInput struct {
	Mode               string
	PickupToTargetKm   *float64
	DeliveryToTargetKm *float64
	ReleaseToPickupKm  *float64
	PickupToDeliveryKm *float64
	ReleaseToTargetKm  *float64
	MaxRouteIncreaseKm *float64
}

// OneLoadBackhaulResult is the one-load feasibility verdict.
// Applicable is false for RADIUS. RouteIncreaseKm is set only when every
// ellipse leg is a measured road kilometre.
type OneLoadBackhaulResult struct {
	Applicable      bool
	RouteIncreaseKm *float64
	Reasons         []string
}

// EvaluateOneLoadBackhaul decides whether one load moves toward the policy target.
// Corridor requires delivery-to-target road distance to be strictly smaller than
// pickup-to-target. Ellipse uses the four-leg insertion
// release→pickup→delivery→target minus release→target.
// It does not search, rank, or compose a second load.
func EvaluateOneLoadBackhaul(in OneLoadBackhaulInput) OneLoadBackhaulResult {
	switch in.Mode {
	case SearchDirectionalCorridor:
		return evaluateCorridorTowardTarget(in)
	case SearchRouteEllipse:
		return evaluateFourLegEllipse(in)
	default:
		return OneLoadBackhaulResult{}
	}
}

func evaluateCorridorTowardTarget(in OneLoadBackhaulInput) OneLoadBackhaulResult {
	out := OneLoadBackhaulResult{Applicable: true}
	if in.PickupToTargetKm == nil || in.DeliveryToTargetKm == nil {
		out.Reasons = []string{ReasonRoadDistanceUnknown}
		return out
	}
	if *in.DeliveryToTargetKm >= *in.PickupToTargetKm {
		out.Reasons = []string{ReasonDeliveryNotTowardTarget}
	}
	return out
}

func evaluateFourLegEllipse(in OneLoadBackhaulInput) OneLoadBackhaulResult {
	out := OneLoadBackhaulResult{Applicable: true}
	if in.ReleaseToPickupKm == nil || in.PickupToDeliveryKm == nil || in.DeliveryToTargetKm == nil || in.ReleaseToTargetKm == nil {
		out.Reasons = []string{ReasonRoadDistanceUnknown}
		return out
	}
	increase := NextLoadInsertionIncreaseKm(*in.ReleaseToTargetKm, *in.ReleaseToPickupKm, *in.PickupToDeliveryKm, *in.DeliveryToTargetKm)
	if in.MaxRouteIncreaseKm == nil {
		return out
	}
	out.RouteIncreaseKm = &increase
	if err := CheckRouteIncrease(increase, *in.MaxRouteIncreaseKm); err != nil {
		out.Reasons = []string{err.Error()}
	}
	return out
}
