package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type Cargo struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	CargoType          string
	Description        *string
	GrossWeight        *float64
	NetWeight          *float64
	Volume             *float64
	TemperatureMin     *float64
	TemperatureMax     *float64
	DangerousGoodsFlag bool
	CustomsRequired    bool
	Items              []CargoItem
	CreatedAt          string
	UpdatedAt          string
	Version            int
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
	TenantID           uuid.UUID
	CargoType          string
	Description        *string
	GrossWeight        *float64
	NetWeight          *float64
	Volume             *float64
	TemperatureMin     *float64
	TemperatureMax     *float64
	DangerousGoodsFlag bool
	CustomsRequired    bool
	Items              []CreateCargoItemInput
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
	return nil
}
