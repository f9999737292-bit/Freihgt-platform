package repository

import (
	"strings"
	"testing"
)

func TestGetShipmentByIDAndTenantQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(getShipmentByIDAndTenantQuery)
	if !strings.Contains(query, "where id = $1 and tenant_id = $2") {
		t.Fatalf("query must filter by id and tenant_id in one lookup: %q", getShipmentByIDAndTenantQuery)
	}
	if strings.Count(query, "from transport.shipments") != 1 {
		t.Fatalf("query must use a single shipments lookup")
	}
}

func TestGetShipmentByIDAndTenantQueryUsesSeparateParameters(t *testing.T) {
	t.Parallel()
	if !strings.Contains(getShipmentByIDAndTenantQuery, "$1") || !strings.Contains(getShipmentByIDAndTenantQuery, "$2") {
		t.Fatalf("tenant_id must be a separate SQL parameter")
	}
}
