package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

const (
	StatusActive  = "ACTIVE"
	StatusDeleted = "DELETED"
)

var allowedCompanyTypes = map[string]struct{}{
	"SHIPPER":              {},
	"CONSIGNEE":            {},
	"CARRIER":              {},
	"FORWARDER":            {},
	"LSP":                  {},
	"WAREHOUSE":            {},
	"TERMINAL":             {},
	"SUPPLIER":             {},
	"GOVERNMENT_AUTHORITY": {},
	"EDO_OPERATOR":         {},
	"EPD_OPERATOR":         {},
	"PLATFORM_OPERATOR":    {},
}

var allowedLocales = map[string]struct{}{
	"ru-RU": {},
	"en-US": {},
	"zh-CN": {},
}

type Company struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	LegalName          string
	ShortName          *string
	LegalNameEN        *string
	LegalNameZH        *string
	CompanyType        string
	TaxID              *string
	RegistrationNumber *string
	CountryCode        string
	PreferredLocale    string
	Status             string
	CreatedAt          time.Time
	CreatedBy          *uuid.UUID
	UpdatedAt          time.Time
	UpdatedBy          *uuid.UUID
	DeletedAt          *time.Time
	Version            int
}

type CreateCompanyInput struct {
	TenantID           uuid.UUID
	LegalName          string
	ShortName          *string
	LegalNameEN        *string
	LegalNameZH        *string
	CompanyType        string
	TaxID              *string
	RegistrationNumber *string
	CountryCode        string
	PreferredLocale    string
}

type UpdateCompanyInput struct {
	LegalName          *string
	ShortName          *string
	LegalNameEN        *string
	LegalNameZH        *string
	CompanyType        *string
	TaxID              *string
	RegistrationNumber *string
	CountryCode        *string
	PreferredLocale    *string
	Status             *string
}

type ListCompaniesFilter struct {
	TenantID    uuid.UUID
	CompanyType *string
	Status      *string
	Search      *string
	Limit       int
	Offset      int
}

func ValidateCreateInput(in CreateCompanyInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.LegalName) == "" {
		return apperrors.Validation("legal_name is required", map[string]any{"field": "legal_name"})
	}
	if err := validateCompanyType(in.CompanyType); err != nil {
		return err
	}
	if err := validateCountryCode(in.CountryCode); err != nil {
		return err
	}
	if err := validatePreferredLocale(in.PreferredLocale); err != nil {
		return err
	}
	return nil
}

func ValidateUpdateInput(in UpdateCompanyInput) error {
	if in.CompanyType != nil {
		if err := validateCompanyType(*in.CompanyType); err != nil {
			return err
		}
	}
	if in.CountryCode != nil {
		if err := validateCountryCode(*in.CountryCode); err != nil {
			return err
		}
	}
	if in.PreferredLocale != nil {
		if err := validatePreferredLocale(*in.PreferredLocale); err != nil {
			return err
		}
	}
	if in.LegalName != nil && strings.TrimSpace(*in.LegalName) == "" {
		return apperrors.Validation("legal_name cannot be empty", map[string]any{"field": "legal_name"})
	}
	if in.Status != nil && strings.TrimSpace(*in.Status) == "" {
		return apperrors.Validation("status cannot be empty", map[string]any{"field": "status"})
	}
	return nil
}

func ValidateListFilter(f ListCompaniesFilter) error {
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
	if f.CompanyType != nil {
		if err := validateCompanyType(*f.CompanyType); err != nil {
			return err
		}
	}
	return nil
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

func validateCompanyType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.Validation("company_type is required", map[string]any{"field": "company_type"})
	}
	if _, ok := allowedCompanyTypes[value]; !ok {
		return apperrors.Validation("invalid company_type", map[string]any{"field": "company_type", "value": value})
	}
	return nil
}

func validateCountryCode(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "RU"
	}
	if len(value) != 2 {
		return apperrors.Validation("country_code must be 2 characters", map[string]any{"field": "country_code"})
	}
	return nil
}

func validatePreferredLocale(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "ru-RU"
	}
	if _, ok := allowedLocales[value]; !ok {
		return apperrors.Validation("invalid preferred_locale", map[string]any{"field": "preferred_locale", "value": value})
	}
	return nil
}

func NormalizeCountryCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "RU"
	}
	return strings.ToUpper(value)
}

func NormalizePreferredLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ru-RU"
	}
	return value
}

func OptionalString(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
