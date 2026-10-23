package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoleFoundationMigrationStaysNoLogin(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	upPath := filepath.Join(root, "infrastructure", "migrations", "000105_platform_postgres_role_foundation_v0_2_i1.up.sql")
	downPath := filepath.Join(root, "infrastructure", "migrations", "000105_platform_postgres_role_foundation_v0_2_i1.down.sql")
	upRaw, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	downRaw, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	up := strings.ToLower(string(upRaw))
	down := strings.ToLower(string(downRaw))
	for _, forbidden := range []string{
		"create role",
		"alter role",
		"login password",
		"password",
		"create trigger",
		"create constraint trigger",
		"pg_hba",
		"lock_award_transport_order_for_execution",
		"security definer",
		"drop role",
		"freight",
	} {
		if strings.Contains(up, forbidden) || strings.Contains(down, forbidden) {
			t.Fatalf("role foundation migration contains %q", forbidden)
		}
	}
	for _, required := range []string{
		"grant insert, update on core.companies to company_runtime",
		"grant insert, update on core.company_memberships to company_runtime",
		"grant insert, delete on core.user_roles to company_runtime",
		"grant insert on transport.shipments to shipment_runtime",
		"grant select, update (status, version, updated_at) on transport.transport_orders to shipment_runtime",
		"grant select on rfx.rfx_award_transport_orders to shipment_runtime",
		"revoke insert, update, delete on rfx.rfx_award_transport_orders from shipment_runtime",
		"revoke insert, update, delete on transport.transport_execution_carrier_relationship_pins from shipment_runtime",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	if strings.Contains(up, "grant update on core.user_roles") || strings.Contains(up, "grant insert, update, delete on core.user_roles") {
		t.Fatal("company runtime must not receive UPDATE on core.user_roles")
	}
	if strings.Contains(up, "grant update on rfx.rfx_award_transport_orders to shipment_runtime") ||
		strings.Contains(up, "grant insert, update, delete on rfx.rfx_award_transport_orders to shipment_runtime") ||
		strings.Contains(up, "grant select, update on rfx.rfx_award_transport_orders to shipment_runtime") {
		t.Fatal("shipment runtime must not receive table UPDATE on the award link")
	}
}
