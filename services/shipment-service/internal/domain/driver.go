package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type Driver struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	CarrierCompanyID uuid.UUID
	UserID           *uuid.UUID
	FullName         string
	Phone            *string
	LicenseNumber    *string
	LicenseCountry   *string
	PreferredLocale  string
	Status           string
}

type CreateDriverInput struct {
	TenantID         uuid.UUID
	CarrierCompanyID uuid.UUID
	UserID           *uuid.UUID
	FullName         string
	Phone            *string
	LicenseNumber    *string
	LicenseCountry   string
	PreferredLocale  string
}

type ListDriversFilter struct {
	TenantID         uuid.UUID
	CarrierCompanyID *uuid.UUID
	Status           *string
	Limit            int
	Offset           int
}

func ValidateCreateDriverInput(in CreateDriverInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if strings.TrimSpace(in.FullName) == "" {
		return apperrors.Validation("full_name is required", map[string]any{"field": "full_name"})
	}
	return nil
}

func ValidateListDriversFilter(f ListDriversFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit == 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	return ValidateListPagination(f.Limit, f.Offset)
}
