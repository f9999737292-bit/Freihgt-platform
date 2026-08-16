//go:build integration

package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/driverconsumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

func newDriverEventStack(t *testing.T, pool *pgxpool.Pool) (*repository.AutomationRepository, *repository.WorkflowRepository, *service.AutomationService, *service.DriverEventService) {
	t.Helper()
	autoRepo, workflowRepo, automationSvc := newAutomationStack(pool)
	driverEventRepo := repository.NewDriverEventRepository(pool)
	ingress := service.NewAutomationTriggerIngress(automationSvc, nil, nil)
	driverEventSvc := service.NewDriverEventService(driverEventRepo, workflowRepo, automationSvc, ingress, nil)
	return autoRepo, workflowRepo, automationSvc, driverEventSvc
}

func seedDelayRule(t *testing.T, ctx context.Context, env *testEnv, fix fixture, autoRepo *repository.AutomationRepository, operatorID uuid.UUID) {
	t.Helper()
	p, pv, err := autoRepo.CreatePlaybook(ctx, fix.TenantID, operatorID, domain.CreatePlaybookInput{
		Name: "Delay response", Steps: []domain.PlaybookStepInput{{Sequence: 1, Title: "Notify", StepType: domain.StepTypeInstruction, ActionCode: "contact_driver", Required: true}},
	})
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, fix.TenantID, p.ID)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, fix.TenantID, pv.ID)
	require.NoError(t, err)
	rule, err := autoRepo.CreateRule(ctx, fix.TenantID, operatorID, domain.CreateRuleInput{
		Name: "Delay rule", TriggerType: domain.DriverTriggerDelayReported, ExecutionMode: domain.ExecutionModeRecommend, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "eventType", Operator: "eq", Value: json.RawMessage(`"driver.delay.reported"`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, fix.TenantID, operatorID, rule.ID, domain.RuleStatusActive)
	require.NoError(t, err)
}

func buildDelayPayload(fix fixture, eventID uuid.UUID) []byte {
	payload, _ := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.delay.reported", "schemaVersion": 1,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId": fix.TenantID.String(), "shipmentId": fix.ShipmentID.String(), "driverId": fix.DriverID.String(),
		"source": "driver", "sourceEventId": uuid.NewString(), "reasonCode": "TRAFFIC",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": fix.ShipmentID.String(), "version": 1},
	})
	return payload
}

func TestDriverEventDuplicateAndDatabaseConstraint(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	operatorID := uuid.New()
	autoRepo, _, _, driverEventSvc := newDriverEventStack(t, env.pool)
	seedDelayRule(t, ctx, env, fix, autoRepo, operatorID)

	eventID := uuid.New()
	payload := buildDelayPayload(fix, eventID)
	_, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, fix.TenantID))

	dup, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, dup.Duplicate)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, fix.TenantID))

	_, err = env.pool.Exec(ctx, `
		INSERT INTO control_tower.driver_event_inbox
		(tenant_id, event_id, event_type, processing_outcome, processed_at)
		VALUES ($1,$2,'driver.delay.reported','ACCEPTED',NOW())`,
		fix.TenantID, eventID)
	require.Error(t, err, "unique constraint uq_driver_event_inbox_tenant_event must reject duplicate insert")
}

func TestKafkaTenantShipmentMismatchRejected(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	tenantB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,'tb','Tenant B')`, tenantB)
	require.NoError(t, err)

	_, _, _, driverEventSvc := newDriverEventStack(t, env.pool)
	eventID := uuid.New()
	payload, _ := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.delay.reported", "schemaVersion": 1,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId": tenantB.String(), "shipmentId": fix.ShipmentID.String(), "driverId": fix.DriverID.String(),
		"source": "driver", "sourceEventId": uuid.NewString(), "reasonCode": "TRAFFIC",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": fix.ShipmentID.String(), "version": 1},
	})
	result, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.Error(t, err)
	var perm *domain.PermanentError
	require.ErrorAs(t, err, &perm)
	require.Equal(t, domain.DriverEventErrorTenantMismatch, perm.Code)
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.driver_event_inbox WHERE event_id=$1`, eventID))
	require.Equal(t, "TENANT_MISMATCH", result.Outcome)
}

func TestCrossTenantAutomationResourcesNotTriggered(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	tenantB := uuid.New()
	userB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,'tb','Tenant B')`, tenantB)
	require.NoError(t, err)

	autoRepo, _, _, driverEventSvc := newDriverEventStack(t, env.pool)
	seedDelayRule(t, ctx, env, fix, autoRepo, uuid.New())

	// Tenant B rule/playbook that must not fire for tenant A event.
	pB, pvB, err := autoRepo.CreatePlaybook(ctx, tenantB, userB, domain.CreatePlaybookInput{
		Name: "Tenant B delay", Steps: []domain.PlaybookStepInput{{Sequence: 1, Title: "B", StepType: domain.StepTypeInstruction, ActionCode: "contact_driver", Required: true}},
	})
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, tenantB, pB.ID)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, tenantB, pvB.ID)
	require.NoError(t, err)
	ruleB, err := autoRepo.CreateRule(ctx, tenantB, userB, domain.CreateRuleInput{
		Name: "Tenant B rule", TriggerType: domain.DriverTriggerDelayReported, ExecutionMode: domain.ExecutionModeRecommend, PlaybookID: &pB.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "eventType", Operator: "eq", Value: json.RawMessage(`"driver.delay.reported"`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, tenantB, userB, ruleB.ID, domain.RuleStatusActive)
	require.NoError(t, err)

	eventID := uuid.New()
	_, err = driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, buildDelayPayload(fix, eventID), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, fix.TenantID))
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, tenantB))
}

func TestDriverProblemCriticalEventAckE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	ackUserID := uuid.New()

	_, workflowRepo, automationSvc, driverEventSvc := newDriverEventStack(t, env.pool)
	ackRepo := repository.NewAckRepository(env.pool)
	ackHandler := handlers.NewAckHandler(ackRepo, workflowRepo, service.NewAutomationTriggerIngress(automationSvc, nil, nil))

	eventID := uuid.New()
	occurredAt := time.Now().UTC().Add(-2 * time.Minute)
	payload, _ := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.problem.reported", "schemaVersion": 1,
		"occurredAt": occurredAt.Format(time.RFC3339Nano),
		"tenantId": fix.TenantID.String(), "shipmentId": fix.ShipmentID.String(), "driverId": fix.DriverID.String(),
		"source": "driver", "sourceEventId": uuid.NewString(), "reasonCode": "VEHICLE_BREAKDOWN", "severity": "critical",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": fix.ShipmentID.String(), "version": 1},
	})
	_, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.NoError(t, err)

	seed := domain.BuildDriverProblemExceptionSeed(domain.ControlTowerEvent{
		ID: eventID, TenantID: fix.TenantID, ShipmentID: fix.ShipmentID, Type: "driver.problem.reported", Severity: "critical", OccurredAt: occurredAt,
	}, domain.DriverDomainEventEnvelope{ReasonCode: "VEHICLE_BREAKDOWN"})
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.critical_event_workflow WHERE tenant_id=$1 AND event_id=$2`, fix.TenantID, seed.EventID))

	body, _ := json.Marshal(map[string]string{
		"shipmentId": fix.ShipmentID.String(),
		"eventType":  seed.EventType,
		"occurredAt": occurredAt.Format(time.RFC3339),
		"source":     "control-tower",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/control-tower/critical-events/"+seed.EventID+"/acknowledge", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", ackUserID.String())
	rec := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("eventId", seed.EventID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	ackHandler.AcknowledgeCriticalEvent(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.critical_event_acknowledgement
		WHERE tenant_id=$1 AND event_id=$2 AND acknowledged_by_user_id=$3`, fix.TenantID, seed.EventID, ackUserID))

	tenantB := uuid.New()
	userB := uuid.New()
	_, err = env.pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,'tb','Tenant B')`, tenantB)
	require.NoError(t, err)
	reqB := httptest.NewRequest(http.MethodPost, "/internal/v1/control-tower/critical-events/"+seed.EventID+"/acknowledge", bytes.NewReader(body))
	reqB.Header.Set("X-Tenant-ID", tenantB.String())
	reqB.Header.Set("X-User-ID", userB.String())
	recB := httptest.NewRecorder()
	reqB = reqB.WithContext(context.WithValue(reqB.Context(), chi.RouteCtxKey, rctx))
	ackHandler.AcknowledgeCriticalEvent(recB, reqB)
	// Tenant A acknowledgement and workflow must remain unchanged (tenant-scoped isolation).
	var tenantAUser uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT acknowledged_by_user_id FROM control_tower.critical_event_acknowledgement
		WHERE tenant_id=$1 AND event_id=$2`, fix.TenantID, seed.EventID).Scan(&tenantAUser))
	require.Equal(t, ackUserID, tenantAUser)
	var tenantAWorkflowUser uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT acknowledged_by_user_id FROM control_tower.critical_event_workflow
		WHERE tenant_id=$1 AND event_id=$2`, fix.TenantID, seed.EventID).Scan(&tenantAWorkflowUser))
	require.Equal(t, ackUserID, tenantAWorkflowUser)
	// Cross-tenant ACK creates tenant-scoped rows only; it must not mutate tenant A state.
	require.NotEqual(t, int64(0), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.critical_event_workflow
		WHERE tenant_id=$1 AND event_id=$2`, tenantB, seed.EventID))
}

func TestDriverEventFailureHandling(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	_, _, _, driverEventSvc := newDriverEventStack(t, env.pool)

	cases := []struct {
		name string
		body []byte
		code string
	}{
		{"malformed", []byte("{"), domain.DriverEventErrorInvalidJSON},
		{"unsupported_version", mustJSON(map[string]any{"eventId": uuid.NewString(), "eventType": "driver.delay.reported", "schemaVersion": 99, "tenantId": fix.TenantID.String(), "shipmentId": fix.ShipmentID.String()}), domain.DriverEventErrorUnsupportedVersion},
		{"missing_tenant", mustJSON(map[string]any{"eventId": uuid.NewString(), "eventType": "driver.delay.reported", "schemaVersion": 1, "shipmentId": fix.ShipmentID.String()}), domain.DriverEventErrorMissingTenant},
		{"unknown_shipment", mustJSON(map[string]any{"eventId": uuid.NewString(), "eventType": "driver.delay.reported", "schemaVersion": 1, "tenantId": fix.TenantID.String(), "shipmentId": uuid.NewString(), "sourceEventId": uuid.NewString()}), "IGNORED_UNKNOWN_SHIPMENT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, tc.body, time.Now().UTC())
			if tc.code == "IGNORED_UNKNOWN_SHIPMENT" {
				require.NoError(t, err)
				require.Equal(t, tc.code, result.Outcome)
				return
			}
			require.Error(t, err)
			var perm *domain.PermanentError
			require.ErrorAs(t, err, &perm)
			require.Equal(t, tc.code, perm.Code)
		})
	}
}

func TestDriverEventMetricsRegistered(t *testing.T) {
	m := driverconsumer.NewMetrics()
	require.NotNil(t, m)
	m.IncConsumed("driver.delay.reported")
	m.IncFailed(domain.DriverEventErrorInvalidJSON)
	m.IncDuplicate("DUPLICATE")
	m.IncRuleMatch(domain.DriverTriggerDelayReported)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
