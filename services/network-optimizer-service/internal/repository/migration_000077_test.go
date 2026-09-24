package repository

import (
	"strings"
	"testing"
)

func TestMigration000077Text(t *testing.T) {
	up := readMigration(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql")
	down := readMigration(t, "000077_bno_cargo_equipment_compatibility_v0_1b2.down.sql")
	for _, required := range []string{
		"CREATE TABLE network_optimizer.reference_catalog_versions",
		"CREATE TABLE network_optimizer.cargo_type_catalog",
		"CREATE TABLE network_optimizer.equipment_type_catalog",
		"CREATE TABLE network_optimizer.pallet_type_catalog",
		"CREATE TABLE network_optimizer.packaging_type_catalog",
		"CREATE TABLE network_optimizer.compatibility_rule_sets",
		"CREATE TABLE network_optimizer.compatibility_rules",
		"reference_catalog_one_active_system_uidx",
		"compatibility_rule_sets_one_active_tenant_uidx",
		"SYSTEM_SEED",
		"layer <> 'REGULATORY'",
		"pallet_count IS NULL OR pallet_count > 0",
		"equipment_unit_kind",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	lower := strings.ToLower(up)
	for _, forbidden := range []string{"trailer_id", "registration_number", " vin "} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("migration invents a physical trailer field %s", forbidden)
		}
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS network_optimizer.reference_catalog_versions") || !strings.Contains(down, "DROP COLUMN IF EXISTS pallet_count") {
		t.Fatal("down migration must remove the new catalog and cargo facts")
	}
}
