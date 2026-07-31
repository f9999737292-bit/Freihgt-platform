package repository

import (
	"strings"
	"testing"
)

func TestGetVehicleByIDAndTenantQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(getVehicleByIDAndTenantQuery)
	if !strings.Contains(query, "where id = $1 and tenant_id = $2") {
		t.Fatalf("query must filter by id and tenant_id: %q", getVehicleByIDAndTenantQuery)
	}
	if strings.Count(query, "from transport.vehicles") != 1 {
		t.Fatalf("query must use a single vehicles lookup")
	}
	if !strings.Contains(query, "deleted_at is null") {
		t.Fatalf("query must exclude soft-deleted vehicles")
	}
}

func TestGetVehicleByIDAndTenantQueryUsesSeparateParameters(t *testing.T) {
	t.Parallel()
	if !strings.Contains(getVehicleByIDAndTenantQuery, "$1") || !strings.Contains(getVehicleByIDAndTenantQuery, "$2") {
		t.Fatalf("tenant_id must be a separate SQL parameter")
	}
}
