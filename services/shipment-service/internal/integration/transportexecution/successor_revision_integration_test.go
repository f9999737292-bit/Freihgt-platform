//go:build integration

package transportexecution

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestSuccessorRevision(t *testing.T) {
	env := startPostgres(t)
	repo := repository.NewTransportExecutionRepository(env.pool)

	t.Run("successor keeps completed history", func(t *testing.T) {
		seed := seedSuccessorRoute(t, env)
		completeStop(t, env, seed.endID)
		arriveStop(t, env, seed.cargoID)
		completeAction(t, env, seed.actionA)
		beforeSeq := eventSeq(t, env, seed.executionID)
		beforeEnd := stopFact(t, env, seed.endID)
		beforeAction := actionFact(t, env, seed.actionA)
		beforeCargo := stopFact(t, env, seed.cargoID)
		statusA := shipmentStatus(t, env, seed.shipA.shipmentID, seed.shipA.tenantID)
		statusB := shipmentStatus(t, env, seed.shipB.shipmentID, seed.shipB.tenantID)
		participants := countWhere(t, env, "transport.transport_execution_participants", "execution_id=$1", seed.executionID)
		cargoTask := optionalTaskStatus(t, env, seed.cargoID)
		endTask := optionalTaskStatus(t, env, seed.endID)

		cmd := successorCommand(seed, "replan-1")
		first, err := repo.CreateSuccessorRevision(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if first.ExecutionID != seed.executionID || first.PreviousRevisionID != seed.revisionID || first.RevisionID == seed.revisionID || first.Replayed {
			t.Fatalf("result %+v", first)
		}
		if currentRevision(t, env, seed.executionID) != first.RevisionID {
			t.Fatal("current revision was not switched")
		}
		if revisionStatus(t, env, seed.revisionID) != "SUPERSEDED" || revisionStatus(t, env, first.RevisionID) != "ACTIVE" {
			t.Fatal("revision statuses")
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1 AND status='ACTIVE'", seed.executionID) != 1 {
			t.Fatal("active revision count")
		}
		if stopFact(t, env, seed.endID) != beforeEnd || actionFact(t, env, seed.actionA) != beforeAction {
			t.Fatal("completed facts changed")
		}
		if stopFact(t, env, seed.cargoID).status != "ARRIVED" || stopFact(t, env, seed.cargoID).arrivedAt != beforeCargo.arrivedAt {
			t.Fatal("arrived stop was rewritten")
		}
		if membership(t, env, "transport.transport_execution_revision_stops", seed.revisionID, seed.endID) != "INTRODUCED" {
			t.Fatal("old completed stop membership changed")
		}
		if membership(t, env, "transport.transport_execution_revision_stops", first.RevisionID, seed.endID) != "INHERITED_COMPLETED" {
			t.Fatal("completed stop was not inherited")
		}
		if membership(t, env, "transport.transport_execution_revision_stops", seed.revisionID, seed.startID) != "SUPERSEDED" ||
			membership(t, env, "transport.transport_execution_revision_stops", seed.revisionID, seed.cargoID) != "SUPERSEDED" {
			t.Fatal("open stops were not superseded")
		}
		if membership(t, env, "transport.transport_execution_revision_actions", seed.revisionID, seed.actionA) != "INTRODUCED" ||
			membership(t, env, "transport.transport_execution_revision_actions", first.RevisionID, seed.actionA) != "INHERITED_COMPLETED" {
			t.Fatal("completed action membership")
		}
		if membership(t, env, "transport.transport_execution_revision_actions", seed.revisionID, seed.actionB) != "SUPERSEDED" {
			t.Fatal("open action was not superseded")
		}
		if countWhere(t, env, "transport.transport_execution_actions", "id=$1", seed.actionA) != 1 {
			t.Fatal("completed action was cloned")
		}
		if first.CurrentStopID == seed.endID || first.CurrentStopID == seed.cargoID || first.CurrentStopID == seed.startID {
			t.Fatal("current stop points at history")
		}
		if stopFact(t, env, first.CurrentStopID).status != "PLANNED" {
			t.Fatal("current stop is not the new planned stop")
		}
		if countWhere(t, env, "transport.transport_execution_participants", "execution_id=$1", seed.executionID) != participants {
			t.Fatal("participants changed")
		}
		if shipmentStatus(t, env, seed.shipA.shipmentID, seed.shipA.tenantID) != statusA || shipmentStatus(t, env, seed.shipB.shipmentID, seed.shipB.tenantID) != statusB {
			t.Fatal("shipment status changed")
		}
		if cargoTask != "" && optionalTaskStatus(t, env, seed.cargoID) != "CANCELLED" {
			t.Fatal("open stop task was not cancelled")
		}
		if endTask != "" && optionalTaskStatus(t, env, seed.endID) != endTask {
			t.Fatal("completed stop task changed")
		}
		if countWhere(t, env, "transport.driver_stop_tasks", "execution_stop_id=$1 AND status='PLANNED'", first.CurrentStopID) != 1 {
			t.Fatal("successor stop has no current driver task")
		}

		superseded := outboxPayloads(t, env, seed.executionID, domain.EventExecutionPlanSuperseded)
		if len(superseded) != 1 {
			t.Fatalf("superseded count %d", len(superseded))
		}
		if superseded[0]["old_revision_id"] != seed.revisionID.String() || superseded[0]["new_revision_id"] != first.RevisionID.String() || superseded[0]["revision_id"] != seed.revisionID.String() {
			t.Fatalf("superseded identity %+v", superseded[0])
		}
		created := outboxPayloads(t, env, seed.executionID, domain.EventExecutionPlanCreated)
		successorCreated := 0
		var successorSeq float64
		for _, payload := range created {
			if payload["revision_id"] == first.RevisionID.String() {
				successorCreated++
				successorSeq = payload["event_sequence"].(float64)
				raw, _ := json.Marshal(payload)
				if bytes.Contains(raw, []byte(seed.endID.String())) || bytes.Contains(raw, []byte(seed.cargoID.String())) {
					t.Fatal("successor plan cloned historical stops")
				}
			}
		}
		if successorCreated != 1 {
			t.Fatalf("successor created count %d", successorCreated)
		}
		supersededSeq := superseded[0]["event_sequence"].(float64)
		if supersededSeq <= float64(beforeSeq) || successorSeq <= supersededSeq {
			t.Fatalf("sequence before %d superseded %v created %v", beforeSeq, supersededSeq, successorSeq)
		}
		assertPrivacy(t, seed, superseded[0])
		for _, payload := range created {
			if payload["revision_id"] == first.RevisionID.String() {
				assertPrivacy(t, seed, payload)
			}
		}

		replay, err := repo.CreateSuccessorRevision(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if !replay.Replayed || replay.RevisionID != first.RevisionID || replay.CurrentStopID != first.CurrentStopID {
			t.Fatalf("replay %+v", replay)
		}
		if countWhere(t, env, "transport.transport_execution_stops", "execution_id=$1", seed.executionID) != 4 {
			t.Fatal("replay inserted stops")
		}
		if len(outboxPayloads(t, env, seed.executionID, domain.EventExecutionPlanSuperseded)) != 1 {
			t.Fatal("replay inserted superseded event")
		}

		conflictCmd := cmd
		conflictCmd.ReasonCode = "OTHER_REASON"
		if _, err := repo.CreateSuccessorRevision(env.ctx, conflictCmd); reasonOf(err) != domain.ReasonCommandBodyConflict {
			t.Fatalf("payload conflict %v", err)
		}

		stale := successorCommand(seed, "stale")
		stale.ExpectedRevisionID = seed.revisionID
		stale.SourceActivationID = uuid.New()
		if _, err := repo.CreateSuccessorRevision(env.ctx, stale); reasonOf(err) != domain.ReasonRevisionConflict {
			t.Fatalf("stale revision %v", err)
		}

		dupActivation := successorCommand(seededRoute{executionID: seed.executionID, revisionID: first.RevisionID, shipB: seed.shipB}, "dup-activation")
		dupActivation.SourceActivationID = cmd.SourceActivationID
		if _, err := repo.CreateSuccessorRevision(env.ctx, dupActivation); err == nil {
			t.Fatal("duplicate activation committed")
		}
		if currentRevision(t, env, seed.executionID) != first.RevisionID || revisionStatus(t, env, first.RevisionID) != "ACTIVE" {
			t.Fatal("failed successor mutated the active revision")
		}
	})

	t.Run("service started rejects without mutation", func(t *testing.T) {
		seed := seedSuccessorRoute(t, env)
		serviceStop(t, env, seed.cargoID)
		before := currentRevision(t, env, seed.executionID)
		events := len(outboxPayloads(t, env, seed.executionID, domain.EventExecutionPlanSuperseded))
		cmd := successorCommand(seed, "in-service")
		_, err := repo.CreateSuccessorRevision(env.ctx, cmd)
		if reasonOf(err) != domain.ReasonExecutionStopInService {
			t.Fatalf("err %v", err)
		}
		if currentRevision(t, env, seed.executionID) != before || revisionStatus(t, env, before) != "ACTIVE" {
			t.Fatal("rejected replan changed the revision")
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID) != 1 {
			t.Fatal("rejected replan created a revision")
		}
		if len(outboxPayloads(t, env, seed.executionID, domain.EventExecutionPlanSuperseded)) != events {
			t.Fatal("rejected replan emitted superseded")
		}
		if membership(t, env, "transport.transport_execution_revision_stops", seed.revisionID, seed.cargoID) != "INTRODUCED" {
			t.Fatal("rejected replan changed membership")
		}
	})

	t.Run("concurrent replan has one winner", func(t *testing.T) {
		seed := seedSuccessorRoute(t, env)
		var wg sync.WaitGroup
		start := make(chan struct{})
		results := make([]domain.SuccessorRevisionResult, 2)
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				cmd := successorCommand(seed, "race-"+string(rune('a'+i)))
				cmd.SourceActivationID = uuid.New()
				results[i], errs[i] = repo.CreateSuccessorRevision(env.ctx, cmd)
			}(i)
		}
		close(start)
		wg.Wait()
		wins := 0
		conflicts := 0
		var winner uuid.UUID
		for i := 0; i < 2; i++ {
			if errs[i] == nil {
				wins++
				winner = results[i].RevisionID
				continue
			}
			if reasonOf(errs[i]) == domain.ReasonRevisionConflict {
				conflicts++
			}
		}
		if wins != 1 || conflicts != 1 || winner == uuid.Nil {
			t.Fatalf("wins %d conflicts %d errs %v", wins, conflicts, errs)
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID) != 2 {
			t.Fatal("concurrent replan created the wrong number of revisions")
		}
		if len(outboxPayloads(t, env, seed.executionID, domain.EventExecutionPlanSuperseded)) != 1 {
			t.Fatal("concurrent replan emitted more than one superseded event")
		}
	})

	t.Run("foreign tenant and execution", func(t *testing.T) {
		seed := seedSuccessorRoute(t, env)
		cmd := successorCommand(seed, "foreign-tenant")
		cmd.OperatingTenantID = uuid.New()
		if _, err := repo.CreateSuccessorRevision(env.ctx, cmd); reasonOf(err) != domain.ReasonTenantDenied {
			t.Fatalf("foreign tenant %v", err)
		}
		missing := cmd
		missing.OperatingTenantID = seed.operating
		missing.ExecutionID = uuid.New()
		_, err := repo.CreateSuccessorRevision(env.ctx, missing)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeNotFound {
			t.Fatalf("foreign execution %v", err)
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", seed.executionID) != 1 {
			t.Fatal("denied replan created a revision")
		}
	})

	t.Run("http uses verified tenant", func(t *testing.T) {
		seed := seedSuccessorRoute(t, env)
		router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "successor-token", repo)
		cmd := successorCommand(seed, "http-1")
		body := map[string]any{
			"operatingTenantId":         uuid.NewString(),
			"currentStopId":             uuid.NewString(),
			"expectedCurrentRevisionId": cmd.ExpectedRevisionID.String(),
			"idempotencyKey":            cmd.IdempotencyKey,
			"reasonCode":                cmd.ReasonCode,
			"occurredAt":                cmd.OccurredAt.Format(time.RFC3339Nano),
			"actorKind":                 cmd.ActorKind,
			"actorId":                   cmd.ActorID.String(),
			"sourceRoutePlanId":         cmd.SourceRoutePlanID.String(),
			"sourceRoutePlanVersion":    cmd.SourceRoutePlanVersion,
			"sourceActivationId":        cmd.SourceActivationID.String(),
			"sourceActivationVersion":   cmd.SourceActivationVersion,
			"planningMode":              cmd.PlanningMode,
			"evaluationFingerprint":     cmd.EvaluationFingerprint,
			"stops": []map[string]any{{
				"stopRole": "CARGO", "pointKind": "CANONICAL_LOCATION", "locationId": seed.shipB.originID.String(),
				"latitude": cmd.Stops[0].Latitude, "longitude": cmd.Stops[0].Longitude,
			}},
			"actions": []map[string]any{{
				"stopIndex": 0, "actionType": "DELIVERY", "shipmentId": seed.shipB.shipmentID.String(), "cargoId": seed.shipB.cargoID.String(),
				"shipmentTenantId": uuid.NewString(),
			}},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/"+seed.executionID.String()+"/successor-revision", bytes.NewReader(raw))
		req.Header.Set("X-Internal-Service-Token", "successor-token")
		req.Header.Set("X-Tenant-ID", seed.operating.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response["executionId"] != seed.executionID.String() || response["currentStopId"] == body["currentStopId"] {
			t.Fatalf("response %+v", response)
		}
		assertPrivacy(t, seed, outboxPayload(t, env, seed.executionID, domain.EventExecutionPlanSuperseded))
	})
}

type seededRoute struct {
	operating   uuid.UUID
	executionID uuid.UUID
	revisionID  uuid.UUID
	shipA       seededShipment
	shipB       seededShipment
	startID     uuid.UUID
	cargoID     uuid.UUID
	endID       uuid.UUID
	actionA     uuid.UUID
	actionB     uuid.UUID
}

func seedSuccessorRoute(t *testing.T, env *execEnv) seededRoute {
	t.Helper()
	operating := seedTenant(t, env, "carrier")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Carrier C")
	shipA := seedShipment(t, env, "shipper-a", "IN_TRANSIT")
	shipB := seedShipment(t, env, "shipper-b", "IN_TRANSIT")
	cmd := routeCommand(operating, carrierID, shipA, shipB)
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := env.pool.Query(env.ctx, `
		SELECT s.id, s.stop_role
		FROM transport.transport_execution_stops s
		WHERE s.execution_id = $1
		ORDER BY s.ordinal
	`, result.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var startID, cargoID, endID uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var role string
		if err := rows.Scan(&id, &role); err != nil {
			t.Fatal(err)
		}
		switch role {
		case "START":
			startID = id
		case "CARGO":
			cargoID = id
		case "END":
			endID = id
		}
	}
	var actionA, actionB uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 AND shipment_id=$2`, cargoID, shipA.shipmentID).Scan(&actionA); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 AND shipment_id=$2`, cargoID, shipB.shipmentID).Scan(&actionB); err != nil {
		t.Fatal(err)
	}
	return seededRoute{
		operating: operating, executionID: result.ExecutionID, revisionID: result.RevisionID,
		shipA: shipA, shipB: shipB, startID: startID, cargoID: cargoID, endID: endID, actionA: actionA, actionB: actionB,
	}
}

func successorCommand(seed seededRoute, key string) domain.SuccessorRevisionCommand {
	return domain.SuccessorRevisionCommand{
		ExecutionID: seed.executionID, ExpectedRevisionID: seed.revisionID, OperatingTenantID: seed.operating,
		IdempotencyKey: key, ReasonCode: "REMAINING_ROUTE_CHANGED",
		OccurredAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		ActorKind:  domain.ActorKindOperator, ActorID: uuid.New(),
		SourceRoutePlanID: uuid.New(), SourceRoutePlanVersion: 2,
		SourceActivationID: uuid.New(), SourceActivationVersion: 1,
		PlanningMode: domain.PlanningModeCurrentTrip, EvaluationFingerprint: "optimizer-internal-score-9",
		Stops: []domain.SuccessorStopInput{{
			StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &seed.shipB.originID,
			Latitude: 41.40338, Longitude: 2.17403,
		}},
		Actions: []domain.SuccessorActionInput{{
			StopIndex: 0, ActionType: domain.ActionTypeDelivery, ShipmentID: seed.shipB.shipmentID, CargoID: seed.shipB.cargoID,
		}},
	}
}

func arriveStop(t *testing.T, env *execEnv, stopID uuid.UUID) {
	t.Helper()
	at := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_stops SET status='ARRIVED', arrived_at=$2 WHERE id=$1 AND status='PLANNED'`, stopID, at); err != nil {
		t.Fatal(err)
	}
}

func serviceStop(t *testing.T, env *execEnv, stopID uuid.UUID) {
	t.Helper()
	arriveStop(t, env, stopID)
	at := time.Date(2026, 10, 1, 8, 5, 0, 0, time.UTC)
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_stops SET status='SERVICE_STARTED', service_started_at=$2 WHERE id=$1 AND status='ARRIVED'`, stopID, at); err != nil {
		t.Fatal(err)
	}
}

func completeStop(t *testing.T, env *execEnv, stopID uuid.UUID) {
	t.Helper()
	serviceStop(t, env, stopID)
	at := time.Date(2026, 10, 1, 8, 20, 0, 0, time.UTC)
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_stops SET status='COMPLETED', completed_at=$2 WHERE id=$1 AND status='SERVICE_STARTED'`, stopID, at); err != nil {
		t.Fatal(err)
	}
}

func completeAction(t *testing.T, env *execEnv, actionID uuid.UUID) {
	t.Helper()
	at := time.Date(2026, 10, 1, 8, 15, 0, 0, time.UTC)
	if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_actions SET status='COMPLETED', completed_at=$2 WHERE id=$1 AND status='PENDING'`, actionID, at); err != nil {
		t.Fatal(err)
	}
}

type factSnapshot struct {
	status      string
	ordinal     int
	executionID string
	arrivedAt   string
	serviceAt   string
	completedAt string
	updatedAt   string
	parentID    string
}

func stopFact(t *testing.T, env *execEnv, stopID uuid.UUID) factSnapshot {
	t.Helper()
	var snap factSnapshot
	var arrived, service, completed, updated *time.Time
	if err := env.pool.QueryRow(env.ctx, `
		SELECT status, ordinal, execution_id::text, arrived_at, service_started_at, completed_at, updated_at
		FROM transport.transport_execution_stops WHERE id=$1
	`, stopID).Scan(&snap.status, &snap.ordinal, &snap.executionID, &arrived, &service, &completed, &updated); err != nil {
		t.Fatal(err)
	}
	snap.arrivedAt = formatFactTime(arrived)
	snap.serviceAt = formatFactTime(service)
	snap.completedAt = formatFactTime(completed)
	snap.updatedAt = formatFactTime(updated)
	return snap
}

func actionFact(t *testing.T, env *execEnv, actionID uuid.UUID) factSnapshot {
	t.Helper()
	var snap factSnapshot
	var completed *time.Time
	var stopID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `
		SELECT status, ordinal, execution_stop_id, completed_at
		FROM transport.transport_execution_actions WHERE id=$1
	`, actionID).Scan(&snap.status, &snap.ordinal, &stopID, &completed); err != nil {
		t.Fatal(err)
	}
	snap.parentID = stopID.String()
	snap.completedAt = formatFactTime(completed)
	return snap
}

func formatFactTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func membership(t *testing.T, env *execEnv, table string, revisionID, id uuid.UUID) string {
	t.Helper()
	column := "stop_id"
	if table == "transport.transport_execution_revision_actions" {
		column = "action_id"
	}
	var value string
	if err := env.pool.QueryRow(env.ctx, "SELECT membership FROM "+table+" WHERE revision_id=$1 AND "+column+"=$2", revisionID, id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func currentRevision(t *testing.T, env *execEnv, executionID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT current_revision_id FROM transport.transport_executions WHERE id=$1`, executionID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func revisionStatus(t *testing.T, env *execEnv, revisionID uuid.UUID) string {
	t.Helper()
	var status string
	if err := env.pool.QueryRow(env.ctx, `SELECT status FROM transport.transport_execution_revisions WHERE id=$1`, revisionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func eventSeq(t *testing.T, env *execEnv, executionID uuid.UUID) int64 {
	t.Helper()
	var seq int64
	if err := env.pool.QueryRow(env.ctx, `SELECT event_seq FROM transport.transport_executions WHERE id=$1`, executionID).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	return seq
}

func optionalTaskStatus(t *testing.T, env *execEnv, stopID uuid.UUID) string {
	t.Helper()
	var status string
	err := env.pool.QueryRow(env.ctx, `SELECT status FROM transport.driver_stop_tasks WHERE execution_stop_id=$1`, stopID).Scan(&status)
	if err != nil {
		return ""
	}
	return status
}

func reasonOf(err error) string {
	var app *apperrors.AppError
	if !errors.As(err, &app) {
		return ""
	}
	if reason, ok := app.Details["reason"].(string); ok {
		return reason
	}
	return app.Message
}

func assertPrivacy(t *testing.T, seed seededRoute, payload map[string]any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{
		seed.shipA.tenantID.String(), seed.shipB.tenantID.String(),
		"latitude", "longitude", "41.40338", "2.17403",
		"optimizer-internal-score-9", "price", "rate", "capacity_snapshot", "leg_geometry",
	} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("payload leaked %s in %s", forbidden, text)
		}
	}
}
