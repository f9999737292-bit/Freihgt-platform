//go:build integration

package transportexecution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestDriverStopTasks(t *testing.T) {
	env := startPostgres(t)
	commandTestCtx = env.ctx
	when := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	commands := service.NewTransportExecutionCommandService(repository.NewTransportExecutionCommandRepository(env.pool))
	repo := repository.NewTransportExecutionCommandRepository(env.pool)
	drivers := repository.NewDriverRepository(env.pool)
	stops := service.NewDriverStopService(drivers, repo)
	ops := service.NewDriverOperationsService(drivers, repository.NewShipmentRepository(env.pool), repository.NewDriverOperationsRepository(env.pool))
	ops.BindMultistopDeparture(repo)
	router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, ops, nil, stops, nil, "test-token", nil)

	t.Run("schema", func(t *testing.T) {
		assertColumns(t, env, "driver_stop_tasks", []string{
			"id", "operating_tenant_id", "execution_id", "execution_stop_id", "driver_id", "vehicle_id",
			"shipment_id", "ordinal", "location_id", "planned_arrival", "action_summary", "status", "version",
		}, []string{"shipment_tenant_id", "price", "rate", "route_plan_id", "evaluation_fingerprint"})
		var noticeCheck string
		if err := env.pool.QueryRow(env.ctx, `
			SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'chk_driver_task_type'
		`).Scan(&noticeCheck); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(noticeCheck, "GENERAL_OPERATIONAL_NOTICE") || strings.Contains(noticeCheck, "SERVICE_STARTED") {
			t.Fatalf("notice task constraint changed: %s", noticeCheck)
		}
	})

	t.Run("materialized with projection", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1", fx.execution) != 1 {
			t.Fatalf("tasks=%d", countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1", fx.execution))
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1", fx.stops[0]) != 0 {
			t.Fatal("position anchor received a task")
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1 AND shipment_id=$2", fx.stops[1], fx.ships[0].shipmentID) != 1 {
			t.Fatal("cargo stop task missing shipment id")
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1", fx.stops[2]) != 0 {
			t.Fatal("redundant end received a task")
		}
		var summary string
		if err := env.pool.QueryRow(env.ctx, `SELECT action_summary::text FROM transport.driver_stop_tasks WHERE execution_stop_id=$1`, fx.stops[1]).Scan(&summary); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(summary, fx.ships[0].tenantID.String()) || strings.Contains(strings.ToLower(summary), "price") || strings.Contains(summary, "LOAD_OPPORTUNITY") {
			t.Fatalf("action summary leaked private data: %s", summary)
		}
	})

	t.Run("replay does not duplicate", func(t *testing.T) {
		ship := seedShipment(t, env, "replay", domain.ShipmentStatusPickupSlotBooked)
		cmd := cargoRoute(t, env, uuidPtr(uuid.New()), []seededShipment{ship}, false)
		first, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		second, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if first.ExecutionID != second.ExecutionID || first.RevisionID != second.RevisionID {
			t.Fatal("replay changed execution identity")
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1", first.ExecutionID) != 1 {
			t.Fatal("replay duplicated tasks")
		}
	})

	t.Run("projection rolls back when task insert fails", func(t *testing.T) {
		restore := repository.SetDriverStopTaskMaterializeForTest(func(context.Context, pgx.Tx, uuid.UUID, time.Time) error {
			return errors.New("task insert failed")
		})
		t.Cleanup(restore)
		ship := seedShipment(t, env, "atomic", domain.ShipmentStatusPickupSlotBooked)
		cmd := cargoRoute(t, env, uuidPtr(uuid.New()), []seededShipment{ship}, false)
		if _, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd); err == nil {
			t.Fatal("projection committed after task failure")
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "source_activation_id=$1", cmd.ActivationID) != 0 {
			t.Fatal("execution committed without tasks")
		}
		restore()
	})

	t.Run("distinct end is tasked", func(t *testing.T) {
		ship := seedShipment(t, env, "distinct-end", domain.ShipmentStatusPickupSlotBooked)
		cmd := cargoRoute(t, env, uuidPtr(uuid.New()), []seededShipment{ship}, true)
		result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1", result.ExecutionID) != 2 {
			t.Fatalf("tasks=%d", countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1", result.ExecutionID))
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1 AND shipment_id IS NULL", result.ExecutionID) != 1 {
			t.Fatal("distinct end without actions kept a shipment id")
		}
	})

	t.Run("multi shipment stop is one task", func(t *testing.T) {
		a := seedShipment(t, env, "multi-a", domain.ShipmentStatusPickupSlotBooked)
		b := seedShipment(t, env, "multi-b", domain.ShipmentStatusPickupSlotBooked)
		cmd := cargoRoute(t, env, uuidPtr(uuid.New()), []seededShipment{a, b}, false)
		result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1", result.ExecutionID) != 1 {
			t.Fatal("multi-shipment stop created more than one task")
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1 AND shipment_id IS NULL", result.ExecutionID) != 1 {
			t.Fatal("multi-shipment stop stored a shipment id")
		}
	})

	t.Run("unassigned task is hidden", func(t *testing.T) {
		ship := seedShipment(t, env, "unassigned", domain.ShipmentStatusPickupSlotBooked)
		cmd := cargoRoute(t, env, nil, []seededShipment{ship}, false)
		result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		var driverID *uuid.UUID
		if err := env.pool.QueryRow(env.ctx, `SELECT driver_id FROM transport.driver_stop_tasks WHERE execution_id=$1`, result.ExecutionID).Scan(&driverID); err != nil {
			t.Fatal(err)
		}
		if driverID != nil {
			t.Fatal("unassigned task stored a driver")
		}
		user := bindExtraDriver(t, env, cmd.OperatingTenantID, cmd.CarrierCompanyID)
		view, err := stops.List(env.ctx, cmd.OperatingTenantID, user)
		if err != nil {
			t.Fatal(err)
		}
		if view.Current != nil || view.Next != nil {
			t.Fatal("unassigned task was visible")
		}
	})

	t.Run("current and next", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		user := bindFixtureDriver(t, env, fx)
		view, err := stops.List(env.ctx, fx.operating, user)
		if err != nil {
			t.Fatal(err)
		}
		if view.Current == nil || view.Next == nil {
			t.Fatal("expected current and next")
		}
		if view.Current.ExecutionStopID != fx.stops[1] || view.Current.Position != domain.DriverStopPositionCurrent {
			t.Fatalf("current %+v", view.Current)
		}
		if view.Next.ExecutionStopID != fx.stops[2] || view.Next.Position != domain.DriverStopPositionNext {
			t.Fatalf("next %+v", view.Next)
		}
		body := listStops(t, router, fx.operating, user)
		if strings.Count(body, `"position"`) != 2 {
			t.Fatalf("payload %s", body)
		}
		if strings.Contains(body, fx.ships[0].tenantID.String()) || strings.Contains(body, "operatingTenantId") || strings.Contains(body, "routePlanId") {
			t.Fatalf("private payload %s", body)
		}
		if !strings.Contains(body, fx.ships[0].shipmentID.String()) {
			t.Fatalf("single-shipment stop hid shipment id: %s", body)
		}
	})

	t.Run("wrong driver and tenant", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		user := bindFixtureDriver(t, env, fx)
		skipStart(t, env, commands, fx, when)
		otherUser := bindExtraDriver(t, env, fx.operating, carrierOf(t, env, fx.execution))
		rec := postStop(t, router, fx.operating, otherUser, fx.stops[1], "arrive", "", "wrong-driver", stopVersion(t, env, fx.stops[1]))
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), domain.ReasonWrongDriver) {
			t.Fatalf("wrong driver %d %s", rec.Code, rec.Body.String())
		}
		foreign := seedTenant(t, env, "foreign")
		foreignUser := bindExtraDriver(t, env, foreign, seedCompany(t, env, foreign, "CARRIER", "Foreign"))
		rec = postStop(t, router, foreign, foreignUser, fx.stops[1], "arrive", "", "foreign-driver", stopVersion(t, env, fx.stops[1]))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("foreign tenant %d %s", rec.Code, rec.Body.String())
		}
		otherView, err := stops.List(env.ctx, fx.operating, otherUser)
		if err != nil {
			t.Fatal(err)
		}
		if otherView.Current != nil {
			t.Fatal("wrong driver read the route")
		}
		own := listStops(t, router, fx.operating, user)
		if !strings.Contains(own, fx.stops[1].String()) {
			t.Fatalf("assigned driver list %s", own)
		}
	})

	t.Run("cross shipper privacy", func(t *testing.T) {
		a := seedShipment(t, env, "priv-a", domain.ShipmentStatusPickupSlotBooked)
		b := seedShipment(t, env, "priv-b", domain.ShipmentStatusPickupSlotBooked)
		driverID := uuid.New()
		cmd := cargoRoute(t, env, &driverID, []seededShipment{a, b}, true)
		result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		user := bindExtraDriver(t, env, cmd.OperatingTenantID, cmd.CarrierCompanyID)
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_executions SET driver_id=$2 WHERE id=$1`, result.ExecutionID, driverID); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.driver_stop_tasks SET driver_id=$2 WHERE execution_id=$1`, result.ExecutionID, driverID); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.drivers SET id=$2 WHERE user_id=$1`, user, driverID); err != nil {
			t.Fatal(err)
		}
		body := listStops(t, router, cmd.OperatingTenantID, user)
		for _, forbidden := range []string{a.tenantID.String(), b.tenantID.String(), "LOAD_OPPORTUNITY", "optimizer", "capacity", "price", "rate"} {
			if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
				t.Fatalf("response contains %s: %s", forbidden, body)
			}
		}
		if !strings.Contains(body, `"shipmentId":null`) {
			t.Fatalf("multi-shipment current exposed a shipment id: %s", body)
		}
	})

	t.Run("multi shipment action identity", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked, domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{
			{0, domain.ActionTypeDelivery},
			{1, domain.ActionTypeDelivery},
		}})
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1 AND shipment_id IS NULL", fx.stops[1]) != 1 {
			t.Fatal("multi-shipment stop stored a stop-level shipment id")
		}
		user := bindFixtureDriver(t, env, fx)
		assertMultiShipmentActions(t, router, fx, user)

		if _, err := env.pool.Exec(env.ctx, `
			UPDATE transport.driver_stop_tasks
			SET action_summary = jsonb_set(
				action_summary,
				'{actions}',
				(
					SELECT COALESCE(jsonb_agg(item - 'shipmentId'), '[]'::jsonb)
					FROM jsonb_array_elements(action_summary->'actions') AS item
				)
			)
			WHERE execution_stop_id = $1
		`, fx.stops[1]); err != nil {
			t.Fatal(err)
		}
		assertMultiShipmentActions(t, router, fx, user)

		stale := uuid.New()
		if _, err := env.pool.Exec(env.ctx, `
			UPDATE transport.driver_stop_tasks
			SET action_summary = jsonb_set(action_summary, '{actions,0,shipmentId}', to_jsonb($2::text), true)
			WHERE execution_stop_id = $1
		`, fx.stops[1], stale.String()); err != nil {
			t.Fatal(err)
		}
		body := assertMultiShipmentActions(t, router, fx, user)
		if strings.Contains(body, stale.String()) {
			t.Fatalf("stale materialized shipment id was returned: %s", body)
		}
		if strings.Contains(body, "00000000-0000-0000-0000-000000000000") {
			t.Fatalf("zero shipment id was exposed: %s", body)
		}

		actionID := fx.actions[1][0]
		shipmentID, cargoID, err := repo.LoadDeliveryAction(env.ctx, fx.stops[1], actionID)
		if err != nil {
			t.Fatal(err)
		}
		if shipmentID != fx.ships[0].shipmentID || cargoID != fx.ships[0].cargoID {
			t.Fatalf("canonical action identity %s %s", shipmentID, cargoID)
		}
		if !strings.Contains(body, `"shipmentId":"`+shipmentID.String()+`"`) || !strings.Contains(body, `"cargoId":"`+cargoID.String()+`"`) {
			t.Fatalf("public action identity is not the disposition pair: %s", body)
		}
	})

	t.Run("http commands", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		user := bindFixtureDriver(t, env, fx)
		skipStart(t, env, commands, fx, when)
		occurred := when.Add(time.Minute).Format(time.RFC3339)
		arriveVersion := stopVersion(t, env, fx.stops[1])
		arriveBody := []byte(`{"expectedVersion":` + strconv.Itoa(arriveVersion) + `,"occurredAt":"` + occurred + `"}`)
		arrive := postStopRaw(t, router, fx.operating, user, "/v1/driver/me/stops/"+fx.stops[1].String()+"/arrive", "http-arrive-"+fx.execution.String(), arriveBody)
		if arrive.Code != http.StatusOK || stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusArrived {
			t.Fatalf("arrive %d %s", arrive.Code, arrive.Body.String())
		}
		assertTaskMirrorsStop(t, env, fx.stops[1])
		replay := postStopRaw(t, router, fx.operating, user, "/v1/driver/me/stops/"+fx.stops[1].String()+"/arrive", "http-arrive-"+fx.execution.String(), arriveBody)
		if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) {
			t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
		}
		conflict := postStopRaw(t, router, fx.operating, user, "/v1/driver/me/stops/"+fx.stops[1].String()+"/arrive", "http-arrive-"+fx.execution.String(), []byte(`{"expectedVersion":1,"occurredAt":"2026-09-29T10:00:00Z"}`))
		if conflict.Code != http.StatusConflict {
			t.Fatalf("conflict %d %s", conflict.Code, conflict.Body.String())
		}
		nextDenied := postStop(t, router, fx.operating, user, fx.stops[2], "arrive", "", "http-next-"+fx.execution.String(), stopVersion(t, env, fx.stops[2]))
		if nextDenied.Code != http.StatusConflict || !strings.Contains(nextDenied.Body.String(), domain.ReasonStopNotCurrent) {
			t.Fatalf("next stop %d %s", nextDenied.Code, nextDenied.Body.String())
		}
		if stopStatusOf(t, env, fx.stops[2]) != domain.StopStatusPlanned {
			t.Fatal("next stop changed")
		}
		start := postStop(t, router, fx.operating, user, fx.stops[1], "start-service", "", "http-start-"+fx.execution.String(), stopVersion(t, env, fx.stops[1]))
		if start.Code != http.StatusOK || stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusServiceStarted {
			t.Fatalf("start %d %s", start.Code, start.Body.String())
		}
		confirm := postStop(t, router, fx.operating, user, fx.stops[1], "confirm", fx.actions[1][0].String(), "http-pickup-"+fx.execution.String(), stopVersion(t, env, fx.stops[1]))
		if confirm.Code != http.StatusOK || actionStatusOf(t, env, fx.actions[1][0]) != domain.ActionStatusCompleted {
			t.Fatalf("pickup %d %s", confirm.Code, confirm.Body.String())
		}
		complete := postStop(t, router, fx.operating, user, fx.stops[1], "complete", "", "http-complete-"+fx.execution.String(), stopVersion(t, env, fx.stops[1]))
		if complete.Code != http.StatusOK || stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusCompleted {
			t.Fatalf("complete %d %s", complete.Code, complete.Body.String())
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1 AND status='COMPLETED'", fx.stops[1]) != 1 {
			t.Fatal("completed task was deleted")
		}
		arrive2 := postStop(t, router, fx.operating, user, fx.stops[2], "arrive", "", "http-arrive2-"+fx.execution.String(), stopVersion(t, env, fx.stops[2]))
		if arrive2.Code != http.StatusOK {
			t.Fatalf("second arrive %d %s", arrive2.Code, arrive2.Body.String())
		}
		start2 := postStop(t, router, fx.operating, user, fx.stops[2], "start-service", "", "http-start2-"+fx.execution.String(), stopVersion(t, env, fx.stops[2]))
		if start2.Code != http.StatusOK {
			t.Fatalf("second start %d %s", start2.Code, start2.Body.String())
		}
		delivery := postStop(t, router, fx.operating, user, fx.stops[2], "confirm", fx.actions[2][0].String(), "http-delivery-"+fx.execution.String(), stopVersion(t, env, fx.stops[2]))
		if delivery.Code != http.StatusOK || actionStatusOf(t, env, fx.actions[2][0]) != domain.ActionStatusCompleted {
			t.Fatalf("delivery %d %s", delivery.Code, delivery.Body.String())
		}
	})

	t.Run("fail action and stale version", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		user := bindFixtureDriver(t, env, fx)
		skipStart(t, env, commands, fx, when)
		if rec := postStop(t, router, fx.operating, user, fx.stops[1], "arrive", "", "fail-arrive-"+fx.execution.String(), stopVersion(t, env, fx.stops[1])); rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		if rec := postStop(t, router, fx.operating, user, fx.stops[1], "start-service", "", "fail-start-"+fx.execution.String(), stopVersion(t, env, fx.stops[1])); rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		stale := postStop(t, router, fx.operating, user, fx.stops[1], "fail", fx.actions[1][0].String(), "fail-stale-"+fx.execution.String(), 1)
		if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), domain.ReasonVersionConflict) {
			t.Fatalf("stale %d %s", stale.Code, stale.Body.String())
		}
		if actionStatusOf(t, env, fx.actions[1][0]) != domain.ActionStatusPending {
			t.Fatal("stale fail mutated the action")
		}
		commented := postStopRaw(t, router, fx.operating, user, "/v1/driver/me/stops/"+fx.stops[1].String()+"/actions/"+fx.actions[1][0].String()+"/fail", "fail-comment-"+fx.execution.String(), []byte(`{"expectedVersion":`+strconv.Itoa(stopVersion(t, env, fx.stops[1]))+`,"occurredAt":"2026-09-29T09:30:00Z","reasonCode":"CARGO_ISSUE","comment":"torn wrap"}`))
		if commented.Code != http.StatusBadRequest {
			t.Fatalf("comment %d %s", commented.Code, commented.Body.String())
		}
		if actionStatusOf(t, env, fx.actions[1][0]) != domain.ActionStatusPending || stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusServiceStarted {
			t.Fatal("rejected comment mutated the stop")
		}
		fail := postStop(t, router, fx.operating, user, fx.stops[1], "fail", fx.actions[1][0].String(), "fail-ok-"+fx.execution.String(), stopVersion(t, env, fx.stops[1]))
		if fail.Code != http.StatusOK || actionStatusOf(t, env, fx.actions[1][0]) != domain.ActionStatusFailed {
			t.Fatalf("fail %d %s", fail.Code, fail.Body.String())
		}
	})

	t.Run("override syncs tasks", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypePickup}},
		})
		mustExec(t, commands, fx.operatorCmd(domain.CommandArriveStop, 2, -1, "override-task-"+fx.execution.String(), when, stopVersion(t, env, fx.stops[2]), "ROUTE_BLOCKED"))
		if taskStatus(t, env, fx.stops[1]) != domain.StopStatusSkipped || taskStatus(t, env, fx.stops[2]) != domain.StopStatusArrived {
			t.Fatalf("skipped=%s target=%s", taskStatus(t, env, fx.stops[1]), taskStatus(t, env, fx.stops[2]))
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_id=$1 AND status='PLANNED'", fx.execution) != 0 {
			t.Fatal("stale planned task remained current")
		}
	})

	t.Run("task update failure rolls back the stop", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		if _, err := env.pool.Exec(env.ctx, `
			CREATE OR REPLACE FUNCTION transport.fail_driver_stop_task_sync() RETURNS trigger AS $$
			BEGIN RAISE EXCEPTION 'task sync failed'; END $$ LANGUAGE plpgsql
		`); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(env.ctx, `
			CREATE TRIGGER fail_driver_stop_task_sync
			BEFORE UPDATE ON transport.driver_stop_tasks
			FOR EACH ROW EXECUTE FUNCTION transport.fail_driver_stop_task_sync()
		`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = env.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS fail_driver_stop_task_sync ON transport.driver_stop_tasks`)
			_, _ = env.pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS transport.fail_driver_stop_task_sync()`)
		})
		skipStart(t, env, commands, fx, when)
		err := execCmd(commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "rollback-task-"+fx.execution.String(), when, stopVersion(t, env, fx.stops[1]), ""))
		if err == nil {
			t.Fatal("stop command committed after task update failure")
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusPlanned || taskStatus(t, env, fx.stops[1]) != domain.StopStatusPlanned {
			t.Fatal("stop and task diverged")
		}
	})

	t.Run("concurrent duplicate", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		user := bindFixtureDriver(t, env, fx)
		skipStart(t, env, commands, fx, when)
		body := []byte(`{"expectedVersion":` + strconv.Itoa(stopVersion(t, env, fx.stops[1])) + `,"occurredAt":"2026-09-29T09:05:00Z"}`)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		payloads := make([]string, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				rec := postStopRaw(t, router, fx.operating, user, "/v1/driver/me/stops/"+fx.stops[1].String()+"/arrive", "concurrent-"+fx.execution.String(), body)
				codes[i] = rec.Code
				payloads[i] = rec.Body.String()
			}(i)
		}
		wg.Wait()
		for i := 0; i < 2; i++ {
			if codes[i] != http.StatusOK {
				t.Fatalf("concurrent %d %s", codes[i], payloads[i])
			}
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusArrived || stopVersion(t, env, fx.stops[1]) != 2 {
			t.Fatalf("status %s version %d", stopStatusOf(t, env, fx.stops[1]), stopVersion(t, env, fx.stops[1]))
		}
		replays := 0
		for _, payload := range payloads {
			if strings.Contains(payload, `"replayed":true`) {
				replays++
			}
		}
		if replays != 1 {
			t.Fatalf("replay count %d %v", replays, payloads)
		}
	})

	t.Run("departure reuse", func(t *testing.T) {
		singleTenant := seedTenant(t, env, "single-owner")
		single := seedAssignedShipment(t, env, singleTenant, seedCompany(t, env, singleTenant, "SHIPPER", "Single Shipper"), seedCompany(t, env, singleTenant, "CONSIGNEE", "Single Consignee"))
		var driverID uuid.UUID
		if err := env.pool.QueryRow(env.ctx, `SELECT driver_id FROM transport.shipments WHERE id=$1`, single.shipmentID).Scan(&driverID); err != nil {
			t.Fatal(err)
		}
		user := uuid.New()
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.drivers SET user_id=$2 WHERE id=$1`, driverID, user); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.shipments SET status='LOADED' WHERE id=$1`, single.shipmentID); err != nil {
			t.Fatal(err)
		}
		occurred := time.Now().UTC()
		result, err := ops.RecordOperationalEvent(env.ctx, single.tenantID, user, single.shipmentID, domain.DriverOperationalEventInput{
			Type: "DEPARTED_PICKUP", IdempotencyKey: "single-depart-" + single.shipmentID.String(), OccurredAt: &occurred,
		}, domain.NewUserTransitionContext(user, nil, occurred))
		if err != nil {
			t.Fatal(err)
		}
		if result.ShipmentStatus != domain.ShipmentStatusInTransit {
			t.Fatalf("single-leg status %s", result.ShipmentStatus)
		}
		if countWhere(t, env, "transport.transport_execution_commands", "idempotency_key=$1", "single-depart-"+single.shipmentID.String()) != 0 {
			t.Fatal("single-leg departure created an execution command")
		}

		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		routeUser := bindFixtureDriver(t, env, fx)
		skipStart(t, env, commands, fx, when)
		walkPickup(t, env, commands, fx, 1, when.Add(2*time.Minute))
		multi, err := ops.RecordOperationalEvent(env.ctx, fx.operating, routeUser, fx.ships[0].shipmentID, domain.DriverOperationalEventInput{
			Type: "DEPARTED_PICKUP", IdempotencyKey: "multi-depart-" + fx.execution.String(), OccurredAt: &occurred,
		}, domain.NewUserTransitionContext(routeUser, nil, occurred))
		if err != nil {
			t.Fatal(err)
		}
		if multi.ShipmentStatus != domain.ShipmentStatusInTransit {
			t.Fatalf("multistop status %s", multi.ShipmentStatus)
		}
		encoded, _ := json.Marshal(multi)
		if strings.Contains(string(encoded), fx.ships[0].tenantID.String()) {
			t.Fatalf("departure response exposed shipment tenant %s", encoded)
		}
		if countWhere(t, env, "transport.transport_execution_commands", "idempotency_key=$1 AND command_name='DEPARTED_PICKUP'", "multi-depart-"+fx.execution.String()) != 1 {
			t.Fatal("multistop departure did not dispatch the execution command")
		}
	})

	t.Run("one execution returns at most current and next", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		user := bindFixtureDriver(t, env, fx)
		body := listStops(t, router, fx.operating, user)
		var payload struct {
			Current *struct {
				ExecutionID string `json:"executionId"`
			} `json:"current"`
			Next *struct {
				ExecutionID string `json:"executionId"`
			} `json:"next"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Current == nil || payload.Next == nil {
			t.Fatalf("expected two stops in one execution: %s", body)
		}
		if payload.Current.ExecutionID != fx.execution.String() || payload.Next.ExecutionID != fx.execution.String() {
			t.Fatalf("current/next left the execution: %s", body)
		}
		idle := bindExtraDriver(t, env, fx.operating, carrierOf(t, env, fx.execution))
		empty := listStops(t, router, fx.operating, idle)
		var idlePayload struct {
			Current *struct {
				ExecutionID string `json:"executionId"`
			} `json:"current"`
			Next *struct {
				ExecutionID string `json:"executionId"`
			} `json:"next"`
		}
		if err := json.Unmarshal([]byte(empty), &idlePayload); err != nil {
			t.Fatal(err)
		}
		if idlePayload.Current != nil || idlePayload.Next != nil {
			t.Fatalf("idle driver %s", empty)
		}
	})

	t.Run("multiple open executions fail closed", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
		user := bindFixtureDriver(t, env, fx)
		second := projectSecondExecution(t, env, fx)
		rec := listStopsRaw(t, router, fx.operating, user)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), domain.ReasonDriverExecutionAmbiguous) {
			t.Fatalf("list %d %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), fx.execution.String()) || strings.Contains(rec.Body.String(), second.String()) {
			t.Fatalf("conflict mixed executions: %s", rec.Body.String())
		}
		if _, err := stops.List(env.ctx, fx.operating, user); reason(err) != domain.ReasonDriverExecutionAmbiguous {
			t.Fatalf("reason %s", reason(err))
		}
	})

	t.Run("cancel remaining syncs driver tasks", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		skipStart(t, env, commands, fx, when)
		sys := fx.operatorCmd(domain.CommandCancelRemainingStops, -1, -1, "cancel-sync-"+fx.execution.String(), when.Add(time.Minute), 1, "OTHER")
		sys.ActorKind = domain.ActorKindSystem
		sys.ActorID = uuid.Nil
		if _, err := commands.Execute(env.ctx, sys); err != nil {
			t.Fatal(err)
		}
		if stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusPlanned || taskStatus(t, env, fx.stops[1]) != domain.StopStatusPlanned {
			t.Fatal("current stop was cancelled")
		}
		if stopStatusOf(t, env, fx.stops[2]) != domain.StopStatusCancelled || actionStatusOf(t, env, fx.actions[2][0]) != domain.ActionStatusCancelled {
			t.Fatal("future stop or action was not cancelled")
		}
		assertTaskMirrorsStop(t, env, fx.stops[2])
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1", fx.stops[3]) != 0 {
			t.Fatal("untasked end received a driver task")
		}
	})

	t.Run("cancel remaining rolls back when task sync fails", func(t *testing.T) {
		fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
			{{0, domain.ActionTypePickup}},
			{{0, domain.ActionTypeDelivery}},
		})
		if _, err := env.pool.Exec(env.ctx, `
			CREATE OR REPLACE FUNCTION transport.fail_cancel_remaining_task_sync() RETURNS trigger AS $$
			BEGIN RAISE EXCEPTION 'cancel remaining task sync failed'; END $$ LANGUAGE plpgsql
		`); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(env.ctx, `
			CREATE TRIGGER fail_cancel_remaining_task_sync
			BEFORE UPDATE ON transport.driver_stop_tasks
			FOR EACH ROW EXECUTE FUNCTION transport.fail_cancel_remaining_task_sync()
		`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = env.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS fail_cancel_remaining_task_sync ON transport.driver_stop_tasks`)
			_, _ = env.pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS transport.fail_cancel_remaining_task_sync()`)
		})
		skipStart(t, env, commands, fx, when)
		beforeActions := actionStatusOf(t, env, fx.actions[2][0])
		sys := fx.operatorCmd(domain.CommandCancelRemainingStops, -1, -1, "cancel-rollback-"+fx.execution.String(), when.Add(time.Minute), 1, "OTHER")
		sys.ActorKind = domain.ActorKindSystem
		sys.ActorID = uuid.Nil
		if _, err := commands.Execute(env.ctx, sys); err == nil {
			t.Fatal("cancel remaining committed after task sync failure")
		}
		if stopStatusOf(t, env, fx.stops[2]) != domain.StopStatusPlanned || taskStatus(t, env, fx.stops[2]) != domain.StopStatusPlanned {
			t.Fatal("stop and task diverged after rollback")
		}
		if actionStatusOf(t, env, fx.actions[2][0]) != beforeActions {
			t.Fatal("action cancellation survived the rollback")
		}
		if stopStatusOf(t, env, fx.stops[3]) != domain.StopStatusPlanned {
			t.Fatal("end stop cancellation survived the rollback")
		}
		if countWhere(t, env, "transport.transport_execution_commands", "idempotency_key=$1", "cancel-rollback-"+fx.execution.String()) != 0 {
			t.Fatal("command row survived the rollback")
		}
	})

	t.Run("departure sequence guard", func(t *testing.T) {
		occurred := "2026-09-29T12:00:00Z"

		t.Run("before pickup confirm", func(t *testing.T) {
			fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
			user := bindFixtureDriver(t, env, fx)
			skipStart(t, env, commands, fx, when)
			mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "dep-arrive-"+fx.execution.String(), when, stopVersion(t, env, fx.stops[1]), ""))
			mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "dep-start-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
			key := "depart-before-" + fx.execution.String()
			before := departureFootprint(t, env, fx.ships[0].shipmentID, key)
			rec := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if rec.Code != http.StatusConflict {
				t.Fatalf("depart %d %s", rec.Code, rec.Body.String())
			}
			if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInPickup {
				t.Fatal("shipment changed before pickup confirmation")
			}
			assertDepartureUnchanged(t, env, fx.ships[0].shipmentID, key, before)
		})

		t.Run("after confirm before stop complete", func(t *testing.T) {
			fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
			user := bindFixtureDriver(t, env, fx)
			skipStart(t, env, commands, fx, when)
			mustExec(t, commands, fx.driverCmd(domain.CommandArriveStop, 1, -1, "dep2-arrive-"+fx.execution.String(), when, stopVersion(t, env, fx.stops[1]), ""))
			mustExec(t, commands, fx.driverCmd(domain.CommandStartStopService, 1, -1, "dep2-start-"+fx.execution.String(), when.Add(time.Minute), stopVersion(t, env, fx.stops[1]), ""))
			mustExec(t, commands, fx.driverCmd(domain.CommandConfirmPickup, 1, 0, "dep2-confirm-"+fx.execution.String(), when.Add(2*time.Minute), stopVersion(t, env, fx.stops[1]), ""))
			if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusLoaded || stopStatusOf(t, env, fx.stops[1]) != domain.StopStatusServiceStarted {
				t.Fatal("fixture did not stop after pickup confirmation")
			}
			key := "depart-open-" + fx.execution.String()
			before := departureFootprint(t, env, fx.ships[0].shipmentID, key)
			rec := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if rec.Code != http.StatusConflict {
				t.Fatalf("depart %d %s", rec.Code, rec.Body.String())
			}
			if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusLoaded {
				t.Fatal("shipment left LOADED before the pickup stop completed")
			}
			assertDepartureUnchanged(t, env, fx.ships[0].shipmentID, key, before)
		})

		t.Run("after pickup stop complete", func(t *testing.T) {
			fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
			user := bindFixtureDriver(t, env, fx)
			skipStart(t, env, commands, fx, when)
			walkPickup(t, env, commands, fx, 1, when)
			key := "depart-ready-" + fx.execution.String()
			rec := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if rec.Code != http.StatusOK || shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInTransit {
				t.Fatalf("depart %d %s", rec.Code, rec.Body.String())
			}
			replay := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) {
				t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
			}
			if countWhere(t, env, "transport.transport_execution_commands", "idempotency_key=$1", key) != 1 {
				t.Fatal("replay wrote a second departure command")
			}
			if countWhere(t, env, "transport.shipment_status_history", "shipment_id=$1 AND to_status=$2", fx.ships[0].shipmentID, domain.ShipmentStatusInTransit) != 1 {
				t.Fatal("replay wrote a second status history row")
			}
			conflict := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, "2026-09-29T13:00:00Z")
			if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), domain.ReasonCommandBodyConflict) {
				t.Fatalf("conflict %d %s", conflict.Code, conflict.Body.String())
			}
			if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInTransit {
				t.Fatal("conflicting replay changed the shipment")
			}
			if countWhere(t, env, "transport.shipment_status_history", "shipment_id=$1 AND to_status=$2", fx.ships[0].shipmentID, domain.ShipmentStatusInTransit) != 1 {
				t.Fatal("conflicting replay wrote status history")
			}
		})

		t.Run("wrong revision", func(t *testing.T) {
			fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
			user := bindFixtureDriver(t, env, fx)
			skipStart(t, env, commands, fx, when)
			walkPickup(t, env, commands, fx, 1, when)
			stale := fx.driverCmd(domain.CommandDepartedPickup, -1, -1, "depart-stale-"+fx.execution.String(), when.Add(10*time.Minute), 1, "")
			stale.RevisionID = uuid.New()
			stale.ShipmentID = fx.ships[0].shipmentID
			stale.ShipmentTenantID = fx.ships[0].tenantID
			if reason(execCmd(commands, stale)) != domain.ReasonStaleRevision {
				t.Fatalf("reason %s", reason(execCmd(commands, stale)))
			}
			activateEmptyRevision(t, env, fx)
			key := "depart-wrong-rev-" + fx.execution.String()
			before := departureFootprint(t, env, fx.ships[0].shipmentID, key)
			rec := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if rec.Code != http.StatusConflict {
				t.Fatalf("depart %d %s", rec.Code, rec.Body.String())
			}
			if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusLoaded {
				t.Fatal("wrong revision departed the shipment")
			}
			assertDepartureUnchanged(t, env, fx.ships[0].shipmentID, key, before)
		})

		t.Run("superseded pickup", func(t *testing.T) {
			fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{{{0, domain.ActionTypePickup}}})
			user := bindFixtureDriver(t, env, fx)
			skipStart(t, env, commands, fx, when)
			walkPickup(t, env, commands, fx, 1, when)
			if _, err := env.pool.Exec(env.ctx, `
				UPDATE transport.transport_execution_revision_actions
				SET membership = 'SUPERSEDED'
				WHERE revision_id = $1 AND action_id = $2
			`, fx.revision, fx.actions[1][0]); err != nil {
				t.Fatal(err)
			}
			key := "depart-superseded-" + fx.execution.String()
			before := departureFootprint(t, env, fx.ships[0].shipmentID, key)
			rec := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if rec.Code != http.StatusConflict {
				t.Fatalf("depart %d %s", rec.Code, rec.Body.String())
			}
			if shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusLoaded {
				t.Fatal("superseded pickup departed the shipment")
			}
			assertDepartureUnchanged(t, env, fx.ships[0].shipmentID, key, before)
		})

		t.Run("cross shipper", func(t *testing.T) {
			fx := newFixture(t, env, []string{domain.ShipmentStatusPickupSlotBooked, domain.ShipmentStatusPickupSlotBooked}, [][]actionSpec{
				{{0, domain.ActionTypePickup}},
				{{1, domain.ActionTypePickup}},
			})
			user := bindFixtureDriver(t, env, fx)
			if fx.operating == fx.ships[0].tenantID || fx.ships[0].tenantID == fx.ships[1].tenantID {
				t.Fatal("fixture did not keep shipper tenants distinct from the carrier")
			}
			skipStart(t, env, commands, fx, when)
			walkPickup(t, env, commands, fx, 1, when)
			beforeB := shipmentStatus(t, env, fx.ships[1].shipmentID, fx.ships[1].tenantID)
			key := "depart-cross-" + fx.execution.String()
			rec := postDepart(t, router, fx.operating, user, fx.ships[0].shipmentID, key, occurred)
			if rec.Code != http.StatusOK || shipmentStatus(t, env, fx.ships[0].shipmentID, fx.ships[0].tenantID) != domain.ShipmentStatusInTransit {
				t.Fatalf("depart %d %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), fx.ships[0].tenantID.String()) || strings.Contains(rec.Body.String(), fx.ships[1].tenantID.String()) {
				t.Fatalf("response exposed a shipment tenant: %s", rec.Body.String())
			}
			if shipmentStatus(t, env, fx.ships[1].shipmentID, fx.ships[1].tenantID) != beforeB {
				t.Fatal("the other shipper changed")
			}
		})
	})

	t.Run("notice tasks unchanged", func(t *testing.T) {
		if countWhere(t, env, "transport.driver_task", "true") != 0 {
			t.Fatal("stop projection wrote notice tasks")
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/driver/me/tasks", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatal("notice task route missing")
		}
	})
}

func cargoRoute(t *testing.T, env *execEnv, driverID *uuid.UUID, ships []seededShipment, distinctEnd bool) domain.ProjectionCommand {
	t.Helper()
	operating := seedTenant(t, env, "route")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Route Carrier")
	version := 1
	subjects := make([]domain.ProjectionSubject, len(ships))
	for i, ship := range ships {
		subjects[i] = materialSubject(ship, version)
	}
	cargoStop := uuid.New()
	loc := ships[0].originID
	duration := 600
	stops := []domain.ProjectionStop{{
		RoutePlanStopID: uuid.New(), Ordinal: 0, StopRole: domain.StopRoleStart,
		PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6,
	}, {
		RoutePlanStopID: cargoStop, Ordinal: 1, StopRole: domain.StopRoleCargo,
		PointKind: domain.PointKindCanonicalLocation, LocationID: &loc,
		Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration,
	}}
	endLoc := loc
	if distinctEnd {
		endLoc = uuid.New()
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE','Distinct end','RU')`, endLoc, ships[0].tenantID); err != nil {
			t.Fatal(err)
		}
	}
	stops = append(stops, domain.ProjectionStop{
		RoutePlanStopID: uuid.New(), Ordinal: 2, StopRole: domain.StopRoleEnd,
		PointKind: domain.PointKindCanonicalLocation, LocationID: &endLoc, Latitude: 56, Longitude: 38,
	})
	actions := make([]domain.ProjectionAction, len(ships))
	for i, ship := range ships {
		action := materialAction(cargoStop, ship, version, i)
		if i == 1 {
			action.ActionType = domain.ActionTypeDelivery
		}
		actions[i] = action
	}
	return domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 2, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 4, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: operating, ContextShipmentID: &ships[0].shipmentID, ContextShipmentTenantID: &ships[0].tenantID,
		ContextShipmentVersion: &version, CarrierCompanyID: carrierID, DriverID: driverID,
		EvaluationFingerprint: "driver-stop-route", ExecutionSubjects: subjects, Stops: stops, Actions: actions,
	}
}

func bindFixtureDriver(t *testing.T, env *execEnv, fx execFixture) uuid.UUID {
	t.Helper()
	return insertDriver(t, env, fx.driver, fx.operating, carrierOf(t, env, fx.execution), uuid.New())
}

func bindExtraDriver(t *testing.T, env *execEnv, tenantID, carrierID uuid.UUID) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	insertDriver(t, env, uuid.New(), tenantID, carrierID, userID)
	return userID
}

func insertDriver(t *testing.T, env *execEnv, driverID, tenantID, carrierID, userID uuid.UUID) uuid.UUID {
	t.Helper()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,'Route Driver','ACTIVE')
	`, driverID, tenantID, carrierID, userID); err != nil {
		t.Fatal(err)
	}
	return userID
}

func carrierOf(t *testing.T, env *execEnv, executionID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT carrier_company_id FROM transport.transport_executions WHERE id=$1`, executionID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func taskStatus(t *testing.T, env *execEnv, stopID uuid.UUID) string {
	t.Helper()
	var status string
	if err := env.pool.QueryRow(env.ctx, `SELECT status FROM transport.driver_stop_tasks WHERE execution_stop_id=$1`, stopID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func assertTaskMirrorsStop(t *testing.T, env *execEnv, stopID uuid.UUID) {
	t.Helper()
	var stopStatus, taskState string
	var stopVersion, taskVersion int
	if err := env.pool.QueryRow(env.ctx, `
		SELECT stop.status, stop.version, task.status, task.version
		FROM transport.transport_execution_stops AS stop
		JOIN transport.driver_stop_tasks AS task ON task.execution_stop_id = stop.id
		WHERE stop.id = $1
	`, stopID).Scan(&stopStatus, &stopVersion, &taskState, &taskVersion); err != nil {
		t.Fatal(err)
	}
	if stopStatus != taskState || stopVersion != taskVersion {
		t.Fatalf("stop %s/%d task %s/%d", stopStatus, stopVersion, taskState, taskVersion)
	}
}

func assertMultiShipmentActions(t *testing.T, router http.Handler, fx execFixture, user uuid.UUID) string {
	t.Helper()
	body := listStops(t, router, fx.operating, user)
	var payload struct {
		Current *struct {
			ShipmentID    *string `json:"shipmentId"`
			ActionSummary struct {
				Actions []struct {
					ActionType string `json:"actionType"`
					ShipmentID string `json:"shipmentId"`
					CargoID    string `json:"cargoId"`
				} `json:"actions"`
			} `json:"actionSummary"`
		} `json:"current"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Current == nil || payload.Current.ShipmentID != nil {
		t.Fatalf("stop-level shipment id = %v body %s", payload.Current, body)
	}
	found := map[string]string{}
	for _, action := range payload.Current.ActionSummary.Actions {
		if action.ActionType != domain.ActionTypeDelivery || action.ShipmentID == "" || action.ShipmentID == uuid.Nil.String() {
			t.Fatalf("action identity %s", body)
		}
		found[action.ShipmentID] = action.CargoID
	}
	if found[fx.ships[0].shipmentID.String()] != fx.ships[0].cargoID.String() {
		t.Fatalf("action A %+v body %s", found, body)
	}
	if found[fx.ships[1].shipmentID.String()] != fx.ships[1].cargoID.String() {
		t.Fatalf("action B %+v body %s", found, body)
	}
	return body
}

func listStops(t *testing.T, router http.Handler, tenantID, userID uuid.UUID) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/driver/me/stops", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func postStop(t *testing.T, router http.Handler, tenantID, userID, stopID uuid.UUID, action, actionID, key string, version int) *httptest.ResponseRecorder {
	t.Helper()
	path := "/v1/driver/me/stops/" + stopID.String() + "/" + action
	if action == "confirm" || action == "fail" {
		path = "/v1/driver/me/stops/" + stopID.String() + "/actions/" + actionID + "/" + action
	}
	body := []byte(`{"expectedVersion":` + strconv.Itoa(version) + `,"occurredAt":"2026-09-29T09:30:00Z"}`)
	if action == "fail" {
		body = []byte(`{"expectedVersion":` + strconv.Itoa(version) + `,"occurredAt":"2026-09-29T09:30:00Z","reasonCode":"CARGO_ISSUE"}`)
	}
	return postStopRaw(t, router, tenantID, userID, path, key, body)
}

func postStopRaw(t *testing.T, router http.Handler, tenantID, userID uuid.UUID, path, key string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }

func listStopsRaw(t *testing.T, router http.Handler, tenantID, userID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/driver/me/stops", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func projectSecondExecution(t *testing.T, env *execEnv, fx execFixture) uuid.UUID {
	t.Helper()
	ship := seedShipment(t, env, "second-route", domain.ShipmentStatusPickupSlotBooked)
	driverID := fx.driver
	cmd := cargoRoute(t, env, &driverID, []seededShipment{ship}, false)
	cmd.OperatingTenantID = fx.operating
	cmd.CarrierCompanyID = carrierOf(t, env, fx.execution)
	cmd.DriverID = &driverID
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	return result.ExecutionID
}

func postDepart(t *testing.T, router http.Handler, tenantID, userID, shipmentID uuid.UUID, key, occurred string) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(`{"type":"DEPARTED_PICKUP","occurredAt":"` + occurred + `","idempotencyKey":"` + key + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/driver/me/shipments/"+shipmentID.String()+"/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type departureCounts struct {
	commands int
	history  int
	outbox   int
}

func departureFootprint(t *testing.T, env *execEnv, shipmentID uuid.UUID, key string) departureCounts {
	t.Helper()
	return departureCounts{
		commands: countWhere(t, env, "transport.transport_execution_commands", "idempotency_key=$1", key),
		history:  countWhere(t, env, "transport.shipment_status_history", "shipment_id=$1 AND to_status=$2", shipmentID, domain.ShipmentStatusInTransit),
		outbox:   countWhere(t, env, "transport.shipment_event_outbox", "aggregate_id=$1", shipmentID),
	}
}

func assertDepartureUnchanged(t *testing.T, env *execEnv, shipmentID uuid.UUID, key string, before departureCounts) {
	t.Helper()
	after := departureFootprint(t, env, shipmentID, key)
	if after != before {
		t.Fatalf("departure committed commands/history/outbox before=%+v after=%+v", before, after)
	}
}

func activateEmptyRevision(t *testing.T, env *execEnv, fx execFixture) {
	t.Helper()
	nextID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			supersedes_revision_id, status, version, contract_sha256, created_at, updated_at
		)
		SELECT $1, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			$2, source_activation_version, evaluation_fingerprint, planning_mode,
			id, 'SUPERSEDED', 1, contract_sha256, now(), now()
		FROM transport.transport_execution_revisions
		WHERE id = $3
	`, nextID, uuid.New(), fx.revision); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_revisions SET status = 'SUPERSEDED' WHERE id = $1`, fx.revision); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_revisions SET status = 'ACTIVE' WHERE id = $1`, nextID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_executions SET current_revision_id = $2 WHERE id = $1`, fx.execution, nextID); err != nil {
		t.Fatal(err)
	}
}
