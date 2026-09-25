package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	VehicleTypeTruck = "TRUCK"
)

type Vehicle struct {
	ID                            uuid.UUID
	TenantID                      uuid.UUID
	CarrierCompanyID              uuid.UUID
	PlateNumber                   string
	VehicleType                   string
	EquipmentType                 *string
	CapacityWeight                *float64
	CapacityVolume                *float64
	RegistrationCountry           string
	Status                        string
	CombinationType               *string
	BodyType                      *string
	LoadingAccess                 []string
	UnloadingAccess               []string
	TemperatureControlMode        *string
	TemperatureCapabilityMinC     *float64
	TemperatureCapabilityMaxC     *float64
	TemperatureZoneCount          *int
	IndependentTemperatureControl *bool
	ContainerSize                 *string
	EquipmentUnitKind             *string
	PalletPositions               *int
	UsableLinearMeters            *float64
	InternalLengthMM              *int
	InternalWidthMM               *int
	InternalHeightMM              *int
	FoodGradeCapability           *bool
	ADRCapability                 *bool
	Version                       int
}

type CreateVehicleInput struct {
	CarrierCompanyID              uuid.UUID
	PlateNumber                   string
	VehicleType                   string
	EquipmentType                 *string
	CapacityWeight                *float64
	CapacityVolume                *float64
	RegistrationCountry           string
	CombinationType               *string
	BodyType                      *string
	LoadingAccess                 []string
	UnloadingAccess               []string
	TemperatureControlMode        *string
	TemperatureCapabilityMinC     *float64
	TemperatureCapabilityMaxC     *float64
	TemperatureZoneCount          *int
	IndependentTemperatureControl *bool
	ContainerSize                 *string
	EquipmentUnitKind             *string
	PalletPositions               *int
	UsableLinearMeters            *float64
	InternalLengthMM              *int
	InternalWidthMM               *int
	InternalHeightMM              *int
	FoodGradeCapability           *bool
	ADRCapability                 *bool
}

type ListVehiclesFilter struct {
	CarrierCompanyID *uuid.UUID
	Status           *string
	Limit            int
	Offset           int
}

func ValidateCreateVehicleInput(in *CreateVehicleInput) error {
	if in == nil {
		return apperrors.Validation("vehicle is required", nil)
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if strings.TrimSpace(in.PlateNumber) == "" {
		return apperrors.Validation("plate_number is required", map[string]any{"field": "plate_number"})
	}
	if strings.TrimSpace(in.VehicleType) == "" {
		in.VehicleType = VehicleTypeTruck
	}
	var err error
	if in.CombinationType, err = normalizeToken("combination_type", in.CombinationType, combinationTypes); err != nil {
		return err
	}
	if in.BodyType, err = normalizeToken("body_type", in.BodyType, bodyTypes); err != nil {
		return err
	}
	if in.TemperatureControlMode, err = normalizeToken("temperature_control_mode", in.TemperatureControlMode, temperatureModes); err != nil {
		return err
	}
	if in.ContainerSize, err = normalizeToken("container_size", in.ContainerSize, containerSizes); err != nil {
		return err
	}
	if in.EquipmentUnitKind, err = normalizeToken("equipment_unit_kind", in.EquipmentUnitKind, equipmentUnitKinds); err != nil {
		return err
	}
	if err := validateAccess("loading_access", in.LoadingAccess); err != nil {
		return err
	}
	if err := validateAccess("unloading_access", in.UnloadingAccess); err != nil {
		return err
	}
	if in.TemperatureCapabilityMinC != nil && in.TemperatureCapabilityMaxC != nil && *in.TemperatureCapabilityMaxC < *in.TemperatureCapabilityMinC {
		return apperrors.Validation("temperature_capability_max_c must be greater than or equal to temperature_capability_min_c", map[string]any{"field": "temperature_capability_max_c"})
	}
	if in.TemperatureZoneCount != nil && *in.TemperatureZoneCount < 1 {
		return apperrors.Validation("temperature_zone_count must be >= 1 when known", map[string]any{"field": "temperature_zone_count"})
	}
	if in.PalletPositions != nil && *in.PalletPositions <= 0 {
		return apperrors.Validation("pallet_positions must be greater than zero when known", map[string]any{"field": "pallet_positions"})
	}
	if in.UsableLinearMeters != nil && *in.UsableLinearMeters <= 0 {
		return apperrors.Validation("usable_linear_meters must be greater than zero when known", map[string]any{"field": "usable_linear_meters"})
	}
	if in.InternalLengthMM != nil && *in.InternalLengthMM <= 0 {
		return apperrors.Validation("internal_length_mm must be greater than zero when known", map[string]any{"field": "internal_length_mm"})
	}
	if in.InternalWidthMM != nil && *in.InternalWidthMM <= 0 {
		return apperrors.Validation("internal_width_mm must be greater than zero when known", map[string]any{"field": "internal_width_mm"})
	}
	if in.InternalHeightMM != nil && *in.InternalHeightMM <= 0 {
		return apperrors.Validation("internal_height_mm must be greater than zero when known", map[string]any{"field": "internal_height_mm"})
	}
	return nil
}

var combinationTypes = map[string]struct{}{
	"TRUCK": {}, "TRACTOR_SEMITRAILER": {}, "TRUCK_TRAILER": {}, "ROAD_TRAIN": {}, "OTHER": {},
}

var bodyTypes = map[string]struct{}{
	"TENT": {}, "CONTAINER": {}, "ISOTHERMAL": {}, "REFRIGERATOR": {}, "BOX": {}, "PLATFORM": {},
	"LOWBED": {}, "TANK": {}, "TIPPER": {}, "CAR_CARRIER": {}, "TIMBER": {}, "OTHER": {},
}

var temperatureModes = map[string]struct{}{"NONE": {}, "PASSIVE": {}, "ACTIVE": {}}

var containerSizes = map[string]struct{}{"20FT": {}, "40FT": {}, "40HC": {}, "45FT": {}, "REEFER_CONTAINER": {}}

var equipmentUnitKinds = map[string]struct{}{
	"TRUCK_BODY": {}, "TRAILER": {}, "SEMITRAILER": {}, "CONTAINER_CHASSIS": {}, "SWAP_BODY": {}, "OTHER": {},
}

var accessSides = map[string]struct{}{"REAR": {}, "SIDE": {}, "TOP": {}}

func normalizeToken(field string, value *string, allowed map[string]struct{}) (*string, error) {
	if value == nil {
		return nil, nil
	}
	token := strings.TrimSpace(*value)
	if token == "" {
		return nil, apperrors.Validation(field+" must be omitted when unknown", map[string]any{"field": field})
	}
	if token == "UNKNOWN" {
		return nil, nil
	}
	if _, ok := allowed[token]; !ok {
		return nil, apperrors.Validation(field+" is not a canonical value", map[string]any{"field": field})
	}
	return &token, nil
}

func validateAccess(field string, values []string) error {
	if values == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := accessSides[value]; !ok {
			return apperrors.Validation(field+" contains an unsupported value", map[string]any{"field": field})
		}
		if _, dup := seen[value]; dup {
			return apperrors.Validation(field+" contains a duplicate value", map[string]any{"field": field})
		}
		seen[value] = struct{}{}
	}
	return nil
}

func ValidateListVehiclesFilter(f ListVehiclesFilter) error {
	if f.Limit == 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	return ValidateListPagination(f.Limit, f.Offset)
}

func NormalizeVehicleType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return VehicleTypeTruck
	}
	return value
}
