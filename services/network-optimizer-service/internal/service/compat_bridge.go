package service

import (
	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func cargoFromLoad(load domain.LoadOpportunity) compat.Cargo {
	cargo := load.Cargo
	return compat.Cargo{
		ID:                  load.ID.String(),
		CargoTypeCode:       cargo.CargoTypeCode,
		WeightKg:            load.WeightKg,
		VolumeM3:            load.VolumeM3,
		PalletCount:         cargo.PalletCount,
		PalletTypeCode:      cargo.PalletTypeCode,
		LinearMeters:        cargo.LinearMeters,
		MaxLoadedHeightMM:   cargo.MaxLoadedHeightMM,
		Stackable:           cargo.Stackable,
		Fragile:             cargo.Fragile,
		PackagingTypeCode:   cargo.PackagingTypeCode,
		FoodGradeRequired:   cargo.FoodGradeRequired,
		TemperatureRequired: cargo.TemperatureRequired,
		TemperatureMinC:     cargo.TemperatureMinC,
		TemperatureMaxC:     cargo.TemperatureMaxC,
		PreferredSetpointC:  cargo.PreferredTemperatureSetpointC,
		DangerousGoods:      cargo.Dangerous,
		HazardClasses:       copyStrings(cargo.HazardClasses),
		OdorEmissionClass:   cargo.OdorEmissionClass,
		OdorSensitive:       cargo.OdorSensitive,
		ContaminationClass:  cargo.ContaminationClass,
	}
}

func accessFromLoad(load domain.LoadOpportunity) compat.AccessNeed {
	cargo := load.Cargo
	return compat.AccessNeed{
		RequiredBodyTypes:       copyStrings(cargo.RequiredBodyTypes),
		RequiredEquipmentTypes:  copyStrings(load.Equipment),
		RequiredLoadingAccess:   copyStrings(cargo.RequiredLoadingAccess),
		AllowedLoadingAccess:    copyStrings(cargo.AllowedLoadingAccess),
		RequiredUnloadingAccess: copyStrings(cargo.RequiredUnloadingAccess),
		AllowedUnloadingAccess:  copyStrings(cargo.AllowedUnloadingAccess),
	}
}

func equipmentFromCapacity(capacity domain.Capacity, prediction *domain.PredictedCapacity) compat.Equipment {
	equipment := compat.Equipment{
		BodyType:          optionalString(capacity.BodyType),
		EquipmentTypeCode: firstEquipment(capacity.Equipment),
		PayloadKg:         capacity.PayloadRemainingKg,
		VolumeM3:          capacity.VolumeRemainingM3,
	}
	if prediction == nil || capacity.Source != domain.SourceCurrentShipmentPrediction {
		return equipment
	}
	if prediction.BodyType != nil && *prediction.BodyType != "" {
		equipment.BodyType = prediction.BodyType
	}
	if equipment.EquipmentTypeCode == nil {
		equipment.EquipmentTypeCode = prediction.LegacyEquipmentType
	}
	equipment.CombinationType = prediction.CombinationType
	equipment.LoadingAccess = copyStrings(prediction.LoadingAccess)
	equipment.UnloadingAccess = copyStrings(prediction.UnloadingAccess)
	if equipment.PayloadKg == nil {
		equipment.PayloadKg = prediction.CapacityWeightKg
	}
	if equipment.VolumeM3 == nil {
		equipment.VolumeM3 = prediction.CapacityVolumeM3
	}
	equipment.TemperatureControlMode = prediction.TemperatureControlMode
	equipment.TemperatureMinC = prediction.TemperatureCapabilityMinC
	equipment.TemperatureMaxC = prediction.TemperatureCapabilityMaxC
	equipment.TemperatureZoneCount = prediction.TemperatureZoneCount
	equipment.IndependentTemperatureControl = prediction.IndependentTemperatureControl
	if prediction.ContainerSize != nil && *prediction.ContainerSize != "" {
		if equipment.Provenance == nil {
			equipment.Provenance = map[string]string{}
		}
		equipment.Provenance["container_size"] = *prediction.ContainerSize
	}
	return equipment
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}
