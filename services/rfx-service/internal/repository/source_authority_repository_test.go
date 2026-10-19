package repository

import (
	"strings"
	"testing"
)

func TestRfxSourceAuthoritySQLIsReadOnly(t *testing.T) {
	statements := rfxSourceAuthorityStatements()
	if len(statements) != 4 {
		t.Fatalf("statement count = %d", len(statements))
	}
	joined := strings.ToLower(strings.Join(statements, "\n"))
	for _, required := range []string{
		"from rfx.rfx_awards",
		"from rfx.rfx_award_transport_orders",
		"left join rfx.rfx_events",
		"left join rfx.rfx_awards",
		"a.version",
		"a.rfx_event_id",
		"a.carrier_company_id",
		"l.version",
		"l.rfx_award_id",
		"l.transport_order_id",
		"l.carrier_company_id",
		"l.buyer_company_id",
		"e.owner_company_id",
		"e.status",
		"deleted_at is null",
		"from core.users",
		"from core.company_memberships",
		"ur.company_id = $3",
		"ur.company_id is null",
		"r.code = 'platform_admin'",
		"u.deleted_at is null",
		"u.status = 'active'",
		"from core.companies",
		"c.status = 'active'",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("source authority SQL missing %s", required)
		}
	}
	for _, statement := range statements {
		if rfxSourceAuthorityWritesRelationship(statement) {
			t.Fatalf("source authority SQL must not write relationship state or read transport orders: %s", statement)
		}
	}
}
