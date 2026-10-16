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
)

func TestPODRequirementFoundation(t *testing.T) {
	env := startPostgres(t)
	up := filepath.Join(env.migrations, "000098_tms_pod_requirement_decision_foundation_v0_4d.up.sql")
	down := filepath.Join(env.migrations, "000098_tms_pod_requirement_decision_foundation_v0_4d.down.sql")

	t.Run("migration up down up", func(t *testing.T) {
		if err := execSQLFile(env.ctx, env.pool, up); err != nil {
			t.Fatal(err)
		}
		if !tableExists(t, env, "transport_execution_pod_requirement_decisions") {
			t.Fatal("up did not create the decision table")
		}
		if err := execSQLFile(env.ctx, env.pool, down); err != nil {
			t.Fatal(err)
		}
		if tableExists(t, env, "transport_execution_pod_requirement_decisions") {
			t.Fatal("down left the decision table")
		}
		if err := execSQLFile(env.ctx, env.pool, up); err != nil {
			t.Fatal(err)
		}
		if !tableExists(t, env, "transport_execution_pod_requirement_decisions") {
			t.Fatal("second up did not create the decision table")
		}
	})

	seed := seedDelivery(t, env, 4, true)
	action := deliveryActionID(t, env, seed.stopID)
	router := podRouter(env)
	token := "pod-token"

	t.Run("constraints reject illegal decisions", func(t *testing.T) {
		local := seedDelivery(t, env, 4, true)
		localAction := deliveryActionID(t, env, local.stopID)
		base := decisionInsert{
			Operating: local.operating, Execution: local.executionID, Action: localAction, Stop: local.stopID,
			Shipment: local.ship.shipmentID, Cargo: local.ship.cargoID, Revision: &local.revisionID,
		}
		expectDecisionReject(t, env, base.withState("UNKNOWN", boolPtr(false), nil, nil, nil, nil))
		expectDecisionReject(t, env, base.withState("UNKNOWN", boolPtr(true), nil, nil, nil, nil))
		expectDecisionReject(t, env, base.withState("KNOWN", nil, nil, nil, nil, timePtr(time.Now().UTC())))
		expectDecisionReject(t, env, base.withState("KNOWN", boolPtr(true), nil, uuidPtr(uuid.New()), intPtr(1), timePtr(time.Now().UTC())))
		expectDecisionReject(t, env, base.withState("KNOWN", boolPtr(true), strPtr("POD"), nil, intPtr(1), timePtr(time.Now().UTC())))
		expectDecisionReject(t, env, base.withState("KNOWN", boolPtr(true), strPtr("POD"), uuidPtr(uuid.New()), nil, timePtr(time.Now().UTC())))
		expectDecisionReject(t, env, base.withState("KNOWN", boolPtr(false), strPtr("POD"), nil, nil, timePtr(time.Now().UTC())))
		policy := uuid.New()
		expectDecisionReject(t, env, base.withState("KNOWN", boolPtr(false), nil, &policy, intPtr(1), timePtr(time.Now().UTC())))

		first := base.withState("UNKNOWN", nil, nil, nil, nil, nil)
		first.ID = uuid.New()
		first.Version = 1
		insertDecision(t, env, first)
		dup := first
		dup.ID = uuid.New()
		expectDecisionReject(t, env, dup)

		self := base.withState("UNKNOWN", nil, nil, nil, nil, nil)
		self.ID = uuid.New()
		self.Version = 2
		self.Supersedes = &self.ID
		expectDecisionReject(t, env, self)

		otherAction := insertSecondDelivery(t, env, local)
		cross := base.withState("UNKNOWN", nil, nil, nil, nil, nil)
		cross.ID = uuid.New()
		cross.Action = otherAction
		cross.Version = 1
		cross.Supersedes = &first.ID
		expectDecisionReject(t, env, cross)

		if _, err := env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_pod_requirement_decisions SET decision_state='KNOWN' WHERE id=$1`, first.ID); err == nil || !strings.Contains(err.Error(), "POD_REQUIREMENT_DECISION_IMMUTABLE") {
			t.Fatalf("update err = %v", err)
		}
	})

	t.Run("POD_FOUNDATION_01 missing decision is unknown", func(t *testing.T) {
		body := getPOD(t, router, token, seed.operating, seed.executionID, action)
		assertUnknownNoRow(t, body, seed, action)
	})

	t.Run("POD_FOUNDATION_02 read does not insert", func(t *testing.T) {
		before := countWhere(t, env, "transport.transport_execution_pod_requirement_decisions", "execution_action_id=$1", action)
		getPOD(t, router, token, seed.operating, seed.executionID, action)
		getPOD(t, router, token, seed.operating, seed.executionID, action)
		after := countWhere(t, env, "transport.transport_execution_pod_requirement_decisions", "execution_action_id=$1", action)
		if before != after {
			t.Fatalf("decision rows %d -> %d", before, after)
		}
	})

	t.Run("POD_FOUNDATION_03 stored unknown", func(t *testing.T) {
		fresh := seedDelivery(t, env, 2, true)
		freshAction := deliveryActionID(t, env, fresh.stopID)
		row := decisionInsert{
			ID: uuid.New(), Version: 3, Operating: fresh.operating, Execution: fresh.executionID, Action: freshAction,
			Stop: fresh.stopID, Shipment: fresh.ship.shipmentID, Cargo: fresh.ship.cargoID, Revision: &fresh.revisionID,
			State: "UNKNOWN",
		}
		insertDecision(t, env, row)
		body := getPOD(t, router, token, fresh.operating, fresh.executionID, freshAction)
		if body["decision_state"] != "UNKNOWN" || body["pod_required"] != nil {
			t.Fatalf("body %+v", body)
		}
		if body["pod_requirement_decision_id"] != row.ID.String() || body["decision_version"] != float64(3) {
			t.Fatalf("identity %+v", body)
		}
		if body["execution_revision_id"] != fresh.revisionID.String() {
			t.Fatalf("revision %+v", body["execution_revision_id"])
		}
	})

	t.Run("POD_FOUNDATION_04 stored known required", func(t *testing.T) {
		fresh := seedDelivery(t, env, 2, true)
		freshAction := deliveryActionID(t, env, fresh.stopID)
		policy := uuid.New()
		decided := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
		row := decisionInsert{
			ID: uuid.New(), Version: 1, Operating: fresh.operating, Execution: fresh.executionID, Action: freshAction,
			Stop: fresh.stopID, Shipment: fresh.ship.shipmentID, Cargo: fresh.ship.cargoID,
			State: "KNOWN", PodRequired: boolPtr(true), DocType: strPtr("POD"), Policy: &policy, PolicyVersion: intPtr(2), DecidedAt: &decided,
		}
		insertDecision(t, env, row)
		body := getPOD(t, router, token, fresh.operating, fresh.executionID, freshAction)
		if body["decision_state"] != "KNOWN" || body["pod_required"] != true || body["required_document_type"] != "POD" || body["acceptance_policy_id"] != policy.String() || body["acceptance_policy_version"] != float64(2) {
			t.Fatalf("body %+v", body)
		}
	})

	t.Run("POD_FOUNDATION_05 stored known not required", func(t *testing.T) {
		fresh := seedDelivery(t, env, 2, true)
		freshAction := deliveryActionID(t, env, fresh.stopID)
		decided := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
		row := decisionInsert{
			ID: uuid.New(), Version: 1, Operating: fresh.operating, Execution: fresh.executionID, Action: freshAction,
			Stop: fresh.stopID, Shipment: fresh.ship.shipmentID, Cargo: fresh.ship.cargoID,
			State: "KNOWN", PodRequired: boolPtr(false), DecidedAt: &decided,
		}
		insertDecision(t, env, row)
		body := getPOD(t, router, token, fresh.operating, fresh.executionID, freshAction)
		if body["decision_state"] != "KNOWN" || body["pod_required"] != false || body["required_document_type"] != nil || body["acceptance_policy_id"] != nil {
			t.Fatalf("body %+v", body)
		}
	})

	t.Run("POD_FOUNDATION_06 pickup is not a delivery", func(t *testing.T) {
		pickup := insertPickupAction(t, env, seed)
		rec := getPODRaw(t, router, token, seed.operating, seed.executionID, pickup)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "ACTION_NOT_DELIVERY") {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POD_FOUNDATION_07_08_09_10 not found surface", func(t *testing.T) {
		other := seedDelivery(t, env, 2, true)
		otherAction := deliveryActionID(t, env, other.stopID)
		unknownExecution := getPODRaw(t, router, token, seed.operating, uuid.New(), action)
		unknownAction := getPODRaw(t, router, token, seed.operating, seed.executionID, uuid.New())
		mismatch := getPODRaw(t, router, token, seed.operating, seed.executionID, otherAction)
		foreign := getPODRaw(t, router, token, uuid.New(), seed.executionID, action)
		bodies := []string{unknownExecution.Body.String(), unknownAction.Body.String(), mismatch.Body.String(), foreign.Body.String()}
		for i, rec := range []*httptest.ResponseRecorder{unknownExecution, unknownAction, mismatch, foreign} {
			if rec.Code != http.StatusNotFound {
				t.Fatalf("case %d status %d %s", i, rec.Code, rec.Body.String())
			}
		}
		for i := 1; i < len(bodies); i++ {
			if bodies[i] != bodies[0] {
				t.Fatalf("404 bodies differ\n%s\n%s", bodies[0], bodies[i])
			}
		}
		for _, secret := range []string{seed.ship.shipmentID.String(), seed.ship.cargoID.String(), other.ship.shipmentID.String(), other.operating.String()} {
			if strings.Contains(bodies[0], secret) {
				t.Fatalf("404 leaked %s in %s", secret, bodies[0])
			}
		}
	})

	t.Run("POD_FOUNDATION_11_12 token", func(t *testing.T) {
		path := podPath(seed.executionID, action)
		missing := httptest.NewRequest(http.MethodGet, path, nil)
		missing.Header.Set("X-Tenant-ID", seed.operating.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, missing)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("missing %d", rec.Code)
		}
		wrong := httptest.NewRequest(http.MethodGet, path, nil)
		wrong.Header.Set("X-Internal-Service-Token", "nope")
		wrong.Header.Set("X-Tenant-ID", seed.operating.String())
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, wrong)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong %d", rec.Code)
		}
	})

	t.Run("POD_FOUNDATION_14 independent delivery actions", func(t *testing.T) {
		second := insertSecondDelivery(t, env, seed)
		firstBody := getPOD(t, router, token, seed.operating, seed.executionID, action)
		secondBody := getPOD(t, router, token, seed.operating, seed.executionID, second)
		if firstBody["execution_action_id"] == secondBody["execution_action_id"] {
			t.Fatal("actions collapsed")
		}
		if firstBody["decision_state"] != "UNKNOWN" || secondBody["decision_state"] != "UNKNOWN" {
			t.Fatalf("states %v %v", firstBody["decision_state"], secondBody["decision_state"])
		}
		if firstBody["shipment_id"] != seed.ship.shipmentID.String() || secondBody["shipment_id"] != seed.ship.shipmentID.String() {
			t.Fatal("shipment provenance drifted")
		}
	})

	t.Run("POD_FOUNDATION_15 requires pod flag is not a decision", func(t *testing.T) {
		actions := domain.AllowedDriverMilestoneActions(domain.ShipmentStatusArrivedAtConsignee)
		var saw bool
		for _, item := range actions {
			if item.Type == "DELIVERY_COMPLETED" {
				saw = true
				if item.RequiresPOD {
					t.Fatal("milestone flag changed")
				}
			}
		}
		if !saw {
			t.Fatal("delivery milestone missing")
		}
		body := getPOD(t, router, token, seed.operating, seed.executionID, action)
		if body["decision_state"] != "UNKNOWN" || body["pod_required"] != nil {
			t.Fatalf("flag became a decision %+v", body)
		}
	})

	t.Run("POD_FOUNDATION_16 customer refusal does not decide pod", func(t *testing.T) {
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000093_tms_delivery_disposition_v0_1.up.sql")); err != nil {
			t.Fatal(err)
		}
		fresh := seedDelivery(t, env, 4, true)
		arriveStop(t, env, fresh.stopID)
		repo := repository.NewTransportExecutionRepository(env.pool)
		recordDelivery(t, repo, env, fresh, 1, 1, domain.ReasonCustomerRefusal, "", "pod-refusal", domain.ActorKindOperator)
		freshAction := deliveryActionID(t, env, fresh.stopID)
		if countWhere(t, env, "transport.transport_execution_pod_requirement_decisions", "execution_action_id=$1", freshAction) != 0 {
			t.Fatal("refusal wrote a pod decision")
		}
		body := getPOD(t, router, token, fresh.operating, fresh.executionID, freshAction)
		if body["decision_state"] != "UNKNOWN" || body["pod_required"] != nil {
			t.Fatalf("refusal body %+v", body)
		}
	})

	t.Run("successor delivery does not inherit a decision", func(t *testing.T) {
		route := seedSuccessorRoute(t, env)
		completeStop(t, env, route.endID)
		arriveStop(t, env, route.cargoID)
		completeAction(t, env, route.actionA)
		repo := repository.NewTransportExecutionRepository(env.pool)
		result, err := repo.CreateSuccessorRevision(env.ctx, successorCommand(route, "pod-successor"))
		if err != nil {
			t.Fatal(err)
		}
		var introduced uuid.UUID
		if err := env.pool.QueryRow(env.ctx, `
			SELECT a.id
			FROM transport.transport_execution_revision_actions AS ra
			JOIN transport.transport_execution_actions AS a ON a.id = ra.action_id
			WHERE ra.revision_id = $1 AND ra.membership = 'INTRODUCED' AND a.action_type = 'DELIVERY'
		`, result.RevisionID).Scan(&introduced); err != nil {
			t.Fatal(err)
		}
		if countWhere(t, env, "transport.transport_execution_pod_requirement_decisions", "execution_action_id=$1", introduced) != 0 {
			t.Fatal("successor delivery received a copied decision")
		}
		body := getPOD(t, router, token, route.operating, route.executionID, introduced)
		if body["decision_state"] != "UNKNOWN" || body["pod_required"] != nil || body["pod_requirement_decision_id"] != nil {
			t.Fatalf("inherited %+v", body)
		}
	})
}

func podRouter(env *execEnv) http.Handler {
	return shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "pod-token", nil)
}

func podPath(executionID, actionID uuid.UUID) string {
	return "/internal/v1/transport-executions/" + executionID.String() + "/actions/" + actionID.String() + "/pod-requirement"
}

func getPODRaw(t *testing.T, router http.Handler, token string, tenant, executionID, actionID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, podPath(executionID, actionID), nil)
	req.Header.Set("X-Internal-Service-Token", token)
	req.Header.Set("X-Tenant-ID", tenant.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func getPOD(t *testing.T, router http.Handler, token string, tenant, executionID, actionID uuid.UUID) map[string]any {
	t.Helper()
	rec := getPODRaw(t, router, token, tenant, executionID, actionID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func assertUnknownNoRow(t *testing.T, body map[string]any, seed deliverySeed, action uuid.UUID) {
	t.Helper()
	if body["decision_state"] != "UNKNOWN" || body["pod_required"] != nil || body["pod_requirement_decision_id"] != nil || body["decision_version"] != nil {
		t.Fatalf("body %+v", body)
	}
	if body["operating_tenant_id"] != seed.operating.String() || body["execution_id"] != seed.executionID.String() || body["execution_action_id"] != action.String() {
		t.Fatalf("identity %+v", body)
	}
	if body["shipment_id"] != seed.ship.shipmentID.String() || body["cargo_id"] != seed.ship.cargoID.String() || body["execution_stop_id"] != seed.stopID.String() {
		t.Fatalf("provenance %+v", body)
	}
	if body["execution_revision_id"] != nil || body["required_document_type"] != nil || body["acceptance_policy_id"] != nil || body["requirement_decided_at"] != nil {
		t.Fatalf("filled nulls %+v", body)
	}
}

func deliveryActionID(t *testing.T, env *execEnv, stopID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 AND action_type='DELIVERY' ORDER BY ordinal LIMIT 1`, stopID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertSecondDelivery(t *testing.T, env *execEnv, seed deliverySeed) uuid.UUID {
	t.Helper()
	stopID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_stops (
			id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind, location_id,
			latitude, longitude, status, version, created_at, updated_at
		) VALUES ($1,$2,$3,9,'CARGO','CANONICAL_LOCATION',$4,55.1,37.1,'PLANNED',1,now(),now())
	`, stopID, seed.executionID, seed.operating, seed.ship.originID); err != nil {
		t.Fatal(err)
	}
	actionID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_actions (
			id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, cargo_version,
			route_subject_type, route_subject_id, action_type, ordinal, status, version
		) VALUES ($1,$2,$3,$4,$5,1,'SHIPMENT_CARGO',$6,'DELIVERY',0,'PENDING',1)
	`, actionID, stopID, seed.ship.shipmentID, seed.ship.tenantID, seed.ship.cargoID, seed.ship.subjectID); err != nil {
		t.Fatal(err)
	}
	return actionID
}

func insertPickupAction(t *testing.T, env *execEnv, seed deliverySeed) uuid.UUID {
	t.Helper()
	actionID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_actions (
			id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, cargo_version,
			route_subject_type, route_subject_id, action_type, ordinal, status, version
		) VALUES ($1,$2,$3,$4,$5,1,'SHIPMENT_CARGO',$6,'PICKUP',1,'PENDING',1)
	`, actionID, seed.stopID, seed.ship.shipmentID, seed.ship.tenantID, seed.ship.cargoID, seed.ship.subjectID); err != nil {
		t.Fatal(err)
	}
	return actionID
}

type decisionInsert struct {
	ID            uuid.UUID
	Version       int
	Operating     uuid.UUID
	Execution     uuid.UUID
	Action        uuid.UUID
	Stop          uuid.UUID
	Shipment      uuid.UUID
	Cargo         uuid.UUID
	Revision      *uuid.UUID
	State         string
	PodRequired   *bool
	DocType       *string
	Policy        *uuid.UUID
	PolicyVersion *int
	DecidedAt     *time.Time
	Supersedes    *uuid.UUID
}

func (d decisionInsert) withState(state string, required *bool, docType *string, policy *uuid.UUID, policyVersion *int, decided *time.Time) decisionInsert {
	d.ID = uuid.New()
	d.Version = 1
	d.State = state
	d.PodRequired = required
	d.DocType = docType
	d.Policy = policy
	d.PolicyVersion = policyVersion
	d.DecidedAt = decided
	return d
}

func insertDecision(t *testing.T, env *execEnv, row decisionInsert) {
	t.Helper()
	if err := execDecision(env, row); err != nil {
		t.Fatal(err)
	}
}

func expectDecisionReject(t *testing.T, env *execEnv, row decisionInsert) {
	t.Helper()
	if err := execDecision(env, row); err == nil {
		t.Fatalf("accepted illegal decision %+v", row)
	}
}

func execDecision(env *execEnv, row decisionInsert) error {
	_, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_pod_requirement_decisions (
			id, decision_version, operating_tenant_id, execution_id, execution_action_id, execution_revision_id,
			execution_stop_id, shipment_id, cargo_id, decision_state, pod_required, required_document_type,
			acceptance_policy_id, acceptance_policy_version, requirement_decided_at, created_at, supersedes_decision_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now(),$16)
	`, row.ID, row.Version, row.Operating, row.Execution, row.Action, row.Revision, row.Stop, row.Shipment, row.Cargo,
		row.State, row.PodRequired, row.DocType, row.Policy, row.PolicyVersion, row.DecidedAt, row.Supersedes)
	return err
}

func boolPtr(v bool) *bool { return &v }

func strPtr(v string) *string { return &v }

func intPtr(v int) *int { return &v }

func timePtr(v time.Time) *time.Time { return &v }
