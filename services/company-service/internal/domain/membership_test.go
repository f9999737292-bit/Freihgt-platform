package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateMembershipInput(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validCompany := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	validUser := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	if err := ValidateCreateMembershipInput(CreateMembershipInput{
		TenantID:  validTenant,
		CompanyID: validCompany,
		UserID:    validUser,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateMembershipInput(CreateMembershipInput{
		CompanyID: validCompany,
		UserID:    validUser,
	}); err == nil {
		t.Fatalf("expected tenant_id validation error")
	}
}

func TestValidateUpdateMembershipInput(t *testing.T) {
	t.Parallel()

	active := MembershipStatusActive
	if err := ValidateUpdateMembershipInput(UpdateMembershipInput{Status: &active}); err != nil {
		t.Fatalf("expected valid status, got %v", err)
	}

	invalid := "UNKNOWN"
	if err := ValidateUpdateMembershipInput(UpdateMembershipInput{Status: &invalid}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestValidateListCompanyMembersFilter(t *testing.T) {
	t.Parallel()

	validTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	validCompany := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	if err := ValidateListCompanyMembersFilter(ListCompanyMembersFilter{
		TenantID:  validTenant,
		CompanyID: validCompany,
		Limit:     20,
	}); err != nil {
		t.Fatalf("expected valid filter, got %v", err)
	}

	if err := ValidateListCompanyMembersFilter(ListCompanyMembersFilter{
		TenantID:  validTenant,
		CompanyID: validCompany,
		Limit:     0,
	}); err == nil {
		t.Fatalf("expected limit validation error")
	}
}
