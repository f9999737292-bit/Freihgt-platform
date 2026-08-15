package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type ShipmentAssignment struct {
	TenantID uuid.UUID
	DriverID *uuid.UUID
}

type ShipmentLookupRepository struct {
	pool *pgxpool.Pool
}

func NewShipmentLookupRepository(pool *pgxpool.Pool) *ShipmentLookupRepository {
	return &ShipmentLookupRepository{pool: pool}
}

func (r *ShipmentLookupRepository) GetAssignedDriver(ctx context.Context, tenantID, shipmentID uuid.UUID) (ShipmentAssignment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT tenant_id, driver_id
		FROM transport.shipments
		WHERE id = $1 AND tenant_id = $2
	`, shipmentID, tenantID)
	var assignment ShipmentAssignment
	if err := row.Scan(&assignment.TenantID, &assignment.DriverID); err == pgx.ErrNoRows {
		return ShipmentAssignment{}, apperrors.NotFound("shipment not found")
	} else if err != nil {
		return ShipmentAssignment{}, err
	}
	return assignment, nil
}

func (r *ShipmentLookupRepository) ValidateDriverTenant(ctx context.Context, tenantID, driverID uuid.UUID) error {
	var foundTenant uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT tenant_id FROM transport.drivers WHERE id=$1`, driverID).Scan(&foundTenant)
	if err == pgx.ErrNoRows {
		return apperrors.NotFound("driver not found")
	}
	if err != nil {
		return err
	}
	if foundTenant != tenantID {
		return apperrors.Forbidden("driver tenant mismatch")
	}
	return nil
}

type GuardedActionRepository struct {
	pool *pgxpool.Pool
}

func NewGuardedActionRepository(pool *pgxpool.Pool) *GuardedActionRepository {
	return &GuardedActionRepository{pool: pool}
}

type CreateGuardedActionParams struct {
	TenantID        uuid.UUID
	ExecutionID     uuid.UUID
	ExecutionStepID uuid.UUID
	ActionType      string
	SafetyClass     string
	GuardDecision   string
	GuardReason     string
	Status          string
	DriverID        *uuid.UUID
	ShipmentID      *uuid.UUID
	CorrelationID   string
	SourceEventID   string
	IdempotencyKey  string
	ExpiresAt       *time.Time
}

func (r *GuardedActionRepository) CreateAction(ctx context.Context, params CreateGuardedActionParams) (domain.GuardedAction, bool, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.automation_guarded_action
		    (tenant_id, execution_id, execution_step_id, action_type, safety_class, guard_decision, guard_reason,
		     status, driver_id, shipment_id, correlation_id, source_event_id, idempotency_key, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (tenant_id, idempotency_key) DO UPDATE
		    SET updated_at = automation_guarded_action.updated_at
		RETURNING id, tenant_id, execution_id, execution_step_id, action_type, safety_class, guard_decision,
		          COALESCE(guard_reason,''), status, driver_id, shipment_id, driver_task_id,
		          COALESCE(correlation_id,''), COALESCE(source_event_id,''), idempotency_key,
		          response_payload, COALESCE(error_reason,''), expires_at, dispatched_at, completed_at,
		          created_at, updated_at, version,
		          (xmax = 0) AS inserted
	`, params.TenantID, params.ExecutionID, params.ExecutionStepID, params.ActionType, params.SafetyClass,
		params.GuardDecision, nullIfEmpty(params.GuardReason), params.Status, params.DriverID, params.ShipmentID,
		nullIfEmpty(params.CorrelationID), nullIfEmpty(params.SourceEventID), params.IdempotencyKey, params.ExpiresAt)
	action, inserted, err := scanGuardedActionWithInserted(row)
	return action, inserted, err
}

func (r *GuardedActionRepository) GetAction(ctx context.Context, tenantID, actionID uuid.UUID) (domain.GuardedAction, error) {
	row := r.pool.QueryRow(ctx, guardedActionSelect+" WHERE tenant_id=$1 AND id=$2", tenantID, actionID)
	action, err := scanGuardedAction(row)
	if err == pgx.ErrNoRows {
		return domain.GuardedAction{}, apperrors.NotFound("guarded action not found")
	}
	return action, err
}

func (r *GuardedActionRepository) GetActionByStep(ctx context.Context, tenantID, executionID, stepID uuid.UUID) (domain.GuardedAction, error) {
	row := r.pool.QueryRow(ctx, guardedActionSelect+" WHERE tenant_id=$1 AND execution_id=$2 AND execution_step_id=$3",
		tenantID, executionID, stepID)
	action, err := scanGuardedAction(row)
	if err == pgx.ErrNoRows {
		return domain.GuardedAction{}, apperrors.NotFound("guarded action not found")
	}
	return action, err
}

func (r *GuardedActionRepository) GetActionByDriverTask(ctx context.Context, tenantID, driverTaskID uuid.UUID) (domain.GuardedAction, error) {
	row := r.pool.QueryRow(ctx, guardedActionSelect+" WHERE tenant_id=$1 AND driver_task_id=$2", tenantID, driverTaskID)
	action, err := scanGuardedAction(row)
	if err == pgx.ErrNoRows {
		return domain.GuardedAction{}, apperrors.NotFound("guarded action not found")
	}
	return action, err
}

func (r *GuardedActionRepository) MarkDriverTaskDispatched(ctx context.Context, tenantID, actionID, driverTaskID uuid.UUID, status string) (domain.GuardedAction, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_guarded_action
		SET driver_task_id=$3, status=$4::varchar, dispatched_at=COALESCE(dispatched_at, NOW()), updated_at=NOW(), version=version+1
		WHERE tenant_id=$1 AND id=$2 AND status IN ('pending','running','waiting_approval')
		RETURNING `+guardedActionColumns,
		tenantID, actionID, driverTaskID, status)
	action, err := scanGuardedAction(row)
	if err == pgx.ErrNoRows {
		return domain.GuardedAction{}, apperrors.Conflict("action cannot accept driver task dispatch", nil)
	}
	return action, err
}

func (r *GuardedActionRepository) TransitionAction(ctx context.Context, tenantID, actionID uuid.UUID, fromStatuses []string, toStatus, reason string, responsePayload []byte) (domain.GuardedAction, error) {
	args := []any{tenantID, actionID, toStatus, nullIfEmpty(reason)}
	setResponse := ""
	if responsePayload != nil {
		setResponse = ", response_payload=$5::jsonb"
		args = append(args, string(responsePayload))
	}
	statusPlaceholders := make([]string, len(fromStatuses))
	base := len(args) + 1
	for i, st := range fromStatuses {
		args = append(args, st)
		statusPlaceholders[i] = fmt.Sprintf("$%d::varchar", base+i)
	}
	whereStatus := "status IN (" + strings.Join(statusPlaceholders, ",") + ")"
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_guarded_action
		SET status=$3::varchar, error_reason=COALESCE(NULLIF($4,''), error_reason)`+setResponse+`,
		    completed_at=CASE WHEN $3::varchar IN ('succeeded','failed','denied','rejected','timed_out','skipped') THEN NOW() ELSE completed_at END,
		    updated_at=NOW(), version=version+1
		WHERE tenant_id=$1 AND id=$2 AND `+whereStatus+`
		RETURNING `+guardedActionColumns, args...)
	action, err := scanGuardedAction(row)
	if err == pgx.ErrNoRows {
		return domain.GuardedAction{}, apperrors.Conflict("invalid guarded action transition", map[string]any{"toStatus": toStatus})
	}
	return action, err
}

func (r *GuardedActionRepository) ListActionsByExecution(ctx context.Context, tenantID, executionID uuid.UUID) ([]domain.GuardedAction, error) {
	rows, err := r.pool.Query(ctx, guardedActionSelect+" WHERE tenant_id=$1 AND execution_id=$2 ORDER BY created_at ASC", tenantID, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.GuardedAction{}
	for rows.Next() {
		action, err := scanGuardedAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, action)
	}
	return out, rows.Err()
}

func (r *GuardedActionRepository) CreateApproval(ctx context.Context, tenantID, actionID uuid.UUID, requiredLevel string) (domain.ActionApproval, bool, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.automation_action_approval
		    (tenant_id, guarded_action_id, required_level, status)
		VALUES ($1,$2,$3,'pending')
		ON CONFLICT (guarded_action_id) DO UPDATE SET guarded_action_id = automation_action_approval.guarded_action_id
		RETURNING id, tenant_id, guarded_action_id, required_level, status, requested_at,
		          approved_at, approved_by, rejected_at, rejected_by, COALESCE(reason,''), version,
		          (xmax = 0) AS inserted
	`, tenantID, actionID, requiredLevel)
	var approval domain.ActionApproval
	var inserted bool
	err := row.Scan(&approval.ID, &approval.TenantID, &approval.GuardedActionID, &approval.RequiredLevel, &approval.Status,
		&approval.RequestedAt, &approval.ApprovedAt, &approval.ApprovedBy, &approval.RejectedAt, &approval.RejectedBy,
		&approval.Reason, &approval.Version, &inserted)
	return approval, inserted, err
}

func (r *GuardedActionRepository) Approve(ctx context.Context, tenantID, actionID, userID uuid.UUID) (domain.ActionApproval, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_action_approval
		SET status='approved', approved_at=NOW(), approved_by=$3, version=version+1
		WHERE tenant_id=$1 AND guarded_action_id=$2 AND status='pending'
		RETURNING id, tenant_id, guarded_action_id, required_level, status, requested_at,
		          approved_at, approved_by, rejected_at, rejected_by, COALESCE(reason,''), version
	`, tenantID, actionID, userID)
	approval, err := scanApproval(row)
	if err == pgx.ErrNoRows {
		return domain.ActionApproval{}, apperrors.Conflict("approval is not pending", nil)
	}
	return approval, err
}

func (r *GuardedActionRepository) RejectApproval(ctx context.Context, tenantID, actionID, userID uuid.UUID, reason string) (domain.ActionApproval, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_action_approval
		SET status='rejected', rejected_at=NOW(), rejected_by=$3, reason=$4, version=version+1
		WHERE tenant_id=$1 AND guarded_action_id=$2 AND status='pending'
		RETURNING id, tenant_id, guarded_action_id, required_level, status, requested_at,
		          approved_at, approved_by, rejected_at, rejected_by, COALESCE(reason,''), version
	`, tenantID, actionID, userID, nullIfEmpty(reason))
	approval, err := scanApproval(row)
	if err == pgx.ErrNoRows {
		return domain.ActionApproval{}, apperrors.Conflict("approval is not pending", nil)
	}
	return approval, err
}

func (r *GuardedActionRepository) GetApprovalByAction(ctx context.Context, tenantID, actionID uuid.UUID) (domain.ActionApproval, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, guarded_action_id, required_level, status, requested_at,
		       approved_at, approved_by, rejected_at, rejected_by, COALESCE(reason,''), version
		FROM control_tower.automation_action_approval
		WHERE tenant_id=$1 AND guarded_action_id=$2
	`, tenantID, actionID)
	approval, err := scanApproval(row)
	if err == pgx.ErrNoRows {
		return domain.ActionApproval{}, apperrors.NotFound("approval not found")
	}
	return approval, err
}

func (r *GuardedActionRepository) GetTenantActionPolicy(ctx context.Context, tenantID uuid.UUID, actionType string) (*domain.TenantActionPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT tenant_id, action_type, enabled, approval_level, priority_ceiling
		FROM control_tower.automation_tenant_action_policy
		WHERE tenant_id=$1 AND action_type=$2
	`, tenantID, actionType)
	var policy domain.TenantActionPolicy
	if err := row.Scan(&policy.TenantID, &policy.ActionType, &policy.Enabled, &policy.ApprovalLevel, &policy.PriorityCeiling); err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *GuardedActionRepository) CreateTimeoutEscalation(ctx context.Context, tenantID, actionID uuid.UUID, caseID *uuid.UUID, idempotencyKey string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	var inserted bool
	err := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.automation_timeout_escalation
		    (tenant_id, guarded_action_id, case_id, idempotency_key)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (tenant_id, idempotency_key) DO UPDATE
		    SET id = automation_timeout_escalation.id
		RETURNING id, (xmax = 0) AS inserted
	`, tenantID, actionID, caseID, idempotencyKey).Scan(&id, &inserted)
	return id, inserted, err
}

func (r *GuardedActionRepository) UpdateExecutionStepStatus(ctx context.Context, tenantID, stepID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE control_tower.playbook_execution_step
		SET status=$3::varchar, completed_at=CASE WHEN $3::varchar IN ('done','skipped','denied','failed','rejected','timed_out') THEN NOW() ELSE completed_at END
		WHERE tenant_id=$1 AND id=$2
	`, tenantID, stepID, status)
	return err
}

func (r *GuardedActionRepository) InsertCaseTimelineEvent(ctx context.Context, tenantID, caseID uuid.UUID, actionType string, meta map[string]any) error {
	raw, _ := json.Marshal(meta)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO control_tower.operational_case_event (tenant_id, case_id, source, action_type, metadata)
		VALUES ($1,$2,'automation_guarded_action',$3,$4::jsonb)
	`, tenantID, caseID, actionType, string(raw))
	return err
}

func (r *GuardedActionRepository) InsertAudit(ctx context.Context, tenantID, eventType string, actorType string, actorUserID *uuid.UUID, executionID *uuid.UUID, payload map[string]any) error {
	raw, _ := json.Marshal(payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO control_tower.automation_audit_event
		    (tenant_id, event_type, actor_type, actor_user_id, execution_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
	`, tenantID, eventType, actorType, actorUserID, executionID, string(raw))
	return err
}

const guardedActionColumns = `id, tenant_id, execution_id, execution_step_id, action_type, safety_class, guard_decision,
	COALESCE(guard_reason,''), status, driver_id, shipment_id, driver_task_id,
	COALESCE(correlation_id,''), COALESCE(source_event_id,''), idempotency_key,
	response_payload, COALESCE(error_reason,''), expires_at, dispatched_at, completed_at,
	created_at, updated_at, version`

const guardedActionSelect = `SELECT ` + guardedActionColumns + ` FROM control_tower.automation_guarded_action`

func scanGuardedAction(row pgx.Row) (domain.GuardedAction, error) {
	var action domain.GuardedAction
	var responseRaw []byte
	err := row.Scan(&action.ID, &action.TenantID, &action.ExecutionID, &action.ExecutionStepID, &action.ActionType,
		&action.SafetyClass, &action.GuardDecision, &action.GuardReason, &action.Status, &action.DriverID, &action.ShipmentID,
		&action.DriverTaskID, &action.CorrelationID, &action.SourceEventID, &action.IdempotencyKey, &responseRaw,
		&action.ErrorReason, &action.ExpiresAt, &action.DispatchedAt, &action.CompletedAt, &action.CreatedAt, &action.UpdatedAt, &action.Version)
	if err != nil {
		return domain.GuardedAction{}, err
	}
	if len(responseRaw) > 0 {
		action.ResponsePayload = append([]byte(nil), responseRaw...)
	}
	return action, nil
}

func scanGuardedActionWithInserted(row pgx.Row) (domain.GuardedAction, bool, error) {
	var action domain.GuardedAction
	var responseRaw []byte
	var inserted bool
	err := row.Scan(&action.ID, &action.TenantID, &action.ExecutionID, &action.ExecutionStepID, &action.ActionType,
		&action.SafetyClass, &action.GuardDecision, &action.GuardReason, &action.Status, &action.DriverID, &action.ShipmentID,
		&action.DriverTaskID, &action.CorrelationID, &action.SourceEventID, &action.IdempotencyKey, &responseRaw,
		&action.ErrorReason, &action.ExpiresAt, &action.DispatchedAt, &action.CompletedAt, &action.CreatedAt, &action.UpdatedAt, &action.Version, &inserted)
	if len(responseRaw) > 0 {
		action.ResponsePayload = append([]byte(nil), responseRaw...)
	}
	return action, inserted, err
}

func scanApproval(row pgx.Row) (domain.ActionApproval, error) {
	var approval domain.ActionApproval
	err := row.Scan(&approval.ID, &approval.TenantID, &approval.GuardedActionID, &approval.RequiredLevel, &approval.Status,
		&approval.RequestedAt, &approval.ApprovedAt, &approval.ApprovedBy, &approval.RejectedAt, &approval.RejectedBy,
		&approval.Reason, &approval.Version)
	return approval, err
}
