package repository

import (
	"strings"
	"testing"
)

func TestMigration000013UpSeedsForwarderManagerRole(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000013_seed_forwarder_manager_role.up.sql"))
	if !strings.Contains(up, "insert into core.roles") {
		t.Fatal("up migration must insert into core.roles")
	}
	if !strings.Contains(up, "forwarder_manager") {
		t.Fatal("up migration must seed FORWARDER_MANAGER")
	}
	if !strings.Contains(up, "where not exists") {
		t.Fatal("up migration must be idempotent via WHERE NOT EXISTS")
	}
}

func TestMigration000013UpDoesNotModifyExistingRoles(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000013_seed_forwarder_manager_role.up.sql"))
	for _, code := range []string{
		"platform_admin",
		"shipper_admin",
		"carrier_admin",
		"consignee_operator",
	} {
		if strings.Contains(up, "update core.roles") && strings.Contains(up, code) {
			t.Fatalf("up migration must not update existing role %s", code)
		}
	}
	if strings.Contains(up, "delete from core.roles") {
		t.Fatal("up migration must not delete roles")
	}
}

func TestMigration000013DownRemovesForwarderManagerOnly(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000013_seed_forwarder_manager_role.down.sql"))
	if !strings.Contains(down, "delete from core.roles") {
		t.Fatal("down migration must delete seeded role")
	}
	if !strings.Contains(down, "forwarder_manager") {
		t.Fatal("down migration must target FORWARDER_MANAGER only")
	}
	for _, code := range []string{
		"platform_admin",
		"shipper_admin",
		"carrier_admin",
	} {
		if strings.Contains(down, code) {
			t.Fatalf("down migration must not remove role %s", code)
		}
	}
}
