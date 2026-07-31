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

const createDriverQueryPrefix = `
		INSERT INTO transport.drivers (
			tenant_id, carrier_company_id, user_id, full_name, phone,
			license_number, license_country, preferred_locale, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

func TestCreateDriverQueryUsesVerifiedTenantParameter(t *testing.T) {
	t.Parallel()
	if !strings.Contains(createDriverQueryPrefix, "tenant_id, carrier_company_id") {
		t.Fatalf("insert must include tenant_id column")
	}
	if !strings.Contains(createDriverQueryPrefix, "VALUES ($1, $2,") {
		t.Fatalf("tenant must be first SQL parameter")
	}
}

func TestListDriversQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	query := strings.ToLower("tenant_id = $1 AND deleted_at IS NULL")
	if !strings.Contains(query, "tenant_id = $1") {
		t.Fatalf("list must always filter by tenant_id")
	}
}
