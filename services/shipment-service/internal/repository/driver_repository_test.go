package repository

import (
	"strings"
	"testing"
)

func TestGetDriverByIDAndTenantQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(getDriverByIDAndTenantQuery)
	if !strings.Contains(query, "where id = $1 and tenant_id = $2") {
		t.Fatalf("query must filter by id and tenant_id: %q", getDriverByIDAndTenantQuery)
	}
	if strings.Count(query, "from transport.drivers") != 1 {
		t.Fatalf("query must use a single drivers lookup")
	}
	if !strings.Contains(query, "deleted_at is null") {
		t.Fatalf("query must exclude soft-deleted drivers")
	}
}

func TestGetDriverByIDAndTenantQueryUsesSeparateParameters(t *testing.T) {
	t.Parallel()
	if !strings.Contains(getDriverByIDAndTenantQuery, "$1") || !strings.Contains(getDriverByIDAndTenantQuery, "$2") {
		t.Fatalf("tenant_id must be a separate SQL parameter")
	}
}
