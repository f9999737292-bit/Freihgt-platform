//go:build integration

package executiontracking

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

func TestDeliveryDispositionReadModel(t *testing.T) {
	ctx := context.Background()
	pool := startExecutionPostgres(t)
	up, err := os.ReadFile(filepath.Join(migrationsDir(t), "000093_tms_delivery_disposition_v0_1.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewExecutionProjectionRepository(pool)
	now := time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC)
	operating := uuid.New()
	executionID := uuid.New()
	revisionID := uuid.New()
	caseID := uuid.New()
	stopID := uuid.New()
	shipmentID := uuid.New()
	cargoID := uuid.New()
	if _, err := pool.Exec(ctx, `
INSERT INTO control_tower.execution_projection (
  execution_id, operating_tenant_id, active_revision_id, current_stop_id, last_event_sequence, last_event_id, last_event_type, last_occurred_at, last_consumed_at, gap_detected, created_at, updated_at
) VALUES ($1,$2,$3,NULL,1,$4,'shipment.execution_plan.created',$5,$5,false,$5,$5)
`, executionID, operating, revisionID, uuid.New(), now); err != nil {
		t.Fatal(err)
	}
	meta := domain.KafkaRecordMeta{Topic: "shipment.status.v1", Partition: 0, Offset: 90}
	first := dispositionPayload(caseID, executionID, operating, stopID, shipmentID, cargoID, domain.EventDeliveryPartiallyRejected, "DAMAGE", "", "DISPOSITION_PENDING", 1, now)
	if err := repo.ApplyRecord(ctx, first, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 91
	pending := dispositionPayload(caseID, executionID, operating, stopID, shipmentID, cargoID, domain.EventCargoDispositionPending, "DAMAGE", "", "DISPOSITION_PENDING", 2, now)
	if err := repo.ApplyRecord(ctx, pending, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 92
	if err := repo.ApplyRecord(ctx, pending, meta, now); err != nil {
		t.Fatal(err)
	}
	target := uuid.New()
	meta.Offset = 93
	authorized := dispositionPayload(caseID, executionID, operating, stopID, shipmentID, cargoID, domain.EventCargoReturnAuthorized, "DAMAGE", "RETURN_TO_ORIGIN", "IN_RETURN_TRANSIT", 3, now.Add(time.Minute))
	var body map[string]any
	if err := json.Unmarshal(authorized, &body); err != nil {
		t.Fatal(err)
	}
	body["target_location_id"] = target.String()
	authorized, _ = json.Marshal(body)
	if err := repo.ApplyRecord(ctx, authorized, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 94
	stale := dispositionPayload(caseID, executionID, operating, stopID, shipmentID, cargoID, domain.EventCargoDispositionPending, "DAMAGE", "", "DISPOSITION_PENDING", 2, now)
	if err := repo.ApplyRecord(ctx, stale, meta, now); err != nil {
		t.Fatal(err)
	}
	var status, kind string
	var rejected int
	var active uuid.UUID
	if err := pool.QueryRow(ctx, `
SELECT status, COALESCE(disposition_type, ''), rejected_quantity::int
FROM control_tower.delivery_disposition_projection WHERE disposition_case_id=$1
`, caseID).Scan(&status, &kind, &rejected); err != nil {
		t.Fatal(err)
	}
	if status != "IN_RETURN_TRANSIT" || kind != "RETURN_TO_ORIGIN" || rejected != 2 {
		t.Fatalf("projection %s %s %d", status, kind, rejected)
	}
	if err := pool.QueryRow(ctx, `SELECT active_revision_id FROM control_tower.execution_projection WHERE execution_id=$1`, executionID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != revisionID {
		t.Fatal("disposition projection changed the execution revision")
	}
}

func dispositionPayload(caseID, executionID, operating, stopID, shipmentID, cargoID uuid.UUID, eventType, reason, kind, status string, sequence int, occurred time.Time) []byte {
	body := map[string]any{
		"event_id": uuid.NewString(), "event_type": eventType, "operating_tenant_id": operating.String(),
		"execution_id": executionID.String(), "source_execution_stop_id": stopID.String(),
		"shipment_id": shipmentID.String(), "cargo_id": cargoID.String(), "disposition_case_id": caseID.String(),
		"reason_code": reason, "accepted_quantity": 8, "rejected_quantity": 2, "uom": "PALLET",
		"disposition_type": kind, "status": status, "disposition_sequence": sequence,
		"occurred_at": occurred.Format(time.RFC3339Nano),
	}
	raw, _ := json.Marshal(body)
	return raw
}
