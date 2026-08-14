package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

const workflowSelectColumns = `
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id,
    priority, exception_category, business_impact, exception_activated_at,
    acknowledge_due_at, assignment_due_at, resolution_due_at, escalation_level,
    ack_sla_breached_at, assign_sla_breached_at, resolve_sla_breached_at,
    created_at, updated_at`

func (r *WorkflowRepository) EnsureExceptionWorkflows(
	ctx context.Context,
	tenantID uuid.UUID,
	seeds []domain.EnsureExceptionSeed,
) error {
	if len(seeds) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	for _, seed := range seeds {
		priority := domain.DefaultPriorityForSeverity(seed.Severity)
		category := domain.DefaultCategoryForEventType(seed.EventType)
		activatedAt := seed.OccurredAt.UTC()
		deadlines := domain.CalculateDeadlines(priority, activatedAt)
		source := seed.Source
		if source == "" {
			source = "control-tower"
		}
		shipmentID, err := uuid.Parse(seed.ShipmentID)
		if err != nil {
			continue
		}

		const insertSQL = `
INSERT INTO control_tower.critical_event_workflow (
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    priority, exception_category, business_impact, exception_activated_at,
    acknowledge_due_at, assignment_due_at, resolution_due_at, escalation_level,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, 'open', 1,
    $7, $8, $9, $10,
    $11, $12, $13, 'none',
    $14, $14
)
ON CONFLICT (tenant_id, event_id) DO NOTHING`

		if _, err := tx.Exec(ctx, insertSQL,
			tenantID,
			seed.EventID,
			shipmentID,
			seed.EventType,
			source,
			activatedAt,
			priority,
			category,
			domain.BusinessImpactNone,
			activatedAt,
			deadlines.AcknowledgeDueAt,
			deadlines.AssignmentDueAt,
			deadlines.ResolutionDueAt,
			now,
		); err != nil {
			return fmt.Errorf("ensure exception workflow: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ensure exception workflows: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) UpdateException(
	ctx context.Context,
	input domain.UpdateExceptionInput,
) (domain.CriticalEventWorkflow, error) {
	if input.Priority == nil && input.Category == nil && input.BusinessImpact == nil {
		return domain.CriticalEventWorkflow{}, apperrors.Validation("at least one field is required", map[string]any{"field": "body"})
	}
	if input.Priority != nil && !domain.ValidPriority(*input.Priority) {
		return domain.CriticalEventWorkflow{}, apperrors.Validation("invalid priority", map[string]any{"field": "priority"})
	}
	if input.Category != nil && !domain.ValidExceptionCategory(*input.Category) {
		return domain.CriticalEventWorkflow{}, apperrors.Validation("invalid category", map[string]any{"field": "category"})
	}
	if input.BusinessImpact != nil && !domain.ValidBusinessImpact(*input.BusinessImpact) {
		return domain.CriticalEventWorkflow{}, apperrors.Validation("invalid businessImpact", map[string]any{"field": "businessImpact"})
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CriticalEventWorkflow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	workflow, err := r.lockWorkflow(ctx, tx, input.TenantID, input.EventID)
	if err != nil {
		return domain.CriticalEventWorkflow{}, err
	}
	if workflow == nil {
		return domain.CriticalEventWorkflow{}, apperrors.NotFound("critical event workflow not found")
	}
	if workflow.Status == domain.WorkflowStatusResolved {
		return domain.CriticalEventWorkflow{}, apperrors.Conflict("cannot update exception on resolved event", map[string]any{"status": workflow.Status})
	}

	changes := map[string]any{}
	newPriority := workflow.Priority
	newCategory := workflow.ExceptionCategory
	newImpact := workflow.BusinessImpact
	if input.Priority != nil {
		newPriority = *input.Priority
		changes["priority"] = newPriority
	}
	if input.Category != nil {
		newCategory = *input.Category
		changes["exceptionCategory"] = newCategory
	}
	if input.BusinessImpact != nil {
		newImpact = *input.BusinessImpact
		changes["businessImpact"] = newImpact
	}

	deadlines := domain.RecalculateUnresolvedDeadlines(*workflow, newPriority)
	now := time.Now().UTC()
	newEscalation := workflow.EscalationLevel

	const updateSQL = `
UPDATE control_tower.critical_event_workflow
SET priority = $3,
    exception_category = $4,
    business_impact = $5,
    acknowledge_due_at = $6,
    assignment_due_at = $7,
    resolution_due_at = $8,
    escalation_level = $9,
    version = version + 1,
    updated_at = $10
WHERE tenant_id = $1
  AND event_id = $2
  AND version = $11
RETURNING` + workflowSelectColumns

	updated, err := scanWorkflow(tx.QueryRow(ctx, updateSQL,
		input.TenantID,
		input.EventID,
		newPriority,
		newCategory,
		newImpact,
		deadlines.AcknowledgeDueAt,
		deadlines.AssignmentDueAt,
		deadlines.ResolutionDueAt,
		newEscalation,
		now,
		workflow.Version,
	))
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.CriticalEventWorkflow{}, apperrors.Conflict("workflow was updated concurrently", map[string]any{"field": "version"})
		}
		return domain.CriticalEventWorkflow{}, fmt.Errorf("update exception: %w", err)
	}

	if err := r.insertAction(ctx, tx, input.TenantID, input.EventID, domain.ActionTypeExceptionUpdated, input.ActorUserID, changes); err != nil {
		return domain.CriticalEventWorkflow{}, err
	}

	updated, err = r.processSLAState(ctx, tx, updated, input.ActorUserID, now)
	if err != nil {
		return domain.CriticalEventWorkflow{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.CriticalEventWorkflow{}, fmt.Errorf("commit update exception: %w", err)
	}
	return updated, nil
}

func (r *WorkflowRepository) LookupWorkflowsWithExceptionProcessing(
	ctx context.Context,
	tenantID uuid.UUID,
	eventIDs []string,
	actorUserID uuid.UUID,
) ([]domain.CriticalEventWorkflow, error) {
	items, err := r.LookupWorkflows(ctx, tenantID, eventIDs)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	processed := make([]domain.CriticalEventWorkflow, 0, len(items))
	for _, item := range items {
		locked, err := r.lockWorkflow(ctx, tx, tenantID, item.EventID)
		if err != nil {
			return nil, err
		}
		if locked == nil {
			continue
		}
		updated, err := r.processSLAState(ctx, tx, *locked, actorUserID, now)
		if err != nil {
			return nil, err
		}
		processed = append(processed, updated)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit sla processing: %w", err)
	}
	return processed, nil
}

func (r *WorkflowRepository) processSLAState(
	ctx context.Context,
	tx pgx.Tx,
	workflow domain.CriticalEventWorkflow,
	actorUserID uuid.UUID,
	now time.Time,
) (domain.CriticalEventWorkflow, error) {
	if workflow.Status == domain.WorkflowStatusResolved {
		return workflow, nil
	}

	sla := domain.EvaluateSLA(workflow, now)
	desiredEscalation := domain.EvaluateEscalation(workflow.Priority, sla)
	metadata := map[string]any{"phase": sla.Phase, "status": sla.Status}

	if sla.Status == domain.SLAStatusBreached {
		switch sla.Phase {
		case domain.SLAPhaseAcknowledgement:
			if workflow.AckSLABreachedAt == nil {
				if err := r.recordSLABreach(ctx, tx, workflow, actorUserID, domain.ActionTypeAckSLABreached, "ack_sla_breached_at", metadata); err != nil {
					return domain.CriticalEventWorkflow{}, err
				}
				workflow.AckSLABreachedAt = &now
			}
		case domain.SLAPhaseAssignment:
			if workflow.AssignSLABreachedAt == nil {
				if err := r.recordSLABreach(ctx, tx, workflow, actorUserID, domain.ActionTypeAssignSLABreached, "assign_sla_breached_at", metadata); err != nil {
					return domain.CriticalEventWorkflow{}, err
				}
				workflow.AssignSLABreachedAt = &now
			}
		case domain.SLAPhaseResolution:
			if workflow.ResolveSLABreachedAt == nil {
				if err := r.recordSLABreach(ctx, tx, workflow, actorUserID, domain.ActionTypeResolveSLABreached, "resolve_sla_breached_at", metadata); err != nil {
					return domain.CriticalEventWorkflow{}, err
				}
				workflow.ResolveSLABreachedAt = &now
			}
		}
	}

	if desiredEscalation != workflow.EscalationLevel {
		const updateEscalationSQL = `
UPDATE control_tower.critical_event_workflow
SET escalation_level = $3, updated_at = $4
WHERE tenant_id = $1 AND event_id = $2`

		if _, err := tx.Exec(ctx, updateEscalationSQL, workflow.TenantID, workflow.EventID, desiredEscalation, now); err != nil {
			return domain.CriticalEventWorkflow{}, fmt.Errorf("update escalation: %w", err)
		}
		escalationMeta := map[string]any{
			"from":   workflow.EscalationLevel,
			"to":     desiredEscalation,
			"reason": sla.Status,
			"phase":  sla.Phase,
		}
		if err := r.insertAction(ctx, tx, workflow.TenantID, workflow.EventID, domain.ActionTypeEscalationChanged, actorUserID, escalationMeta); err != nil {
			return domain.CriticalEventWorkflow{}, err
		}
		workflow.EscalationLevel = desiredEscalation
	}

	return workflow, nil
}

func (r *WorkflowRepository) recordSLABreach(
	ctx context.Context,
	tx pgx.Tx,
	workflow domain.CriticalEventWorkflow,
	actorUserID uuid.UUID,
	actionType string,
	column string,
	metadata map[string]any,
) error {
	query := fmt.Sprintf(`
UPDATE control_tower.critical_event_workflow
SET %s = NOW(), updated_at = NOW()
WHERE tenant_id = $1 AND event_id = $2 AND %s IS NULL`, column, column)
	if _, err := tx.Exec(ctx, query, workflow.TenantID, workflow.EventID); err != nil {
		return fmt.Errorf("record sla breach column: %w", err)
	}
	systemActor := actorUserID
	if systemActor == uuid.Nil {
		systemActor = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	}
	raw, _ := json.Marshal(metadata)
	_ = raw
	return r.insertAction(ctx, tx, workflow.TenantID, workflow.EventID, actionType, systemActor, metadata)
}
