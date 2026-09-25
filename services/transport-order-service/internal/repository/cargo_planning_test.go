package repository

import (
	"strings"
	"testing"
)

func TestBNO137CargoSourceReadIsTenantScoped(t *testing.T) {
	if !strings.Contains(getCargoByIDAndTenantQuery, "tenant_id = $2") || !strings.Contains(getCargoByIDAndTenantQuery, "pallet_count") {
		t.Fatal("cargo planning read must stay tenant scoped and include planning facts")
	}
	if strings.Contains(getCargoByIDAndTenantQuery, "OR tenant_id IS NULL") {
		t.Fatal("cargo read must not accept a missing tenant")
	}
}
