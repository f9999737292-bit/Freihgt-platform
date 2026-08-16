//go:build integration

package automation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

func TestDriverDelayEventAutomationE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	operatorID := uuid.New()

	autoRepo, workflowRepo, automationSvc := newAutomationStack(env.pool)
	driverEventRepo := repository.NewDriverEventRepository(env.pool)
	ingress := service.NewAutomationTriggerIngress(automationSvc, nil, nil)
	driverEventSvc := service.NewDriverEventService(driverEventRepo, workflowRepo, automationSvc, ingress, nil)

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

	eventID := uuid.New()
	sourceID := uuid.New()
	payload, _ := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.delay.reported", "schemaVersion": 1,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId": fix.TenantID.String(), "shipmentId": fix.ShipmentID.String(), "driverId": fix.DriverID.String(),
		"source": "driver", "sourceEventId": sourceID.String(), "reasonCode": "TRAFFIC",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": fix.ShipmentID.String(), "version": 1},
	})

	result, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, "PROCESSED", result.Outcome)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, fix.TenantID))

	dup, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, dup.Duplicate)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, fix.TenantID))
}

func TestDriverProblemEventCreatesWorkflow(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	_, workflowRepo, automationSvc := newAutomationStack(env.pool)
	driverEventRepo := repository.NewDriverEventRepository(env.pool)
	driverEventSvc := service.NewDriverEventService(driverEventRepo, workflowRepo, automationSvc, service.NewAutomationTriggerIngress(automationSvc, nil, nil), nil)

	eventID := uuid.New()
	payload, _ := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.problem.reported", "schemaVersion": 1,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId": fix.TenantID.String(), "shipmentId": fix.ShipmentID.String(), "driverId": fix.DriverID.String(),
		"source": "driver", "sourceEventId": uuid.NewString(), "reasonCode": "VEHICLE_BREAKDOWN", "severity": "critical",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": fix.ShipmentID.String(), "version": 1},
	})
	_, err := driverEventSvc.Handle(ctx, domain.KafkaRecordMeta{Topic: "driver.events.v1"}, payload, time.Now().UTC())
	require.NoError(t, err)
	eventHex := domain.BuildDriverProblemExceptionSeed(domain.ControlTowerEvent{
		ID: eventID, TenantID: fix.TenantID, ShipmentID: fix.ShipmentID, Type: "driver.problem.reported", Severity: "critical",
	}, domain.DriverDomainEventEnvelope{ReasonCode: "VEHICLE_BREAKDOWN"}).EventID
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.critical_event_workflow WHERE tenant_id=$1 AND event_id=$2`, fix.TenantID, eventHex))
}
