//go:build integration

package executiontracking

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

func TestSuccessorRevisionReadModel(t *testing.T) {
	ctx := context.Background()
	pool := startExecutionPostgres(t)
	repo := repository.NewExecutionProjectionRepository(pool)
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	meta := domain.KafkaRecordMeta{Topic: "shipment.status.v1", Partition: 1, Offset: 1}

	operating := uuid.New()
	executionID := uuid.New()
	revisionR1 := uuid.New()
	revisionR2 := uuid.New()
	stopCompleted := uuid.New()
	stopOpen := uuid.New()
	stopNext := uuid.New()
	carrier := uuid.New()
	shipA := uuid.New()
	cargoA := uuid.New()

	created := executionPlan(t, operating, executionID, revisionR1, carrier, 1, now, []map[string]any{
		{"stop_id": stopCompleted.String(), "ordinal": 0, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "COMPLETED"},
		{"stop_id": stopOpen.String(), "ordinal": 1, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
	}, nil)
	if err := repo.ApplyRecord(ctx, created, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset++
	current := stopEvent(t, domain.EventRouteStopCurrent, operating, executionID, revisionR1, stopOpen, 2, now.Add(time.Minute))
	if err := repo.ApplyRecord(ctx, current, meta, now); err != nil {
		t.Fatal(err)
	}

	meta.Offset++
	superseded := planSuperseded(t, operating, executionID, revisionR1, revisionR2, 3, now.Add(2*time.Minute))
	if err := repo.ApplyRecord(ctx, superseded, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset++
	successor := executionPlan(t, operating, executionID, revisionR2, carrier, 4, now.Add(3*time.Minute), []map[string]any{
		{"stop_id": stopNext.String(), "ordinal": 0, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
	}, []map[string]any{
		{"action_id": uuid.NewString(), "stop_id": stopNext.String(), "action_type": "DELIVERY", "shipment_id": shipA.String(), "cargo_id": cargoA.String(), "status": "PENDING"},
	})
	if err := repo.ApplyRecord(ctx, successor, meta, now); err != nil {
		t.Fatal(err)
	}
	view := mustGet(t, repo, executionID)
	if view.ActiveRevisionID != revisionR2 || view.CurrentStopID == nil || *view.CurrentStopID != stopNext {
		t.Fatalf("active %s current %v", view.ActiveRevisionID, view.CurrentStopID)
	}
	if statusOf(view, stopCompleted) != "COMPLETED" || revisionOf(view, stopCompleted) != revisionR1 {
		t.Fatal("completed history was not retained")
	}
	if statusOf(view, stopOpen) != "SUPERSEDED" || revisionOf(view, stopOpen) != revisionR1 {
		t.Fatal("open stop was not superseded on the old revision")
	}
	if statusOf(view, stopNext) != "PLANNED" || revisionOf(view, stopNext) != revisionR2 {
		t.Fatal("new stop missing")
	}

	meta.Offset++
	if err := repo.ApplyRecord(ctx, superseded, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset++
	if err := repo.ApplyRecord(ctx, successor, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if len(view.Stops) != 3 || view.ActiveRevisionID != revisionR2 || view.CurrentStopID == nil || *view.CurrentStopID != stopNext {
		t.Fatalf("duplicate changed projection stops=%d active=%s", len(view.Stops), view.ActiveRevisionID)
	}

	meta.Offset++
	lateArrived := stopEvent(t, domain.EventRouteStopArrived, operating, executionID, revisionR1, stopOpen, 5, now.Add(4*time.Minute))
	if err := repo.ApplyRecord(ctx, lateArrived, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset++
	lateCurrent := stopEvent(t, domain.EventRouteStopCurrent, operating, executionID, revisionR1, stopOpen, 6, now.Add(5*time.Minute))
	if err := repo.ApplyRecord(ctx, lateCurrent, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset++
	approach := []byte(`{"eventId":"` + uuid.NewString() + `","eventType":"tracking.stop.approaching","operatingTenantId":"` + operating.String() + `","executionId":"` + executionID.String() + `","revisionId":"` + revisionR1.String() + `","executionStopId":"` + stopOpen.String() + `","occurredAt":"` + now.Format(time.RFC3339Nano) + `","distanceMeters":40}`)
	if err := repo.ApplyRecord(ctx, approach, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.ActiveRevisionID != revisionR2 || view.CurrentStopID == nil || *view.CurrentStopID != stopNext || statusOf(view, stopOpen) != "SUPERSEDED" {
		t.Fatalf("late old revision reactivated active=%s current=%v open=%s", view.ActiveRevisionID, view.CurrentStopID, statusOf(view, stopOpen))
	}

	t.Run("created before superseded", func(t *testing.T) {
		executionID := uuid.New()
		revisionR1 := uuid.New()
		revisionR2 := uuid.New()
		stopOpen := uuid.New()
		stopNext := uuid.New()
		created := executionPlan(t, operating, executionID, revisionR1, carrier, 1, now, []map[string]any{
			{"stop_id": stopOpen.String(), "ordinal": 0, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
		}, nil)
		meta := domain.KafkaRecordMeta{Topic: "shipment.status.v1", Partition: 2, Offset: 1}
		if err := repo.ApplyRecord(ctx, created, meta, now); err != nil {
			t.Fatal(err)
		}
		meta.Offset++
		if err := repo.ApplyRecord(ctx, stopEvent(t, domain.EventRouteStopCurrent, operating, executionID, revisionR1, stopOpen, 2, now), meta, now); err != nil {
			t.Fatal(err)
		}
		meta.Offset++
		successor := executionPlan(t, operating, executionID, revisionR2, carrier, 4, now.Add(time.Minute), []map[string]any{
			{"stop_id": stopNext.String(), "ordinal": 0, "stop_role": "END", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
		}, nil)
		if err := repo.ApplyRecord(ctx, successor, meta, now); err != nil {
			t.Fatal(err)
		}
		view := mustGet(t, repo, executionID)
		if view.ActiveRevisionID != revisionR1 || (view.CurrentStopID != nil && *view.CurrentStopID != stopOpen) {
			t.Fatalf("gap applied early active=%s", view.ActiveRevisionID)
		}
		meta.Offset++
		if err := repo.ApplyRecord(ctx, planSuperseded(t, operating, executionID, revisionR1, revisionR2, 3, now.Add(2*time.Minute)), meta, now); err != nil {
			t.Fatal(err)
		}
		view = mustGet(t, repo, executionID)
		if view.ActiveRevisionID != revisionR2 || view.CurrentStopID == nil || *view.CurrentStopID != stopNext {
			t.Fatalf("held successor active=%s current=%v", view.ActiveRevisionID, view.CurrentStopID)
		}
		if statusOf(view, stopOpen) != "SUPERSEDED" || statusOf(view, stopNext) != "PLANNED" {
			t.Fatalf("held order open=%s next=%s", statusOf(view, stopOpen), statusOf(view, stopNext))
		}
	})
}

func executionPlan(t *testing.T, operating, executionID, revisionID, carrier uuid.UUID, seq int, now time.Time, stops, actions []map[string]any) []byte {
	t.Helper()
	if actions == nil {
		actions = []map[string]any{}
	}
	body := map[string]any{
		"event_id": uuid.NewString(), "event_type": domain.EventExecutionPlanCreated,
		"operating_tenant_id": operating.String(), "execution_id": executionID.String(),
		"revision_id": revisionID.String(), "revision_version": 1, "event_sequence": seq,
		"occurred_at": now.Format(time.RFC3339Nano), "carrier_company_id": carrier.String(),
		"stops": stops, "actions": actions,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func planSuperseded(t *testing.T, operating, executionID, oldRevision, newRevision uuid.UUID, seq int, now time.Time) []byte {
	t.Helper()
	body := map[string]any{
		"event_id": uuid.NewString(), "event_type": domain.EventExecutionPlanSuperseded,
		"operating_tenant_id": operating.String(), "execution_id": executionID.String(),
		"revision_id": oldRevision.String(), "old_revision_id": oldRevision.String(), "new_revision_id": newRevision.String(),
		"event_sequence": seq, "occurred_at": now.Format(time.RFC3339Nano), "reason_code": "REMAINING_ROUTE_CHANGED",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func statusOf(view repository.ExecutionView, stopID uuid.UUID) string {
	for _, stop := range view.Stops {
		if stop.ExecutionStopID == stopID {
			return stop.Status
		}
	}
	return ""
}

func revisionOf(view repository.ExecutionView, stopID uuid.UUID) uuid.UUID {
	for _, stop := range view.Stops {
		if stop.ExecutionStopID == stopID {
			return stop.RevisionID
		}
	}
	return uuid.Nil
}
