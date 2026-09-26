package currenttrip

import "strings"

type residualInput struct {
	Resolution   string
	Units        []OnboardCargoUnit
	Vehicle      *VehicleCapability
	Equivalences []PalletEquivalence
}

func residualSnapshot(in residualInput) ResidualCapacitySnapshot {
	if in.Resolution != EvidenceConfirmedOnboard && in.Resolution != EvidenceUnloaded {
		return unprovenResidual()
	}
	confirmed := confirmedUnits(in.Units)
	if in.Resolution == EvidenceUnloaded && len(confirmed) == 0 {
		return emptyOccupancyResidual(in.Vehicle)
	}
	snap := ResidualCapacitySnapshot{
		Payload:         subtractWeight(in.Vehicle, confirmed),
		Volume:          subtractVolume(in.Vehicle, confirmed),
		PalletPositions: subtractPallets(in.Vehicle, confirmed, in.Equivalences),
		LinearMeters:    subtractLinear(in.Vehicle, confirmed),
		Height:          heightCheck(in.Vehicle, confirmed),
	}
	snap.TemperatureAllocation = temperatureAllocation(in.Vehicle)
	return snap
}

func unprovenResidual() ResidualCapacitySnapshot {
	unknown := SubtractiveDimension{Status: DimensionUnknown, Provenance: ProvenanceUnknown, Reason: ReasonUnproven}
	return ResidualCapacitySnapshot{
		Payload: unknown, Volume: unknown, PalletPositions: unknown, LinearMeters: unknown,
		Height:                HeightCheck{Status: HeightUnknown, Provenance: ProvenanceUnknown},
		TemperatureAllocation: DimensionUnknown,
	}
}

func emptyOccupancyResidual(vehicle *VehicleCapability) ResidualCapacitySnapshot {
	zero := 0.0
	return ResidualCapacitySnapshot{
		Payload:               knownRemainder(vehicleWeight(vehicle), &zero),
		Volume:                knownRemainder(vehicleVolume(vehicle), &zero),
		PalletPositions:       knownRemainder(vehiclePallets(vehicle), &zero),
		LinearMeters:          knownRemainder(vehicleLinear(vehicle), &zero),
		Height:                HeightCheck{Status: HeightUnknown, Provenance: ProvenanceUnknown},
		TemperatureAllocation: temperatureAllocation(vehicle),
	}
}

func confirmedUnits(units []OnboardCargoUnit) []OnboardCargoUnit {
	out := make([]OnboardCargoUnit, 0, len(units))
	for _, unit := range units {
		if unit.EvidenceState == EvidenceConfirmedOnboard {
			out = append(out, unit)
		}
	}
	return out
}

func subtractWeight(vehicle *VehicleCapability, units []OnboardCargoUnit) SubtractiveDimension {
	occupied, ok := sumOptional(len(units), func(i int) *float64 { return units[i].Profile.WeightKg })
	return finish(vehicleWeight(vehicle), occupied, ok, ProvenanceOnboard)
}

func subtractVolume(vehicle *VehicleCapability, units []OnboardCargoUnit) SubtractiveDimension {
	occupied, ok := sumOptional(len(units), func(i int) *float64 { return units[i].Profile.VolumeM3 })
	return finish(vehicleVolume(vehicle), occupied, ok, ProvenanceOnboard)
}

func subtractLinear(vehicle *VehicleCapability, units []OnboardCargoUnit) SubtractiveDimension {
	occupied, ok := sumOptional(len(units), func(i int) *float64 { return units[i].Profile.LinearMeters })
	return finish(vehicleLinear(vehicle), occupied, ok, ProvenanceOnboard)
}

func subtractPallets(vehicle *VehicleCapability, units []OnboardCargoUnit, equivalences []PalletEquivalence) SubtractiveDimension {
	occupied, reason, ok := palletOccupied(units, equivalences)
	if !ok {
		return SubtractiveDimension{Status: DimensionUnknown, Provenance: ProvenanceUnknown, Reason: reason}
	}
	return finish(vehiclePallets(vehicle), occupied, true, ProvenanceOnboard)
}

func palletOccupied(units []OnboardCargoUnit, equivalences []PalletEquivalence) (*float64, string, bool) {
	sum := 0.0
	used := false
	basis := ""
	for _, unit := range units {
		if !usesPallets(unit.Profile) {
			continue
		}
		used = true
		if unit.Profile.PalletCount == nil {
			return nil, ReasonPalletCount, false
		}
		if unit.Profile.PalletTypeCode == nil || strings.TrimSpace(*unit.Profile.PalletTypeCode) == "" {
			return nil, ReasonPalletType, false
		}
		factor, itemBasis, ok := provenFactor(strings.TrimSpace(*unit.Profile.PalletTypeCode), equivalences)
		if !ok {
			return nil, ReasonPalletEquivalence, false
		}
		if basis == "" {
			basis = itemBasis
		}
		if basis != itemBasis {
			return nil, ReasonPalletEquivalence, false
		}
		sum += float64(*unit.Profile.PalletCount) * factor
	}
	if !used {
		zero := 0.0
		return &zero, "", true
	}
	return &sum, "", true
}

func usesPallets(profile CargoProfile) bool {
	if profile.PalletCount != nil || profile.PalletTypeCode != nil {
		return true
	}
	return profile.PackagingTypeCode != nil && strings.EqualFold(strings.TrimSpace(*profile.PackagingTypeCode), "PALLETIZED")
}

func provenFactor(code string, equivalences []PalletEquivalence) (float64, string, bool) {
	for _, row := range equivalences {
		if !row.OwnershipProven || row.PositionsEach <= 0 {
			continue
		}
		if strings.TrimSpace(row.FromCode) == code && strings.TrimSpace(row.BasisCode) != "" {
			return row.PositionsEach, strings.TrimSpace(row.BasisCode), true
		}
	}
	return 0, "", false
}

func heightCheck(vehicle *VehicleCapability, units []OnboardCargoUnit) HeightCheck {
	if vehicle == nil || vehicle.InternalHeightMM == nil {
		return HeightCheck{Status: HeightUnknown, Provenance: ProvenanceUnknown}
	}
	maxHeight, ok := maxOptional(len(units), func(i int) *int { return units[i].Profile.MaxLoadedHeightMM })
	if !ok {
		return HeightCheck{VehicleInternalHeightMM: vehicle.InternalHeightMM, Status: HeightUnknown, Provenance: ProvenanceUnknown}
	}
	status := HeightKnownOK
	if *maxHeight > *vehicle.InternalHeightMM {
		status = HeightKnownExceeded
	}
	return HeightCheck{
		VehicleInternalHeightMM: vehicle.InternalHeightMM,
		MaxOnboardHeightMM:      maxHeight,
		Status:                  status,
		Provenance:              ProvenanceAsset,
	}
}

func temperatureAllocation(vehicle *VehicleCapability) string {
	if vehicle == nil || vehicle.IndependentTemperatureControl == nil || vehicle.TemperatureZoneCount == nil {
		return DimensionUnknown
	}
	if *vehicle.IndependentTemperatureControl && *vehicle.TemperatureZoneCount > 1 {
		return MultiZoneRequired
	}
	return DimensionKnown
}

func finish(total, occupied *float64, occupiedKnown bool, provenance string) SubtractiveDimension {
	if total == nil {
		return SubtractiveDimension{Status: DimensionUnknown, Provenance: ProvenanceUnknown, Reason: ReasonVehicleUnknown}
	}
	if !occupiedKnown || occupied == nil {
		return SubtractiveDimension{Total: total, Status: DimensionUnknown, Provenance: provenance, Reason: ReasonOccupancyUnknown}
	}
	if *occupied > *total {
		return SubtractiveDimension{
			Total: total, Occupied: occupied, Status: DimensionExceeds,
			Provenance: provenance, Reason: DimensionExceeds,
		}
	}
	remaining := *total - *occupied
	return SubtractiveDimension{
		Total: total, Occupied: occupied, Remaining: &remaining,
		Status: DimensionKnown, Provenance: provenance,
	}
}

func knownRemainder(total, occupied *float64) SubtractiveDimension {
	return finish(total, occupied, occupied != nil, ProvenanceOnboard)
}

func vehicleWeight(vehicle *VehicleCapability) *float64 {
	if vehicle == nil {
		return nil
	}
	return vehicle.CapacityWeight
}

func vehicleVolume(vehicle *VehicleCapability) *float64 {
	if vehicle == nil {
		return nil
	}
	return vehicle.CapacityVolume
}

func vehicleLinear(vehicle *VehicleCapability) *float64 {
	if vehicle == nil {
		return nil
	}
	return vehicle.UsableLinearMeters
}

func vehiclePallets(vehicle *VehicleCapability) *float64 {
	if vehicle == nil || vehicle.PalletPositions == nil {
		return nil
	}
	value := float64(*vehicle.PalletPositions)
	return &value
}

func sumOptional(n int, value func(int) *float64) (*float64, bool) {
	sum := 0.0
	for i := 0; i < n; i++ {
		item := value(i)
		if item == nil {
			return nil, false
		}
		sum += *item
	}
	return &sum, true
}

func maxOptional(n int, value func(int) *int) (*int, bool) {
	if n == 0 {
		return nil, false
	}
	max := 0
	for i := 0; i < n; i++ {
		item := value(i)
		if item == nil {
			return nil, false
		}
		if i == 0 || *item > max {
			max = *item
		}
	}
	return &max, true
}
