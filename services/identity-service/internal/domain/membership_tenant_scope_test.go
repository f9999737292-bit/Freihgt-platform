package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateListUserCompaniesFilterRequiresTenantScope(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	if err := ValidateListUserCompaniesFilter(ListUserCompaniesFilter{
		UserID: userID,
	}); err == nil {
		t.Fatal("expected tenant_id validation error")
	}

	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if err := ValidateListUserCompaniesFilter(ListUserCompaniesFilter{
		TenantID: tenantA,
		UserID:   userID,
	}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if tenantA == tenantB {
		t.Fatal("test tenants must differ")
	}
}
