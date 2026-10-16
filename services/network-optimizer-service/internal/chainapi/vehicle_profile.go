package chainapi

import (
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// routingProfileFromCanonical copies only routing facts whose semantics match
// routing.VehicleProfile. Vehicle payload capacity, internal cargo dimensions,
// and ADR capability are not routing mass, external clearance, or onboard
// dangerous-cargo state, so those fields stay nil.
func routingProfileFromCanonical(capability *currenttrip.VehicleCapability, onboard []currenttrip.OnboardCargoUnit, observeOnboard bool) routing.VehicleProfile {
	_ = capability
	profile := routing.VehicleProfile{}
	if observeOnboard {
		profile.DangerousCargo = confirmedOnboardDangerousCargo(onboard)
	}
	return profile
}

func confirmedOnboardDangerousCargo(units []currenttrip.OnboardCargoUnit) *bool {
	dangerous := false
	for _, unit := range units {
		if unit.EvidenceState != currenttrip.EvidenceConfirmedOnboard {
			continue
		}
		known, carrying := knownDangerousCargo(unit.Profile)
		if !known {
			return nil
		}
		if carrying {
			dangerous = true
		}
	}
	return &dangerous
}

func knownDangerousCargo(profile currenttrip.CargoProfile) (bool, bool) {
	if len(profile.HazardClasses) > 0 {
		return true, true
	}
	if profile.DangerousGoods == nil {
		return false, false
	}
	return true, *profile.DangerousGoods
}
