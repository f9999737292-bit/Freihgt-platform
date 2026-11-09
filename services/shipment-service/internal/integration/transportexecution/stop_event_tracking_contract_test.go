//go:build integration

package transportexecution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
	"github.com/freight-platform/tracking-service/executioncontract"
)

// TestStopEventTrackingContract09B feeds shipment-service execution outbox bytes
// into tracking ExecutionConsumer.Apply. It does not start Kafka.
func TestStopEventTrackingContract09B(t *testing.T) {
	env := startPostgres(t)
	commandTestCtx = env.ctx
	if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000091_tms_execution_stop_tracking_v0_1d.up.sql")); err != nil {
		t.Fatal(err)
	}
	h := newContractHarness(t, env)

	t.Run("PICKUP_DELIVERY_SEQUENCE", func(t *testing.T) {
		fx := newPickupDelivery(t, env)
		h.watchDelivery = fx.stops[2]
		initial := h.rows(fx.execution, fx.operating)
		if len(initial) < 2 {
			t.Fatalf("projection outbox rows %d", len(initial))
		}
		t.Logf("IDENTIFIER_SEMANTICS payload_event_id_equals_outbox_row_id=false source_event_id_equals_outbox_row_id=true first_event_sequence=%d", initial[0].sequence)
		firstCurrent, ok := firstOfType(initial, domain.EventRouteStopCurrent)
		if !ok {
			t.Fatal("initial current event missing")
		}
		if err := executioncontract.ApplyPayloadWithContextError(env.ctx, env.pool, firstCurrent.payload); err == nil {
			t.Fatal("context failure before the tracking transaction returned success")
		}
		if h.inboxCount(fx.execution) != 0 || h.state(fx.execution).exists {
			t.Fatal("context failure wrote tracking state or inbox")
		}
		if err := h.apply([]byte(`{`)); err == nil {
			t.Fatal("malformed payload returned success")
		}

		h.consume(fx.execution, nil)
		if h.inboxHas(firstCurrent.eventID) {
			if !h.state(fx.execution).exists {
				t.Fatal("initial current payload did not create tracking state")
			}
		} else {
			t.Fatal("initial current payload was not applied")
		}
		if plan, ok := firstOfType(initial, domain.EventExecutionPlanCreated); ok && h.inboxHas(plan.eventID) {
			t.Fatal("plan-created payload was applied; the tracking consumer does not accept that event type")
		}
		if st := h.state(fx.execution); !sameStop(st.current, fx.stops[0]) {
			t.Fatalf("initial current stop %v", st.current)
		}

		when := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
		skipStart(t, env, h.commands, fx, when)
		h.consume(fx.execution, nil)
		if st := h.state(fx.execution); !sameStop(st.current, fx.stops[1]) || !sameStop(st.live, fx.stops[1]) {
			t.Fatalf("after pickup current, state current=%v live=%v", st.current, st.live)
		}

		arrive := fx.driverCmd(domain.CommandArriveStop, 1, -1, "arrive-pickup-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), "")
		mustExec(t, h.commands, arrive)
		seq := executionEventSeq(t, env, fx.execution)
		outboxCount := executionOutboxCount(t, env, fx.execution)
		mustExec(t, h.commands, arrive)
		if executionEventSeq(t, env, fx.execution) != seq || executionOutboxCount(t, env, fx.execution) != outboxCount {
			t.Fatal("same idempotency key created another execution sequence")
		}
		h.consume(fx.execution, nil)
		mustExec(t, h.commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "start-pickup-"+fx.execution.String(), when.Add(2*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		h.consume(fx.execution, nil)
		mustExec(t, h.commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "confirm-pickup-"+fx.execution.String(), when.Add(3*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		h.consume(fx.execution, nil)
		mustExec(t, h.commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "complete-pickup-"+fx.execution.String(), when.Add(4*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		h.consume(fx.execution, nil)
		if st := h.state(fx.execution); !sameStop(st.current, fx.stops[2]) || !sameStop(st.live, fx.stops[2]) {
			t.Fatalf("after pickup completion the target is not delivery: current=%v live=%v", st.current, st.live)
		}

		mustExec(t, h.commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "arrive-delivery-"+fx.execution.String(), when.Add(10*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		h.consume(fx.execution, nil)
		mustExec(t, h.commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "start-delivery-"+fx.execution.String(), when.Add(11*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		h.consume(fx.execution, nil)
		mustExec(t, h.commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "confirm-delivery-"+fx.execution.String(), when.Add(12*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		h.consume(fx.execution, nil)
		mustExec(t, h.commands, fx.driverCmd(domain.CommandCompleteStop, 2, -1, "complete-delivery-"+fx.execution.String(), when.Add(13*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		h.consume(fx.execution, nil)
		st := h.state(fx.execution)
		if sameStop(st.current, fx.stops[2]) || sameStop(st.live, fx.stops[2]) {
			t.Fatalf("completed delivery remained the ETA target: current=%v live=%v", st.current, st.live)
		}
		if !h.inboxHas(firstCurrent.eventID) {
			t.Fatal("later events removed the initial inbox row")
		}
		assertRouteOrder(t, h.rows(fx.execution, fx.operating), fx.stops[1], fx.stops[2])

		if h.deliveryCurrent == nil {
			t.Fatal("delivery current payload was not captured")
		}
		before := h.state(fx.execution)
		inbox := h.inboxCount(fx.execution)
		if err := h.apply(h.deliveryCurrent); err != nil {
			t.Fatal(err)
		}
		if h.inboxCount(fx.execution) != inbox || !sameContractState(before, h.state(fx.execution)) {
			t.Fatal("duplicate payload changed tracking state or inbox")
		}

		foreign := uuid.New()
		adversarial := clonePayload(t, h.deliveryCurrent)
		adversarial["event_id"] = uuid.NewString()
		adversarial["operating_tenant_id"] = foreign.String()
		raw, err := json.Marshal(adversarial)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("FOREIGN_PAYLOAD=SYNTHETIC_ADVERSARIAL; this does not prove JWT to gateway to shipment-service")
		path := "/internal/v1/transport-executions/" + fx.execution.String() + "/stops/" + fx.stops[2].String() + "/tracking-context"
		status, body := contextRequest(h.router, path, "ctx-token", foreign.String())
		if status != http.StatusNotFound {
			t.Fatalf("foreign context status %d", status)
		}
		if bodyContains(body, fx.ships[0].tenantID.String()) || bodyContains(body, "pointKind") || bodyContains(body, "liveEta") {
			t.Fatalf("foreign context body revealed stop context: %s", body)
		}
		foreignInbox := h.inboxCount(fx.execution)
		foreignEventID := uuid.MustParse(adversarial["event_id"].(string))
		if err := h.apply(raw); err == nil {
			t.Fatal("foreign payload reused a stored execution sequence and was accepted")
		}
		if !sameContractState(before, h.state(fx.execution)) || h.inboxCount(fx.execution) != foreignInbox || h.inboxHas(foreignEventID) {
			t.Fatalf("foreign operating tenant changed state or inbox: %+v", h.state(fx.execution))
		}
		if h.stateCount(fx.execution) != 1 || h.state(fx.execution).tenant != fx.operating {
			t.Fatal("foreign payload created or rewrote execution tracking state")
		}
		t.Log("FOREIGN_APPLY=REJECTED_BY_uq_execution_event_inbox_sequence; transaction rolled back")
		if err := h.apply([]byte(`{`)); err == nil {
			t.Fatal("malformed payload after state existed returned success")
		}
		if !sameContractState(before, h.state(fx.execution)) {
			t.Fatal("malformed payload changed the ETA target")
		}
	})

	t.Run("OUT_OF_ORDER_PREDECESSOR", func(t *testing.T) {
		fx := newPickupDelivery(t, env)
		h.watchDelivery = fx.stops[2]
		h.consume(fx.execution, nil)
		when := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
		skipStart(t, env, h.commands, fx, when)
		var withheld *executionOutboxRow
		h.consume(fx.execution, func(row executionOutboxRow) bool {
			if withheld == nil && row.eventType == domain.EventRouteStopCurrent && row.stopID == fx.stops[1] {
				copyRow := row
				withheld = &copyRow
				return true
			}
			return false
		})
		if withheld == nil {
			t.Fatal("pickup current payload missing")
		}
		driveDelivery(t, h, fx, when.Add(time.Minute))
		before := h.state(fx.execution)
		inbox := h.inboxCount(fx.execution)
		if err := h.apply(withheld.payload); err != nil {
			t.Fatal(err)
		}
		after := h.state(fx.execution)
		if after.seq < before.seq || sameStop(after.current, fx.stops[1]) || sameStop(after.live, fx.stops[1]) {
			t.Fatalf("older payload rolled state back: before=%+v after=%+v", before, after)
		}
		if !sameContractState(before, after) {
			t.Fatalf("older payload changed state: before=%+v after=%+v", before, after)
		}
		if h.inboxCount(fx.execution) != inbox+1 || !h.inboxHas(withheld.eventID) {
			t.Fatal("late predecessor was not recorded in the inbox")
		}
		t.Log("OUT_OF_ORDER_SEMANTICS=INBOX_RECORD_NO_STATE_REGRESSION; no reorder buffer")
	})

	t.Run("FAIL_DELIVERY_ACTION", func(t *testing.T) {
		fx := newPickupDelivery(t, env)
		h.watchDelivery = fx.stops[2]
		h.consume(fx.execution, nil)
		when := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
		skipStart(t, env, h.commands, fx, when)
		h.consume(fx.execution, nil)
		driveThroughDeliveryService(t, h, fx, when.Add(time.Minute))
		if st := h.state(fx.execution); !sameStop(st.live, fx.stops[2]) {
			t.Fatalf("delivery target before fail current=%v live=%v", st.current, st.live)
		}
		completed := completedCount(t, env, fx.execution)
		fail := fx.driverCmd(domain.CommandFailAction, 2, 0, "fail-delivery-"+fx.execution.String(), when.Add(20*time.Minute), stopVersion(t, env, fx.stops[2]), "CARGO_ISSUE")
		mustExec(t, h.commands, fail)
		if actionStatusOf(t, env, fx.actions[2][0]) != domain.ActionStatusFailed {
			t.Fatalf("action status %s", actionStatusOf(t, env, fx.actions[2][0]))
		}
		if stopStatusOf(t, env, fx.stops[2]) == domain.StopStatusCompleted {
			t.Fatal("FailAction completed the delivery stop")
		}
		if completedCount(t, env, fx.execution) != completed {
			t.Fatal("FailAction emitted shipment.route_stop.completed")
		}
		problem := problemPayload(t, env, fx.ships[0].shipmentID)
		if problem["eventType"] != domain.DriverEventTypeProblemReported || problem["reasonCode"] != "CARGO_ISSUE" {
			t.Fatalf("problem payload %+v", problem)
		}
		before := h.state(fx.execution)
		raw, err := json.Marshal(problem)
		if err != nil {
			t.Fatal(err)
		}
		applyErr := h.apply(raw)
		if !sameContractState(before, h.state(fx.execution)) {
			t.Fatalf("problem payload changed tracking state: %v", applyErr)
		}
		t.Logf("PROBLEM_PAYLOAD_APPLY_ERROR=%v", applyErr != nil)

		mustExec(t, h.commands, fx.driverCmd(domain.CommandCompleteStop, 2, -1, "complete-after-fail-"+fx.execution.String(), when.Add(21*time.Minute), stopVersion(t, env, fx.stops[2]), ""))
		if stopStatusOf(t, env, fx.stops[2]) != domain.StopStatusCompleted || stopReason(t, env, fx.stops[2]) != domain.StopStatusReasonPartial {
			t.Fatalf("later complete status %s reason %s", stopStatusOf(t, env, fx.stops[2]), stopReason(t, env, fx.stops[2]))
		}
		if completedCount(t, env, fx.execution) != completed+1 {
			t.Fatal("CompleteStop after a failed action did not emit route_stop.completed")
		}
		h.consume(fx.execution, nil)
		if st := h.state(fx.execution); sameStop(st.current, fx.stops[2]) || sameStop(st.live, fx.stops[2]) {
			t.Fatalf("partial completion left the delivery ETA target: current=%v live=%v", st.current, st.live)
		}
	})

	t.Run("COMMAND_OUTBOX_ATOMICITY", func(t *testing.T) {
		fx := newPickupDelivery(t, env)
		when := time.Date(2026, 11, 9, 11, 0, 0, 0, time.UTC)
		skipStart(t, env, h.commands, fx, when)
		seq := executionEventSeq(t, env, fx.execution)
		count := executionOutboxCount(t, env, fx.execution)
		restore := repository.SetExecutionOutboxInsertForTest(func(context.Context, pgx.Tx, domain.ShipmentOutboxEvent) error {
			return errors.New("synthetic outbox insert failure")
		})
		defer restore()
		_, err := h.commands.Execute(env.ctx, fx.driverCmd(domain.CommandArriveStop, 1, -1, "arrive-rollback-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if err == nil {
			t.Fatal("outbox insert failure returned success")
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusPlanned {
			t.Fatalf("rolled-back arrive left status %s", stopStatusOf(t, env, fx.stops[1]))
		}
		if executionEventSeq(t, env, fx.execution) != seq || executionOutboxCount(t, env, fx.execution) != count {
			t.Fatal("outbox insert failure committed the command sequence")
		}
	})
}

type executionOutboxRow struct {
	id            uuid.UUID
	sourceEventID uuid.UUID
	eventID       uuid.UUID
	eventType     string
	stopID        uuid.UUID
	sequence      int64
	payload       []byte
}

type contractState struct {
	exists  bool
	tenant  uuid.UUID
	current *uuid.UUID
	live    *uuid.UUID
	seq     int64
}

type contractHarness struct {
	t               *testing.T
	env             *execEnv
	commands        *service.TransportExecutionCommandService
	baseURL         string
	router          http.Handler
	seen            map[uuid.UUID]struct{}
	watchDelivery   uuid.UUID
	deliveryCurrent []byte
}

func newContractHarness(t *testing.T, env *execEnv) *contractHarness {
	t.Helper()
	repo := repository.NewTransportExecutionRepository(env.pool)
	router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "ctx-token", repo, "analytics-source-caller")
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &contractHarness{
		t:        t,
		env:      env,
		commands: service.NewTransportExecutionCommandService(repository.NewTransportExecutionCommandRepository(env.pool)),
		baseURL:  server.URL,
		router:   router,
		seen:     map[uuid.UUID]struct{}{},
	}
}

func (h *contractHarness) apply(payload []byte) error {
	return executioncontract.ApplyPayload(h.env.ctx, h.env.pool, h.baseURL, "ctx-token", payload)
}

func newPickupDelivery(t *testing.T, env *execEnv) execFixture {
	t.Helper()
	return newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
		{{0, domain.ActionTypePickup}},
		{{0, domain.ActionTypeDelivery}},
	})
}

func driveThroughDeliveryService(t *testing.T, h *contractHarness, fx execFixture, when time.Time) {
	t.Helper()
	mustExec(t, h.commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "svc-arrive-pickup-"+fx.execution.String(), when, stopVersion(t, h.env, fx.stops[1]), ""))
	h.consume(fx.execution, nil)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "svc-start-pickup-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, h.env, fx.stops[1]), ""))
	h.consume(fx.execution, nil)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "svc-confirm-pickup-"+fx.execution.String(), when.Add(2*time.Minute), stopVersion(t, h.env, fx.stops[1]), ""))
	h.consume(fx.execution, nil)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandCompleteStop, 1, -1, "svc-complete-pickup-"+fx.execution.String(), when.Add(3*time.Minute), stopVersion(t, h.env, fx.stops[1]), ""))
	h.consume(fx.execution, nil)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandArriveStop, 2, -1, "svc-arrive-delivery-"+fx.execution.String(), when.Add(4*time.Minute), stopVersion(t, h.env, fx.stops[2]), ""))
	h.consume(fx.execution, nil)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandStartStopService, 2, -1, "svc-start-delivery-"+fx.execution.String(), when.Add(5*time.Minute), stopVersion(t, h.env, fx.stops[2]), ""))
	h.consume(fx.execution, nil)
}

func driveDelivery(t *testing.T, h *contractHarness, fx execFixture, when time.Time) {
	t.Helper()
	driveThroughDeliveryService(t, h, fx, when)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandConfirmDelivery, 2, 0, "ooo-confirm-delivery-"+fx.execution.String(), when.Add(6*time.Minute), stopVersion(t, h.env, fx.stops[2]), ""))
	h.consume(fx.execution, nil)
	mustExec(t, h.commands, fx.driverCmd(domain.CommandCompleteStop, 2, -1, "ooo-complete-delivery-"+fx.execution.String(), when.Add(7*time.Minute), stopVersion(t, h.env, fx.stops[2]), ""))
	h.consume(fx.execution, nil)
}

func (h *contractHarness) consume(executionID uuid.UUID, skip func(executionOutboxRow) bool) {
	h.t.Helper()
	for _, row := range h.rows(executionID, uuid.Nil) {
		if _, ok := h.seen[row.id]; ok {
			continue
		}
		h.seen[row.id] = struct{}{}
		if skip != nil && skip(row) {
			continue
		}
		if !isTrackingStopEvent(row.eventType) {
			continue
		}
		if err := h.apply(row.payload); err != nil {
			h.t.Fatalf("apply %s sequence %d: %v", row.eventType, row.sequence, err)
		}
		st := h.state(executionID)
		if row.eventType == domain.EventRouteStopCurrent && row.stopID == h.watchDelivery {
			h.deliveryCurrent = append([]byte(nil), row.payload...)
			if !sameStop(st.current, h.watchDelivery) || !sameStop(st.live, h.watchDelivery) {
				h.t.Fatalf("delivery current did not bind the ETA target: current=%v live=%v", st.current, st.live)
			}
		}
		if row.eventType == domain.EventRouteStopCompleted && row.stopID == h.watchDelivery {
			if sameStop(st.current, h.watchDelivery) || sameStop(st.live, h.watchDelivery) {
				h.t.Fatalf("delivery completed did not clear the ETA target: current=%v live=%v", st.current, st.live)
			}
		}
	}
}

func (h *contractHarness) rows(executionID, operating uuid.UUID) []executionOutboxRow {
	h.t.Helper()
	dbRows, err := h.env.pool.Query(h.env.ctx, `
		SELECT id, source_event_id, event_type, payload
		FROM transport.shipment_event_outbox
		WHERE aggregate_id = $1
		ORDER BY created_at, id
	`, executionID)
	if err != nil {
		h.t.Fatal(err)
	}
	defer dbRows.Close()
	var out []executionOutboxRow
	for dbRows.Next() {
		var row executionOutboxRow
		if err := dbRows.Scan(&row.id, &row.sourceEventID, &row.eventType, &row.payload); err != nil {
			h.t.Fatal(err)
		}
		h.decodeRow(&row, operating)
		out = append(out, row)
	}
	if err := dbRows.Err(); err != nil {
		h.t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].sequence < out[j].sequence })
	for i := 1; i < len(out); i++ {
		if out[i].sequence <= out[i-1].sequence {
			h.t.Fatalf("event_sequence did not increase: %d then %d", out[i-1].sequence, out[i].sequence)
		}
	}
	return out
}

func (h *contractHarness) decodeRow(row *executionOutboxRow, operating uuid.UUID) {
	h.t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(row.payload, &raw); err != nil {
		h.t.Fatal(err)
	}
	for _, key := range []string{"shipment_tenant_id", "shipmentTenantId", "price", "rate", "capacity_snapshot", "capacitySnapshot"} {
		if _, ok := raw[key]; ok {
			h.t.Fatalf("%s contains private field %s", row.eventType, key)
		}
	}
	var body map[string]any
	if err := json.Unmarshal(row.payload, &body); err != nil {
		h.t.Fatal(err)
	}
	eventID, err := uuid.Parse(stringField(body, "event_id"))
	if err != nil {
		h.t.Fatal(err)
	}
	row.eventID = eventID
	if eventID == row.id {
		h.t.Fatalf("%s payload event_id equals the outbox row id", row.eventType)
	}
	if row.sourceEventID != row.id {
		h.t.Fatalf("%s source_event_id %s row id %s", row.eventType, row.sourceEventID, row.id)
	}
	row.sequence = sequenceField(body)
	if row.sequence <= 0 {
		h.t.Fatalf("%s event_sequence %d", row.eventType, row.sequence)
	}
	if operating != uuid.Nil && stringField(body, "operating_tenant_id") != operating.String() {
		h.t.Fatalf("%s operating tenant %s", row.eventType, stringField(body, "operating_tenant_id"))
	}
	if isTrackingStopEvent(row.eventType) {
		for _, key := range []string{"event_id", "event_type", "operating_tenant_id", "execution_id", "revision_id", "revision_version", "event_sequence", "stop_id", "action_id", "occurred_at", "skipped_ordinals"} {
			if _, ok := raw[key]; !ok {
				h.t.Fatalf("%s missing %s", row.eventType, key)
			}
		}
		row.stopID, err = uuid.Parse(stringField(body, "stop_id"))
		if err != nil {
			h.t.Fatal(err)
		}
	}
}

func (h *contractHarness) state(executionID uuid.UUID) contractState {
	h.t.Helper()
	var tenant string
	var current, live *string
	var seq int64
	err := h.env.pool.QueryRow(h.env.ctx, `
		SELECT operating_tenant_id::text, current_stop_id::text, live_eta_stop_id::text, last_execution_event_sequence
		FROM tracking.execution_tracking_state
		WHERE execution_id = $1
	`, executionID).Scan(&tenant, &current, &live, &seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return contractState{}
	}
	if err != nil {
		h.t.Fatal(err)
	}
	return contractState{exists: true, tenant: uuid.MustParse(tenant), current: parseOptionalUUID(current), live: parseOptionalUUID(live), seq: seq}
}

func (h *contractHarness) stateCount(executionID uuid.UUID) int {
	h.t.Helper()
	var count int
	if err := h.env.pool.QueryRow(h.env.ctx, `SELECT count(*) FROM tracking.execution_tracking_state WHERE execution_id=$1`, executionID).Scan(&count); err != nil {
		h.t.Fatal(err)
	}
	return count
}

func (h *contractHarness) inboxCount(executionID uuid.UUID) int {
	h.t.Helper()
	var count int
	if err := h.env.pool.QueryRow(h.env.ctx, `SELECT count(*) FROM tracking.execution_event_inbox WHERE execution_id=$1`, executionID).Scan(&count); err != nil {
		h.t.Fatal(err)
	}
	return count
}

func (h *contractHarness) inboxHas(eventID uuid.UUID) bool {
	h.t.Helper()
	var count int
	if err := h.env.pool.QueryRow(h.env.ctx, `SELECT count(*) FROM tracking.execution_event_inbox WHERE event_id=$1`, eventID).Scan(&count); err != nil {
		h.t.Fatal(err)
	}
	return count == 1
}

func (h *contractHarness) inboxTenant(eventID uuid.UUID) uuid.UUID {
	h.t.Helper()
	var tenant string
	if err := h.env.pool.QueryRow(h.env.ctx, `SELECT operating_tenant_id::text FROM tracking.execution_event_inbox WHERE event_id=$1`, eventID).Scan(&tenant); err != nil {
		h.t.Fatal(err)
	}
	return uuid.MustParse(tenant)
}

func isTrackingStopEvent(eventType string) bool {
	switch eventType {
	case domain.EventRouteStopCurrent, domain.EventRouteStopArrived, domain.EventRouteStopServiceStarted,
		domain.EventRouteStopCompleted, domain.EventRouteStopSequenceOverridden:
		return true
	default:
		return false
	}
}

func firstOfType(rows []executionOutboxRow, eventType string) (executionOutboxRow, bool) {
	for _, row := range rows {
		if row.eventType == eventType {
			return row, true
		}
	}
	return executionOutboxRow{}, false
}

func assertRouteOrder(t *testing.T, rows []executionOutboxRow, pickup, delivery uuid.UUID) {
	t.Helper()
	var steps []string
	for _, row := range rows {
		if !isTrackingStopEvent(row.eventType) {
			continue
		}
		switch {
		case row.stopID == pickup && row.eventType == domain.EventRouteStopCurrent:
			steps = append(steps, "pickup-current")
		case row.stopID == pickup && row.eventType == domain.EventRouteStopArrived:
			steps = append(steps, "pickup-arrived")
		case row.stopID == pickup && row.eventType == domain.EventRouteStopServiceStarted:
			steps = append(steps, "pickup-service")
		case row.stopID == pickup && row.eventType == domain.EventRouteStopCompleted:
			steps = append(steps, "pickup-completed")
		case row.stopID == delivery && row.eventType == domain.EventRouteStopCurrent:
			steps = append(steps, "delivery-current")
		case row.stopID == delivery && row.eventType == domain.EventRouteStopArrived:
			steps = append(steps, "delivery-arrived")
		case row.stopID == delivery && row.eventType == domain.EventRouteStopServiceStarted:
			steps = append(steps, "delivery-service")
		case row.stopID == delivery && row.eventType == domain.EventRouteStopCompleted:
			steps = append(steps, "delivery-completed")
		}
	}
	want := []string{"pickup-current", "pickup-arrived", "pickup-service", "pickup-completed", "delivery-current", "delivery-arrived", "delivery-service", "delivery-completed"}
	at := 0
	for _, step := range steps {
		if at < len(want) && step == want[at] {
			at++
		}
	}
	if at != len(want) {
		t.Fatalf("route payload order %v", steps)
	}
}

func executionEventSeq(t *testing.T, env *execEnv, executionID uuid.UUID) int64 {
	t.Helper()
	var seq int64
	if err := env.pool.QueryRow(env.ctx, `SELECT event_seq FROM transport.transport_executions WHERE id=$1`, executionID).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	return seq
}

func executionOutboxCount(t *testing.T, env *execEnv, executionID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1`, executionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func completedCount(t *testing.T, env *execEnv, executionID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND event_type=$2`, executionID, domain.EventRouteStopCompleted).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func problemPayload(t *testing.T, env *execEnv, shipmentID uuid.UUID) map[string]any {
	t.Helper()
	var raw []byte
	if err := env.pool.QueryRow(env.ctx, `
		SELECT payload FROM transport.shipment_event_outbox
		WHERE aggregate_id=$1 AND event_type=$2
		ORDER BY created_at DESC
		LIMIT 1
	`, shipmentID, domain.DriverEventTypeProblemReported).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func clonePayload(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func stringField(body map[string]any, key string) string {
	value, _ := body[key].(string)
	return value
}

func sequenceField(body map[string]any) int64 {
	switch value := body["event_sequence"].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	default:
		return 0
	}
}

func parseOptionalUUID(value *string) *uuid.UUID {
	if value == nil || *value == "" {
		return nil
	}
	id := uuid.MustParse(*value)
	return &id
}

func sameStop(id *uuid.UUID, want uuid.UUID) bool {
	return id != nil && *id == want
}

func sameContractState(left, right contractState) bool {
	return left.exists == right.exists && left.tenant == right.tenant && left.seq == right.seq &&
		sameOptionalStop(left.current, right.current) && sameOptionalStop(left.live, right.live)
}

func sameOptionalStop(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
