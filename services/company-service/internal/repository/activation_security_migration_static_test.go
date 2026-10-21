package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActivationSecurityMigrationKeepsEnforcementGated(t *testing.T) {
	up := readMigration(t, "000103_tms_relationship_pins_security_foundation_v0_1.up.sql")
	down := readMigration(t, "000103_tms_relationship_pins_security_foundation_v0_1.down.sql")
	gateRaw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "postgres", "tms_activation_enforcement_gate_v0.1.sql"))
	if err != nil {
		t.Fatal(err)
	}
	gate := string(gateRaw)
	for _, name := range []string{"freight_migrations", "company_runtime", "shipment_runtime", "search_path = pg_catalog", "SECURITY DEFINER", "fk_pin_subject_identity", "fk_pin_subject_locked_version"} {
		if !strings.Contains(up, name) {
			t.Fatalf("up migration missing %s", name)
		}
	}
	if strings.Contains(strings.ToUpper(up), " PASSWORD ") || strings.Contains(up, "PASSWORD '") {
		t.Fatal("migration stores a role password")
	}
	if strings.Contains(up, "CREATE CONSTRAINT TRIGGER") || strings.Contains(up, "CREATE TRIGGER trg_new_activation_authorization") {
		t.Fatal("preparatory migration installs the revision enforcement trigger")
	}
	if strings.Contains(up, "EXECUTE '") || strings.Contains(up, "EXECUTE format") {
		t.Fatal("security functions use dynamic SQL")
	}
	propagate := functionBody(t, up, "CREATE FUNCTION transport.propagate_execution_relationship_pins")
	if strings.Contains(propagate, "carrier_relationship_heads") || strings.Contains(propagate, "authorize_carrier_relationship_activation") {
		t.Fatal("successor pin copy reads the head or calls the authorizer")
	}
	if !strings.Contains(gate, "CREATE CONSTRAINT TRIGGER trg_new_activation_authorization") || !strings.Contains(gate, "assert_activation_enforcement_role_split") {
		t.Fatal("enforcement gate is not the deferred revision check")
	}
	if !strings.Contains(down, "DROP TRIGGER IF EXISTS trg_new_activation_authorization") || !strings.Contains(down, "DROP ROLE IF EXISTS freight_migrations") {
		t.Fatal("down migration does not remove the gate trigger and the migration role")
	}
}

func functionBody(t *testing.T, sql, signature string) string {
	t.Helper()
	start := strings.Index(sql, signature)
	if start < 0 {
		t.Fatalf("missing %s", signature)
	}
	rest := sql[start:]
	end := strings.Index(rest, "\n$fn$;")
	if end < 0 {
		t.Fatal("function body is not closed")
	}
	return rest[:end]
}
