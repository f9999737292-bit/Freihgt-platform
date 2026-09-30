//go:build integration

package transportexecution

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/http/handlers"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestDriverExecutionContextServerValidation(t *testing.T) {
	env := startPostgres(t)
	if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000092_tms_control_tower_execution_projection_v0_1e.up.sql")); err != nil {
		t.Fatal(err)
	}
	ops := service.NewDriverOperationsService(
		repository.NewDriverRepository(env.pool),
		repository.NewShipmentRepository(env.pool),
		repository.NewDriverOperationsRepository(env.pool),
	)
	ops.BindDriverExecutionContext(repository.NewTransportExecutionRepository(env.pool))

	t.Run("E1R01_VALID_DELAY_STOP_CONTEXT", func(t *testing.T) {
		world, ship, stopID, _ := ownedCargoStop(t, env)
		payload := mustDelay(t, env, ops, world, ship.shipmentID, &stopID, nil)
		if payload["executionStopId"] != stopID.String() {
			t.Fatalf("executionStopId = %v", payload["executionStopId"])
		}
		if _, ok := payload["actionId"]; ok {
			t.Fatal("delay payload published an action id")
		}
	})

	t.Run("E1R02_VALID_PROBLEM_STOP_CONTEXT", func(t *testing.T) {
		world, ship, stopID, _ := ownedCargoStop(t, env)
		payload := mustProblem(t, env, ops, world, ship.shipmentID, &stopID, nil)
		if payload["executionStopId"] != stopID.String() {
			t.Fatalf("executionStopId = %v", payload["executionStopId"])
		}
		if _, ok := payload["actionId"]; ok {
			t.Fatal("stop-only problem published an action id")
		}
	})

	t.Run("E1R03_VALID_PROBLEM_STOP_ACTION_CONTEXT", func(t *testing.T) {
		world, ship, stopID, actionID := ownedCargoStop(t, env)
		payload := mustProblem(t, env, ops, world, ship.shipmentID, &stopID, &actionID)
		if payload["executionStopId"] != stopID.String() || payload["actionId"] != actionID.String() {
			t.Fatalf("payload context = %v %v", payload["executionStopId"], payload["actionId"])
		}
	})

	t.Run("E1R04_FOREIGN_EXECUTION_STOP_REJECTED", func(t *testing.T) {
		world, ship, _, _ := ownedCargoStop(t, env)
		foreign := uuid.New()
		mustRejectDelay(t, env, ops, world, ship.shipmentID, foreign)
	})

	t.Run("E1R05_STOP_OTHER_SHIPMENT_REJECTED", func(t *testing.T) {
		world := seedDriverWorld(t, env)
		shipA := seedDriverShipment(t, env, world)
		shipB := seedDriverShipment(t, env, world)
		graph := projectDriverRoute(t, env, world.tenantID, world.carrierID, world.driverID, []seededShipment{shipA, shipB}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{1, domain.ActionTypeDelivery}},
		})
		stopB := graph.stops[2]
		mustRejectDelay(t, env, ops, world, shipA.shipmentID, stopB)
	})

	t.Run("E1R06_STOP_OTHER_DRIVER_REJECTED", func(t *testing.T) {
		world := seedDriverWorld(t, env)
		ship := seedDriverShipment(t, env, world)
		graph := projectDriverRoute(t, env, world.tenantID, world.carrierID, uuid.New(), []seededShipment{ship}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
		})
		mustRejectDelay(t, env, ops, world, ship.shipmentID, graph.stops[1])
	})

	t.Run("E1R07_STOP_OTHER_OPERATING_TENANT_REJECTED", func(t *testing.T) {
		world := seedDriverWorld(t, env)
		ship := seedDriverShipment(t, env, world)
		otherTenant := seedTenant(t, env, "other-op")
		otherCarrier := seedCompany(t, env, otherTenant, "CARRIER", "Other carrier")
		graph := projectDriverRoute(t, env, otherTenant, otherCarrier, world.driverID, []seededShipment{ship}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
		})
		mustRejectDelay(t, env, ops, world, ship.shipmentID, graph.stops[1])
	})

	t.Run("E1R08_ACTION_OTHER_STOP_REJECTED", func(t *testing.T) {
		world := seedDriverWorld(t, env)
		ship := seedDriverShipment(t, env, world)
		graph := projectDriverRoute(t, env, world.tenantID, world.carrierID, world.driverID, []seededShipment{ship}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		stopA := graph.stops[1]
		actionB := graph.actions[2][0]
		mustRejectProblem(t, env, ops, world, ship.shipmentID, stopA, actionB)
	})

	t.Run("E1R09_ACTION_OTHER_SHIPMENT_REJECTED", func(t *testing.T) {
		world := seedDriverWorld(t, env)
		shipA := seedDriverShipment(t, env, world)
		shipB := seedDriverShipment(t, env, world)
		graph := projectDriverRoute(t, env, world.tenantID, world.carrierID, world.driverID, []seededShipment{shipA, shipB}, [][]actionSpec{
			{{0, domain.ActionTypePickup}, {1, domain.ActionTypeDelivery}},
		})
		stopID := graph.stops[1]
		actionB := graph.actions[1][1]
		mustRejectProblem(t, env, ops, world, shipA.shipmentID, stopID, actionB)
	})

	t.Run("E1R10_ACTION_OTHER_EXECUTION_REJECTED", func(t *testing.T) {
		world, shipA, stopA, _ := ownedCargoStop(t, env)
		_, _, _, actionOther := ownedCargoStop(t, env)
		mustRejectProblem(t, env, ops, world, shipA.shipmentID, stopA, actionOther)
	})

	t.Run("E1R11_MALFORMED_STOP_UUID_REJECTED", func(t *testing.T) {
		rec := postDriverJSON(t, "/v1/driver/me/shipments/"+uuid.NewString()+"/delays", uuid.New(), uuid.New(), []byte(`{"reasonCode":"TRAFFIC","idempotencyKey":"bad-stop","executionStopId":"not-a-uuid"}`))
		if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("VALIDATION_ERROR")) {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("E1R12_MALFORMED_ACTION_UUID_REJECTED", func(t *testing.T) {
		body := []byte(`{"category":"CARGO_ISSUE","idempotencyKey":"bad-action","executionStopId":"` + uuid.NewString() + `","actionId":"not-a-uuid"}`)
		rec := postDriverJSON(t, "/v1/driver/me/shipments/"+uuid.NewString()+"/exceptions", uuid.New(), uuid.New(), body)
		if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("VALIDATION_ERROR")) {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("E1R13_LEGACY_DELAY_WITHOUT_STOP", func(t *testing.T) {
		world, ship, stopID, _ := ownedCargoStop(t, env)
		payload := mustDelay(t, env, ops, world, ship.shipmentID, nil, nil)
		if _, ok := payload["executionStopId"]; ok {
			t.Fatal("legacy delay published executionStopId")
		}
		if bytes.Contains([]byte(mustJSON(payload)), []byte(stopID.String())) {
			t.Fatal("legacy delay payload contained the stop id")
		}
	})

	t.Run("E1R14_LEGACY_PROBLEM_WITHOUT_STOP", func(t *testing.T) {
		world, ship, stopID, actionID := ownedCargoStop(t, env)
		payload := mustProblem(t, env, ops, world, ship.shipmentID, nil, nil)
		if _, ok := payload["executionStopId"]; ok || payload["actionId"] != nil {
			t.Fatalf("legacy problem context = %v %v", payload["executionStopId"], payload["actionId"])
		}
		raw := mustJSON(payload)
		if bytes.Contains(raw, []byte(stopID.String())) || bytes.Contains(raw, []byte(actionID.String())) {
			t.Fatal("legacy problem payload contained execution context ids")
		}
	})

	t.Run("E1R15_REJECTED_CONTEXT_CREATES_NO_DRIVER_OUTBOX_EVENT", func(t *testing.T) {
		world, ship, _, _ := ownedCargoStop(t, env)
		foreign := uuid.New()
		mustRejectDelay(t, env, ops, world, ship.shipmentID, foreign)
		if n := driverEventCount(t, env, ship.shipmentID); n != 0 {
			t.Fatalf("driver outbox events = %d", n)
		}
		if n := payloadMentions(t, env, foreign); n != 0 {
			t.Fatalf("malicious stop appeared in %d outbox payloads", n)
		}
		if n := reportedDelayCount(t, env, ship.shipmentID); n != 0 {
			t.Fatalf("driver_reported_delay rows = %d", n)
		}
	})

	t.Run("E1R16_REJECTED_CONTEXT_CREATES_NO_FALSE_CT_PROGRESS", func(t *testing.T) {
		world, ship, _, _ := ownedCargoStop(t, env)
		foreignStop := uuid.New()
		foreignAction := uuid.New()
		mustRejectProblem(t, env, ops, world, ship.shipmentID, foreignStop, foreignAction)
		if n := driverEventCount(t, env, ship.shipmentID); n != 0 {
			t.Fatalf("driver outbox events = %d", n)
		}
		if n := payloadMentions(t, env, foreignStop) + payloadMentions(t, env, foreignAction); n != 0 {
			t.Fatalf("malicious context appeared in outbox %d times", n)
		}
		if n := progressEventCount(t, env); n != 0 {
			t.Fatalf("execution_progress_event rows = %d", n)
		}
	})
}

type driverWorld struct {
	tenantID    uuid.UUID
	userID      uuid.UUID
	driverID    uuid.UUID
	carrierID   uuid.UUID
	shipperID   uuid.UUID
	consigneeID uuid.UUID
}

func seedDriverWorld(t *testing.T, env *execEnv) driverWorld {
	t.Helper()
	tenantID := seedTenant(t, env, "drv")
	world := driverWorld{
		tenantID:    tenantID,
		userID:      uuid.New(),
		driverID:    uuid.New(),
		carrierID:   seedCompany(t, env, tenantID, "CARRIER", "Driver carrier"),
		shipperID:   seedCompany(t, env, tenantID, "SHIPPER", "Driver shipper"),
		consigneeID: seedCompany(t, env, tenantID, "CONSIGNEE", "Driver consignee"),
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,'Route Driver','ACTIVE')
	`, world.driverID, world.tenantID, world.carrierID, world.userID); err != nil {
		t.Fatal(err)
	}
	return world
}

func seedDriverShipment(t *testing.T, env *execEnv, world driverWorld) seededShipment {
	t.Helper()
	originID := uuid.New()
	destID := uuid.New()
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{{originID, "Origin"}, {destID, "Destination"}} {
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, row.id, world.tenantID, row.name); err != nil {
			t.Fatal(err)
		}
	}
	cargoID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description) VALUES ($1,$2,'GENERAL','Cargo')`, cargoID, world.tenantID); err != nil {
		t.Fatal(err)
	}
	orderID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
		VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')
	`, orderID, world.tenantID, "TO-"+orderID.String()[:8], world.shipperID, world.consigneeID, originID, destID); err != nil {
		t.Fatal(err)
	}
	shipmentID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, driver_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ROAD','DRIVER_ASSIGNED',1)
	`, shipmentID, world.tenantID, "SHP-"+shipmentID.String()[:8], orderID, world.shipperID, world.consigneeID, world.carrierID, world.driverID, originID, destID, cargoID); err != nil {
		t.Fatal(err)
	}
	return seededShipment{
		tenantID: world.tenantID, shipperID: world.shipperID, consigneeID: world.consigneeID,
		originID: originID, cargoID: cargoID, shipmentID: shipmentID, subjectID: uuid.New(),
	}
}

func ownedCargoStop(t *testing.T, env *execEnv) (driverWorld, seededShipment, uuid.UUID, uuid.UUID) {
	t.Helper()
	world := seedDriverWorld(t, env)
	ship := seedDriverShipment(t, env, world)
	graph := projectDriverRoute(t, env, world.tenantID, world.carrierID, world.driverID, []seededShipment{ship}, [][]actionSpec{
		{{0, domain.ActionTypePickup}},
	})
	return world, ship, graph.stops[1], graph.actions[1][0]
}

func projectDriverRoute(t *testing.T, env *execEnv, operating, carrier, driverID uuid.UUID, ships []seededShipment, cargo [][]actionSpec) execFixture {
	t.Helper()
	version := 1
	subjects := make([]domain.ProjectionSubject, len(ships))
	for i, ship := range ships {
		subjects[i] = materialSubject(ship, version)
	}
	stops := []domain.ProjectionStop{{
		RoutePlanStopID: uuid.New(), Ordinal: 0, StopRole: domain.StopRoleStart,
		PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6,
	}}
	var planned []domain.ProjectionAction
	for _, group := range cargo {
		stopID := uuid.New()
		loc := ships[group[0].ship].originID
		duration := 600
		stops = append(stops, domain.ProjectionStop{
			RoutePlanStopID: stopID, Ordinal: len(stops), StopRole: domain.StopRoleCargo,
			PointKind: domain.PointKindCanonicalLocation, LocationID: &loc,
			Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration,
		})
		for i, spec := range group {
			action := materialAction(stopID, ships[spec.ship], version, i)
			action.ActionType = spec.kind
			planned = append(planned, action)
		}
	}
	endLoc := ships[0].originID
	stops = append(stops, domain.ProjectionStop{
		RoutePlanStopID: uuid.New(), Ordinal: len(stops), StopRole: domain.StopRoleEnd,
		PointKind: domain.PointKindCanonicalLocation, LocationID: &endLoc, Latitude: 56, Longitude: 38,
	})
	cmd := domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 2, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 4, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: operating, ContextShipmentID: &ships[0].shipmentID, ContextShipmentTenantID: &ships[0].tenantID,
		ContextShipmentVersion: &version, CarrierCompanyID: carrier, DriverID: &driverID,
		EvaluationFingerprint: "driver-context-" + uuid.NewString(), ExecutionSubjects: subjects, Stops: stops, Actions: planned,
	}
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	fx := execFixture{operating: operating, driver: driverID, execution: result.ExecutionID, revision: result.RevisionID, ships: ships}
	rows, err := env.pool.Query(env.ctx, `SELECT id FROM transport.transport_execution_stops WHERE execution_id=$1 ORDER BY ordinal`, result.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		fx.stops = append(fx.stops, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, id := range fx.stops {
		actionRows, err := env.pool.Query(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 ORDER BY ordinal`, id)
		if err != nil {
			t.Fatal(err)
		}
		var ids []uuid.UUID
		for actionRows.Next() {
			var actionID uuid.UUID
			if err := actionRows.Scan(&actionID); err != nil {
				actionRows.Close()
				t.Fatal(err)
			}
			ids = append(ids, actionID)
		}
		if err := actionRows.Err(); err != nil {
			actionRows.Close()
			t.Fatal(err)
		}
		actionRows.Close()
		fx.actions = append(fx.actions, ids)
	}
	return fx
}

func mustDelay(t *testing.T, env *execEnv, ops *service.DriverOperationsService, world driverWorld, shipmentID uuid.UUID, stopID *uuid.UUID, _ *uuid.UUID) map[string]any {
	t.Helper()
	result, err := ops.ReportDelay(env.ctx, world.tenantID, world.userID, shipmentID, domain.DriverDelayInput{
		ReasonCode:      "TRAFFIC",
		IdempotencyKey:  "delay-" + uuid.NewString(),
		ExecutionStopID: stopID,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutboxEventID == nil {
		t.Fatal("delay did not create an outbox event")
	}
	return driverOutboxPayload(t, env, *result.OutboxEventID)
}

func mustProblem(t *testing.T, env *execEnv, ops *service.DriverOperationsService, world driverWorld, shipmentID uuid.UUID, stopID, actionID *uuid.UUID) map[string]any {
	t.Helper()
	result, err := ops.ReportException(env.ctx, world.tenantID, world.userID, shipmentID, domain.DriverExceptionInput{
		Category:        "CARGO_ISSUE",
		IdempotencyKey:  "problem-" + uuid.NewString(),
		ExecutionStopID: stopID,
		ActionID:        actionID,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutboxEventID == nil {
		t.Fatal("problem did not create an outbox event")
	}
	payload := driverOutboxPayload(t, env, *result.OutboxEventID)
	if payload["eventType"] != domain.DriverEventTypeProblemReported {
		t.Fatalf("eventType = %v", payload["eventType"])
	}
	return payload
}

func mustRejectDelay(t *testing.T, env *execEnv, ops *service.DriverOperationsService, world driverWorld, shipmentID, stopID uuid.UUID) {
	t.Helper()
	_, err := ops.ReportDelay(env.ctx, world.tenantID, world.userID, shipmentID, domain.DriverDelayInput{
		ReasonCode:      "TRAFFIC",
		IdempotencyKey:  "reject-delay-" + uuid.NewString(),
		ExecutionStopID: &stopID,
	}, nil)
	assertContextRejected(t, err)
	if n := payloadMentions(t, env, stopID); n != 0 {
		t.Fatalf("rejected stop %s appeared in %d outbox payloads", stopID, n)
	}
}

func mustRejectProblem(t *testing.T, env *execEnv, ops *service.DriverOperationsService, world driverWorld, shipmentID, stopID, actionID uuid.UUID) {
	t.Helper()
	_, err := ops.ReportException(env.ctx, world.tenantID, world.userID, shipmentID, domain.DriverExceptionInput{
		Category:        "CARGO_ISSUE",
		IdempotencyKey:  "reject-problem-" + uuid.NewString(),
		ExecutionStopID: &stopID,
		ActionID:        &actionID,
	}, nil)
	assertContextRejected(t, err)
	if n := payloadMentions(t, env, stopID) + payloadMentions(t, env, actionID); n != 0 {
		t.Fatalf("rejected context appeared in %d outbox payloads", n)
	}
}

func assertContextRejected(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}

func driverOutboxPayload(t *testing.T, env *execEnv, id uuid.UUID) map[string]any {
	t.Helper()
	var raw []byte
	var eventType string
	if err := env.pool.QueryRow(env.ctx, `SELECT event_type, payload FROM transport.shipment_event_outbox WHERE id=$1`, id).Scan(&eventType, &raw); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if eventType == domain.DriverEventTypeDelayReported && payload["eventType"] != eventType {
		t.Fatalf("payload eventType = %v", payload["eventType"])
	}
	return payload
}

func driverEventCount(t *testing.T, env *execEnv, shipmentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `
		SELECT COUNT(*) FROM transport.shipment_event_outbox
		WHERE aggregate_id = $1 AND event_type IN ('driver.delay.reported', 'driver.problem.reported')
	`, shipmentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func payloadMentions(t *testing.T, env *execEnv, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `
		SELECT COUNT(*) FROM transport.shipment_event_outbox
		WHERE event_type IN ('driver.delay.reported', 'driver.problem.reported')
		  AND (position($1 in payload::text) > 0 OR position($1 in headers::text) > 0)
	`, id.String()).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func reportedDelayCount(t *testing.T, env *execEnv, shipmentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM transport.driver_reported_delay WHERE shipment_id=$1`, shipmentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func progressEventCount(t *testing.T, env *execEnv) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM control_tower.execution_progress_event`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func postDriverJSON(t *testing.T, path string, tenantID, userID uuid.UUID, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	handler := handlers.NewDriverOperationsHandler(nil)
	router := chi.NewRouter()
	router.Post("/v1/driver/me/shipments/{id}/delays", handler.ReportDelay)
	router.Post("/v1/driver/me/shipments/{id}/exceptions", handler.ReportException)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func mustJSON(payload map[string]any) []byte {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return raw
}
