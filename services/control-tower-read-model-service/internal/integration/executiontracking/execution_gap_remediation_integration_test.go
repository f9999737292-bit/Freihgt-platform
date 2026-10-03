//go:build integration

package executiontracking

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

func TestStagingExecutionSequenceGapRemediation(t *testing.T) {
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
	now := time.Date(2026, 10, 3, 9, 31, 47, 0, time.UTC)
	meta := domain.KafkaRecordMeta{Topic: "shipment.status.v1", Partition: 1, Offset: 42}

	operating := uuid.New()
	executionID := uuid.New()
	revisionR1 := uuid.New()
	revisionR2 := uuid.New()
	stopA := uuid.New()
	stopB := uuid.New()
	carrier := uuid.New()
	caseID := uuid.New()
	shipmentID := uuid.New()
	cargoID := uuid.New()

	created := executionPlan(t, operating, executionID, revisionR1, carrier, 1, now, []map[string]any{
		{"stop_id": stopA.String(), "ordinal": 0, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
	}, nil)
	if err := repo.ApplyRecord(ctx, created, meta, now); err != nil {
		t.Fatal(err)
	}

	meta.Offset = 43
	current := spacedEmptyActionStop(t, domain.EventRouteStopCurrent, operating, executionID, revisionR1, stopA, 2, now)
	if err := repo.ApplyRecord(ctx, current, meta, now); err != nil {
		t.Fatal(err)
	}
	view := mustGet(t, repo, executionID)
	if view.GapDetected || view.LastEventSequence != 2 || view.ActiveRevisionID != revisionR1 {
		t.Fatalf("spaced action_id left a gap: gap=%v seq=%d revision=%s", view.GapDetected, view.LastEventSequence, view.ActiveRevisionID)
	}

	meta.Offset = 54
	successor := executionPlan(t, operating, executionID, revisionR2, carrier, 4, now.Add(time.Second), []map[string]any{
		{"stop_id": stopB.String(), "ordinal": 0, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
	}, nil)
	if err := repo.ApplyRecord(ctx, successor, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if !view.GapDetected || view.ActiveRevisionID != revisionR1 || view.LastEventSequence != 2 {
		t.Fatalf("successor applied through a hole: gap=%v seq=%d revision=%s", view.GapDetected, view.LastEventSequence, view.ActiveRevisionID)
	}

	meta.Offset = 55
	superseded := planSuperseded(t, operating, executionID, revisionR1, revisionR2, 3, now.Add(2*time.Second))
	if err := repo.ApplyRecord(ctx, superseded, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.GapDetected || view.LastEventSequence != 4 || view.ActiveRevisionID != revisionR2 {
		t.Fatalf("gap did not drain: gap=%v seq=%d revision=%s", view.GapDetected, view.LastEventSequence, view.ActiveRevisionID)
	}
	if statusOf(view, stopA) != "SUPERSEDED" || statusOf(view, stopB) != "PLANNED" {
		t.Fatalf("revision projection old=%s new=%s", statusOf(view, stopA), statusOf(view, stopB))
	}

	meta.Offset = 56
	disposition := dispositionPayload(caseID, executionID, operating, stopA, shipmentID, cargoID, domain.EventCargoDispositionPending, "DAMAGE", "RETURN_TO_ORIGIN", "IN_RETURN_TRANSIT", 2, now)
	if err := repo.ApplyRecord(ctx, disposition, meta, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.delivery_disposition_projection WHERE execution_id=$1`, executionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("disposition count %d", count)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.delivery_disposition_inbox WHERE disposition_case_id=$1 AND processing_outcome='applied'`, caseID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("disposition inbox %d", count)
	}

	if err := repo.ApplyRecord(ctx, current, meta, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyRecord(ctx, successor, meta, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyRecord(ctx, disposition, meta, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.GapDetected || view.ActiveRevisionID != revisionR2 || view.LastEventSequence != 4 {
		t.Fatalf("replay changed projection gap=%v seq=%d revision=%s", view.GapDetected, view.LastEventSequence, view.ActiveRevisionID)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.execution_projection`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("replay created an execution root: %d", count)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.delivery_disposition_projection WHERE execution_id=$1`, executionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("disposition replay duplicated the case: %d", count)
	}

	meta.Offset = 99
	foreign := spacedEmptyActionStop(t, domain.EventRouteStopArrived, uuid.New(), uuid.New(), uuid.New(), uuid.New(), 1, now)
	if err := repo.ApplyRecord(ctx, foreign, meta, now); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.execution_projection`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("foreign stop created an execution root")
	}

	execFormats := uuid.New()
	revFormats := uuid.New()
	stopFormats := uuid.New()
	meta.Offset = 200
	formatPlan := executionPlan(t, operating, execFormats, revFormats, carrier, 1, now, []map[string]any{
		{"stop_id": stopFormats.String(), "ordinal": 0, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
	}, nil)
	if err := repo.ApplyRecord(ctx, formatPlan, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 201
	compact := []byte(`{"event_id":"` + uuid.NewString() + `","event_type":"` + domain.EventRouteStopCurrent + `","operating_tenant_id":"` + operating.String() + `","execution_id":"` + execFormats.String() + `","revision_id":"` + revFormats.String() + `","event_sequence":2,"stop_id":"` + stopFormats.String() + `","action_id":"","occurred_at":"` + now.Format(time.RFC3339Nano) + `"}`)
	if err := repo.ApplyRecord(ctx, compact, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 202
	nullAction := []byte(`{"event_id":"` + uuid.NewString() + `","event_type":"` + domain.EventRouteStopArrived + `","operating_tenant_id":"` + operating.String() + `","execution_id":"` + execFormats.String() + `","revision_id":"` + revFormats.String() + `","event_sequence":3,"stop_id":"` + stopFormats.String() + `","action_id":null,"occurred_at":"` + now.Format(time.RFC3339Nano) + `"}`)
	if err := repo.ApplyRecord(ctx, nullAction, meta, now); err != nil {
		t.Fatal(err)
	}
	formats := mustGet(t, repo, execFormats)
	if formats.GapDetected || formats.LastEventSequence != 3 || formats.ActiveRevisionID != revFormats {
		t.Fatalf("compact and null action_id gap=%v seq=%d", formats.GapDetected, formats.LastEventSequence)
	}
}

func spacedEmptyActionStop(t *testing.T, eventType string, operating, executionID, revisionID, stopID uuid.UUID, seq int, occurred time.Time) []byte {
	t.Helper()
	return []byte(`{"event_id":"` + uuid.NewString() + `","event_type":"` + eventType + `","operating_tenant_id":"` + operating.String() + `","execution_id":"` + executionID.String() + `","revision_id":"` + revisionID.String() + `","event_sequence":` + itoa(seq) + `,"stop_id":"` + stopID.String() + `","action_id": "","occurred_at":"` + occurred.Format(time.RFC3339Nano) + `"}`)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
