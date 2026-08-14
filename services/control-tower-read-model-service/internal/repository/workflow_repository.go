package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

var ErrWorkflowNotFound = errors.New("workflow not found")

type WorkflowRepository struct {
	pool *pgxpool.Pool
}

func NewWorkflowRepository(pool *pgxpool.Pool) *WorkflowRepository {
	return &WorkflowRepository{pool: pool}
}

func (r *WorkflowRepository) AcknowledgeWithWorkflow(
	ctx context.Context,
	ackInput domain.AcknowledgeCriticalEventInput,
) (domain.CriticalEventAcknowledgement, domain.CriticalEventWorkflow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	source := ackInput.Source
	if source == "" {
		source = "control-tower"
	}

	const insertAckSQL = `
INSERT INTO control_tower.critical_event_acknowledgement (
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, acknowledged_at, acknowledged_by_user_id
) VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
ON CONFLICT (tenant_id, event_id) DO NOTHING`

	if _, err := tx.Exec(ctx, insertAckSQL,
		ackInput.TenantID,
		ackInput.EventID,
		ackInput.ShipmentID,
		ackInput.EventType,
		source,
		ackInput.OccurredAt.UTC(),
		ackInput.UserID,
	); err != nil {
		return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, fmt.Errorf("insert acknowledgement: %w", err)
	}

	workflow, created, err := r.ensureAcknowledgedWorkflow(ctx, tx, domain.AcknowledgeCriticalEventWorkflowInput{
		TenantID:   ackInput.TenantID,
		UserID:     ackInput.UserID,
		EventID:    ackInput.EventID,
		ShipmentID: ackInput.ShipmentID,
		EventType:  ackInput.EventType,
		Source:     source,
		OccurredAt: ackInput.OccurredAt,
	})
	if err != nil {
		return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, err
	}

	if created {
		if err := r.insertAction(ctx, tx, ackInput.TenantID, ackInput.EventID, domain.ActionTypeAcknowledged, ackInput.UserID, nil); err != nil {
			return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, err
		}
	}

	row, err := loadAcknowledgementTx(ctx, tx, ackInput.TenantID, ackInput.EventID)
	if err != nil {
		return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, err
	}
	if row == nil {
		return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, fmt.Errorf("acknowledgement missing after upsert")
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.CriticalEventAcknowledgement{}, domain.CriticalEventWorkflow{}, fmt.Errorf("commit tx: %w", err)
	}

	return *row, workflow, nil
}

func (r *WorkflowRepository) AssignCriticalEvent(
	ctx context.Context,
	input domain.AssignCriticalEventInput,
) (domain.CriticalEventWorkflow, error) {
	if input.AssignedToUser == uuid.Nil {
		return domain.CriticalEventWorkflow{}, apperrors.Validation("userId is required", map[string]any{"field": "userId"})
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
		return domain.CriticalEventWorkflow{}, apperrors.Conflict(
			"event must be acknowledged before assignment",
			map[string]any{"status": domain.WorkflowStatusOpen},
		)
	}
	if !domain.CanAssignFromStatus(workflow.Status) {
		return domain.CriticalEventWorkflow{}, apperrors.Conflict(
			fmt.Sprintf("cannot assign event in status %s", workflow.Status),
			map[string]any{"status": workflow.Status},
		)
	}

	actionType := domain.AssignActionType(workflow.Status)
	now := time.Now().UTC()

	const updateSQL = `
UPDATE control_tower.critical_event_workflow
SET status = $3,
    assigned_to_user_id = $4,
    assigned_by_user_id = $5,
    assigned_at = $6,
    resolved_by_user_id = NULL,
    resolved_at = NULL,
    resolution_code = NULL,
    resolution_comment = NULL,
    version = version + 1,
    updated_at = $6
WHERE tenant_id = $1
  AND event_id = $2
  AND version = $7
RETURNING
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at`

	updated, err := scanWorkflow(tx.QueryRow(ctx, updateSQL,
		input.TenantID,
		input.EventID,
		domain.WorkflowStatusAssigned,
		input.AssignedToUser,
		input.ActorUserID,
		now,
		workflow.Version,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CriticalEventWorkflow{}, apperrors.Conflict("workflow was updated concurrently", map[string]any{"field": "version"})
		}
		return domain.CriticalEventWorkflow{}, fmt.Errorf("update workflow assign: %w", err)
	}

	metadata := map[string]any{"assignedToUserId": input.AssignedToUser.String()}
	if err := r.insertAction(ctx, tx, input.TenantID, input.EventID, actionType, input.ActorUserID, metadata); err != nil {
		return domain.CriticalEventWorkflow{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.CriticalEventWorkflow{}, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

func (r *WorkflowRepository) ResolveCriticalEvent(
	ctx context.Context,
	input domain.ResolveCriticalEventInput,
) (domain.CriticalEventWorkflow, error) {
	code := input.ResolutionCode
	if !domain.ValidResolutionCode(code) {
		return domain.CriticalEventWorkflow{}, apperrors.Validation("invalid resolutionCode", map[string]any{"field": "resolutionCode"})
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
	if workflow.Status != domain.WorkflowStatusAssigned {
		return domain.CriticalEventWorkflow{}, apperrors.Conflict(
			fmt.Sprintf("cannot resolve event in status %s", workflow.Status),
			map[string]any{"status": workflow.Status},
		)
	}

	now := time.Now().UTC()
	const updateSQL = `
UPDATE control_tower.critical_event_workflow
SET status = $3,
    resolved_by_user_id = $4,
    resolved_at = $5,
    resolution_code = $6,
    resolution_comment = $7,
    version = version + 1,
    updated_at = $5
WHERE tenant_id = $1
  AND event_id = $2
  AND version = $8
RETURNING
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at`

	updated, err := scanWorkflow(tx.QueryRow(ctx, updateSQL,
		input.TenantID,
		input.EventID,
		domain.WorkflowStatusResolved,
		input.ActorUserID,
		now,
		code,
		input.ResolutionComment,
		workflow.Version,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CriticalEventWorkflow{}, apperrors.Conflict("workflow was updated concurrently", map[string]any{"field": "version"})
		}
		return domain.CriticalEventWorkflow{}, fmt.Errorf("update workflow resolve: %w", err)
	}

	metadata := map[string]any{"resolutionCode": code}
	if input.ResolutionComment != nil && *input.ResolutionComment != "" {
		metadata["comment"] = *input.ResolutionComment
	}
	if err := r.insertAction(ctx, tx, input.TenantID, input.EventID, domain.ActionTypeResolved, input.ActorUserID, metadata); err != nil {
		return domain.CriticalEventWorkflow{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.CriticalEventWorkflow{}, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

func (r *WorkflowRepository) ReopenCriticalEvent(
	ctx context.Context,
	input domain.ReopenCriticalEventInput,
) (domain.CriticalEventWorkflow, error) {
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
	if workflow.Status != domain.WorkflowStatusResolved {
		return domain.CriticalEventWorkflow{}, apperrors.Conflict(
			fmt.Sprintf("cannot reopen event in status %s", workflow.Status),
			map[string]any{"status": workflow.Status},
		)
	}

	now := time.Now().UTC()
	const updateSQL = `
UPDATE control_tower.critical_event_workflow
SET status = $3,
    assigned_to_user_id = NULL,
    assigned_by_user_id = NULL,
    assigned_at = NULL,
    resolved_by_user_id = NULL,
    resolved_at = NULL,
    resolution_code = NULL,
    resolution_comment = NULL,
    last_reopened_at = $4,
    last_reopened_by_user_id = $5,
    version = version + 1,
    updated_at = $4
WHERE tenant_id = $1
  AND event_id = $2
  AND version = $6
RETURNING
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at`

	updated, err := scanWorkflow(tx.QueryRow(ctx, updateSQL,
		input.TenantID,
		input.EventID,
		domain.WorkflowStatusOpen,
		now,
		input.ActorUserID,
		workflow.Version,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CriticalEventWorkflow{}, apperrors.Conflict("workflow was updated concurrently", map[string]any{"field": "version"})
		}
		return domain.CriticalEventWorkflow{}, fmt.Errorf("update workflow reopen: %w", err)
	}

	if err := r.insertAction(ctx, tx, input.TenantID, input.EventID, domain.ActionTypeReopened, input.ActorUserID, nil); err != nil {
		return domain.CriticalEventWorkflow{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.CriticalEventWorkflow{}, fmt.Errorf("commit tx: %w", err)
	}
	return updated, nil
}

func (r *WorkflowRepository) ListActions(
	ctx context.Context,
	tenantID uuid.UUID,
	eventID string,
) ([]domain.CriticalEventAction, error) {
	const selectSQL = `
SELECT id, tenant_id, event_id, action_type, actor_user_id, occurred_at, metadata
FROM control_tower.critical_event_action
WHERE tenant_id = $1
  AND event_id = $2
ORDER BY occurred_at ASC, id ASC`

	rows, err := r.pool.Query(ctx, selectSQL, tenantID, eventID)
	if err != nil {
		return nil, fmt.Errorf("list actions: %w", err)
	}
	defer rows.Close()

	items := make([]domain.CriticalEventAction, 0)
	for rows.Next() {
		item, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *WorkflowRepository) LookupWorkflows(
	ctx context.Context,
	tenantID uuid.UUID,
	eventIDs []string,
) ([]domain.CriticalEventWorkflow, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}

	const lookupSQL = `
SELECT
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at
FROM control_tower.critical_event_workflow
WHERE tenant_id = $1
  AND event_id = ANY($2::text[])`

	rows, err := r.pool.Query(ctx, lookupSQL, tenantID, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("lookup workflows: %w", err)
	}
	defer rows.Close()

	items := make([]domain.CriticalEventWorkflow, 0)
	for rows.Next() {
		item, err := scanActionWorkflow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *WorkflowRepository) GetWorkflow(
	ctx context.Context,
	tenantID uuid.UUID,
	eventID string,
) (*domain.CriticalEventWorkflow, error) {
	const selectSQL = `
SELECT
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at
FROM control_tower.critical_event_workflow
WHERE tenant_id = $1
  AND event_id = $2`

	row := r.pool.QueryRow(ctx, selectSQL, tenantID, eventID)
	item, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *WorkflowRepository) ensureAcknowledgedWorkflow(
	ctx context.Context,
	tx pgx.Tx,
	input domain.AcknowledgeCriticalEventWorkflowInput,
) (domain.CriticalEventWorkflow, bool, error) {
	workflow, err := r.lockWorkflow(ctx, tx, input.TenantID, input.EventID)
	if err != nil {
		return domain.CriticalEventWorkflow{}, false, err
	}

	now := time.Now().UTC()
	if workflow == nil {
		const insertSQL = `
INSERT INTO control_tower.critical_event_workflow (
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, $9, $8, $8)
RETURNING
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at`

		created, err := scanWorkflow(tx.QueryRow(ctx, insertSQL,
			input.TenantID,
			input.EventID,
			input.ShipmentID,
			input.EventType,
			input.Source,
			input.OccurredAt.UTC(),
			domain.WorkflowStatusAcknowledged,
			now,
			input.UserID,
		))
		if err != nil {
			return domain.CriticalEventWorkflow{}, false, fmt.Errorf("insert workflow: %w", err)
		}
		return created, true, nil
	}

	switch workflow.Status {
	case domain.WorkflowStatusOpen:
		const updateSQL = `
UPDATE control_tower.critical_event_workflow
SET status = $3,
    acknowledged_at = COALESCE(acknowledged_at, $4),
    acknowledged_by_user_id = COALESCE(acknowledged_by_user_id, $5),
    version = version + 1,
    updated_at = $4
WHERE tenant_id = $1
  AND event_id = $2
  AND version = $6
RETURNING
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at`

		updated, err := scanWorkflow(tx.QueryRow(ctx, updateSQL,
			input.TenantID,
			input.EventID,
			domain.WorkflowStatusAcknowledged,
			now,
			input.UserID,
			workflow.Version,
		))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.CriticalEventWorkflow{}, false, apperrors.Conflict("workflow was updated concurrently", map[string]any{"field": "version"})
			}
			return domain.CriticalEventWorkflow{}, false, fmt.Errorf("update workflow acknowledge: %w", err)
		}
		return updated, true, nil
	default:
		return *workflow, false, nil
	}
}

func (r *WorkflowRepository) lockWorkflow(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	eventID string,
) (*domain.CriticalEventWorkflow, error) {
	const selectSQL = `
SELECT
    tenant_id, event_id, shipment_id, event_type, source, occurred_at, status, version,
    acknowledged_at, acknowledged_by_user_id,
    assigned_to_user_id, assigned_by_user_id, assigned_at,
    resolved_by_user_id, resolved_at, resolution_code, resolution_comment,
    last_reopened_at, last_reopened_by_user_id, created_at, updated_at
FROM control_tower.critical_event_workflow
WHERE tenant_id = $1
  AND event_id = $2
FOR UPDATE`

	row := tx.QueryRow(ctx, selectSQL, tenantID, eventID)
	item, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *WorkflowRepository) insertAction(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	eventID string,
	actionType string,
	actorUserID uuid.UUID,
	metadata map[string]any,
) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal action metadata: %w", err)
	}

	const insertSQL = `
INSERT INTO control_tower.critical_event_action (
    tenant_id, event_id, action_type, actor_user_id, occurred_at, metadata
) VALUES ($1, $2, $3, $4, NOW(), $5::jsonb)`

	_, err = tx.Exec(ctx, insertSQL, tenantID, eventID, actionType, actorUserID, string(raw))
	if err != nil {
		return fmt.Errorf("insert action: %w", err)
	}
	return nil
}

func loadAcknowledgementTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	eventID string,
) (*domain.CriticalEventAcknowledgement, error) {
	const selectSQL = `
SELECT tenant_id, event_id, shipment_id, event_type, source, occurred_at, acknowledged_at, acknowledged_by_user_id
FROM control_tower.critical_event_acknowledgement
WHERE tenant_id = $1 AND event_id = $2`

	row := tx.QueryRow(ctx, selectSQL, tenantID, eventID)
	item, err := scanAcknowledgement(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

type workflowScanner interface {
	Scan(dest ...any) error
}

func scanWorkflow(row workflowScanner) (domain.CriticalEventWorkflow, error) {
	var item domain.CriticalEventWorkflow
	var (
		occurredAt time.Time
		createdAt  time.Time
		updatedAt  time.Time
		ackAt      *time.Time
		ackBy      *uuid.UUID
		assignTo   *uuid.UUID
		assignBy   *uuid.UUID
		assignAt   *time.Time
		resolvedBy *uuid.UUID
		resolvedAt *time.Time
		resCode    *string
		resComment *string
		reopenedAt *time.Time
		reopenedBy *uuid.UUID
	)
	if err := row.Scan(
		&item.TenantID,
		&item.EventID,
		&item.ShipmentID,
		&item.EventType,
		&item.Source,
		&occurredAt,
		&item.Status,
		&item.Version,
		&ackAt,
		&ackBy,
		&assignTo,
		&assignBy,
		&assignAt,
		&resolvedBy,
		&resolvedAt,
		&resCode,
		&resComment,
		&reopenedAt,
		&reopenedBy,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.CriticalEventWorkflow{}, err
	}
	item.OccurredAt = occurredAt.UTC()
	item.AcknowledgedAt = ackAt
	item.AcknowledgedByUserID = ackBy
	item.AssignedToUserID = assignTo
	item.AssignedByUserID = assignBy
	item.AssignedAt = assignAt
	item.ResolvedByUserID = resolvedBy
	item.ResolvedAt = resolvedAt
	item.ResolutionCode = resCode
	item.ResolutionComment = resComment
	item.LastReopenedAt = reopenedAt
	item.LastReopenedByUserID = reopenedBy
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()
	return item, nil
}

func scanActionWorkflow(row workflowScanner) (domain.CriticalEventWorkflow, error) {
	return scanWorkflow(row)
}

func scanAction(row workflowScanner) (domain.CriticalEventAction, error) {
	var item domain.CriticalEventAction
	var occurredAt time.Time
	var metadataRaw []byte
	if err := row.Scan(
		&item.ID,
		&item.TenantID,
		&item.EventID,
		&item.ActionType,
		&item.ActorUserID,
		&occurredAt,
		&metadataRaw,
	); err != nil {
		return domain.CriticalEventAction{}, err
	}
	item.OccurredAt = occurredAt.UTC()
	item.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
	}
	return item, nil
}
