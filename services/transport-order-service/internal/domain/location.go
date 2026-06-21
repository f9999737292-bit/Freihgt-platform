package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

const DefaultTimezone = "Europe/Moscow"

var allowedLocationTypes = map[string]struct{}{
	"WAREHOUSE":           {},
	"FACTORY":             {},
	"DISTRIBUTION_CENTER": {},
	"TERMINAL":            {},
	"PORT":                {},
	"AIRPORT":             {},
	"RAIL_STATION":        {},
	"BORDER_CHECKPOINT":   {},
	"CUSTOMER_SITE":       {},
}

type Location struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	CompanyID    *uuid.UUID
	LocationType string
	Name         string
	CountryCode  string
	Region       *string
	City         *string
	AddressLine  *string
	PostalCode   *string
	Lat          *float64
	Lon          *float64
	Timezone     string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int
}

type CreateLocationInput struct {
	TenantID     uuid.UUID
	CompanyID    *uuid.UUID
	LocationType string
	Name         string
	CountryCode  string
	Region       *string
	City         *string
	AddressLine  *string
	PostalCode   *string
	Lat          *float64
	Lon          *float64
	Timezone     string
}

type ListLocationsFilter struct {
	TenantID     uuid.UUID
	CompanyID    *uuid.UUID
	LocationType *string
	Search       *string
	Limit        int
	Offset       int
}

func ParseUUID(value, field string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid "+field, map[string]any{"field": field})
	}
	return id, nil
}

func ValidateCreateLocationInput(in CreateLocationInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.Name) == "" {
		return apperrors.Validation("name is required", map[string]any{"field": "name"})
	}
	if err := validateLocationType(in.LocationType); err != nil {
		return err
	}
	if err := validateCountryCode(in.CountryCode); err != nil {
		return err
	}
	return nil
}

func ValidateListLocationsFilter(f ListLocationsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit <= 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	if f.Limit > 100 {
		return apperrors.Validation("limit must be less than or equal to 100", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be greater than or equal to 0", map[string]any{"field": "offset"})
	}
	if f.LocationType != nil {
		if err := validateLocationType(*f.LocationType); err != nil {
			return err
		}
	}
	return nil
}

func NormalizeTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultTimezone
	}
	return value
}

func validateLocationType(value string) error {
	value = strings.TrimSpace(value)
	if _, ok := allowedLocationTypes[value]; !ok {
		return apperrors.Validation("invalid location_type", map[string]any{"field": "location_type", "value": value})
	}
	return nil
}

func validateCountryCode(value string) error {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 2 {
		return apperrors.Validation("country_code must be 2 characters", map[string]any{"field": "country_code"})
	}
	return nil
}

func NormalizeCountryCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
