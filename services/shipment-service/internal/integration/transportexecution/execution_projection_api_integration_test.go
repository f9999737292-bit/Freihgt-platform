//go:build integration

package transportexecution

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/http/handlers"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestExecutionProjectionAPI(t *testing.T) {
	env := startPostgres(t)
	repo := repository.NewTransportExecutionRepository(env.pool)
	router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "projection-token", repo)

	t.Run("I1-01 I1-06 I1-07 valid projection returns committed ack", func(t *testing.T) {
		cmd := projectionAPICommand(t, env)
		rec := postProjection(t, router, cmd.OperatingTenantID, cmd, "projection-token", handlers.AuthorizedProjectionCaller)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d %s", rec.Code, rec.Body.String())
		}
		ack := decodeAck(t, rec)
		assertAck(t, ack, cmd)
		assertStoredAck(t, env, ack)
		if countWhere(t, env, "transport.transport_executions", "id=$1", ack.ExecutionID) != 1 {
			t.Fatal("ack returned before the execution row was committed")
		}
	})

	t.Run("I1-02 I1-08 same activation returns the same root", func(t *testing.T) {
		cmd := projectionAPICommand(t, env)
		first := decodeAck(t, mustPost(t, router, cmd, http.StatusCreated))
		second := decodeAck(t, mustPost(t, router, cmd, http.StatusOK))
		if second.ExecutionID != first.ExecutionID || second.RevisionID != first.RevisionID {
			t.Fatalf("replay changed ids %+v %+v", first, second)
		}
		if countWhere(t, env, "transport.transport_executions", "id=$1", first.ExecutionID) != 1 {
			t.Fatal("replay created another execution root")
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "source_activation_id=$1", cmd.ActivationID) != 1 {
			t.Fatal("replay created another revision for the activation")
		}
	})

	t.Run("I1-03 changed body conflicts", func(t *testing.T) {
		cmd := projectionAPICommand(t, env)
		first := decodeAck(t, mustPost(t, router, cmd, http.StatusCreated))
		changed := cmd
		changed.EvaluationFingerprint = "changed-fingerprint"
		rec := postProjection(t, router, changed.OperatingTenantID, changed, "projection-token", handlers.AuthorizedProjectionCaller)
		if rec.Code != http.StatusConflict || !bytes.Contains(rec.Body.Bytes(), []byte(domain.ReasonActivationBodyConflict)) {
			t.Fatalf("conflict %d %s", rec.Code, rec.Body.String())
		}
		if countWhere(t, env, "transport.transport_executions", "id=$1", first.ExecutionID) != 1 {
			t.Fatal("conflict created another execution")
		}
	})

	t.Run("I1-04 foreign tenant denied", func(t *testing.T) {
		cmd := projectionAPICommand(t, env)
		foreign := seedTenant(t, env, "foreign-projection")
		rec := postProjection(t, router, foreign, cmd, "projection-token", handlers.AuthorizedProjectionCaller)
		if rec.Code != http.StatusForbidden || !bytes.Contains(rec.Body.Bytes(), []byte(domain.ReasonTenantDenied)) {
			t.Fatalf("foreign header %d %s", rec.Code, rec.Body.String())
		}
		if countWhere(t, env, "transport.transport_executions", "operating_tenant_id=$1", foreign) != 0 {
			t.Fatal("foreign tenant created an execution")
		}
	})

	t.Run("I1-05 unauthorized caller denied", func(t *testing.T) {
		cmd := projectionAPICommand(t, env)
		missing := postProjection(t, router, cmd.OperatingTenantID, cmd, "", handlers.AuthorizedProjectionCaller)
		if missing.Code != http.StatusUnauthorized {
			t.Fatalf("missing token %d %s", missing.Code, missing.Body.String())
		}
		wrong := postProjection(t, router, cmd.OperatingTenantID, cmd, "projection-token", "shipment-service")
		if wrong.Code != http.StatusForbidden {
			t.Fatalf("wrong caller %d %s", wrong.Code, wrong.Body.String())
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "source_activation_id=$1", cmd.ActivationID) != 0 {
			t.Fatal("unauthorized caller created a revision")
		}
	})

	t.Run("I1-09 shipment already on an active execution", func(t *testing.T) {
		cmd := projectionAPICommand(t, env)
		_ = decodeAck(t, mustPost(t, router, cmd, http.StatusCreated))
		again := cmd
		again.ActivationID = uuid.New()
		again.RoutePlanID = uuid.New()
		rec := postProjection(t, router, again.OperatingTenantID, again, "projection-token", handlers.AuthorizedProjectionCaller)
		if rec.Code != http.StatusConflict || !bytes.Contains(rec.Body.Bytes(), []byte(domain.ReasonExecutionPlanConflict)) {
			t.Fatalf("overlap %d %s", rec.Code, rec.Body.String())
		}
		if countWhere(t, env, "transport.transport_executions", "operating_tenant_id=$1", cmd.OperatingTenantID) != 1 {
			t.Fatal("overlapping activation created a second execution")
		}
	})
}

func projectionAPICommand(t *testing.T, env *execEnv) domain.ProjectionCommand {
	t.Helper()
	operating := seedTenant(t, env, "projection-carrier")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Projection Carrier")
	a := seedShipment(t, env, "projection-a", "IN_TRANSIT")
	b := seedShipment(t, env, "projection-b", "IN_TRANSIT")
	return routeCommand(operating, carrierID, a, b)
}

func mustPost(t *testing.T, router http.Handler, cmd domain.ProjectionCommand, status int) *httptest.ResponseRecorder {
	t.Helper()
	rec := postProjection(t, router, cmd.OperatingTenantID, cmd, "projection-token", handlers.AuthorizedProjectionCaller)
	if rec.Code != status {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	return rec
}

func postProjection(t *testing.T, router http.Handler, tenantID uuid.UUID, cmd domain.ProjectionCommand, token, caller string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-executions/from-route-plan-activation", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	if caller != "" {
		req.Header.Set(handlers.HeaderInternalServiceName, caller)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type projectionAckBody struct {
	OperatingTenantID uuid.UUID `json:"operating_tenant_id"`
	ActivationID      uuid.UUID `json:"activation_id"`
	RoutePlanID       uuid.UUID `json:"route_plan_id"`
	ExecutionID       uuid.UUID `json:"execution_id"`
	RevisionID        uuid.UUID `json:"execution_revision_id"`
}

func decodeAck(t *testing.T, rec *httptest.ResponseRecorder) projectionAckBody {
	t.Helper()
	var ack projectionAckBody
	if err := json.Unmarshal(rec.Body.Bytes(), &ack); err != nil {
		t.Fatal(err)
	}
	return ack
}

func assertAck(t *testing.T, ack projectionAckBody, cmd domain.ProjectionCommand) {
	t.Helper()
	if ack.OperatingTenantID != cmd.OperatingTenantID || ack.ActivationID != cmd.ActivationID || ack.RoutePlanID != cmd.RoutePlanID || ack.ExecutionID == uuid.Nil || ack.RevisionID == uuid.Nil {
		t.Fatalf("ack %+v command tenant %s activation %s plan %s", ack, cmd.OperatingTenantID, cmd.ActivationID, cmd.RoutePlanID)
	}
}

func assertStoredAck(t *testing.T, env *execEnv, ack projectionAckBody) {
	t.Helper()
	var operating, activation, plan, execution, revision uuid.UUID
	err := env.pool.QueryRow(env.ctx, `
		SELECT rev.operating_tenant_id, rev.source_activation_id, rev.source_route_plan_id, rev.execution_id, rev.id
		FROM transport.transport_execution_revisions AS rev
		JOIN transport.transport_executions AS execution
		  ON execution.id = rev.execution_id AND execution.operating_tenant_id = rev.operating_tenant_id
		WHERE rev.id = $1
	`, ack.RevisionID).Scan(&operating, &activation, &plan, &execution, &revision)
	if err != nil {
		t.Fatal(err)
	}
	if operating != ack.OperatingTenantID || activation != ack.ActivationID || plan != ack.RoutePlanID || execution != ack.ExecutionID || revision != ack.RevisionID {
		t.Fatalf("stored correlation does not match ack")
	}
}
