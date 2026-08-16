//go:build integration

package automation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

func TestAckUnknownEventNoRow(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	workflowRepo := repository.NewWorkflowRepository(env.pool)

	_, _, err := workflowRepo.AcknowledgeWithWorkflow(ctx, domain.AcknowledgeCriticalEventInput{
		TenantID:   fix.TenantID,
		UserID:     uuid.New(),
		EventID:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ShipmentID: fix.ShipmentID,
		EventType:  "vehicle_breakdown",
		Source:     domain.ControlTowerEventSourceDriver,
		OccurredAt: time.Now().UTC(),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)

	require.Equal(t, int64(0), countRows(ctx, env.pool,
		`SELECT COUNT(*) FROM control_tower.critical_event_acknowledgement WHERE tenant_id=$1 AND event_id=$2`,
		fix.TenantID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))
}

func TestAckCrossTenantNoRow(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	otherTenant := uuid.New()
	workflowRepo := repository.NewWorkflowRepository(env.pool)

	eventID := "cccccccccccccccccccccccccccccccc"
	_, err := env.pool.Exec(ctx, `
INSERT INTO control_tower.critical_event_workflow (
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    priority, exception_category, business_impact, exception_activated_at,
    acknowledge_due_at, assignment_due_at, resolution_due_at, escalation_level,
    created_at, updated_at
	) VALUES (
	    $1, $2, $3, 'vehicle_breakdown', 'driver', NOW(), 'open', 1,
	    'p1', 'other', 'none', NOW(),
    NOW() + interval '15 minutes', NOW() + interval '30 minutes', NOW() + interval '2 hours', 'none',
    NOW(), NOW()
)`, fix.TenantID, eventID, fix.ShipmentID)
	require.NoError(t, err)

	_, _, err = workflowRepo.AcknowledgeWithWorkflow(ctx, domain.AcknowledgeCriticalEventInput{
		TenantID:   otherTenant,
		UserID:     uuid.New(),
		EventID:    eventID,
		ShipmentID: fix.ShipmentID,
		EventType:  "vehicle_breakdown",
		Source:     domain.ControlTowerEventSourceDriver,
		OccurredAt: time.Now().UTC(),
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)

	require.Equal(t, int64(0), countRows(ctx, env.pool,
		`SELECT COUNT(*) FROM control_tower.critical_event_acknowledgement WHERE tenant_id=$1 AND event_id=$2`,
		otherTenant, eventID))
	require.Equal(t, int64(0), countRows(ctx, env.pool,
		`SELECT COUNT(*) FROM control_tower.critical_event_workflow WHERE tenant_id=$1 AND event_id=$2`,
		otherTenant, eventID))
}

func TestAckValidEventPersistsAcknowledgement(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	workflowRepo := repository.NewWorkflowRepository(env.pool)
	actorID := uuid.New()
	eventID := "dddddddddddddddddddddddddddddddd"

	_, err := env.pool.Exec(ctx, `
INSERT INTO control_tower.critical_event_workflow (
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    priority, exception_category, business_impact, exception_activated_at,
    acknowledge_due_at, assignment_due_at, resolution_due_at, escalation_level,
    created_at, updated_at
	) VALUES (
	    $1, $2, $3, 'vehicle_breakdown', 'driver', NOW(), 'open', 1,
	    'p1', 'other', 'none', NOW(),
    NOW() + interval '15 minutes', NOW() + interval '30 minutes', NOW() + interval '2 hours', 'none',
    NOW(), NOW()
)`, fix.TenantID, eventID, fix.ShipmentID)
	require.NoError(t, err)

	ack, workflow, err := workflowRepo.AcknowledgeWithWorkflow(ctx, domain.AcknowledgeCriticalEventInput{
		TenantID:   fix.TenantID,
		UserID:     actorID,
		EventID:    eventID,
		ShipmentID: fix.ShipmentID,
		EventType:  "vehicle_breakdown",
		Source:     domain.ControlTowerEventSourceDriver,
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, eventID, ack.EventID)
	require.Equal(t, domain.WorkflowStatusAcknowledged, workflow.Status)
	require.Equal(t, int64(1), countRows(ctx, env.pool,
		`SELECT COUNT(*) FROM control_tower.critical_event_acknowledgement WHERE tenant_id=$1 AND event_id=$2`,
		fix.TenantID, eventID))
}

func TestListOpenWorkflowsBySourceReturnsDriverWorkflows(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	workflowRepo := repository.NewWorkflowRepository(env.pool)
	eventID := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

	_, err := env.pool.Exec(ctx, `
INSERT INTO control_tower.critical_event_workflow (
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    priority, exception_category, business_impact, exception_activated_at,
    acknowledge_due_at, assignment_due_at, resolution_due_at, escalation_level,
    created_at, updated_at
	) VALUES (
	    $1, $2, $3, 'vehicle_breakdown', 'driver', NOW(), 'open', 1,
	    'p1', 'other', 'none', NOW(),
    NOW() + interval '15 minutes', NOW() + interval '30 minutes', NOW() + interval '2 hours', 'none',
    NOW(), NOW()
)`, fix.TenantID, eventID, fix.ShipmentID)
	require.NoError(t, err)

	items, err := workflowRepo.ListOpenWorkflowsBySource(ctx, fix.TenantID, domain.ControlTowerEventSourceDriver)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, eventID, items[0].EventID)
}
