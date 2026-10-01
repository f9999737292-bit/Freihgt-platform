//go:build integration

package transportexecution

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestExecutionStopTrackingEvents(t *testing.T) {
	env := startPostgres(t)
	commandTestCtx = env.ctx
	commands := service.NewTransportExecutionCommandService(repository.NewTransportExecutionCommandRepository(env.pool))
	when := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

	t.Run("INITIAL_CURRENT_AND_REPLAY", func(t *testing.T) {
		ship := seedShipment(t, env, "track-init", domain.ShipmentStatusPickupSlotBooked)
		operating := seedTenant(t, env, "track-op")
		version := 1
		stopPlan := uuid.New()
		loc := ship.originID
		duration := 600
		cmd := domain.ProjectionCommand{
			ActivationID: uuid.New(), ActivationVersion: 2, ActivationStatus: domain.ActivationStatusPendingExecution,
			RoutePlanID: uuid.New(), RoutePlanVersion: 4, PlanningMode: domain.PlanningModeCurrentTrip,
			OperatingTenantID: operating, ContextShipmentID: &ship.shipmentID, ContextShipmentTenantID: &ship.tenantID,
			ContextShipmentVersion: &version, CarrierCompanyID: seedCompany(t, env, operating, "CARRIER", "Track Carrier"),
			EvaluationFingerprint: "tracking-initial", ExecutionSubjects: []domain.ProjectionSubject{materialSubject(ship, version)},
			Stops: []domain.ProjectionStop{
				{RoutePlanStopID: uuid.New(), Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.75, Longitude: 37.62},
				{RoutePlanStopID: stopPlan, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &loc, Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration},
			},
			Actions: []domain.ProjectionAction{materialAction(stopPlan, ship, version, 0)},
		}
		first, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", first.ExecutionID, domain.EventExecutionPlanCreated) != 1 {
			t.Fatal("initial execution plan created event missing")
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", first.ExecutionID, domain.EventRouteStopCurrent) != 1 {
			t.Fatal("initial current event missing")
		}
		created := outboxPayload(t, env, first.ExecutionID, domain.EventExecutionPlanCreated)
		current := outboxPayload(t, env, first.ExecutionID, domain.EventRouteStopCurrent)
		if created["event_sequence"] != float64(1) || current["event_sequence"] != float64(2) {
			t.Fatalf("sequences created=%v current=%v", created["event_sequence"], current["event_sequence"])
		}
		for _, key := range []string{"shipment_tenant_id", "price", "rate", "capacity_snapshot", "raw_load_opportunity", "leg_geometry", "latitude", "longitude", "route_subject_type"} {
			if _, ok := created[key]; ok {
				t.Fatalf("private field %s in plan created", key)
			}
		}
		second, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil || second.Created {
			t.Fatalf("replay created=%v err=%v", second.Created, err)
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", first.ExecutionID, domain.EventExecutionPlanCreated) != 1 {
			t.Fatal("replay inserted another plan created event")
		}
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", first.ExecutionID, domain.EventRouteStopCurrent) != 1 {
			t.Fatal("replay inserted another current event")
		}
		payload := outboxPayload(t, env, first.ExecutionID, domain.EventRouteStopCurrent)
		if payload["operating_tenant_id"] != cmd.OperatingTenantID.String() {
			t.Fatalf("operating tenant %+v", payload["operating_tenant_id"])
		}
		for _, key := range []string{"shipment_tenant_id", "price", "rate", "capacity_snapshot", "route_subject_type"} {
			if _, ok := payload[key]; ok {
				t.Fatalf("private field %s in %+v", key, payload)
			}
		}
		if _, ok := payload["event_id"]; !ok {
			t.Fatal("event_id missing")
		}
	})

	t.Run("NORMAL_ARRIVE_DOES_NOT_EMIT_CURRENT", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		before := countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopCurrent)
		skipStart(t, env, commands, fx, when)
		afterSkip := countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopCurrent)
		if afterSkip != before+1 {
			t.Fatalf("skip current events %d -> %d", before, afterSkip)
		}
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "arrive-no-current-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopCurrent) != afterSkip {
			t.Fatal("normal arrive emitted a current event")
		}
	})

	t.Run("COMPLETE_ADVANCES_AND_FINAL_DOES_NOT", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		skipStart(t, env, commands, fx, when)
		walkPickup(t, env, commands, fx, 1, when.Add(2*time.Minute))
		endID := fx.stops[len(fx.stops)-1]
		currentPayloads := outboxPayloads(t, env, fx.execution, domain.EventRouteStopCurrent)
		if !payloadMentionsStop(currentPayloads, endID) {
			t.Fatal("completing the cargo stop did not emit the next current stop")
		}
		beforeFinal := len(currentPayloads)
		mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, len(fx.stops)-1, -1, "final-arrive-"+fx.execution.String(), when.Add(20*time.Minute), stopVersion(t, env, endID), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, len(fx.stops)-1, -1, "final-start-"+fx.execution.String(), when.Add(21*time.Minute), stopVersion(t, env, endID), ""))
		mustExec(t, commands, fx.driverCmd(domain.CommandCompleteStop, len(fx.stops)-1, -1, "final-complete-"+fx.execution.String(), when.Add(22*time.Minute), stopVersion(t, env, endID), ""))
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopCurrent) != beforeFinal {
			t.Fatal("final completion invented a current stop")
		}
	})

	t.Run("CONTROLLED_OVERRIDE_EMITS_CURRENT", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypePickup}},
		})
		skipStart(t, env, commands, fx, when)
		before := countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopCurrent)
		mustExec(t, commands, fx.operatorCmd(domain.CommandArriveStop, 2, -1, "override-current-"+fx.execution.String(), when.Add(3*time.Minute), stopVersion(t, env, fx.stops[2]), "ROUTE_BLOCKED"))
		if countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1 AND event_type=$2", fx.execution, domain.EventRouteStopCurrent) != before+1 {
			t.Fatal("controlled arrive did not emit current")
		}
	})

	t.Run("TRACKING_CONTEXT", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked, domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}, {1, domain.ActionTypePickup}},
		})
		repo := repository.NewTransportExecutionRepository(env.pool)
		router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "ctx-token", repo)
		path := "/internal/v1/transport-executions/" + fx.execution.String() + "/stops/" + fx.stops[0].String() + "/tracking-context"
		if status, _ := contextRequest(router, path, "", fx.operating.String()); status != http.StatusUnauthorized {
			t.Fatalf("missing token status %d", status)
		}
		if status, _ := contextRequest(router, path, "wrong", fx.operating.String()); status != http.StatusUnauthorized {
			t.Fatalf("wrong token status %d", status)
		}
		if status, body := contextRequest(router, path, "ctx-token", uuid.NewString()); status != http.StatusNotFound {
			t.Fatalf("foreign tenant status %d body %s", status, body)
		}
		status, body := contextRequest(router, path, "ctx-token", fx.operating.String())
		if status != http.StatusOK {
			t.Fatalf("context status %d body %s", status, body)
		}
		var view map[string]any
		if err := json.Unmarshal([]byte(body), &view); err != nil {
			t.Fatal(err)
		}
		for _, ship := range fx.ships {
			if bodyContains(body, ship.tenantID.String()) {
				t.Fatalf("response exposed participant tenant %s", ship.tenantID)
			}
		}
		if view["pointKind"] != domain.PointKindPositionAnchor {
			t.Fatalf("current point %+v", view["pointKind"])
		}
		if view["liveEtaLatitude"] != nil || view["targetLatitude"] == nil {
			t.Fatalf("anchor coordinates %+v live %+v", view["targetLatitude"], view["liveEtaLatitude"])
		}
		future := "/internal/v1/transport-executions/" + fx.execution.String() + "/stops/" + fx.stops[1].String() + "/tracking-context"
		if status, _ := contextRequest(router, future, "ctx-token", fx.operating.String()); status != http.StatusNotFound {
			t.Fatalf("future stop status %d", status)
		}
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.locations SET lat=55.81, lon=37.71 WHERE id=$1`, fx.ships[0].originID); err != nil {
			t.Fatal(err)
		}
		_, body = contextRequest(router, path, "ctx-token", fx.operating.String())
		if err := json.Unmarshal([]byte(body), &view); err != nil {
			t.Fatal(err)
		}
		if view["liveEtaLatitude"] == nil || view["liveEtaLongitude"] == nil {
			t.Fatalf("authoritative coordinates missing %+v", view)
		}
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_revisions SET status='SUPERSEDED' WHERE id=$1`, fx.revision); err != nil {
			t.Fatal(err)
		}
		if status, _ := contextRequest(router, path, "ctx-token", fx.operating.String()); status != http.StatusNotFound {
			t.Fatalf("superseded revision status %d", status)
		}
	})
}

func TestMigration000091(t *testing.T) {
	env := startPostgres(t)
	up := filepath.Join(env.migrations, "000091_tms_execution_stop_tracking_v0_1d.up.sql")
	down := filepath.Join(env.migrations, "000091_tms_execution_stop_tracking_v0_1d.down.sql")
	if err := execSQLFile(env.ctx, env.pool, up); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"execution_tracking_state", "execution_event_inbox", "execution_stop_eta_state", "event_outbox"} {
		if !relationExists(t, env, "tracking", name) {
			t.Fatalf("missing tracking.%s", name)
		}
	}
	if nullable, _ := columnNullable(t, env, "shipment_id"); nullable != "YES" {
		t.Fatalf("shipment_id nullable %s", nullable)
	}
	if err := execSQLFile(env.ctx, env.pool, down); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"execution_tracking_state", "execution_event_inbox", "execution_stop_eta_state", "event_outbox"} {
		if relationExists(t, env, "tracking", name) {
			t.Fatalf("down left tracking.%s", name)
		}
	}
	if !tableExists(t, env, "transport_executions") || !tableExists(t, env, "driver_stop_tasks") {
		t.Fatal("down removed a prior migration table")
	}
	if nullable, _ := columnNullable(t, env, "shipment_id"); nullable != "NO" {
		t.Fatalf("down left shipment_id nullable %s", nullable)
	}
	if err := execSQLFile(env.ctx, env.pool, up); err != nil {
		t.Fatal(err)
	}
	if !relationExists(t, env, "tracking", "execution_tracking_state") {
		t.Fatal("second up failed")
	}
}

func outboxPayload(t *testing.T, env *execEnv, executionID uuid.UUID, eventType string) map[string]any {
	t.Helper()
	rows := outboxPayloads(t, env, executionID, eventType)
	if len(rows) == 0 {
		t.Fatal("payload missing")
	}
	return rows[0]
}

func outboxPayloads(t *testing.T, env *execEnv, executionID uuid.UUID, eventType string) []map[string]any {
	t.Helper()
	rows, err := env.pool.Query(env.ctx, `SELECT payload FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND event_type=$2 ORDER BY created_at`, executionID, eventType)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		out = append(out, payload)
	}
	return out
}

func payloadMentionsStop(payloads []map[string]any, stopID uuid.UUID) bool {
	for _, payload := range payloads {
		if payload["stop_id"] == stopID.String() {
			return true
		}
	}
	return false
}

func contextRequest(router http.Handler, path, token, tenant string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func bodyContains(body, value string) bool {
	return strings.Contains(body, value)
}

func columnNullable(t *testing.T, env *execEnv, column string) (string, error) {
	t.Helper()
	var nullable string
	err := env.pool.QueryRow(env.ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_schema='tracking' AND table_name='eta_observation' AND column_name=$1
	`, column).Scan(&nullable)
	if err != nil {
		t.Fatal(err)
	}
	return nullable, nil
}
