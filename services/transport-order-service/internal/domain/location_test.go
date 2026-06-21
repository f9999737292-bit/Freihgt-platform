package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateLocationInput(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if err := ValidateCreateLocationInput(CreateLocationInput{
		TenantID:     validTenant,
		LocationType: "WAREHOUSE",
		Name:         "Склад Москва",
		CountryCode:  "RU",
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateLocationInput(CreateLocationInput{
		LocationType: "WAREHOUSE",
		Name:         "Склад",
		CountryCode:  "RU",
	}); err == nil {
		t.Fatalf("expected tenant_id validation error")
	}

	if err := ValidateCreateLocationInput(CreateLocationInput{
		TenantID:     validTenant,
		LocationType: "INVALID",
		Name:         "Склад",
		CountryCode:  "RU",
	}); err == nil {
		t.Fatalf("expected location_type validation error")
	}

	if err := ValidateCreateLocationInput(CreateLocationInput{
		TenantID:     validTenant,
		LocationType: "WAREHOUSE",
		Name:         "Склад",
		CountryCode:  "RUS",
	}); err == nil {
		t.Fatalf("expected country_code validation error")
	}
}

func TestNormalizeTimezone(t *testing.T) {
	t.Parallel()

	if got := NormalizeTimezone(""); got != DefaultTimezone {
		t.Fatalf("expected default timezone, got %s", got)
	}
}
