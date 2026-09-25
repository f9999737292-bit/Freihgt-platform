package repository

import (
	"strings"
	"testing"
)

func TestMigration000078Text(t *testing.T) {
	up := readMigration(t, "000078_bno_geography_routing_foundation_v0_1c0.up.sql")
	down := readMigration(t, "000078_bno_geography_routing_foundation_v0_1c0.down.sql")
	for _, required := range []string{
		"ADD COLUMN location_id uuid",
		"pickup_country_code",
		"delivery_city",
		"CREATE TABLE network_optimizer.carrier_search_policies",
		"CREATE TABLE network_optimizer.capacity_search_policies",
		"DIRECTIONAL_CORRIDOR",
		"ROUTE_ELLIPSE",
		"allow_unknown_road_distance boolean NOT NULL DEFAULT false",
		"capacity_search_policies_capacity_owner_fk",
		"capacities_id_owner_uidx",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	if strings.Contains(strings.ToLower(up), "create table network_optimizer.locations") || strings.Contains(strings.ToLower(up), "create table transport.locations") {
		t.Fatal("migration created a second location master")
	}
	for _, required := range []string{"DROP COLUMN IF EXISTS location_id", "DROP TABLE IF EXISTS network_optimizer.carrier_search_policies"} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %s", required)
		}
	}
}
