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
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

func TestFullDriverAutomationE2E(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	insertDriverException(t, env.pool, fix, "idem-e2e-1")
	operatorID := uuid.New()

	autoRepo, workflowRepo, automationSvc := newAutomationStack(env.pool)
	rule, playbook := seedMatchingRule(t, ctx, autoRepo, env.pool, fix.TenantID, operatorID)

	eventID := eventIDHex(fix.ExceptionID)
	var occurredAt time.Time
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT occurred_at FROM transport.driver_reported_exception WHERE id=$1`, fix.ExceptionID).Scan(&occurredAt))
	seed := domain.EnsureExceptionSeed{
		EventID: eventID, ShipmentID: fix.ShipmentID.String(), EventType: "vehicle_breakdown",
		Source: "driver", Severity: "high", OccurredAt: occurredAt,
	}

	created, err := workflowRepo.EnsureExceptionWorkflows(ctx, fix.TenantID, []domain.EnsureExceptionSeed{seed})
	require.NoError(t, err)
	require.Contains(t, created, eventID)

	trigger := service.BuildExceptionCreatedTrigger(fix.TenantID, seed)
	outcome, err := automationSvc.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Len(t, outcome.Matches, 1)
	require.NotEqual(t, uuid.Nil, outcome.Matches[0].PlaybookVersionID)
	require.Equal(t, rule.ID, outcome.Matches[0].Rule.ID)
	require.Equal(t, playbook.ID, *outcome.Matches[0].SelectedPlaybookID)
	require.Len(t, outcome.Recommendations, 1)

	rec := outcome.Recommendations[0]
	_, exec, err := autoRepo.AcceptRecommendation(ctx, fix.TenantID, operatorID, rec.ID)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, exec.ID)

	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_reported_exception WHERE id=$1`, fix.ExceptionID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE source_event_id=$1`, fix.ExceptionID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.critical_event_workflow WHERE event_id=$1`, eventID))
	require.Equal(t, int64(1), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.playbook_execution WHERE recommendation_id=$1`, rec.ID))
	require.GreaterOrEqual(t, countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.playbook_execution_step WHERE execution_id=$1`, exec.ID), int64(1))
}

func TestAutomationReplayIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	insertDriverException(t, env.pool, fix, "replay-1")
	operatorID := uuid.New()

	autoRepo, workflowRepo, automationSvc := newAutomationStack(env.pool)
	seedMatchingRule(t, ctx, autoRepo, env.pool, fix.TenantID, operatorID)

	eventID := eventIDHex(fix.ExceptionID)
	seed := domain.EnsureExceptionSeed{EventID: eventID, ShipmentID: fix.ShipmentID.String(), EventType: "vehicle_breakdown", Source: "driver", Severity: "high"}
	_, err := workflowRepo.EnsureExceptionWorkflows(ctx, fix.TenantID, []domain.EnsureExceptionSeed{seed})
	require.NoError(t, err)

	trigger := service.BuildExceptionCreatedTrigger(fix.TenantID, seed)
	first, err := automationSvc.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Len(t, first.Recommendations, 1)

	second, err := automationSvc.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Empty(t, second.Recommendations)
	require.Equal(t, 1, second.Deduplicated)
}

func TestCrossTenantAutomationIsolation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	insertDriverException(t, env.pool, fix, "cross-1")
	tenantB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,'tb','Tenant B')`, tenantB)
	require.NoError(t, err)

	autoRepo, workflowRepo, automationSvc := newAutomationStack(env.pool)
	seedMatchingRule(t, ctx, autoRepo, env.pool, tenantB, uuid.New())

	eventID := eventIDHex(fix.ExceptionID)
	seed := domain.EnsureExceptionSeed{EventID: eventID, ShipmentID: fix.ShipmentID.String(), EventType: "vehicle_breakdown", Source: "driver", Severity: "high"}
	_, err = workflowRepo.EnsureExceptionWorkflows(ctx, fix.TenantID, []domain.EnsureExceptionSeed{seed})
	require.NoError(t, err)

	trigger := service.BuildExceptionCreatedTrigger(fix.TenantID, seed)
	outcome, err := automationSvc.EvaluateTrigger(ctx, fix.TenantID, trigger, true)
	require.NoError(t, err)
	require.Empty(t, outcome.Recommendations)
	require.Equal(t, int64(0), countRows(ctx, env.pool, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, tenantB))
}

func TestNegativeRuleCondition(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	insertDriverException(t, env.pool, fix, "neg-1")
	operatorID := uuid.New()

	autoRepo, workflowRepo, automationSvc := newAutomationStack(env.pool)
	p, pv, err := autoRepo.CreatePlaybook(ctx, fix.TenantID, operatorID, domain.CreatePlaybookInput{
		Name: "Noop", Steps: []domain.PlaybookStepInput{{Sequence: 1, Title: "x", StepType: domain.StepTypeInstruction, ActionCode: "monitor", Required: true}},
	})
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, fix.TenantID, p.ID)
	require.NoError(t, err)
	_, err = env.pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, fix.TenantID, pv.ID)
	require.NoError(t, err)
	rule, err := autoRepo.CreateRule(ctx, fix.TenantID, operatorID, domain.CreateRuleInput{
		Name: "No match", TriggerType: "exception_created", ExecutionMode: domain.ExecutionModeRecommend, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "exceptionCategory", Operator: "eq", Value: json.RawMessage(`"traffic_delay"`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, fix.TenantID, operatorID, rule.ID, domain.RuleStatusActive)
	require.NoError(t, err)

	eventID := eventIDHex(fix.ExceptionID)
	seed := domain.EnsureExceptionSeed{EventID: eventID, ShipmentID: fix.ShipmentID.String(), EventType: "vehicle_breakdown", Source: "driver", Severity: "high"}
	_, err = workflowRepo.EnsureExceptionWorkflows(ctx, fix.TenantID, []domain.EnsureExceptionSeed{seed})
	require.NoError(t, err)

	outcome, err := automationSvc.EvaluateTrigger(ctx, fix.TenantID, service.BuildExceptionCreatedTrigger(fix.TenantID, seed), true)
	require.NoError(t, err)
	require.Empty(t, outcome.Recommendations)
}
