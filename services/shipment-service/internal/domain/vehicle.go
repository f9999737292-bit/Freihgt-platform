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
	ID                  uuid.UUID
	TenantID            uuid.UUID
	CarrierCompanyID    uuid.UUID
	PlateNumber         string
	VehicleType         string
	EquipmentType       *string
	CapacityWeight      *float64
	CapacityVolume      *float64
	RegistrationCountry string
	Status              string
}

type CreateVehicleInput struct {
	TenantID            uuid.UUID
	CarrierCompanyID    uuid.UUID
	PlateNumber         string
	VehicleType         string
	EquipmentType       *string
	CapacityWeight      *float64
	CapacityVolume      *float64
	RegistrationCountry string
}

type ListVehiclesFilter struct {
	TenantID         uuid.UUID
	CarrierCompanyID *uuid.UUID
	Status           *string
	Limit            int
	Offset           int
}

func ValidateCreateVehicleInput(in CreateVehicleInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
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
	return nil
}

func ValidateListVehiclesFilter(f ListVehiclesFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
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
