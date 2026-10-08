package domain

import "testing"

func TestNLO05B3OneLoadBackhaulFeasibility(t *testing.T) {
	pickup, closer, equal, farther := 500.0, 499.0, 500.0, 700.0
	releaseToPickup, loaded, deliveryToTarget, baseline := 100.0, 200.0, 800.0, 1000.0
	max := 150.0

	t.Run("radius_is_not_backhaul", func(t *testing.T) {
		got := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode: SearchRadius, PickupToTargetKm: &pickup, DeliveryToTargetKm: &farther,
		})
		if got.Applicable || len(got.Reasons) != 0 || got.RouteIncreaseKm != nil {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("corridor_requires_strictly_closer_delivery", func(t *testing.T) {
		pass := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode: SearchDirectionalCorridor, PickupToTargetKm: &pickup, DeliveryToTargetKm: &closer,
		})
		if !pass.Applicable || len(pass.Reasons) != 0 {
			t.Fatalf("closer %+v", pass)
		}
		same := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode: SearchDirectionalCorridor, PickupToTargetKm: &pickup, DeliveryToTargetKm: &equal,
		})
		if len(same.Reasons) != 1 || same.Reasons[0] != ReasonDeliveryNotTowardTarget {
			t.Fatalf("equal %+v", same)
		}
		away := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode: SearchDirectionalCorridor, PickupToTargetKm: &pickup, DeliveryToTargetKm: &farther,
		})
		if len(away.Reasons) != 1 || away.Reasons[0] != ReasonDeliveryNotTowardTarget {
			t.Fatalf("farther %+v", away)
		}
	})

	t.Run("unknown_road_is_not_zero", func(t *testing.T) {
		zero := 0.0
		missing := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode: SearchDirectionalCorridor, PickupToTargetKm: &pickup,
		})
		if len(missing.Reasons) != 1 || missing.Reasons[0] != ReasonRoadDistanceUnknown {
			t.Fatalf("nil %+v", missing)
		}
		measured := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode: SearchDirectionalCorridor, PickupToTargetKm: &pickup, DeliveryToTargetKm: &zero,
		})
		if len(measured.Reasons) != 0 {
			t.Fatalf("measured zero %+v", measured)
		}
	})

	t.Run("ellipse_uses_four_legs", func(t *testing.T) {
		got := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode:               SearchRouteEllipse,
			ReleaseToPickupKm:  &releaseToPickup,
			PickupToDeliveryKm: &loaded,
			DeliveryToTargetKm: &deliveryToTarget,
			ReleaseToTargetKm:  &baseline,
			MaxRouteIncreaseKm: &max,
		})
		threeLeg := RouteIncreaseKm(baseline, releaseToPickup, 900)
		if !got.Applicable || got.RouteIncreaseKm == nil || *got.RouteIncreaseKm != 100 || *got.RouteIncreaseKm == threeLeg || len(got.Reasons) != 0 {
			t.Fatalf("four-leg %+v three=%v", got, threeLeg)
		}
		over := 50.0
		rejected := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode:               SearchRouteEllipse,
			ReleaseToPickupKm:  &releaseToPickup,
			PickupToDeliveryKm: &loaded,
			DeliveryToTargetKm: &deliveryToTarget,
			ReleaseToTargetKm:  &baseline,
			MaxRouteIncreaseKm: &over,
		})
		if rejected.RouteIncreaseKm == nil || *rejected.RouteIncreaseKm != 100 || len(rejected.Reasons) != 1 || rejected.Reasons[0] != ReasonRouteIncreaseExceeded {
			t.Fatalf("limit %+v", rejected)
		}
	})

	t.Run("missing_ellipse_leg_does_not_invent_distance", func(t *testing.T) {
		got := EvaluateOneLoadBackhaul(OneLoadBackhaulInput{
			Mode:               SearchRouteEllipse,
			ReleaseToPickupKm:  &releaseToPickup,
			DeliveryToTargetKm: &deliveryToTarget,
			ReleaseToTargetKm:  &baseline,
			MaxRouteIncreaseKm: &max,
		})
		// A zero loaded leg would be 100+0+800-1000 = -100 and would pass the cap.
		if got.RouteIncreaseKm != nil || len(got.Reasons) != 1 || got.Reasons[0] != ReasonRoadDistanceUnknown {
			t.Fatalf("%+v", got)
		}
	})
}
