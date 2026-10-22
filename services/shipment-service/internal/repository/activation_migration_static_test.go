package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActivationGrantMigrationLeavesEnforcementUninstalled(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	grant, err := os.ReadFile(filepath.Join(root, "infrastructure", "migrations", "000104_tms_activation_runtime_grants_v0_1.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(grant)
	if strings.Contains(text, "CREATE TRIGGER") || strings.Contains(text, "CREATE CONSTRAINT TRIGGER") {
		t.Fatal("runtime grant migration installs the enforcement trigger")
	}
	if strings.Contains(text, "LOGIN PASSWORD") || strings.Contains(text, "ALTER ROLE shipment_runtime LOGIN") {
		t.Fatal("runtime grant migration creates a service password")
	}
	gate, err := os.ReadFile(filepath.Join(root, "infrastructure", "postgres", "tms_activation_enforcement_gate_v0.1.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gate), "trg_new_activation_authorization") || !strings.Contains(string(gate), "assert_activation_enforcement_role_split") {
		t.Fatal("enforcement gate file no longer installs the deferred trigger")
	}
}
