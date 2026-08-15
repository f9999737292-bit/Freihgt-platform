//go:build integration

package automation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	ctclient "github.com/freight-platform/control-tower-read-model-service/internal/client"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
	"github.com/freight-platform/shipment-service/testutil/drivertaskserver"
)

type guardedStack struct {
	autoRepo   *repository.AutomationRepository
	automation *service.AutomationService
	guarded    *service.GuardedActionService
	driverEnv  *drivertaskserver.Env
}

func setupGuardedStack(t *testing.T, pool *pgxpool.Pool) guardedStack {
	t.Helper()
	service.SetGuardedActionsGlobalEnabled(true)
	driverEnv := drivertaskserver.New(pool, "test-internal-token")
	t.Cleanup(driverEnv.Close)
	autoRepo := repository.NewAutomationRepository(pool)
	guardedRepo := repository.NewGuardedActionRepository(pool)
	shipmentLookup := repository.NewShipmentLookupRepository(pool)
	driverClient := ctclient.NewHTTPDriverTaskClient(driverEnv.Server.URL, "test-internal-token")
	guardEvaluator := service.NewGuardEvaluator(shipmentLookup, guardedRepo)
	guardedSvc := service.NewGuardedActionService(guardedRepo, shipmentLookup, autoRepo, guardEvaluator, driverClient)
	automationSvc := service.NewAutomationService(autoRepo)
	automationSvc.SetGuardedAutoRunner(service.NewGuardedAutoRunner(autoRepo, guardedSvc))
	return guardedStack{autoRepo: autoRepo, automation: automationSvc, guarded: guardedSvc, driverEnv: driverEnv}
}

func seedGuardedDelayRule(t *testing.T, ctx context.Context, pool *pgxpool.Pool, autoRepo *repository.AutomationRepository, fix fixture, operatorID uuid.UUID) domain.AutomationRule {
	t.Helper()
	p, pv, err := autoRepo.CreatePlaybook(ctx, fix.TenantID, operatorID, domain.CreatePlaybookInput{
		Name: "Delay reason",
		Steps: []domain.PlaybookStepInput{{
			Sequence: 1, Title: "Request delay reason", StepType: domain.StepTypeSystemAction,
			ActionCode: domain.ActionRequestDriverDelayReason, Required: true,
		}},
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, fix.TenantID, p.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, fix.TenantID, pv.ID)
	require.NoError(t, err)
	rule, err := autoRepo.CreateRule(ctx, fix.TenantID, operatorID, domain.CreateRuleInput{
		Name: "ETA delay", TriggerType: "eta_at_risk", ExecutionMode: domain.ExecutionModeGuardedAuto, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "projectedDelaySeconds", Operator: "gte", Value: json.RawMessage(`3600`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, fix.TenantID, operatorID, rule.ID, domain.RuleStatusActive)
	require.NoError(t, err)
	return rule
}

func TestGuardedActionClosedLoopE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())

	trigger := service.BuildEtaAtRiskTrigger(fix.TenantID, fix.ShipmentID, "eta-dup-1", "corr-dup", 7200)
	outcome, err := stack.automation.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Len(t, outcome.Recommendations, 1)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1`, fix.TenantID))

	items, total, err := stack.driverEnv.ListTasks(ctx, fix.TenantID, fix.UserID)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)

	require.NoError(t, stack.driverEnv.SubmitDelayReason(ctx, fix.TenantID, fix.UserID, items[0].ID, "resp-1"))

	var payload []byte
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT payload FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND event_type='driver.task_completed' ORDER BY created_at DESC LIMIT 1
	`, fix.TenantID).Scan(&payload))
	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, service.DriverTaskEventInput{
		TenantID: fix.TenantID, EventType: "driver.task_completed", TaskID: items[0].ID, Payload: payload,
	}))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='succeeded'
	`, fix.TenantID))
}

func TestGuardedActionDuplicateTrigger(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())

	trigger := service.BuildEtaAtRiskTrigger(fix.TenantID, fix.ShipmentID, "eta-dup-1", "corr-dup", 7200)
	first, err := stack.automation.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Len(t, first.Recommendations, 1)

	second, err := stack.automation.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Empty(t, second.Recommendations)
	require.Equal(t, 1, second.Deduplicated)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestForbiddenActionValidation(t *testing.T) {
	require.Error(t, domain.ValidateGuardedActionCode("CHANGE_ROUTE"))
}

func TestGuardedActionTimeoutE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())

	trigger := service.BuildEtaAtRiskTrigger(fix.TenantID, fix.ShipmentID, "eta-timeout-1", "corr-timeout", 7200)
	_, err := stack.automation.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)

	var taskID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT id FROM transport.driver_task WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT 1
	`, fix.TenantID).Scan(&taskID))
	_, err = env.pool.Exec(ctx, `UPDATE transport.driver_task SET expires_at=$2 WHERE id=$1`, taskID, time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)
	n, err := stack.driverEnv.ExpireDueTasks(ctx, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)

	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, service.DriverTaskEventInput{
		TenantID: fix.TenantID, EventType: "driver.task_expired", TaskID: taskID,
	}))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='timed_out'
	`, fix.TenantID))
}
