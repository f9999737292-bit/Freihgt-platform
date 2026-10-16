package chainapi

import (
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

// routingProfileFromCanonical copies only routing facts whose semantics match
// routing.VehicleProfile. Vehicle payload capacity, internal cargo dimensions,
// and ADR capability are not routing mass, external clearance, or onboard
// dangerous-cargo state, so those fields stay nil.
func routingProfileFromCanonical(capability *currenttrip.VehicleCapability, resolution string, onboard []currenttrip.OnboardCargoUnit) routing.VehicleProfile {
	_ = capability
	return routing.VehicleProfile{DangerousCargo: dangerousCargoState(resolution, onboard)}
}

func dangerousCargoState(resolution string, units []currenttrip.OnboardCargoUnit) *bool {
	if resolution != currenttrip.EvidenceConfirmedOnboard && resolution != currenttrip.EvidenceUnloaded {
		return nil
	}
	dangerous := false
	unknown := false
	confirmed := 0
	for _, unit := range units {
		if unit.EvidenceState != currenttrip.EvidenceConfirmedOnboard {
			continue
		}
		confirmed++
		known, carrying := knownDangerousCargo(unit.Profile)
		if !known {
			unknown = true
			continue
		}
		if carrying {
			dangerous = true
		}
	}
	if dangerous {
		value := true
		return &value
	}
	if unknown {
		return nil
	}
	if confirmed == 0 {
		if resolution == currenttrip.EvidenceUnloaded {
			value := false
			return &value
		}
		return nil
	}
	value := false
	return &value
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
