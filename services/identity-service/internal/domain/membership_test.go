package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateListUserCompaniesFilter(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validUser := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	if err := ValidateListUserCompaniesFilter(ListUserCompaniesFilter{
		TenantID: validTenant,
		UserID:   validUser,
	}); err != nil {
		t.Fatalf("expected valid filter, got %v", err)
	}

	if err := ValidateListUserCompaniesFilter(ListUserCompaniesFilter{
		UserID: validUser,
	}); err == nil {
		t.Fatalf("expected tenant_id validation error")
	}
}

func TestValidateAssignCompanyRoleInput(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validUser := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	validCompany := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	validRole := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	if err := ValidateAssignCompanyRoleInput(AssignCompanyRoleInput{
		TenantID:  validTenant,
		UserID:    validUser,
		CompanyID: validCompany,
		RoleID:    validRole,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateAssignCompanyRoleInput(AssignCompanyRoleInput{
		TenantID:  validTenant,
		UserID:    validUser,
		CompanyID: validCompany,
	}); err == nil {
		t.Fatalf("expected role_id validation error")
	}
}
