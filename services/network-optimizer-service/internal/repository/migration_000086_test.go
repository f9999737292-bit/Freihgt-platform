package repository

import (
	"strings"
	"testing"
)

func TestNLO04CMigration000086Text(t *testing.T) {
	up := readMigration(t, "000086_nlo_route_plan_accept_activate_v0_4c.up.sql")
	down := readMigration(t, "000086_nlo_route_plan_accept_activate_v0_4c.down.sql")
	for _, required := range []string{
		"CREATE TABLE network_optimizer.route_plan_activations",
		"PENDING_EXECUTION",
		"EXECUTION_LINKED",
		"REJECTED",
		"UNIQUE (tenant_id, route_plan_id)",
		"UNIQUE (tenant_id, idempotency_key)",
		"execution_shipment_id uuid NULL",
		"effective_shipment_id uuid NULL",
		"route_plan_activations_one_effective_shipment_idx",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	if strings.Contains(up, "shipment-service") || strings.Contains(up, "execution_supported = true") || strings.Contains(up, "ALTER TABLE network_optimizer.route_plans") {
		t.Fatal("activation migration must not mutate plans or shipments")
	}
	for _, required := range []string{
		"nlo-0.4c down migration refused",
		"DROP TABLE IF EXISTS network_optimizer.route_plan_activations",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %s", required)
		}
	}
	if strings.Contains(down, "DROP TABLE IF EXISTS network_optimizer.route_plans") {
		t.Fatal("down migration must keep route plans")
	}
}
