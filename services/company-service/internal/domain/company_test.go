package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateInput(t *testing.T) {
	t.Parallel()

	validTenant := mustUUID(t, "11111111-1111-1111-1111-111111111111")

	tests := []struct {
		name    string
		input   CreateCompanyInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: CreateCompanyInput{
				TenantID:        validTenant,
				LegalName:       "ООО Ромашка",
				CompanyType:     "SHIPPER",
				CountryCode:     "RU",
				PreferredLocale: "ru-RU",
			},
		},
		{
			name: "missing tenant",
			input: CreateCompanyInput{
				LegalName:       "ООО Ромашка",
				CompanyType:     "SHIPPER",
				PreferredLocale: "ru-RU",
			},
			wantErr: true,
		},
		{
			name: "invalid company type",
			input: CreateCompanyInput{
				TenantID:        validTenant,
				LegalName:       "ООО Ромашка",
				CompanyType:     "INVALID",
				PreferredLocale: "ru-RU",
			},
			wantErr: true,
		},
		{
			name: "invalid locale",
			input: CreateCompanyInput{
				TenantID:        validTenant,
				LegalName:       "ООО Ромашка",
				CompanyType:     "SHIPPER",
				PreferredLocale: "fr-FR",
			},
			wantErr: true,
		},
		{
			name: "invalid country code",
			input: CreateCompanyInput{
				TenantID:        validTenant,
				LegalName:       "ООО Ромашка",
				CompanyType:     "SHIPPER",
				CountryCode:     "RUS",
				PreferredLocale: "ru-RU",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCreateInput(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateListFilter(t *testing.T) {
	t.Parallel()

	validTenant := mustUUID(t, "11111111-1111-1111-1111-111111111111")

	if err := ValidateListFilter(ListCompaniesFilter{TenantID: validTenant, Limit: 20, Offset: 0}); err != nil {
		t.Fatalf("expected valid filter, got %v", err)
	}

	if err := ValidateListFilter(ListCompaniesFilter{Limit: 20, Offset: 0}); err == nil {
		t.Fatalf("expected missing tenant error")
	}

	if err := ValidateListFilter(ListCompaniesFilter{TenantID: validTenant, Limit: 0, Offset: 0}); err == nil {
		t.Fatalf("expected invalid limit error")
	}
}

func mustUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := ParseUUID(value, "tenant_id")
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}
