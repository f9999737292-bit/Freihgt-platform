package repository

import (
	"strings"
	"testing"
)

func TestShipmentSourceAuthoritySQLIsReadOnly(t *testing.T) {
	statements := shipmentSourceAuthorityStatements()
	if len(statements) != 3 {
		t.Fatalf("statement count = %d", len(statements))
	}
	joined := strings.ToLower(strings.Join(statements, "\n"))
	for _, required := range []string{
		"from transport.shipments",
		"deleted_at is null",
		"shipper_company_id",
		"carrier_company_id",
		"from core.users",
		"from core.company_memberships",
		"ur.company_id = $3",
		"ur.company_id is null",
		"r.code = 'platform_admin'",
		"from core.companies",
		"c.status = 'active'",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("source authority SQL missing %s", required)
		}
	}
	for _, statement := range statements {
		if shipmentSourceAuthorityWritesRelationship(statement) {
			t.Fatalf("source authority SQL must not write relationship state: %s", statement)
		}
	}
}
