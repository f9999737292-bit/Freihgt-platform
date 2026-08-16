package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type PlaybookFilter struct {
	Status string
	Page   int
	Limit  int
}

func (r *AutomationRepository) ListPlaybooks(ctx context.Context, tenantID uuid.UUID, filter PlaybookFilter) (domain.Page[domain.OperationalPlaybook], error) {
	page, limit := normalizePage(filter.Page, filter.Limit)
	args := []any{tenantID}
	where := "tenant_id = $1"
	if s := strings.TrimSpace(filter.Status); s != "" {
		args = append(args, s)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM control_tower.operational_playbook WHERE "+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.OperationalPlaybook]{}, err
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, COALESCE(description,''), status, current_version,
		       created_by_user_id, updated_by_user_id, created_at, updated_at
		FROM control_tower.operational_playbook WHERE %s ORDER BY updated_at DESC LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.OperationalPlaybook]{}, err
	}
	defer rows.Close()
	items := []domain.OperationalPlaybook{}
	for rows.Next() {
		var p domain.OperationalPlaybook
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CurrentVersion,
			&p.CreatedByUserID, &p.UpdatedByUserID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return domain.Page[domain.OperationalPlaybook]{}, err
		}
		items = append(items, p)
	}
	return domain.Page[domain.OperationalPlaybook]{Items: items, Page: page, Limit: limit, Total: total, HasNext: page*limit < total}, rows.Err()
}

func (r *AutomationRepository) GetPlaybook(ctx context.Context, tenantID, playbookID uuid.UUID) (domain.OperationalPlaybook, domain.PlaybookVersion, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), status, current_version,
		       created_by_user_id, updated_by_user_id, created_at, updated_at
		FROM control_tower.operational_playbook WHERE tenant_id=$1 AND id=$2
	`, tenantID, playbookID)
	var p domain.OperationalPlaybook
	if err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CurrentVersion,
		&p.CreatedByUserID, &p.UpdatedByUserID, &p.CreatedAt, &p.UpdatedAt); err == pgx.ErrNoRows {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, apperrors.NotFound("playbook not found")
	} else if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	if p.CurrentVersion <= 0 {
		return p, domain.PlaybookVersion{}, nil
	}
	pv, err := r.getPlaybookVersion(ctx, tenantID, playbookID, p.CurrentVersion)
	return p, pv, err
}

func (r *AutomationRepository) CreatePlaybook(ctx context.Context, tenantID, userID uuid.UUID, input domain.CreatePlaybookInput) (domain.OperationalPlaybook, domain.PlaybookVersion, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > domain.MaxPlaybookNameLength {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, apperrors.Validation("invalid playbook name", map[string]any{"field": "name"})
	}
	if len(input.Steps) > domain.MaxPlaybookSteps {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, apperrors.Validation("step count exceeds limit", map[string]any{"max": domain.MaxPlaybookSteps})
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	defer tx.Rollback(ctx)

	var p domain.OperationalPlaybook
	err = tx.QueryRow(ctx, `
		INSERT INTO control_tower.operational_playbook
		    (tenant_id, name, description, status, current_version, created_by_user_id, updated_by_user_id)
		VALUES ($1,$2,$3,'draft',0,$4,$4)
		RETURNING id, tenant_id, name, COALESCE(description,''), status, current_version,
		          created_by_user_id, updated_by_user_id, created_at, updated_at
	`, tenantID, name, nullIfEmpty(input.Description), userID).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CurrentVersion,
		&p.CreatedByUserID, &p.UpdatedByUserID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	pv, err := r.createPlaybookVersionTx(ctx, tx, tenantID, userID, p.ID, 1, input.Steps, domain.PlaybookStatusDraft)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	_ = r.insertAudit(ctx, tenantID, "playbook_created", domain.ActorTypeUser, &userID, nil, &p.ID, nil, nil, map[string]any{"name": name})
	return p, pv, nil
}

func (r *AutomationRepository) PublishPlaybookVersion(ctx context.Context, tenantID, userID, playbookID uuid.UUID, steps []domain.PlaybookStepInput) (domain.OperationalPlaybook, domain.PlaybookVersion, error) {
	p, _, err := r.GetPlaybook(ctx, tenantID, playbookID)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	if len(steps) > domain.MaxPlaybookSteps {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, apperrors.Validation("step count exceeds limit", map[string]any{"max": domain.MaxPlaybookSteps})
	}
	nextVersion := p.CurrentVersion + 1
	if nextVersion <= 0 {
		nextVersion = 1
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	defer tx.Rollback(ctx)
	pv, err := r.createPlaybookVersionTx(ctx, tx, tenantID, userID, playbookID, nextVersion, steps, domain.PlaybookStatusActive)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	err = tx.QueryRow(ctx, `
		UPDATE control_tower.operational_playbook
		SET current_version=$3, status='active', updated_by_user_id=$4, updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2
		RETURNING id, tenant_id, name, COALESCE(description,''), status, current_version,
		          created_by_user_id, updated_by_user_id, created_at, updated_at
	`, tenantID, playbookID, nextVersion, userID).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CurrentVersion,
		&p.CreatedByUserID, &p.UpdatedByUserID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.OperationalPlaybook{}, domain.PlaybookVersion{}, err
	}
	_ = r.insertAudit(ctx, tenantID, "playbook_version_created", domain.ActorTypeUser, &userID, nil, &playbookID, nil, nil, map[string]any{"version": nextVersion})
	_ = r.insertAudit(ctx, tenantID, "playbook_activated", domain.ActorTypeUser, &userID, nil, &playbookID, nil, nil, nil)
	return p, pv, nil
}

func (r *AutomationRepository) UpdatePlaybookMeta(ctx context.Context, tenantID, userID, playbookID uuid.UUID, name, description *string, status *string) (domain.OperationalPlaybook, error) {
	p, _, err := r.GetPlaybook(ctx, tenantID, playbookID)
	if err != nil {
		return domain.OperationalPlaybook{}, err
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return domain.OperationalPlaybook{}, apperrors.Validation("invalid playbook name", map[string]any{"field": "name"})
		}
		p.Name = n
	}
	if description != nil {
		p.Description = strings.TrimSpace(*description)
	}
	if status != nil {
		s := strings.TrimSpace(*status)
		switch s {
		case domain.PlaybookStatusDraft, domain.PlaybookStatusActive, domain.PlaybookStatusRetired:
			p.Status = s
		default:
			return domain.OperationalPlaybook{}, apperrors.Validation("invalid playbook status", map[string]any{"status": s})
		}
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_playbook
		SET name=$3, description=$4, status=$5, updated_by_user_id=$6, updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2
		RETURNING id, tenant_id, name, COALESCE(description,''), status, current_version,
		          created_by_user_id, updated_by_user_id, created_at, updated_at
	`, tenantID, playbookID, p.Name, nullIfEmpty(p.Description), p.Status, userID)
	if err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Status, &p.CurrentVersion,
		&p.CreatedByUserID, &p.UpdatedByUserID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.OperationalPlaybook{}, err
	}
	return p, nil
}

func (r *AutomationRepository) createPlaybookVersionTx(ctx context.Context, tx pgx.Tx, tenantID, userID, playbookID uuid.UUID, version int, steps []domain.PlaybookStepInput, status string) (domain.PlaybookVersion, error) {
	var pv domain.PlaybookVersion
	err := tx.QueryRow(ctx, `
		INSERT INTO control_tower.operational_playbook_version
		    (tenant_id, playbook_id, version, status, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, playbook_id, version, status, created_by_user_id, created_at
	`, tenantID, playbookID, version, status, userID).Scan(
		&pv.ID, &pv.TenantID, &pv.PlaybookID, &pv.Version, &pv.Status, &pv.CreatedByUserID, &pv.CreatedAt)
	if err != nil {
		return domain.PlaybookVersion{}, err
	}
	for _, step := range steps {
		stepType := strings.TrimSpace(step.StepType)
		if stepType == "" {
			stepType = domain.StepTypeInstruction
		}
		if stepType == domain.StepTypeSystemAction {
			return domain.PlaybookVersion{}, apperrors.Validation("system_action is not supported in v0.8.0", map[string]any{"stepType": stepType})
		}
		if err := domain.ValidateActionCode(step.ActionCode); err != nil {
			return domain.PlaybookVersion{}, err
		}
		title := strings.TrimSpace(step.Title)
		if title == "" {
			return domain.PlaybookVersion{}, apperrors.Validation("step title is required", map[string]any{"field": "title"})
		}
		var ps domain.PlaybookStep
		err = tx.QueryRow(ctx, `
			INSERT INTO control_tower.operational_playbook_step
			    (tenant_id, playbook_version_id, sequence, title, description, step_type, required, estimated_duration_minutes, action_code)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id, tenant_id, playbook_version_id, sequence, title, COALESCE(description,''), step_type, required,
			          estimated_duration_minutes, COALESCE(action_code,'')
		`, tenantID, pv.ID, step.Sequence, title, nullIfEmpty(step.Description), stepType, step.Required,
			step.EstimatedDurationMinutes, nullIfEmpty(step.ActionCode)).Scan(
			&ps.ID, &ps.TenantID, &ps.PlaybookVersionID, &ps.Sequence, &ps.Title, &ps.Description, &ps.StepType,
			&ps.Required, &ps.EstimatedDurationMinutes, &ps.ActionCode)
		if err != nil {
			return domain.PlaybookVersion{}, err
		}
		pv.Steps = append(pv.Steps, ps)
	}
	return pv, nil
}

func (r *AutomationRepository) getPlaybookVersion(ctx context.Context, tenantID, playbookID uuid.UUID, version int) (domain.PlaybookVersion, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, playbook_id, version, status, created_by_user_id, created_at
		FROM control_tower.operational_playbook_version
		WHERE tenant_id=$1 AND playbook_id=$2 AND version=$3
	`, tenantID, playbookID, version)
	var pv domain.PlaybookVersion
	if err := row.Scan(&pv.ID, &pv.TenantID, &pv.PlaybookID, &pv.Version, &pv.Status, &pv.CreatedByUserID, &pv.CreatedAt); err != nil {
		return domain.PlaybookVersion{}, err
	}
	pv.Steps, _ = r.loadPlaybookSteps(ctx, tenantID, pv.ID)
	return pv, nil
}

func (r *AutomationRepository) loadPlaybookSteps(ctx context.Context, tenantID, versionID uuid.UUID) ([]domain.PlaybookStep, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, playbook_version_id, sequence, title, COALESCE(description,''), step_type, required,
		       estimated_duration_minutes, COALESCE(action_code,'')
		FROM control_tower.operational_playbook_step
		WHERE tenant_id=$1 AND playbook_version_id=$2 ORDER BY sequence ASC
	`, tenantID, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlaybookSteps(rows)
}

func (r *AutomationRepository) ListExecutions(ctx context.Context, tenantID uuid.UUID, filter ExecutionFilter) (domain.Page[domain.PlaybookExecution], error) {
	page, limit := normalizePage(filter.Page, filter.Limit)
	args := []any{tenantID}
	where := "e.tenant_id = $1"
	if s := strings.TrimSpace(filter.Status); s != "" {
		args = append(args, s)
		where += fmt.Sprintf(" AND e.status = $%d", len(args))
	}
	if filter.CaseID != nil {
		args = append(args, *filter.CaseID)
		where += fmt.Sprintf(" AND e.case_id = $%d", len(args))
	}
	if wt := strings.TrimSpace(filter.WorkItemType); wt != "" {
		args = append(args, wt)
		where += fmt.Sprintf(" AND e.work_item_type = $%d", len(args))
	}
	if wi := strings.TrimSpace(filter.WorkItemID); wi != "" {
		args = append(args, wi)
		where += fmt.Sprintf(" AND e.work_item_id = $%d", len(args))
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM control_tower.playbook_execution e WHERE "+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.PlaybookExecution]{}, err
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT e.id, e.tenant_id, e.recommendation_id, e.playbook_id, e.playbook_version, e.playbook_version_id,
		       e.shipment_id, COALESCE(e.work_item_type,''), COALESCE(e.work_item_id,''), e.case_id, e.owner_user_id,
		       e.status, e.started_at, e.completed_at, e.created_by_user_id, e.created_at, e.updated_at,
		       COALESCE(p.name,'')
		FROM control_tower.playbook_execution e
		LEFT JOIN control_tower.operational_playbook p ON p.id = e.playbook_id
		WHERE %s ORDER BY e.updated_at DESC LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.PlaybookExecution]{}, err
	}
	defer rows.Close()
	items := []domain.PlaybookExecution{}
	for rows.Next() {
		var e domain.PlaybookExecution
		if err := rows.Scan(&e.ID, &e.TenantID, &e.RecommendationID, &e.PlaybookID, &e.PlaybookVersion, &e.PlaybookVersionID,
			&e.ShipmentID, &e.WorkItemType, &e.WorkItemID, &e.CaseID, &e.OwnerUserID,
			&e.Status, &e.StartedAt, &e.CompletedAt, &e.CreatedByUserID, &e.CreatedAt, &e.UpdatedAt, &e.PlaybookName); err != nil {
			return domain.Page[domain.PlaybookExecution]{}, err
		}
		items = append(items, e)
	}
	return domain.Page[domain.PlaybookExecution]{Items: items, Page: page, Limit: limit, Total: total, HasNext: page*limit < total}, rows.Err()
}

func (r *AutomationRepository) GetExecution(ctx context.Context, tenantID, execID uuid.UUID) (domain.PlaybookExecution, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT e.id, e.tenant_id, e.recommendation_id, e.playbook_id, e.playbook_version, e.playbook_version_id,
		       e.shipment_id, COALESCE(e.work_item_type,''), COALESCE(e.work_item_id,''), e.case_id, e.owner_user_id,
		       e.status, e.started_at, e.completed_at, e.created_by_user_id, e.created_at, e.updated_at,
		       COALESCE(p.name,'')
		FROM control_tower.playbook_execution e
		LEFT JOIN control_tower.operational_playbook p ON p.id = e.playbook_id
		WHERE e.tenant_id=$1 AND e.id=$2
	`, tenantID, execID)
	var exec domain.PlaybookExecution
	if err := row.Scan(&exec.ID, &exec.TenantID, &exec.RecommendationID, &exec.PlaybookID, &exec.PlaybookVersion, &exec.PlaybookVersionID,
		&exec.ShipmentID, &exec.WorkItemType, &exec.WorkItemID, &exec.CaseID, &exec.OwnerUserID,
		&exec.Status, &exec.StartedAt, &exec.CompletedAt, &exec.CreatedByUserID, &exec.CreatedAt, &exec.UpdatedAt, &exec.PlaybookName); err == pgx.ErrNoRows {
		return domain.PlaybookExecution{}, apperrors.NotFound("playbook execution not found")
	} else if err != nil {
		return domain.PlaybookExecution{}, err
	}
	exec.Steps, _ = r.loadExecutionSteps(ctx, tenantID, execID)
	return exec, nil
}

func (r *AutomationRepository) StartExecution(ctx context.Context, tenantID, userID, execID uuid.UUID) (domain.PlaybookExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.playbook_execution
		SET status='in_progress', started_at=COALESCE(started_at, NOW()), updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2 AND status IN ('not_started','in_progress')
		RETURNING id
	`, tenantID, execID)
	if err := row.Scan(&execID); err == pgx.ErrNoRows {
		return domain.PlaybookExecution{}, apperrors.Conflict("execution cannot be started", nil)
	}
	exec, err := r.GetExecution(ctx, tenantID, execID)
	if err == nil {
		_ = r.insertAudit(ctx, tenantID, "playbook_started", domain.ActorTypeUser, &userID, nil, &exec.PlaybookID, nil, &execID, nil)
	}
	return exec, err
}

func (r *AutomationRepository) StartExecutionStep(ctx context.Context, tenantID, userID, execID, stepID uuid.UUID) (domain.PlaybookExecution, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE control_tower.playbook_execution_step
		SET status='in_progress', started_at=COALESCE(started_at, NOW()), started_by_user_id=$4
		WHERE tenant_id=$1 AND execution_id=$2 AND id=$3 AND status='pending'
	`, tenantID, execID, stepID, userID)
	if err != nil {
		return domain.PlaybookExecution{}, err
	}
	exec, err := r.GetExecution(ctx, tenantID, execID)
	if err == nil {
		_ = r.insertAudit(ctx, tenantID, "playbook_step_started", domain.ActorTypeUser, &userID, nil, &exec.PlaybookID, nil, &execID, map[string]any{"stepId": stepID.String()})
	}
	return exec, err
}

func (r *AutomationRepository) CompleteExecutionStep(ctx context.Context, tenantID, userID, execID, stepID uuid.UUID) (domain.PlaybookExecution, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE control_tower.playbook_execution_step
		SET status='done', completed_at=NOW(), completed_by_user_id=$4
		WHERE tenant_id=$1 AND execution_id=$2 AND id=$3 AND status IN ('pending','in_progress')
	`, tenantID, execID, stepID, userID)
	if err != nil {
		return domain.PlaybookExecution{}, err
	}
	exec, err := r.GetExecution(ctx, tenantID, execID)
	if err == nil {
		_ = r.insertAudit(ctx, tenantID, "playbook_step_completed", domain.ActorTypeUser, &userID, nil, &exec.PlaybookID, nil, &execID, map[string]any{"stepId": stepID.String()})
	}
	return exec, err
}

func (r *AutomationRepository) SkipExecutionStep(ctx context.Context, tenantID, userID, execID, stepID uuid.UUID, reason string) (domain.PlaybookExecution, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE control_tower.playbook_execution_step
		SET status='skipped', skip_reason=$5, completed_at=NOW(), completed_by_user_id=$4
		WHERE tenant_id=$1 AND execution_id=$2 AND id=$3 AND required=FALSE AND status IN ('pending','in_progress')
	`, tenantID, execID, stepID, userID, nullIfEmpty(reason))
	if err != nil {
		return domain.PlaybookExecution{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.PlaybookExecution{}, apperrors.Conflict("required steps cannot be skipped", nil)
	}
	exec, err := r.GetExecution(ctx, tenantID, execID)
	if err == nil {
		_ = r.insertAudit(ctx, tenantID, "playbook_step_skipped", domain.ActorTypeUser, &userID, nil, &exec.PlaybookID, nil, &execID, map[string]any{"stepId": stepID.String(), "reason": reason})
	}
	return exec, err
}

func (r *AutomationRepository) CompleteExecution(ctx context.Context, tenantID, userID, execID uuid.UUID) (domain.PlaybookExecution, error) {
	var incomplete int
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.playbook_execution_step
		WHERE tenant_id=$1 AND execution_id=$2 AND required=TRUE AND status NOT IN ('done','skipped')
	`, tenantID, execID).Scan(&incomplete)
	if incomplete > 0 {
		return domain.PlaybookExecution{}, apperrors.Conflict("required steps are incomplete", map[string]any{"incomplete": incomplete})
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.playbook_execution
		SET status='completed', completed_at=NOW(), updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2 AND status IN ('not_started','in_progress')
		RETURNING recommendation_id
	`, tenantID, execID)
	var recID *uuid.UUID
	if err := row.Scan(&recID); err == pgx.ErrNoRows {
		return domain.PlaybookExecution{}, apperrors.Conflict("execution cannot be completed", nil)
	} else if err != nil {
		return domain.PlaybookExecution{}, err
	}
	if recID != nil {
		_, _ = r.pool.Exec(ctx, `
			UPDATE control_tower.automation_recommendation SET status='completed', completed_at=NOW()
			WHERE tenant_id=$1 AND id=$2
		`, tenantID, *recID)
	}
	exec, err := r.GetExecution(ctx, tenantID, execID)
	if err == nil {
		_ = r.insertAudit(ctx, tenantID, "playbook_completed", domain.ActorTypeUser, &userID, nil, &exec.PlaybookID, recID, &execID, nil)
	}
	return exec, err
}

func (r *AutomationRepository) CancelExecution(ctx context.Context, tenantID, userID, execID uuid.UUID) (domain.PlaybookExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.playbook_execution
		SET status='cancelled', updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2 AND status IN ('not_started','in_progress')
		RETURNING id
	`, tenantID, execID)
	if err := row.Scan(&execID); err == pgx.ErrNoRows {
		return domain.PlaybookExecution{}, apperrors.Conflict("execution cannot be cancelled", nil)
	}
	exec, err := r.GetExecution(ctx, tenantID, execID)
	if err == nil {
		_ = r.insertAudit(ctx, tenantID, "playbook_cancelled", domain.ActorTypeUser, &userID, nil, &exec.PlaybookID, nil, &execID, nil)
	}
	return exec, err
}

func (r *AutomationRepository) CountPlaybookSteps(ctx context.Context, tenantID, playbookID uuid.UUID) (int, error) {
	p, _, err := r.GetPlaybook(ctx, tenantID, playbookID)
	if err != nil {
		return 0, err
	}
	if p.CurrentVersion <= 0 {
		return 0, nil
	}
	pv, err := r.getPlaybookVersion(ctx, tenantID, playbookID, p.CurrentVersion)
	if err != nil {
		return 0, err
	}
	return len(pv.Steps), nil
}
