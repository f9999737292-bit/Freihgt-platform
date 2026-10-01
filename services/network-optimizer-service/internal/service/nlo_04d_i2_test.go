package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/executionproj"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

func TestNLO04DI2ServiceDurationAndExecutionLink(t *testing.T) {
	t.Run("I2-01-02-07_policy_resolves_pickup_and_delivery", func(t *testing.T) {
		w, src, load := durationWorld(t)
		policy := publishDuration(t, w, 900, 1200)
		doc := evaluatePlan(t, w, currentTripBody(src.execution.ShipmentID, load.ID), "eval-duration")
		reasons, _ := doc["reason_codes"].([]any)
		if jsonContains(reasons, routeplan.ReasonServiceDurationUnknown) {
			t.Fatalf("known policy still unknown %v", reasons)
		}
		if seconds, ok := soleActionSeconds(doc, routeplan.ActionPickup); !ok || seconds != 900 {
			t.Fatalf("pickup duration %d ok=%v", seconds, ok)
		}
		if seconds, ok := soleActionSeconds(doc, routeplan.ActionDelivery); !ok || seconds != 1200 {
			t.Fatalf("delivery duration %d ok=%v", seconds, ok)
		}
		graph := loadPlan(t, w, uuid.MustParse(doc["id"].(string)))
		if doc["evaluation_fingerprint"] == "" || doc["evaluation_fingerprint"] != graph.Plan.EvaluationFingerprint {
			t.Fatalf("fingerprint %v", doc["evaluation_fingerprint"])
		}
		deps := 0
		for _, dep := range graph.Dependencies {
			if dep.DependencyKind != "SERVICE_DURATION_POLICY" {
				continue
			}
			deps++
			if dep.SubjectID == nil || *dep.SubjectID != policy.ID || dep.SubjectVersion == nil || *dep.SubjectVersion != 1 || dep.Fingerprint != "PICKUP=900|DELIVERY=1200" {
				t.Fatalf("dependency %+v", dep)
			}
		}
		if deps != 1 {
			t.Fatalf("policy dependencies %d", deps)
		}
	})

	t.Run("I2-03_no_active_policy_fail_closed", func(t *testing.T) {
		w, src, load := durationWorld(t)
		doc := evaluatePlan(t, w, currentTripBody(src.execution.ShipmentID, load.ID), "eval-none")
		reasons, _ := doc["reason_codes"].([]any)
		if !jsonContains(reasons, routeplan.ReasonServiceDurationUnknown) {
			t.Fatalf("reasons %v", reasons)
		}
		decidePlan(t, w, doc["id"].(string), `{"version":1}`, "accept-none", true)
		_, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-none", uuid.MustParse(doc["id"].(string)), []byte(`{"version":1}`))
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonServiceDurationUnknown {
			t.Fatal(err)
		}
		if len(w.store.TestingActivations()) != 0 {
			t.Fatal("unknown duration wrote an activation")
		}
	})

	t.Run("I2-05-06_active_immutable_and_one_active", func(t *testing.T) {
		w, _, _ := durationWorld(t)
		first := publishDuration(t, w, 100, 200)
		err := w.svc.UpdateServiceDurationDraft(context.Background(), w.actor(), first.ID, 101, 201)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeConflict {
			t.Fatal(err)
		}
		secondDraft, err := w.svc.CreateServiceDurationDraft(context.Background(), w.actor(), 300, 400)
		if err != nil || secondDraft.Version != 2 {
			t.Fatal(err)
		}
		if err := w.svc.UpdateServiceDurationDraft(context.Background(), w.actor(), secondDraft.ID, 330, 440); err != nil {
			t.Fatal(err)
		}
		second, err := w.svc.PublishServiceDurationPolicy(context.Background(), w.actor(), secondDraft.ID)
		if err != nil || second.Version != 2 || second.Status != repository.DurationPolicyActive || second.PickupSeconds != 330 || second.DeliverySeconds != 440 {
			t.Fatalf("%+v %v", second, err)
		}
		var active repository.ServiceDurationPolicy
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			var getErr error
			active, getErr = tx.ActiveServiceDurationPolicy(context.Background(), w.actor().TenantID)
			return getErr
		}); err != nil || active.ID != second.ID || active.Version != 2 {
			t.Fatalf("active %+v %v", active, err)
		}
	})

	t.Run("I2-08_policy_change_plan_stale", func(t *testing.T) {
		w, src, load := durationWorld(t)
		publishDuration(t, w, 900, 1200)
		statusBefore := src.execution.ShipmentStatus
		doc := evaluatePlan(t, w, currentTripBody(src.execution.ShipmentID, load.ID), "eval-stale-policy")
		publishDuration(t, w, 901, 1201)
		_, err := w.svc.AcceptRoutePlan(context.Background(), w.actor(), "accept-stale-policy", uuid.MustParse(doc["id"].(string)), []byte(`{"version":1}`))
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != routeplan.ReasonPlanStale {
			t.Fatal(err)
		}
		if src.execution.ShipmentStatus != statusBefore || loadPlan(t, w, uuid.MustParse(doc["id"].(string))).Plan.Status != routeplan.StatusEvaluated {
			t.Fatal("stale accept mutated the trip or the plan")
		}
	})

	t.Run("I2-09-17_pending_then_lost_response_then_link", func(t *testing.T) {
		w, src, id, client := activatablePlan(t)
		beforeFP := loadPlan(t, w, id).Plan.EvaluationFingerprint
		onboardBefore := len(src.onboard.Items)
		resolutionBefore := src.onboard.Resolution
		client.failRemaining = 1
		client.onProject = func(cmd executionproj.Command) {
			rows := w.store.TestingActivations()
			if len(rows) != 1 || rows[0].Status != routeplan.ActivationPending || rows[0].ExecutionID != nil || rows[0].ExecutionRevisionID != nil {
				t.Errorf("handoff saw %+v", rows)
			}
			if cmd.ActivationStatus != routeplan.ActivationPending || cmd.OperatingTenantID != w.actor().TenantID || cmd.RoutePlanID != id {
				t.Errorf("command %+v", cmd)
			}
			if src.execution.CarrierCompanyID == nil || cmd.CarrierCompanyID != *src.execution.CarrierCompanyID {
				t.Errorf("carrier %s", cmd.CarrierCompanyID)
			}
		}
		first := decidePlan(t, w, id.String(), `{"version":1}`, "activate-lost", false)
		activation := first["activation"].(map[string]any)
		if activation["status"] != routeplan.ActivationPending || activation["execution_id"] != nil || client.calls != 1 {
			t.Fatalf("%v calls %d", activation, client.calls)
		}
		second := decidePlan(t, w, id.String(), `{"version":1}`, "activate-lost", false)
		linked := second["activation"].(map[string]any)
		plan := second["plan"].(map[string]any)
		if linked["status"] != routeplan.ActivationLinked || linked["execution_id"] != client.execution.String() || linked["execution_revision_id"] != client.revision.String() {
			t.Fatalf("%v", linked)
		}
		if plan["status"] != routeplan.StatusAccepted || plan["evaluation_fingerprint"] != beforeFP || plan["execution_supported"] != false {
			t.Fatalf("plan reoptimized %+v", plan["status"])
		}
		if src.execution.ShipmentStatus != "IN_TRANSIT" || len(src.onboard.Items) != onboardBefore || src.onboard.Resolution != resolutionBefore {
			t.Fatal("current trip or onboard changed")
		}
		third := decidePlan(t, w, id.String(), `{"version":1}`, "activate-again", false)
		if third["activation"].(map[string]any)["execution_id"] != linked["execution_id"] || client.calls != 2 {
			t.Fatalf("duplicate handoff calls %d id %v", client.calls, third["activation"])
		}
		if countEvents(t, w, routePlanExecutionLinkedEvent, id) != 1 {
			t.Fatal("execution linked event was not exactly once")
		}
		assertEventPrivacy(t, w, id)
		rows := w.store.TestingActivations()
		if len(rows) != 1 || rows[0].ExecutionID == nil || *rows[0].ExecutionID != client.execution || rows[0].ExecutionRevisionID == nil || *rows[0].ExecutionRevisionID != client.revision {
			t.Fatalf("stored %+v", rows)
		}
	})

	t.Run("I2-18_foreign_ack_denied", func(t *testing.T) {
		w, _, id, client := activatablePlan(t)
		client.foreign = true
		_, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), "activate-foreign-ack", id, []byte(`{"version":1}`))
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeConflict || app.Details["reason"] != reasonAckCorrelationMismatch {
			t.Fatal(err)
		}
		rows := w.store.TestingActivations()
		if len(rows) != 1 || rows[0].Status != routeplan.ActivationPending || rows[0].ExecutionID != nil {
			t.Fatalf("%+v", rows)
		}
		if countEvents(t, w, routePlanExecutionLinkedEvent, id) != 0 {
			t.Fatal("foreign ack emitted execution_linked")
		}
		if _, err := w.svc.ActivateRoutePlan(context.Background(), serviceActor(uuid.New(), nil), "foreign-tenant", id, []byte(`{"version":1}`)); err == nil {
			t.Fatal("foreign tenant activated the plan")
		}
	})

	t.Run("I2-19-20_concurrent_activation_one_root", func(t *testing.T) {
		w, _, id, client := activatablePlan(t)
		client.delay = 30 * time.Millisecond
		var wg sync.WaitGroup
		errCh := make(chan error, 2)
		for _, key := range []string{"activate-a", "activate-b"} {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				_, err := w.svc.ActivateRoutePlan(context.Background(), w.actor(), key, id, []byte(`{"version":1}`))
				errCh <- err
			}(key)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		rows := w.store.TestingActivations()
		if len(rows) != 1 || rows[0].Status != routeplan.ActivationLinked || rows[0].ExecutionID == nil || *rows[0].ExecutionID != client.execution || rows[0].ExecutionRevisionID == nil || *rows[0].ExecutionRevisionID != client.revision {
			t.Fatalf("%+v execution %s", rows, client.execution)
		}
		if countEvents(t, w, routePlanExecutionLinkedEvent, id) != 1 {
			t.Fatal("concurrent activation emitted more than one link")
		}
	})
}

func durationWorld(t *testing.T) (*world, *fillSources, domain.LoadOpportunity) {
	t.Helper()
	w, src, load := newFill(t)
	w.svc.UseRouting(&countingRoute{})
	w.svc.SetClock(func() time.Time { return w.at })
	return w, src, load
}

func publishDuration(t *testing.T, w *world, pickup, delivery int) repository.ServiceDurationPolicy {
	t.Helper()
	draft, err := w.svc.CreateServiceDurationDraft(context.Background(), w.actor(), pickup, delivery)
	if err != nil {
		t.Fatal(err)
	}
	published, err := w.svc.PublishServiceDurationPolicy(context.Background(), w.actor(), draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	return published
}

func activatablePlan(t *testing.T) (*world, *fillSources, uuid.UUID, *scriptProjection) {
	t.Helper()
	w, src, load := durationWorld(t)
	carrier := uuid.New()
	src.execution.CarrierCompanyID = &carrier
	publishDuration(t, w, 900, 1200)
	doc := evaluatePlan(t, w, currentTripBody(src.execution.ShipmentID, load.ID), "eval-activate-i2")
	id := uuid.MustParse(doc["id"].(string))
	makeActivatable(t, w, id)
	decidePlan(t, w, id.String(), `{"version":1}`, "accept-i2", true)
	client := &scriptProjection{}
	w.svc.UseExecutionProjection(client)
	return w, src, id, client
}

func soleActionSeconds(doc map[string]any, actionType string) (int, bool) {
	stops, _ := doc["stops"].([]any)
	for _, raw := range stops {
		stop := raw.(map[string]any)
		actions, _ := stop["actions"].([]any)
		if len(actions) == 0 {
			continue
		}
		only := true
		for _, rawAction := range actions {
			if rawAction.(map[string]any)["action_type"] != actionType {
				only = false
			}
		}
		if !only || stop["service_duration_seconds"] == nil {
			continue
		}
		return int(stop["service_duration_seconds"].(float64)), true
	}
	return 0, false
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func countEvents(t *testing.T, w *world, name string, id uuid.UUID) int {
	t.Helper()
	events, err := w.store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.EventName == name && event.AggregateID == id {
			count++
		}
	}
	return count
}

func assertEventPrivacy(t *testing.T, w *world, id uuid.UUID) {
	t.Helper()
	events, err := w.store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]struct{}{
		"eventId": {}, "eventName": {}, "schemaVersion": {}, "tenantId": {}, "aggregateId": {},
		"aggregateVersion": {}, "occurredAt": {}, "status": {}, "visibilityScope": {},
		"activationId": {}, "operatingTenantId": {}, "routePlanId": {}, "executionId": {}, "executionRevisionId": {},
	}
	found := false
	for _, event := range events {
		if event.EventName != routePlanExecutionLinkedEvent || event.AggregateID != id {
			continue
		}
		found = true
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		for key := range payload {
			if _, ok := allowed[key]; !ok {
				t.Fatalf("event field %s", key)
			}
		}
		if payload["status"] != routeplan.ActivationLinked || payload["executionId"] == nil || payload["executionRevisionId"] == nil {
			t.Fatalf("payload %v", payload)
		}
	}
	if !found {
		t.Fatal("missing execution linked event")
	}
}

type scriptProjection struct {
	mu            sync.Mutex
	calls         int
	failRemaining int
	foreign       bool
	delay         time.Duration
	execution     uuid.UUID
	revision      uuid.UUID
	onProject     func(executionproj.Command)
}

func (s *scriptProjection) Project(_ context.Context, tenant uuid.UUID, cmd executionproj.Command) (executionproj.Ack, error) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.onProject != nil {
		s.onProject(cmd)
	}
	if s.failRemaining > 0 {
		s.failRemaining--
		return executionproj.Ack{}, &executionproj.Error{Temporary: true, Reason: "unavailable"}
	}
	if tenant != cmd.OperatingTenantID {
		return executionproj.Ack{}, &executionproj.Error{Temporary: false, Reason: "TENANT_DENIED"}
	}
	if s.execution == uuid.Nil {
		s.execution = uuid.New()
		s.revision = uuid.New()
	}
	ack := executionproj.Ack{
		OperatingTenantID: cmd.OperatingTenantID, ActivationID: cmd.ActivationID, RoutePlanID: cmd.RoutePlanID,
		ExecutionID: s.execution, RevisionID: s.revision,
	}
	if s.foreign {
		ack.OperatingTenantID = uuid.New()
	}
	return ack, nil
}
