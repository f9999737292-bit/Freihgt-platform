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
		"grant insert on core.companies to company_runtime",
		"grant update (legal_name, short_name, legal_name_en, legal_name_zh, company_type, tax_id, registration_number, country_code, preferred_locale, status, updated_at, version, deleted_at) on core.companies to company_runtime",
		"grant insert on core.company_memberships to company_runtime",
		"grant update (position, status, deleted_at, updated_at, version) on core.company_memberships to company_runtime",
		"grant insert, delete on core.user_roles to company_runtime",
		"grant insert on transport.shipments to shipment_runtime",
		"grant select (id, tenant_id, status, deleted_at) on core.users to shipment_runtime",
		"grant select, update (status, version, updated_at) on transport.transport_orders to shipment_runtime",
		"grant update (disposition_type, status, pending_quantity, return_reserved_quantity, redirect_reserved_quantity, hold_reserved_quantity, return_target_location_id, redirect_target_location_id, successor_revision_id, resolved_quantity, version, updated_at) on transport.delivery_disposition_cases to shipment_runtime",
		"grant update (result_json) on transport.delivery_disposition_commands to shipment_runtime",
		"grant update (push_token_hash, push_token_ciphertext, platform, app_version, locale, last_seen_at, updated_at, revoked_at) on transport.driver_device to shipment_runtime",
		"grant update (status, claimed_by, claimed_until, updated_at, attempt_count, provider_message_id, sent_at, error_code, next_attempt_at) on transport.driver_notification_delivery to shipment_runtime",
		"grant update (id, status, version, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at) on transport.driver_task to shipment_runtime",
		"grant select on rfx.rfx_award_transport_orders to shipment_runtime",
		"revoke insert, update, delete on rfx.rfx_award_transport_orders from shipment_runtime",
		"revoke insert, update, delete on transport.transport_execution_carrier_relationship_pins from shipment_runtime",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	for _, required := range []string{
		"revoke update (legal_name, short_name, legal_name_en, legal_name_zh, company_type, tax_id, registration_number, country_code, preferred_locale, status, updated_at, version, deleted_at) on core.companies from company_runtime",
		"revoke update (position, status, deleted_at, updated_at, version) on core.company_memberships from company_runtime",
		"revoke select (id, tenant_id, status, deleted_at) on core.users from shipment_runtime",
		"revoke update (id, status, version, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at) on transport.driver_task from shipment_runtime",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %s", required)
		}
	}
	for _, line := range strings.Split(up, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "grant update") {
			continue
		}
		for _, denied := range []string{
			"tenant_id",
			"company_id",
			"user_id",
			"operating_tenant_id",
			"shipment_tenant_id",
			"email",
			"phone",
			"mfa_enabled",
			"last_login_at",
		} {
			if strings.Contains(trimmed, denied) {
				t.Fatalf("update grant includes %s: %s", denied, trimmed)
			}
		}
	}
	for _, tableWide := range []string{
		"grant update on core.companies to company_runtime",
		"grant update on core.company_memberships to company_runtime",
		"grant select on core.users to shipment_runtime",
		"grant update on transport.delivery_disposition_cases to shipment_runtime",
		"grant update on transport.delivery_disposition_commands to shipment_runtime",
		"grant update on transport.driver_device to shipment_runtime",
		"grant update on transport.driver_notification_delivery to shipment_runtime",
		"grant update on transport.driver_task to shipment_runtime",
		"grant select, insert, update on transport.delivery_disposition_cases to shipment_runtime",
		"grant select, insert, update on transport.driver_task to shipment_runtime",
	} {
		if strings.Contains(up, tableWide) {
			t.Fatalf("table-wide grant still present: %s", tableWide)
		}
	}
	if strings.Contains(down, "revoke select on core.users from shipment_runtime") ||
		strings.Contains(down, "revoke insert, update on core.companies from company_runtime") ||
		strings.Contains(down, "revoke select on core.companies from company_runtime") ||
		strings.Contains(down, "revoke select, update on transport.shipments from shipment_runtime") {
		t.Fatal("down migration removes a privilege that 000105 did not add")
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
