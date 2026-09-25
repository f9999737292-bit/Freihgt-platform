package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type Cargo struct {
	ID                            uuid.UUID
	TenantID                      uuid.UUID
	CargoType                     string
	Description                   *string
	GrossWeight                   *float64
	NetWeight                     *float64
	Volume                        *float64
	TemperatureMin                *float64
	TemperatureMax                *float64
	DangerousGoodsFlag            bool
	CustomsRequired               bool
	CargoTypeCode                 *string
	PalletCount                   *int
	PalletTypeCode                *string
	LinearMeters                  *float64
	MaxLoadedHeightMM             *int
	Stackable                     *bool
	Fragile                       *bool
	PackagingTypeCode             *string
	FoodGradeRequired             *bool
	TemperatureRequired           *bool
	PreferredTemperatureSetpointC *float64
	OdorEmissionClass             *string
	OdorSensitive                 *bool
	ContaminationClass            *string
	Items                         []CargoItem
	CreatedAt                     string
	UpdatedAt                     string
	Version                       int
}

type CargoItem struct {
	ID          uuid.UUID
	CargoID     uuid.UUID
	SKU         *string
	Name        string
	Quantity    float64
	Unit        string
	Weight      *float64
	Volume      *float64
	PackageType *string
	HazardClass *string
}

type CreateCargoItemInput struct {
	SKU         *string
	Name        string
	Quantity    float64
	Unit        string
	Weight      *float64
	Volume      *float64
	PackageType *string
	HazardClass *string
}

type CreateCargoInput struct {
	TenantID                      uuid.UUID
	CargoType                     string
	Description                   *string
	GrossWeight                   *float64
	NetWeight                     *float64
	Volume                        *float64
	TemperatureMin                *float64
	TemperatureMax                *float64
	DangerousGoodsFlag            bool
	CustomsRequired               bool
	CargoTypeCode                 *string
	PalletCount                   *int
	PalletTypeCode                *string
	LinearMeters                  *float64
	MaxLoadedHeightMM             *int
	Stackable                     *bool
	Fragile                       *bool
	PackagingTypeCode             *string
	FoodGradeRequired             *bool
	TemperatureRequired           *bool
	PreferredTemperatureSetpointC *float64
	OdorEmissionClass             *string
	OdorSensitive                 *bool
	ContaminationClass            *string
	Items                         []CreateCargoItemInput
}

func ValidateCreateCargoInput(in CreateCargoInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.CargoType) == "" {
		return apperrors.Validation("cargo_type is required", map[string]any{"field": "cargo_type"})
	}
	if len(in.Items) == 0 {
		return apperrors.Validation("at least one cargo item is required", map[string]any{"field": "items"})
	}
	for i, item := range in.Items {
		if strings.TrimSpace(item.Name) == "" {
			return apperrors.Validation("item name is required", map[string]any{"field": "items", "index": i})
		}
		if item.Quantity <= 0 {
			return apperrors.Validation("item quantity must be greater than 0", map[string]any{"field": "items", "index": i})
		}
		if strings.TrimSpace(item.Unit) == "" {
			return apperrors.Validation("item unit is required", map[string]any{"field": "items", "index": i})
		}
	}
	if in.PalletCount != nil && *in.PalletCount <= 0 {
		return apperrors.Validation("pallet_count must be greater than zero when known", map[string]any{"field": "pallet_count"})
	}
	if in.LinearMeters != nil && *in.LinearMeters <= 0 {
		return apperrors.Validation("linear_meters must be greater than zero when known", map[string]any{"field": "linear_meters"})
	}
	if in.MaxLoadedHeightMM != nil && *in.MaxLoadedHeightMM <= 0 {
		return apperrors.Validation("max_loaded_height_mm must be greater than zero when known", map[string]any{"field": "max_loaded_height_mm"})
	}
	if in.TemperatureMin != nil && in.TemperatureMax != nil && *in.TemperatureMax < *in.TemperatureMin {
		return apperrors.Validation("temperature_max must be greater than or equal to temperature_min", map[string]any{"field": "temperature_max"})
	}
	return nil
}
