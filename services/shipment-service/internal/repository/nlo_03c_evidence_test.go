package repository

import (
	"os"
	"strings"
	"testing"
)

func TestNLO03C_009_EvidenceInsertIsInsideStatusTransaction(t *testing.T) {
	raw, err := os.ReadFile("shipment_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	start := strings.Index(src, "func (r *ShipmentRepository) updateStatus")
	if start < 0 {
		t.Fatal("updateStatus missing")
	}
	body := src[start:]
	insertAt := strings.Index(body, "insertCargoEvidence")
	commitAt := strings.Index(body, "tx.Commit")
	if insertAt < 0 || commitAt < 0 || insertAt > commitAt {
		t.Fatal("evidence insert must run before commit")
	}
	if !strings.Contains(src, "UpdateStatusWithCargoEvidence") {
		t.Fatal("driver evidence path missing")
	}
	manual := src[strings.Index(src, "func (r *ShipmentRepository) UpdateStatus("):strings.Index(src, "func (r *ShipmentRepository) UpdateStatusWithCargoEvidence")]
	if strings.Contains(manual, "CONFIRMED_ONBOARD") {
		t.Fatal("manual UpdateStatus must not name onboard evidence")
	}
}

func TestNLO03C_004_NO_HISTORICAL_LOADED_BACKFILL(t *testing.T) {
	raw, err := os.ReadFile("../../../../infrastructure/migrations/000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToUpper(string(raw))
	if strings.Contains(sql, "INSERT INTO") {
		t.Fatal("migration must not backfill evidence")
	}
	if strings.Contains(sql, "'LOADED'") || strings.Contains(sql, "IN_TRANSIT") {
		t.Fatal("migration must not infer evidence from shipment status")
	}
}
