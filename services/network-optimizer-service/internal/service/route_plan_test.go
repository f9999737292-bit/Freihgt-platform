package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

type countingRoute struct{ calls atomic.Int64 }

func (c *countingRoute) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	c.calls.Add(1)
	return routing.RouteResult{
		Provider: "script", DistanceM: 1200, DurationSeconds: 90,
		CalculatedAt: time.Unix(10, 0).UTC(), ExpiresAt: time.Unix(20, 0).UTC(),
	}, nil
}

func (c *countingRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func TestNLO04BEvaluateCurrentTripAndReplay(t *testing.T) {
	w, src, load := newFill(t)
	counter := &countingRoute{}
	w.svc.UseRouting(counter)
	w.svc.SetClock(func() time.Time { return w.at })
	body := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + src.execution.ShipmentID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	first := evaluatePlan(t, w, body, "same-key")
	if first["status"] != "EVALUATED" || first["execution_supported"] != false || first["result_status"] != "INDETERMINATE_PLAN_FOUND" {
		t.Fatalf("%v", first)
	}
	reasons, _ := first["reason_codes"].([]any)
	if !jsonContains(reasons, "SERVICE_DURATION_UNKNOWN") {
		t.Fatalf("reasons %v", reasons)
	}
	stops := first["stops"].([]any)
	start := stops[0].(map[string]any)
	if start["stop_role"] != "START" || start["point_kind"] != "POSITION_ANCHOR" || start["location_id"] != nil {
		t.Fatalf("start %+v", start)
	}
	if start["latitude"] == nil || start["point_observed_at"] == nil {
		t.Fatalf("tracking anchor %+v", start)
	}
	end := stops[len(stops)-1].(map[string]any)
	if end["stop_role"] != "END" || end["point_kind"] != "CANONICAL_LOCATION" {
		t.Fatalf("end %+v", end)
	}
	actions := end["actions"].([]any)
	sawDelivery := false
	for _, raw := range actions {
		action := raw.(map[string]any)
		if action["subject_type"] == "SHIPMENT_CARGO" && action["action_type"] != "DELIVERY" {
			t.Fatalf("synthetic pickup %+v", action)
		}
		if action["subject_type"] == "SHIPMENT_CARGO" && action["action_type"] == "DELIVERY" {
			sawDelivery = true
		}
	}
	if !sawDelivery {
		t.Fatal("missing onboard delivery")
	}
	calls := counter.calls.Load()
	second := evaluatePlan(t, w, body, "same-key")
	if second["id"] != first["id"] || counter.calls.Load() != calls {
		t.Fatalf("replay id %v calls %d->%d", second["id"], calls, counter.calls.Load())
	}
	events, err := w.store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evaluated := 0
	for _, event := range events {
		if event.EventName == routePlanEvaluatedEvent && event.AggregateID.String() == first["id"] {
			evaluated++
		}
	}
	if evaluated != 1 {
		t.Fatalf("events %d", evaluated)
	}
	_, err = w.svc.EvaluateRoutePlan(context.Background(), w.actor(), "same-key", RoutePlanCommand{
		PlanningMode: "CURRENT_TRIP", ShipmentID: &src.execution.ShipmentID, CandidateLoadIDs: []uuid.UUID{uuid.New()},
		Raw: []byte(body + " "),
	})
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Code != apperrors.CodeConflict {
		t.Fatal(err)
	}
	got, err := w.svc.GetRoutePlan(context.Background(), w.actor(), uuid.MustParse(first["id"].(string)))
	if err != nil || got.Status != http.StatusOK {
		t.Fatal(err)
	}
	if _, err := w.svc.GetRoutePlan(context.Background(), serviceActor(uuid.New(), nil), uuid.MustParse(first["id"].(string))); err == nil {
		t.Fatal("foreign tenant read succeeded")
	}
}

func TestNLO04BDepotStartAndSecurity(t *testing.T) {
	w, _, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
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
	doc := evaluatePlan(t, w, body, "depot")
	stops := doc["stops"].([]any)
	start := stops[0].(map[string]any)
	if start["point_kind"] != "CANONICAL_LOCATION" || start["location_id"] != location.String() {
		t.Fatalf("depot start %+v", start)
	}
	if _, err := w.svc.EvaluateRoutePlan(context.Background(), serviceActor(uuid.New(), nil), "foreign-cap", RoutePlanCommand{
		PlanningMode: "DEPOT_START", CapacityID: &cap.ID, CandidateLoadIDs: []uuid.UUID{load.ID},
		Raw: []byte(body),
	}); err == nil {
		t.Fatal("foreign capacity visible")
	}
	hidden := load
	hidden.ID = uuid.New()
	hidden.SourceID = uuid.New()
	hidden.VisibilityScope = domain.VisPrivate
	hidden.CrossShipperConsolidationAllowed = false
	hidden.ConsolidationAllowed = false
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertLoad(context.Background(), hidden)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.EvaluateRoutePlan(context.Background(), w.actor(), "hidden", RoutePlanCommand{
		PlanningMode: "DEPOT_START", CapacityID: &cap.ID, CandidateLoadIDs: []uuid.UUID{hidden.ID},
		Raw: []byte(`{"planning_mode":"DEPOT_START","capacity_id":"` + cap.ID.String() + `","candidate_load_ids":["` + hidden.ID.String() + `"]}`),
	}); err == nil {
		t.Fatal("private load visible")
	}
}

func TestNLO04BIdempotencyAndRace(t *testing.T) {
	w, src, load := newFill(t)
	counter := &countingRoute{}
	w.svc.UseRouting(counter)
	body := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + src.execution.ShipmentID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	var cmd RoutePlanCommand
	if err := json.Unmarshal([]byte(body), &cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Raw = []byte(body)
	lat, lon := 55.4, 37.4
	cap := domain.Capacity{
		ID: uuid.New(), OwnerTenantID: w.carrier, Latitude: &lat, Longitude: &lon,
		AvailableFrom: w.at, AvailableUntil: w.at.Add(time.Hour), Source: domain.SourceManual,
		VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
		PayloadRemainingKg: f64(15000), VolumeRemainingM3: f64(50),
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertCapacity(context.Background(), cap)
	}); err != nil {
		t.Fatal(err)
	}
	depotBody := `{"planning_mode":"DEPOT_START","capacity_id":"` + cap.ID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	var depot RoutePlanCommand
	if err := json.Unmarshal([]byte(depotBody), &depot); err != nil {
		t.Fatal(err)
	}
	depot.Raw = []byte(depotBody)
	var wg sync.WaitGroup
	ids := make([]string, 2)
	errs := make([]error, 2)
	wg.Add(4)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			result, err := w.svc.EvaluateRoutePlan(context.Background(), w.actor(), "race-key", cmd)
			errs[i] = err
			if err == nil {
				var doc map[string]any
				_ = json.Unmarshal(result.Body, &doc)
				ids[i], _ = doc["id"].(string)
			}
		}(i)
	}
	go func() {
		defer wg.Done()
		_, _ = w.svc.EvaluateRoutePlan(context.Background(), w.actor(), "race-depot", depot)
	}()
	go func() {
		defer wg.Done()
		_, _ = w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{ShipmentID: src.execution.ShipmentID, Pattern: PatternCurrentTripFill})
		_, _ = w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestinationNMember})
	}()
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("ids %v errs %v", ids, errs)
	}
	events, _ := w.store.ListOutbox(context.Background())
	count := 0
	for _, event := range events {
		if event.EventName == routePlanEvaluatedEvent && event.AggregateID.String() == ids[0] {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("events %d", count)
	}
}

func evaluatePlan(t *testing.T, w *world, body, key string) map[string]any {
	t.Helper()
	var cmd RoutePlanCommand
	if err := json.Unmarshal([]byte(body), &cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Raw = []byte(body)
	result, err := w.svc.EvaluateRoutePlan(context.Background(), w.actor(), key, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusCreated {
		t.Fatalf("status %d", result.Status)
	}
	var doc map[string]any
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func jsonContains(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
