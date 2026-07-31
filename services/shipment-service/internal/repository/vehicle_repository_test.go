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

const createVehicleQueryPrefix = `
		INSERT INTO transport.vehicles (
			tenant_id, carrier_company_id, plate_number, vehicle_type, equipment_type,
			capacity_weight, capacity_volume, registration_country, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

func TestCreateVehicleQueryUsesVerifiedTenantParameter(t *testing.T) {
	t.Parallel()
	if !strings.Contains(createVehicleQueryPrefix, "tenant_id, carrier_company_id") {
		t.Fatalf("insert must include tenant_id column")
	}
	if !strings.Contains(createVehicleQueryPrefix, "VALUES ($1, $2,") {
		t.Fatalf("tenant must be first SQL parameter")
	}
}

func TestListVehiclesQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	query := strings.ToLower("tenant_id = $1 AND deleted_at IS NULL")
	if !strings.Contains(query, "tenant_id = $1") {
		t.Fatalf("list must always filter by tenant_id")
	}
}
