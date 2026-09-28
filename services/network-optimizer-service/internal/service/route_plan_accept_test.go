package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

func TestNLO04CAcceptFreezesPlanAndReplays(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	created := evaluatePlan(t, w, body, "eval-accept")
	before := created["evaluation_fingerprint"]
	stops := created["stops"].([]any)
	raw := `{"version":1}`
	first := decidePlan(t, w, created["id"].(string), raw, "accept-key", true)
	if first["status"] != routeplan.StatusAccepted || first["accepted_at"] == nil || first["execution_supported"] != false {
		t.Fatalf("%v", first)
	}
	if first["evaluation_fingerprint"] != before || len(first["stops"].([]any)) != len(stops) {
		t.Fatal("accept changed plan structure")
	}
	if first["stops"].([]any)[0].(map[string]any)["id"] != stops[0].(map[string]any)["id"] {
		t.Fatal("accept rewrote stops")
	}
	second := decidePlan(t, w, created["id"].(string), raw, "accept-key", true)
	if second["id"] != first["id"] || second["accepted_at"] != first["accepted_at"] {
		t.Fatalf("replay %+v %+v", first["accepted_at"], second["accepted_at"])
	}
	events, err := w.store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	accepted := 0
	for _, event := range events {
		if event.EventName == routePlanAcceptedEvent && event.AggregateID.String() == first["id"] {
			accepted++
		}
		if event.EventName == "shipment.status.changed" {
			t.Fatal("accept emitted a shipment status event")
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted events %d", accepted)
	}
	_, err = w.svc.AcceptRoutePlan(context.Background(), w.actor(), "accept-key", uuid.MustParse(first["id"].(string)), []byte(`{"version":2}`))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Code != apperrors.CodeConflict {
		t.Fatal(err)
	}
	if _, err := w.svc.AcceptRoutePlan(context.Background(), serviceActor(uuid.New(), nil), "other", uuid.MustParse(first["id"].(string)), []byte(raw)); err == nil {
		t.Fatal("foreign tenant accepted the plan")
	}
}

func TestNLO04CStaleAndIndeterminateActivation(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	created := evaluatePlan(t, w, body, "eval-stale")
	statusBefore := src.execution.ShipmentStatus
	src.execution.ShipmentVersion = 99
	_, err := w.svc.AcceptRoutePlan(context.Background(), w.actor(), "stale", uuid.MustParse(created["id"].(string)), []byte(`{"version":1}`))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Code != apperrors.CodeConflict || app.Details["reason"] != routeplan.ReasonPlanStale {
		t.Fatal(err)
	}
	if src.execution.ShipmentStatus != statusBefore {
		t.Fatal("stale accept changed shipment status")
	}
	src.execution.ShipmentVersion = 3
	accepted := decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-indeterminate", true)
	if accepted["result_status"] != routeplan.ResultIndeterminate {
		t.Fatalf("%v", accepted["result_status"])
	}
	_, err = w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-indeterminate", uuid.MustParse(created["id"].(string)), []byte(`{"version":1}`))
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonServiceDurationUnknown {
		t.Fatal(err)
	}
	if src.execution.ShipmentStatus != "IN_TRANSIT" {
		t.Fatalf("status %s", src.execution.ShipmentStatus)
	}
	if len(w.store.TestingActivations()) != 0 {
		t.Fatal("indeterminate activation wrote a row")
	}
}

func TestNLO04CActivateIsIdempotentAndDoesNotMoveShipment(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	created := evaluatePlan(t, w, body, "eval-activate")
	makeActivatable(t, w, uuid.MustParse(created["id"].(string)))
	decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-live", true)
	first := decidePlan(t, w, created["id"].(string), `{"version":1}`, "activate-live", false)
	activation := first["activation"].(map[string]any)
	plan := first["plan"].(map[string]any)
	if activation["status"] != routeplan.ActivationPending || activation["version"] != float64(1) || activation["execution_id"] != nil || activation["execution_revision_id"] != nil || plan["execution_supported"] != false || plan["status"] != routeplan.StatusAccepted {
		t.Fatalf("%v", first)
	}
	for _, row := range w.store.TestingActivations() {
		if row.EffectiveShipmentID != nil || row.ExecutionID != nil || row.ExecutionRevisionID != nil || row.Status != routeplan.ActivationPending {
			t.Fatalf("pending row claimed execution %+v", row)
		}
	}
	if src.execution.ShipmentStatus != "IN_TRANSIT" {
		t.Fatalf("shipment status moved to %s", src.execution.ShipmentStatus)
	}
	second := decidePlan(t, w, created["id"].(string), `{"version":1}`, "activate-live", false)
	if second["activation"].(map[string]any)["id"] != activation["id"] {
		t.Fatal("same key created another activation")
	}
	third := decidePlan(t, w, created["id"].(string), `{"version":1}`, "activate-again", false)
	if third["activation"].(map[string]any)["id"] != activation["id"] {
		t.Fatal("new key created another activation")
	}
	if len(w.store.TestingActivations()) != 1 {
		t.Fatalf("activations %d", len(w.store.TestingActivations()))
	}
	events, _ := w.store.ListOutbox(context.Background())
	requested := 0
	for _, event := range events {
		if event.EventName == "network.route_plan.superseded" || event.EventName == "network.route_plan.execution_linked" {
			t.Fatalf("unexpected event %s", event.EventName)
		}
		if event.EventName == routePlanActivationRequestedEvent {
			requested++
			if !strings.Contains(string(event.Payload), `"status":"PENDING_EXECUTION"`) {
				t.Fatalf("activation event status %s", event.Payload)
			}
		}
	}
	if requested != 1 {
		t.Fatalf("activation events %d", requested)
	}
}

func TestNLO04CRoutingExpiryAndShipmentStatus(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	created := evaluatePlan(t, w, body, "eval-expiry")
	id := uuid.MustParse(created["id"].(string))
	makeActivatable(t, w, id)
	expirePlan(t, w, id)
	decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-expired", true)
	_, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-expired", id, []byte(`{"version":1}`))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ResultRoutingDown {
		t.Fatal(err)
	}

	fresh := evaluatePlan(t, w, body, "eval-status")
	src.execution.ShipmentStatus = "UNLOADING"
	replacement := evaluatePlan(t, w, body, "eval-status-fingerprint")
	copyContextFingerprint(t, w, uuid.MustParse(fresh["id"].(string)), replacement["context_fingerprint"].(string))
	makeActivatable(t, w, uuid.MustParse(fresh["id"].(string)))
	decidePlan(t, w, fresh["id"].(string), `{"version":1}`, "accept-status", true)
	_, err = w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-status", uuid.MustParse(fresh["id"].(string)), []byte(`{"version":1}`))
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonShipmentStatusIneligible {
		t.Fatal(err)
	}
	if src.execution.ShipmentStatus != "UNLOADING" {
		t.Fatalf("status reset to %s", src.execution.ShipmentStatus)
	}
}

func TestNLO04CSuccessorPendingPreservesPredecessor(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	firstDoc := evaluatePlan(t, w, body, "eval-prev")
	secondDoc := evaluatePlan(t, w, body, "eval-next")
	firstID := uuid.MustParse(firstDoc["id"].(string))
	secondID := uuid.MustParse(secondDoc["id"].(string))
	makeActivatable(t, w, firstID)
	makeActivatable(t, w, secondID)
	linkSuccessor(t, w, secondID, firstID)
	decidePlan(t, w, firstDoc["id"].(string), `{"version":1}`, "accept-prev", true)
	decidePlan(t, w, secondDoc["id"].(string), `{"version":1}`, "accept-next", true)
	decidePlan(t, w, firstDoc["id"].(string), `{"version":1}`, "activate-prev", false)
	executionID := uuid.New()
	revisionID := uuid.New()
	shipmentID := src.execution.ShipmentID
	var linked repository.RoutePlanActivationRow
	for _, row := range w.store.TestingActivations() {
		if row.RoutePlanID == firstID {
			linked = row
		}
	}
	linked.Status = routeplan.ActivationLinked
	linked.ExecutionID = &executionID
	linked.ExecutionRevisionID = &revisionID
	linked.EffectiveShipmentID = &shipmentID
	if err := w.store.TestingReplaceActivation(linked); err != nil {
		t.Fatal(err)
	}
	pending := decidePlan(t, w, secondDoc["id"].(string), `{"version":1}`, "activate-next", false)
	activation := pending["activation"].(map[string]any)
	if activation["status"] != routeplan.ActivationPending || activation["execution_id"] != nil {
		t.Fatalf("%v", activation)
	}
	prev, err := w.svc.GetRoutePlan(context.Background(), w.actor(), firstID)
	if err != nil {
		t.Fatal(err)
	}
	var prevDoc map[string]any
	if err := json.Unmarshal(prev.Body, &prevDoc); err != nil {
		t.Fatal(err)
	}
	if prevDoc["status"] != routeplan.StatusAccepted || src.execution.ShipmentStatus != "IN_TRANSIT" {
		t.Fatalf("status %v shipment %s", prevDoc["status"], src.execution.ShipmentStatus)
	}
	rows := w.store.TestingActivations()
	if len(rows) != 2 {
		t.Fatalf("activations %d", len(rows))
	}
	for _, row := range rows {
		if row.RoutePlanID == firstID {
			if row.Status != routeplan.ActivationLinked || row.EffectiveShipmentID == nil || row.ExecutionID == nil {
				t.Fatalf("predecessor lost %+v", row)
			}
		}
		if row.RoutePlanID == secondID && (row.Status != routeplan.ActivationPending || row.EffectiveShipmentID != nil) {
			t.Fatalf("successor claimed execution %+v", row)
		}
	}
	events, _ := w.store.ListOutbox(context.Background())
	for _, event := range events {
		if event.EventName == "network.route_plan.superseded" || event.EventName == "network.route_plan.execution_linked" {
			t.Fatalf("unexpected event %s", event.EventName)
		}
	}
}

func TestNLO04CDepotStartActivationFailsClosedWithoutShipmentStatus(t *testing.T) {
	w, _, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	location := uuid.New()
	lat, lon := 55.5, 37.5
	cap := domain.Capacity{
		ID: uuid.New(), OwnerTenantID: w.carrier, LocationID: &location, Latitude: &lat, Longitude: &lon,
		AvailableFrom: w.at, AvailableUntil: w.at.Add(time.Hour), Source: domain.SourceManual,
		VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 2,
		PayloadRemainingKg: f64(18000), VolumeRemainingM3: f64(70),
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertCapacity(context.Background(), cap)
	}); err != nil {
		t.Fatal(err)
	}
	body := `{"planning_mode":"DEPOT_START","capacity_id":"` + cap.ID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	created := evaluatePlan(t, w, body, "eval-depot-accept")
	makeActivatable(t, w, uuid.MustParse(created["id"].(string)))
	accepted := decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-depot", true)
	if accepted["status"] != routeplan.StatusAccepted || accepted["shipment_id"] != nil {
		t.Fatalf("%v", accepted)
	}
	_, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-depot", uuid.MustParse(created["id"].(string)), []byte(`{"version":1}`))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonShipmentStatusIneligible {
		t.Fatal(err)
	}
	if len(w.store.TestingActivations()) != 0 {
		t.Fatal("depot activation wrote a row without a shipment status")
	}
}

func TestNLO04CAcceptDeniesInvalidStateVersionAndStaleContext(t *testing.T) {
	w, src, load := newFill(t)
	counter := &countingRoute{}
	w.svc.UseRouting(counter)
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	created := evaluatePlan(t, w, body, "eval-guards")
	id := uuid.MustParse(created["id"].(string))
	calls := counter.calls.Load()
	fingerprint := created["evaluation_fingerprint"]
	contextFP := created["context_fingerprint"]
	decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-guards", true)
	if counter.calls.Load() != calls {
		t.Fatalf("accept reoptimized %d -> %d", calls, counter.calls.Load())
	}
	accepted := decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-guards-replay", true)
	if accepted["evaluation_fingerprint"] != fingerprint || accepted["context_fingerprint"] != contextFP {
		t.Fatal("accept changed fingerprints")
	}
	legs := accepted["legs"].([]any)
	if len(legs) == 0 || legs[0].(map[string]any)["id"] == nil {
		t.Fatal("legs missing")
	}

	_, err := w.svc.AcceptRoutePlan(context.Background(), w.actor(), "", id, []byte(`{"version":1}`))
	if err == nil {
		t.Fatal("missing key accepted")
	}
	graph := loadPlan(t, w, id)
	graph.Plan.Version = 2
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
	_, err = w.svc.AcceptRoutePlan(context.Background(), w.actor(), "accept-old-version", id, []byte(`{"version":1}`))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonPlanStale {
		t.Fatal(err)
	}
	graph.Plan.Version = 1
	graph.Plan.Status = routeplan.StatusSuperseded
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
	_, err = w.svc.AcceptRoutePlan(context.Background(), w.actor(), "accept-superseded", id, []byte(`{"version":1}`))
	if !errors.As(err, &app) || app.Code != apperrors.CodeValidation {
		t.Fatal(err)
	}

	fresh, src2, load2 := newFill(t)
	fresh.svc.UseRouting(&countingRoute{})
	fresh.svc.SetClock(func() time.Time { return fresh.at })
	second := evaluatePlan(t, fresh, currentTripBody(src2.execution.ShipmentID, load2.ID), "eval-stale-context")
	later := fresh.at.Add(time.Minute)
	src2.position.RecordedAt = &later
	_, err = fresh.svc.AcceptRoutePlan(context.Background(), fresh.actor(), "stale-context", uuid.MustParse(second["id"].(string)), []byte(`{"version":1}`))
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonPlanStale {
		t.Fatal(err)
	}
	reloaded, err := fresh.svc.GetRoutePlan(context.Background(), fresh.actor(), uuid.MustParse(second["id"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(reloaded.Body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["status"] != routeplan.StatusEvaluated || doc["context_fingerprint"] != second["context_fingerprint"] {
		t.Fatalf("stale accept mutated %+v", doc["status"])
	}
	load2.Version = 2
	replaceLoad(t, fresh, load2)
	src2.position.RecordedAt = &fresh.at
	_, err = fresh.svc.AcceptRoutePlan(context.Background(), fresh.actor(), "stale-load", uuid.MustParse(second["id"].(string)), []byte(`{"version":1}`))
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonPlanStale {
		t.Fatal(err)
	}
}

func TestNLO04CActivateDeniesInvalidStateAndIsolatesTenant(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	created := evaluatePlan(t, w, currentTripBody(src.execution.ShipmentID, load.ID), "eval-activate-deny")
	id := uuid.MustParse(created["id"].(string))
	_, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "too-soon", id, []byte(`{"version":1}`))
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonActivationNotAccepted {
		t.Fatal(err)
	}
	makeActivatable(t, w, id)
	decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-before-version", true)
	graph := loadPlan(t, w, id)
	graph.Plan.Version = 2
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
	_, err = w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-old-version", id, []byte(`{"version":1}`))
	if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonPlanStale {
		t.Fatal(err)
	}
	graph.Plan.Version = 1
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
	_, err = w.svc.ActivateRoutePlan(context.Background(), serviceActor(uuid.New(), nil), "foreign-activate", id, []byte(`{"version":1}`))
	var foreign *apperrors.AppError
	if !errors.As(err, &foreign) || foreign.Code != apperrors.CodeNotFound {
		t.Fatal(err)
	}
}

func TestNLO04CConcurrentActivateAndSuccessor(t *testing.T) {
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	body := currentTripBody(src.execution.ShipmentID, load.ID)
	created := evaluatePlan(t, w, body, "eval-race")
	id := uuid.MustParse(created["id"].(string))
	makeActivatable(t, w, id)
	decidePlan(t, w, created["id"].(string), `{"version":1}`, "accept-race", true)
	const n = 8
	var wg sync.WaitGroup
	ids := make([]string, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			result, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "race-"+string(rune('a'+i)), id, []byte(`{"version":1}`))
			errs[i] = err
			if err == nil {
				var doc map[string]any
				_ = json.Unmarshal(result.Body, &doc)
				ids[i], _ = doc["activation"].(map[string]any)["id"].(string)
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d: %v", i, err)
		}
		if ids[i] == "" || ids[i] != ids[0] {
			t.Fatalf("activation ids %v", ids)
		}
	}
	rows := w.store.TestingActivations()
	if len(rows) != 1 || rows[0].Status != routeplan.ActivationPending {
		t.Fatalf("rows %+v", rows)
	}
	executionID := uuid.New()
	revisionID := uuid.New()
	shipmentID := src.execution.ShipmentID
	linked := rows[0]
	linked.Status = routeplan.ActivationLinked
	linked.ExecutionID = &executionID
	linked.ExecutionRevisionID = &revisionID
	linked.EffectiveShipmentID = &shipmentID
	if err := w.store.TestingReplaceActivation(linked); err != nil {
		t.Fatal(err)
	}

	next := evaluatePlan(t, w, body, "eval-race-b")
	nextID := uuid.MustParse(next["id"].(string))
	makeActivatable(t, w, nextID)
	linkSuccessor(t, w, nextID, id)
	decidePlan(t, w, next["id"].(string), `{"version":1}`, "accept-b", true)
	var succWG sync.WaitGroup
	succIDs := make([]string, 2)
	succErr := make([]error, 2)
	succWG.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer succWG.Done()
			result, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "succ-"+string(rune('a'+i)), nextID, []byte(`{"version":1}`))
			succErr[i] = err
			if err == nil {
				var doc map[string]any
				_ = json.Unmarshal(result.Body, &doc)
				succIDs[i], _ = doc["activation"].(map[string]any)["id"].(string)
			}
		}(i)
	}
	succWG.Wait()
	for i, err := range succErr {
		if err != nil || succIDs[i] == "" || succIDs[i] != succIDs[0] {
			t.Fatalf("successor %d %v ids %v", i, err, succIDs)
		}
	}
	prev, err := w.svc.GetRoutePlan(context.Background(), w.actor(), id)
	if err != nil {
		t.Fatal(err)
	}
	var prevDoc map[string]any
	if err := json.Unmarshal(prev.Body, &prevDoc); err != nil {
		t.Fatal(err)
	}
	effective := 0
	pendingSuccessors := 0
	for _, row := range w.store.TestingActivations() {
		if row.EffectiveShipmentID != nil {
			effective++
			if row.RoutePlanID != id || row.Status != routeplan.ActivationLinked {
				t.Fatalf("effective row %+v", row)
			}
		}
		if row.RoutePlanID == nextID && row.Status == routeplan.ActivationPending {
			pendingSuccessors++
		}
	}
	if prevDoc["status"] != routeplan.StatusAccepted || effective != 1 || pendingSuccessors != 1 || src.execution.ShipmentStatus != "IN_TRANSIT" {
		t.Fatalf("status %v effective %d pending %d shipment %s", prevDoc["status"], effective, pendingSuccessors, src.execution.ShipmentStatus)
	}
}

func currentTripBody(shipment, load uuid.UUID) string {
	return `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + shipment.String() + `","candidate_load_ids":["` + load.String() + `"]}`
}

func decidePlan(t *testing.T, w *world, id, raw, key string, accept bool) map[string]any {
	t.Helper()
	var result Result
	var err error
	planID := uuid.MustParse(id)
	if accept {
		result, err = w.svc.AcceptRoutePlan(context.Background(), w.actor(), key, planID, []byte(raw))
	} else {
		result, err = w.svc.ActivateRoutePlan(context.Background(), w.actor(), key, planID, []byte(raw))
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusOK {
		t.Fatalf("status %d %s", result.Status, result.Body)
	}
	var doc map[string]any
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func makeActivatable(t *testing.T, w *world, id uuid.UUID) {
	t.Helper()
	graph := loadPlan(t, w, id)
	graph.Plan.ResultStatus = routeplan.ResultFeasible
	graph.Plan.ReasonCodes = []string{}
	seconds := 60
	for i := range graph.Stops {
		if graph.Stops[i].StopRole == routeplan.RoleCargo {
			graph.Stops[i].ServiceDurationSeconds = &seconds
		}
	}
	expires := w.at.Add(time.Hour)
	for i := range graph.Legs {
		graph.Legs[i].ExpiresAt = expires
	}
	for i := range graph.Snapshots {
		if graph.Snapshots[i].CompatibilityStatus == "INDETERMINATE" {
			graph.Snapshots[i].CompatibilityStatus = "COMPATIBLE"
		}
	}
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
}

func expirePlan(t *testing.T, w *world, id uuid.UUID) {
	t.Helper()
	graph := loadPlan(t, w, id)
	for i := range graph.Legs {
		graph.Legs[i].ExpiresAt = w.at.Add(-time.Minute)
	}
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
}

func copyContextFingerprint(t *testing.T, w *world, id uuid.UUID, fingerprint string) {
	t.Helper()
	graph := loadPlan(t, w, id)
	graph.Plan.ContextFingerprint = fingerprint
	for i := range graph.Dependencies {
		if graph.Dependencies[i].DependencyKind == "CURRENT_TRIP_CONTEXT" {
			graph.Dependencies[i].Fingerprint = fingerprint
		}
	}
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
}

func linkSuccessor(t *testing.T, w *world, id, previous uuid.UUID) {
	t.Helper()
	graph := loadPlan(t, w, id)
	graph.Plan.SupersedesPlanID = &previous
	if err := w.store.TestingReplaceRoutePlan(graph); err != nil {
		t.Fatal(err)
	}
}

func loadPlan(t *testing.T, w *world, id uuid.UUID) repository.RoutePlanGraph {
	t.Helper()
	var graph repository.RoutePlanGraph
	err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		var getErr error
		graph, getErr = tx.GetRoutePlan(context.Background(), w.carrier, id)
		return getErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}
