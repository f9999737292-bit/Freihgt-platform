package repository

import (
	"strings"
	"testing"
)

func TestNLO04DMigration000094Text(t *testing.T) {
	up := readMigration(t, "000094_nlo_service_duration_policy_v0_4d.up.sql")
	down := readMigration(t, "000094_nlo_service_duration_policy_v0_4d.down.sql")
	for _, required := range []string{
		"CREATE TABLE network_optimizer.service_duration_policies",
		"CREATE TABLE network_optimizer.service_duration_policy_entries",
		"tenant_id uuid NOT NULL",
		"version integer NOT NULL CHECK (version >= 1)",
		"CHECK (status IN ('DRAFT', 'ACTIVE', 'RETIRED'))",
		"UNIQUE (tenant_id, version)",
		"service_duration_policies_one_active_idx",
		"WHERE status = 'ACTIVE'",
		"CHECK (action_type IN ('PICKUP', 'DELIVERY'))",
		"CHECK (duration_seconds >= 0)",
		"SERVICE_DURATION_POLICY_IMMUTABLE",
		"'SERVICE_DURATION_POLICY'",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("up migration missing %s", required)
		}
	}
	for _, forbidden := range []string{
		"transport.transport_executions",
		"shipment.status",
		"execution_supported = true",
		"CREATE TABLE network_optimizer.route_plans",
	} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("up migration touches %s", forbidden)
		}
	}
	for _, required := range []string{
		"nlo-0.4d down migration refused: service duration policy rows exist",
		"DROP TABLE IF EXISTS network_optimizer.service_duration_policy_entries",
		"DROP TABLE IF EXISTS network_optimizer.service_duration_policies",
		"DROP FUNCTION IF EXISTS network_optimizer.reject_service_duration_policy_mutation()",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("down migration missing %s", required)
		}
	}
	if strings.Contains(down, "'SERVICE_DURATION_POLICY'") || strings.Contains(down, "DROP TABLE IF EXISTS network_optimizer.route_plans") {
		t.Fatal("down migration must keep route plans and drop the policy dependency kind")
	}
}
