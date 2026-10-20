package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRelationshipMigrationDoesNotRewriteHistoricalData(t *testing.T) {
	up := readMigration(t, "000100_company_carrier_relationship_foundation_v0_1c.up.sql")
	down := readMigration(t, "000100_company_carrier_relationship_foundation_v0_1c.down.sql")
	if !strings.Contains(up, "CREATE SCHEMA IF NOT EXISTS company") {
		t.Fatal("migration must create the company schema")
	}
	if !strings.Contains(up, "acceptance_token_hash BYTEA NOT NULL") {
		t.Fatal("token storage must be an HMAC fingerprint column")
	}
	if regexp.MustCompile(`(?i)acceptance_token\s+(text|varchar|bytea)`).MatchString(up) {
		t.Fatal("raw acceptance token column is present")
	}
	historical := regexp.MustCompile(`(?i)\b(update|delete|insert)\s+(core|transport|rfx|documents|billing)\.`)
	if historical.MatchString(up) || historical.MatchString(down) {
		t.Fatal("migration writes existing historical schemas")
	}
	if !strings.Contains(down, "DROP SCHEMA IF EXISTS company") {
		t.Fatal("down migration must remove only the new company schema")
	}
	for _, name := range []string{
		"carrier_relationship_requests",
		"carrier_relationship_request_events",
		"carrier_relationship_request_heads",
		"carrier_relationship_request_idempotency",
		"carrier_relationships",
		"carrier_relationship_versions",
		"carrier_relationship_heads",
		"carrier_relationship_audit",
		"carrier_relationship_outbox",
	} {
		if !strings.Contains(up, name) || !strings.Contains(down, name) {
			t.Fatalf("table %s is not paired in up and down", name)
		}
	}
}

func TestRevokeMigrationDoesNotTouchHistoricalSchemas(t *testing.T) {
	up := readMigration(t, "000102_company_carrier_relationship_revoke_v0_2.up.sql")
	down := readMigration(t, "000102_company_carrier_relationship_revoke_v0_2.down.sql")
	command, err := os.ReadFile("relationship_revoke.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{up, down, string(command)} {
		for _, forbidden := range []string{"transport.", "billing.", "documents.", "control_tower."} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("revoke artifact names %s", forbidden)
			}
		}
	}
	if strings.Contains(strings.ToLower(up), "delete from") || strings.Contains(strings.ToLower(down), "delete from") {
		t.Fatal("revoke migration deletes rows")
	}
	if !strings.Contains(down, "btrim(revoke_reason)") || !strings.Contains(up, "carrier_relationship_revoke_idempotency") {
		t.Fatal("revoke migration is missing the idempotency table or the down guard")
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
