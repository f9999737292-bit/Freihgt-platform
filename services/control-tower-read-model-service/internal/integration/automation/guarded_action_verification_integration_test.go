//go:build integration

package automation

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

func seedGuardedNoticeRule(t *testing.T, ctx context.Context, pool *pgxpool.Pool, autoRepo *repository.AutomationRepository, fix fixture, operatorID uuid.UUID) domain.AutomationRule {
	t.Helper()
	p, pv, err := autoRepo.CreatePlaybook(ctx, fix.TenantID, operatorID, domain.CreatePlaybookInput{
		Name: "Operational notice",
		Steps: []domain.PlaybookStepInput{{
			Sequence: 1, Title: "Send notice", StepType: domain.StepTypeSystemAction,
			ActionCode: domain.ActionCreateDriverOperationalNotice, Required: true,
		}},
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, fix.TenantID, p.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, fix.TenantID, pv.ID)
	require.NoError(t, err)
	rule, err := autoRepo.CreateRule(ctx, fix.TenantID, operatorID, domain.CreateRuleInput{
		Name: "ETA notice", TriggerType: "eta_at_risk", ExecutionMode: domain.ExecutionModeGuardedAuto, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "projectedDelaySeconds", Operator: "gte", Value: json.RawMessage(`3600`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, fix.TenantID, operatorID, rule.ID, domain.RuleStatusActive)
	require.NoError(t, err)
	return rule
}

func triggerGuardedAuto(t *testing.T, ctx context.Context, stack guardedStack, fix fixture, triggerID string) service.EvaluateOutcome {
	t.Helper()
	trigger := service.BuildEtaAtRiskTrigger(fix.TenantID, fix.ShipmentID, triggerID, "corr-"+triggerID, 7200)
	outcome, err := stack.automation.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	return outcome
}

func latestGuardedAction(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) domain.GuardedAction {
	t.Helper()
	repo := repository.NewGuardedActionRepository(pool)
	var actionID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id FROM control_tower.automation_guarded_action WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT 1
	`, tenantID).Scan(&actionID))
	action, err := repo.GetAction(ctx, tenantID, actionID)
	require.NoError(t, err)
	return action
}

func authCtx(perms ...string) context.Context {
	return service.WithAutomationPermissions(context.Background(), perms...)
}

func TestApprovalClosedLoopE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())

	triggerGuardedAuto(t, ctx, stack, fix, "approval-e2e-1")
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))

	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Equal(t, domain.GuardedActionStatusWaitingApproval, action.Status)
	require.Equal(t, domain.GuardDecisionRequireApproval, action.GuardDecision)

	approver := uuid.New()
	approved, err := stack.guarded.ApproveAction(authCtx("automation.approve"), fix.TenantID, approver, action.ID)
	require.NoError(t, err)
	require.NotNil(t, approved.DriverTaskID)
	require.Equal(t, domain.GuardedActionStatusSucceeded, approved.Status)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestApprovalRejectE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())

	triggerGuardedAuto(t, ctx, stack, fix, "reject-e2e-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	rejected, err := stack.guarded.RejectAction(authCtx("automation.approve"), fix.TenantID, uuid.New(), action.ID, "not needed")
	require.NoError(t, err)
	require.Equal(t, domain.GuardedActionStatusRejected, rejected.Status)
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestApprovalRBACFailClosed(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "rbac-e2e-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)

	_, err := stack.guarded.ApproveAction(context.Background(), fix.TenantID, uuid.New(), action.ID)
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, apperrors.CodeForbidden, appErr.Code)

	_, err = stack.guarded.ApproveAction(authCtx("automation.view"), fix.TenantID, uuid.New(), action.ID)
	require.Error(t, err)
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestApprovalIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "idem-approve-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	approver := uuid.New()
	ctxPerm := authCtx("automation.approve")

	first, err := stack.guarded.ApproveAction(ctxPerm, fix.TenantID, approver, action.ID)
	require.NoError(t, err)
	second, err := stack.guarded.ApproveAction(ctxPerm, fix.TenantID, approver, action.ID)
	require.NoError(t, err)
	require.Equal(t, first.DriverTaskID, second.DriverTaskID)
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_action_approval WHERE tenant_id=$1 AND status='approved'`, fix.TenantID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestConcurrentApprovalRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "conc-approve-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = stack.guarded.ApproveAction(authCtx("automation.approve"), fix.TenantID, uuid.New(), action.ID)
		}(i)
	}
	wg.Wait()
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_action_approval WHERE tenant_id=$1 AND status='approved'`, fix.TenantID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestApproveRejectRaceRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "race-ar-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)

	var wg sync.WaitGroup
	var approveErr, rejectErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, approveErr = stack.guarded.ApproveAction(authCtx("automation.approve"), fix.TenantID, uuid.New(), action.ID)
	}()
	go func() {
		defer wg.Done()
		_, rejectErr = stack.guarded.RejectAction(authCtx("automation.approve"), fix.TenantID, uuid.New(), action.ID, "no")
	}()
	wg.Wait()

	approved := countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_action_approval WHERE tenant_id=$1 AND status='approved'`, fix.TenantID)
	rejected := countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_action_approval WHERE tenant_id=$1 AND status='rejected'`, fix.TenantID)
	require.True(t, (approved == 1 && rejected == 0) || (approved == 0 && rejected == 1))
	final := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	if final.Status == domain.GuardedActionStatusRejected {
		require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
	} else {
		require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
	}
	_ = approveErr
	_ = rejectErr
}

func TestCompletionReplayRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "replay-complete-1")

	items, total, err := stack.driverEnv.ListTasks(ctx, fix.TenantID, fix.UserID)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.NoError(t, stack.driverEnv.SubmitDelayReason(ctx, fix.TenantID, fix.UserID, items[0].ID, "resp-replay"))

	var payload []byte
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT payload FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND event_type='driver.task_completed' ORDER BY created_at DESC LIMIT 1
	`, fix.TenantID).Scan(&payload))

	event := service.DriverTaskEventInput{TenantID: fix.TenantID, EventType: "driver.task_completed", TaskID: items[0].ID, Payload: payload}
	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, event))
	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, event))

	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='succeeded'
	`, fix.TenantID))
}

func TestCompletionConcurrencyRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "conc-complete-1")

	items, _, err := stack.driverEnv.ListTasks(ctx, fix.TenantID, fix.UserID)
	require.NoError(t, err)
	require.NoError(t, stack.driverEnv.SubmitDelayReason(ctx, fix.TenantID, fix.UserID, items[0].ID, "resp-conc"))
	var payload []byte
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT payload FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND event_type='driver.task_completed' ORDER BY created_at DESC LIMIT 1
	`, fix.TenantID).Scan(&payload))
	event := service.DriverTaskEventInput{TenantID: fix.TenantID, EventType: "driver.task_completed", TaskID: items[0].ID, Payload: payload}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = stack.guarded.HandleDriverTaskEvent(ctx, event)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='succeeded'
	`, fix.TenantID))
}

func TestTimeoutReplayRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "replay-timeout-1")

	var taskID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT id FROM transport.driver_task WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT 1`, fix.TenantID).Scan(&taskID))
	_, err := env.pool.Exec(ctx, `UPDATE transport.driver_task SET expires_at=$2 WHERE id=$1`, taskID, time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)
	_, err = stack.driverEnv.ExpireDueTasks(ctx, 10)
	require.NoError(t, err)

	event := service.DriverTaskEventInput{TenantID: fix.TenantID, EventType: "driver.task_expired", TaskID: taskID}
	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, event))
	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, event))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='timed_out'
	`, fix.TenantID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM control_tower.automation_timeout_escalation WHERE tenant_id=$1
	`, fix.TenantID))
}

func TestCompletionTimeoutRaceRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "race-ct-1")

	var taskID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT id FROM transport.driver_task WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT 1`, fix.TenantID).Scan(&taskID))
	require.NoError(t, stack.driverEnv.SubmitDelayReason(ctx, fix.TenantID, fix.UserID, taskID, "resp-race"))
	var payload []byte
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT payload FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND event_type='driver.task_completed' ORDER BY created_at DESC LIMIT 1
	`, fix.TenantID).Scan(&payload))

	_, err := env.pool.Exec(ctx, `UPDATE transport.driver_task SET expires_at=$2 WHERE id=$1`, taskID, time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)

	complete := service.DriverTaskEventInput{TenantID: fix.TenantID, EventType: "driver.task_completed", TaskID: taskID, Payload: payload}
	expired := service.DriverTaskEventInput{TenantID: fix.TenantID, EventType: "driver.task_expired", TaskID: taskID}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = stack.guarded.HandleDriverTaskEvent(ctx, complete) }()
	go func() { defer wg.Done(); _ = stack.guarded.HandleDriverTaskEvent(ctx, expired) }()
	wg.Wait()

	final := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Contains(t, []string{domain.GuardedActionStatusSucceeded, domain.GuardedActionStatusTimedOut}, final.Status)
	succeeded := countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='succeeded'`, fix.TenantID)
	timedOut := countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_guarded_action WHERE tenant_id=$1 AND status='timed_out'`, fix.TenantID)
	require.Equal(t, int64(1), succeeded+timedOut)
}

func seedForbiddenPlaybookRule(t *testing.T, ctx context.Context, pool *pgxpool.Pool, autoRepo *repository.AutomationRepository, fix fixture, operatorID uuid.UUID, actionCode string) {
	t.Helper()
	p, pv, err := autoRepo.CreatePlaybook(ctx, fix.TenantID, operatorID, domain.CreatePlaybookInput{
		Name: "Forbidden action",
		Steps: []domain.PlaybookStepInput{{
			Sequence: 1, Title: "Forbidden", StepType: domain.StepTypeInstruction, Required: true,
		}},
	})
	require.NoError(t, err)
	stepID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO control_tower.operational_playbook_step
		(id, tenant_id, playbook_version_id, sequence, title, step_type, action_code, required)
		VALUES ($1,$2,$3,2,$4,'system_action',$5,true)
	`, stepID, fix.TenantID, pv.ID, "Forbidden step", actionCode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, fix.TenantID, p.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, fix.TenantID, pv.ID)
	require.NoError(t, err)
	rule, err := autoRepo.CreateRule(ctx, fix.TenantID, operatorID, domain.CreateRuleInput{
		Name: "Forbidden rule", TriggerType: "eta_at_risk", ExecutionMode: domain.ExecutionModeGuardedAuto, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "projectedDelaySeconds", Operator: "gte", Value: json.RawMessage(`3600`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, fix.TenantID, operatorID, rule.ID, domain.RuleStatusActive)
	require.NoError(t, err)
}

func TestForbiddenActionRuntimeRealPostgres(t *testing.T) {
	cases := []string{"CHANGE_ROUTE", "CANCEL_SHIPMENT", "REBOOK_SLOT", "AUTHORIZE_PAYMENT", "ARBITRARY_DRIVER_COMMAND", "ARBITRARY_URL", "ARBITRARY_WEBHOOK"}
	for _, actionCode := range cases {
		t.Run(actionCode, func(t *testing.T) {
			env := setupTestEnv(t)
			ctx := context.Background()
			fix := seedFixture(t, env.pool)
			stack := setupGuardedStack(t, env.pool)
			seedForbiddenPlaybookRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New(), actionCode)
			triggerGuardedAuto(t, ctx, stack, fix, "forbidden-"+actionCode)
			action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
			require.Equal(t, domain.GuardDecisionDeny, action.GuardDecision)
			require.Equal(t, domain.GuardedActionStatusDenied, action.Status)
			require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
		})
	}
}

func TestUnknownActionFailClosedRuntime(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedForbiddenPlaybookRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New(), "SOME_NEW_UNKNOWN_ACTION")
	triggerGuardedAuto(t, ctx, stack, fix, "unknown-action-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Equal(t, domain.GuardDecisionDeny, action.GuardDecision)
	require.Equal(t, "UNKNOWN_ACTION", action.GuardReason)
}

func TestTenantPolicyCannotOverrideForbiddenFloor(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	_, err := env.pool.Exec(ctx, `
		INSERT INTO control_tower.automation_tenant_action_policy (tenant_id, action_type, enabled, approval_level)
		VALUES ($1,'CHANGE_ROUTE',true,'none')
		ON CONFLICT (tenant_id, action_type) DO UPDATE SET enabled=true, approval_level='none'
	`, fix.TenantID)
	require.NoError(t, err)
	seedForbiddenPlaybookRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New(), "CHANGE_ROUTE")
	triggerGuardedAuto(t, ctx, stack, fix, "tenant-floor-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Equal(t, domain.GuardDecisionDeny, action.GuardDecision)
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestGlobalKillSwitchRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	service.SetGuardedActionsGlobalEnabled(false)
	t.Cleanup(func() { service.SetGuardedActionsGlobalEnabled(true) })
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "kill-global-1")
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Equal(t, "GUARDED_ACTIONS_DISABLED", action.GuardReason)
}

func TestTenantActionKillSwitchRealPostgres(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	_, err := env.pool.Exec(ctx, `
		INSERT INTO control_tower.automation_tenant_action_policy (tenant_id, action_type, enabled)
		VALUES ($1,$2,false)
		ON CONFLICT (tenant_id, action_type) DO UPDATE SET enabled=false
	`, fix.TenantID, domain.ActionRequestDriverDelayReason)
	require.NoError(t, err)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "kill-action-1")
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestCrossTenantApprovalDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "cross-tenant-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)

	otherTenant := uuid.New()
	_, err := stack.guarded.ApproveAction(authCtx("automation.approve"), otherTenant, uuid.New(), action.ID)
	require.Error(t, err)
}

func TestAuditChainReconstructable(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	stack := setupGuardedStack(t, env.pool)
	rule := seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerID := "audit-chain-1"
	triggerGuardedAuto(t, ctx, stack, fix, triggerID)

	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.NotEmpty(t, action.SourceEventID)
	require.NotEmpty(t, action.CorrelationID)
	require.NotEmpty(t, action.IdempotencyKey)

	var execID, ruleID, playbookID, stepID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT execution_id, execution_step_id FROM control_tower.automation_guarded_action WHERE id=$1
	`, action.ID).Scan(&execID, &stepID))
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT ar.rule_id, pe.playbook_id
		FROM control_tower.playbook_execution pe
		JOIN control_tower.automation_recommendation ar ON ar.id = pe.recommendation_id
		WHERE pe.id=$1
	`, execID).Scan(&ruleID, &playbookID))
	require.Equal(t, rule.ID, ruleID)

	items, _, err := stack.driverEnv.ListTasks(ctx, fix.TenantID, fix.UserID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NoError(t, stack.driverEnv.SubmitDelayReason(ctx, fix.TenantID, fix.UserID, items[0].ID, "audit-resp"))
	var payload []byte
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT payload FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND event_type='driver.task_completed' ORDER BY created_at DESC LIMIT 1
	`, fix.TenantID).Scan(&payload))
	require.NoError(t, stack.guarded.HandleDriverTaskEvent(ctx, service.DriverTaskEventInput{
		TenantID: fix.TenantID, EventType: "driver.task_completed", TaskID: items[0].ID, Payload: payload,
	}))

	final, err := repository.NewGuardedActionRepository(env.pool).GetAction(ctx, fix.TenantID, action.ID)
	require.NoError(t, err)
	require.Equal(t, domain.GuardedActionStatusSucceeded, final.Status)
	require.NotNil(t, final.DriverTaskID)
	_ = playbookID
	_ = stepID
}

func TestNoDriverShipmentDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	_, err := env.pool.Exec(ctx, `UPDATE transport.shipments SET driver_id=NULL WHERE id=$1`, fix.ShipmentID)
	require.NoError(t, err)
	stack := setupGuardedStack(t, env.pool)
	seedGuardedDelayRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "no-driver-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Equal(t, domain.GuardDecisionDeny, action.GuardDecision)
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1`, fix.TenantID))
}

func TestStaleDriverRevalidation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	newDriver := uuid.New()
	newUser := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status) VALUES ($1,$2,$3,$4,'Driver B','ACTIVE')`,
		newDriver, fix.TenantID, fix.CarrierID, newUser)
	require.NoError(t, err)

	stack := setupGuardedStack(t, env.pool)
	seedGuardedNoticeRule(t, ctx, env.pool, stack.autoRepo, fix, uuid.New())
	triggerGuardedAuto(t, ctx, stack, fix, "stale-driver-1")
	action := latestGuardedAction(t, ctx, env.pool, fix.TenantID)
	require.Equal(t, domain.GuardedActionStatusWaitingApproval, action.Status)

	_, err = env.pool.Exec(ctx, `UPDATE transport.shipments SET driver_id=$2 WHERE id=$1`, fix.ShipmentID, newDriver)
	require.NoError(t, err)
	approved, err := stack.guarded.ApproveAction(authCtx("automation.approve"), fix.TenantID, uuid.New(), action.ID)
	require.NoError(t, err)
	require.NotNil(t, approved.DriverTaskID)

	var taskDriver uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT driver_id FROM transport.driver_task WHERE id=$1`, approved.DriverTaskID).Scan(&taskDriver))
	require.Equal(t, newDriver, taskDriver)
}
