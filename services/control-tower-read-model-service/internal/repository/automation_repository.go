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

type AutomationRepository struct {
	pool *pgxpool.Pool
}

func NewAutomationRepository(pool *pgxpool.Pool) *AutomationRepository {
	return &AutomationRepository{pool: pool}
}

type RuleFilter struct {
	Status      string
	TriggerType string
	Page        int
	Limit       int
}

type RecommendationFilter struct {
	Status       string
	ShipmentID   *uuid.UUID
	WorkItemType string
	WorkItemID   string
	CaseID       *uuid.UUID
	Page         int
	Limit        int
}

type ExecutionFilter struct {
	Status       string
	CaseID       *uuid.UUID
	WorkItemType string
	WorkItemID   string
	Page         int
	Limit       int
}

func (r *AutomationRepository) ListActiveRulesByTrigger(ctx context.Context, tenantID uuid.UUID, triggerType string) ([]domain.AutomationRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), status, trigger_type, conditions,
		       condition_schema_version, playbook_id, execution_mode, priority, version,
		       created_by_user_id, updated_by_user_id, created_at, updated_at
		FROM control_tower.automation_rule
		WHERE tenant_id = $1 AND status = 'active' AND trigger_type = $2
		ORDER BY priority DESC, id ASC
	`, tenantID, triggerType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRules(rows)
}

func (r *AutomationRepository) ListRules(ctx context.Context, tenantID uuid.UUID, filter RuleFilter) (domain.Page[domain.AutomationRule], error) {
	page, limit := normalizePage(filter.Page, filter.Limit)
	args := []any{tenantID}
	where := "tenant_id = $1"
	if s := strings.TrimSpace(filter.Status); s != "" {
		args = append(args, s)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if t := strings.TrimSpace(filter.TriggerType); t != "" {
		args = append(args, t)
		where += fmt.Sprintf(" AND trigger_type = $%d", len(args))
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM control_tower.automation_rule WHERE "+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.AutomationRule]{}, err
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, COALESCE(description,''), status, trigger_type, conditions,
		       condition_schema_version, playbook_id, execution_mode, priority, version,
		       created_by_user_id, updated_by_user_id, created_at, updated_at
		FROM control_tower.automation_rule WHERE %s
		ORDER BY updated_at DESC LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.AutomationRule]{}, err
	}
	defer rows.Close()
	items, err := scanRules(rows)
	if err != nil {
		return domain.Page[domain.AutomationRule]{}, err
	}
	return domain.Page[domain.AutomationRule]{Items: items, Page: page, Limit: limit, Total: total, HasNext: page*limit < total}, nil
}

func (r *AutomationRepository) GetRule(ctx context.Context, tenantID, ruleID uuid.UUID) (domain.AutomationRule, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), status, trigger_type, conditions,
		       condition_schema_version, playbook_id, execution_mode, priority, version,
		       created_by_user_id, updated_by_user_id, created_at, updated_at
		FROM control_tower.automation_rule WHERE tenant_id = $1 AND id = $2
	`, tenantID, ruleID)
	rule, err := scanRule(row)
	if err == pgx.ErrNoRows {
		return domain.AutomationRule{}, apperrors.NotFound("automation rule not found")
	}
	return rule, err
}

func (r *AutomationRepository) CreateRule(ctx context.Context, tenantID, userID uuid.UUID, input domain.CreateRuleInput) (domain.AutomationRule, error) {
	if err := domain.ValidateTriggerType(input.TriggerType); err != nil {
		return domain.AutomationRule{}, err
	}
	if err := domain.ValidateExecutionMode(defaultMode(input.ExecutionMode)); err != nil {
		return domain.AutomationRule{}, err
	}
	if err := domain.ValidateConditionGroup(input.Conditions, 1); err != nil {
		return domain.AutomationRule{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > domain.MaxRuleNameLength {
		return domain.AutomationRule{}, apperrors.Validation("invalid rule name", map[string]any{"field": "name"})
	}
	condJSON, err := json.Marshal(input.Conditions)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	priority := input.Priority
	if priority <= 0 {
		priority = 50
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.automation_rule
		    (tenant_id, name, description, status, trigger_type, conditions, condition_schema_version,
		     playbook_id, execution_mode, priority, version, created_by_user_id, updated_by_user_id)
		VALUES ($1,$2,$3,'draft',$4,$5,$6,$7,$8,$9,1,$10,$10)
		RETURNING id, tenant_id, name, COALESCE(description,''), status, trigger_type, conditions,
		          condition_schema_version, playbook_id, execution_mode, priority, version,
		          created_by_user_id, updated_by_user_id, created_at, updated_at
	`, tenantID, name, nullIfEmpty(input.Description), input.TriggerType, condJSON,
		domain.AutomationConditionSchemaVersion, input.PlaybookID, defaultMode(input.ExecutionMode), priority, userID)
	rule, err := scanRule(row)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	_ = r.insertAudit(ctx, tenantID, "rule_created", domain.ActorTypeUser, &userID, &rule.ID, nil, nil, nil, map[string]any{"name": name})
	return rule, nil
}

func (r *AutomationRepository) UpdateRule(ctx context.Context, tenantID, userID, ruleID uuid.UUID, input domain.UpdateRuleInput) (domain.AutomationRule, error) {
	rule, err := r.GetRule(ctx, tenantID, ruleID)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	if rule.Status == domain.RuleStatusRetired {
		return domain.AutomationRule{}, apperrors.Conflict("retired rules cannot be updated", nil)
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len(name) > domain.MaxRuleNameLength {
			return domain.AutomationRule{}, apperrors.Validation("invalid rule name", map[string]any{"field": "name"})
		}
		rule.Name = name
	}
	if input.Description != nil {
		rule.Description = strings.TrimSpace(*input.Description)
	}
	if input.TriggerType != nil {
		if err := domain.ValidateTriggerType(*input.TriggerType); err != nil {
			return domain.AutomationRule{}, err
		}
		rule.TriggerType = *input.TriggerType
	}
	if input.Conditions != nil {
		if err := domain.ValidateConditionGroup(*input.Conditions, 1); err != nil {
			return domain.AutomationRule{}, err
		}
		rule.Conditions = *input.Conditions
	}
	if input.PlaybookID != nil {
		rule.PlaybookID = input.PlaybookID
	}
	if input.ExecutionMode != nil {
		if err := domain.ValidateExecutionMode(*input.ExecutionMode); err != nil {
			return domain.AutomationRule{}, err
		}
		rule.ExecutionMode = *input.ExecutionMode
	}
	if input.Priority != nil {
		rule.Priority = *input.Priority
	}
	condJSON, _ := json.Marshal(rule.Conditions)
	rule.Version++
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_rule
		SET name=$3, description=$4, trigger_type=$5, conditions=$6, playbook_id=$7,
		    execution_mode=$8, priority=$9, version=$10, updated_by_user_id=$11, updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2
		RETURNING id, tenant_id, name, COALESCE(description,''), status, trigger_type, conditions,
		          condition_schema_version, playbook_id, execution_mode, priority, version,
		          created_by_user_id, updated_by_user_id, created_at, updated_at
	`, tenantID, ruleID, rule.Name, nullIfEmpty(rule.Description), rule.TriggerType, condJSON,
		rule.PlaybookID, rule.ExecutionMode, rule.Priority, rule.Version, userID)
	updated, err := scanRule(row)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	_ = r.snapshotRuleVersion(ctx, updated, userID)
	_ = r.insertAudit(ctx, tenantID, "rule_updated", domain.ActorTypeUser, &userID, &ruleID, nil, nil, nil, map[string]any{"version": updated.Version})
	return updated, nil
}

func (r *AutomationRepository) SetRuleStatus(ctx context.Context, tenantID, userID, ruleID uuid.UUID, status string) (domain.AutomationRule, error) {
	rule, err := r.GetRule(ctx, tenantID, ruleID)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	switch status {
	case domain.RuleStatusActive, domain.RuleStatusDisabled, domain.RuleStatusRetired, domain.RuleStatusDraft:
	default:
		return domain.AutomationRule{}, apperrors.Validation("invalid rule status", map[string]any{"status": status})
	}
	if status == domain.RuleStatusActive {
		if err := domain.ValidateExecutionMode(rule.ExecutionMode); err != nil {
			return domain.AutomationRule{}, err
		}
		if rule.PlaybookID == nil && rule.ExecutionMode == domain.ExecutionModeRecommend {
			return domain.AutomationRule{}, apperrors.Validation("playbook is required for recommend mode", map[string]any{"field": "playbookId"})
		}
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_rule SET status=$3, updated_by_user_id=$4, updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2
		RETURNING id, tenant_id, name, COALESCE(description,''), status, trigger_type, conditions,
		          condition_schema_version, playbook_id, execution_mode, priority, version,
		          created_by_user_id, updated_by_user_id, created_at, updated_at
	`, tenantID, ruleID, status, userID)
	updated, err := scanRule(row)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	eventType := "rule_updated"
	switch status {
	case domain.RuleStatusActive:
		eventType = "rule_activated"
	case domain.RuleStatusDisabled:
		eventType = "rule_disabled"
	case domain.RuleStatusRetired:
		eventType = "rule_retired"
	}
	_ = r.insertAudit(ctx, tenantID, eventType, domain.ActorTypeUser, &userID, &ruleID, nil, nil, nil, map[string]any{"status": status})
	return updated, nil
}

func (r *AutomationRepository) GetActivePlaybookVersions(ctx context.Context, tenantID uuid.UUID, playbookIDs []uuid.UUID) (map[uuid.UUID]domain.PlaybookVersion, error) {
	out := map[uuid.UUID]domain.PlaybookVersion{}
	if len(playbookIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT pv.id, pv.tenant_id, pv.playbook_id, pv.version, pv.status, pv.created_by_user_id, pv.created_at
		FROM control_tower.operational_playbook_version pv
		JOIN control_tower.operational_playbook p ON p.id = pv.playbook_id AND p.tenant_id = pv.tenant_id
		WHERE pv.tenant_id = $1 AND p.status = 'active' AND pv.playbook_id = ANY($2)
		  AND pv.version = p.current_version
	`, tenantID, playbookIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pv domain.PlaybookVersion
		if err := rows.Scan(&pv.ID, &pv.TenantID, &pv.PlaybookID, &pv.Version, &pv.Status, &pv.CreatedByUserID, &pv.CreatedAt); err != nil {
			return nil, err
		}
		out[pv.PlaybookID] = pv
	}
	return out, rows.Err()
}

func (r *AutomationRepository) CreateRecommendation(ctx context.Context, rec domain.AutomationRecommendation) (domain.AutomationRecommendation, bool, error) {
	explanation, _ := json.Marshal(rec.MatchExplanation)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.automation_recommendation
		    (tenant_id, rule_id, rule_version, playbook_id, playbook_version, playbook_version_id,
		     trigger_id, trigger_type, correlation_id, causation_id, shipment_id, work_item_type, work_item_id,
		     case_id, risk_id, exception_id, status, match_explanation, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'pending',$17,$18)
		ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
		RETURNING id, tenant_id, rule_id, rule_version, playbook_id, playbook_version, playbook_version_id,
		          trigger_id, trigger_type, COALESCE(correlation_id,''), COALESCE(causation_id,''),
		          shipment_id, COALESCE(work_item_type,''), COALESCE(work_item_id,''), case_id,
		          COALESCE(risk_id,''), COALESCE(exception_id,''), status, match_explanation, idempotency_key,
		          created_at, expires_at, accepted_by_user_id, accepted_at, dismissed_by_user_id, dismissed_at,
		          COALESCE(dismiss_reason,''), completed_at
	`, rec.TenantID, rec.RuleID, rec.RuleVersion, rec.PlaybookID, rec.PlaybookVersion, rec.PlaybookVersionID,
		rec.TriggerID, rec.TriggerType, nullIfEmpty(rec.CorrelationID), nullIfEmpty(rec.CausationID),
		rec.ShipmentID, nullIfEmpty(rec.WorkItemType), nullIfEmpty(rec.WorkItemID), rec.CaseID,
		nullIfEmpty(rec.RiskID), nullIfEmpty(rec.ExceptionID), explanation, rec.IdempotencyKey)
	item, err := scanRecommendation(row)
	if err == pgx.ErrNoRows {
		existing, getErr := r.GetRecommendationByKey(ctx, rec.TenantID, rec.IdempotencyKey)
		return existing, false, getErr
	}
	if err != nil {
		return domain.AutomationRecommendation{}, false, err
	}
	_ = r.insertAudit(ctx, rec.TenantID, "recommendation_created", domain.ActorTypeSystem, nil, &rec.RuleID, &rec.PlaybookID, &item.ID, nil, map[string]any{"triggerType": rec.TriggerType})
	return item, true, nil
}

func (r *AutomationRepository) GetRecommendationByKey(ctx context.Context, tenantID uuid.UUID, key string) (domain.AutomationRecommendation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT r.id, r.tenant_id, r.rule_id, r.rule_version, r.playbook_id, r.playbook_version, r.playbook_version_id,
		       r.trigger_id, r.trigger_type, COALESCE(r.correlation_id,''), COALESCE(r.causation_id,''),
		       r.shipment_id, COALESCE(r.work_item_type,''), COALESCE(r.work_item_id,''), r.case_id,
		       COALESCE(r.risk_id,''), COALESCE(r.exception_id,''), r.status, r.match_explanation, r.idempotency_key,
		       r.created_at, r.expires_at, r.accepted_by_user_id, r.accepted_at, r.dismissed_by_user_id, r.dismissed_at,
		       COALESCE(r.dismiss_reason,''), r.completed_at,
		       COALESCE(p.name,''), COALESCE(ar.name,'')
		FROM control_tower.automation_recommendation r
		LEFT JOIN control_tower.operational_playbook p ON p.id = r.playbook_id
		LEFT JOIN control_tower.automation_rule ar ON ar.id = r.rule_id
		WHERE r.tenant_id = $1 AND r.idempotency_key = $2
	`, tenantID, key)
	return scanRecommendationJoined(row)
}

func (r *AutomationRepository) GetRecommendation(ctx context.Context, tenantID, recID uuid.UUID) (domain.AutomationRecommendation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT r.id, r.tenant_id, r.rule_id, r.rule_version, r.playbook_id, r.playbook_version, r.playbook_version_id,
		       r.trigger_id, r.trigger_type, COALESCE(r.correlation_id,''), COALESCE(r.causation_id,''),
		       r.shipment_id, COALESCE(r.work_item_type,''), COALESCE(r.work_item_id,''), r.case_id,
		       COALESCE(r.risk_id,''), COALESCE(r.exception_id,''), r.status, r.match_explanation, r.idempotency_key,
		       r.created_at, r.expires_at, r.accepted_by_user_id, r.accepted_at, r.dismissed_by_user_id, r.dismissed_at,
		       COALESCE(r.dismiss_reason,''), r.completed_at,
		       COALESCE(p.name,''), COALESCE(ar.name,'')
		FROM control_tower.automation_recommendation r
		LEFT JOIN control_tower.operational_playbook p ON p.id = r.playbook_id
		LEFT JOIN control_tower.automation_rule ar ON ar.id = r.rule_id
		WHERE r.tenant_id = $1 AND r.id = $2
	`, tenantID, recID)
	rec, err := scanRecommendationJoined(row)
	if err == pgx.ErrNoRows {
		return domain.AutomationRecommendation{}, apperrors.NotFound("recommendation not found")
	}
	return rec, err
}

func (r *AutomationRepository) ListRecommendations(ctx context.Context, tenantID uuid.UUID, filter RecommendationFilter) (domain.Page[domain.AutomationRecommendation], error) {
	page, limit := normalizePage(filter.Page, filter.Limit)
	args := []any{tenantID}
	where := "r.tenant_id = $1"
	if s := strings.TrimSpace(filter.Status); s != "" {
		args = append(args, s)
		where += fmt.Sprintf(" AND r.status = $%d", len(args))
	}
	if filter.ShipmentID != nil {
		args = append(args, *filter.ShipmentID)
		where += fmt.Sprintf(" AND r.shipment_id = $%d", len(args))
	}
	if filter.CaseID != nil {
		args = append(args, *filter.CaseID)
		where += fmt.Sprintf(" AND r.case_id = $%d", len(args))
	}
	if wt := strings.TrimSpace(filter.WorkItemType); wt != "" {
		args = append(args, wt)
		where += fmt.Sprintf(" AND r.work_item_type = $%d", len(args))
	}
	if wi := strings.TrimSpace(filter.WorkItemID); wi != "" {
		args = append(args, wi)
		where += fmt.Sprintf(" AND r.work_item_id = $%d", len(args))
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM control_tower.automation_recommendation r WHERE "+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.AutomationRecommendation]{}, err
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id, r.tenant_id, r.rule_id, r.rule_version, r.playbook_id, r.playbook_version, r.playbook_version_id,
		       r.trigger_id, r.trigger_type, COALESCE(r.correlation_id,''), COALESCE(r.causation_id,''),
		       r.shipment_id, COALESCE(r.work_item_type,''), COALESCE(r.work_item_id,''), r.case_id,
		       COALESCE(r.risk_id,''), COALESCE(r.exception_id,''), r.status, r.match_explanation, r.idempotency_key,
		       r.created_at, r.expires_at, r.accepted_by_user_id, r.accepted_at, r.dismissed_by_user_id, r.dismissed_at,
		       COALESCE(r.dismiss_reason,''), r.completed_at,
		       COALESCE(p.name,''), COALESCE(ar.name,'')
		FROM control_tower.automation_recommendation r
		LEFT JOIN control_tower.operational_playbook p ON p.id = r.playbook_id
		LEFT JOIN control_tower.automation_rule ar ON ar.id = r.rule_id
		WHERE %s ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.AutomationRecommendation]{}, err
	}
	defer rows.Close()
	items := []domain.AutomationRecommendation{}
	for rows.Next() {
		rec, err := scanRecommendationJoined(rows)
		if err != nil {
			return domain.Page[domain.AutomationRecommendation]{}, err
		}
		items = append(items, rec)
	}
	return domain.Page[domain.AutomationRecommendation]{Items: items, Page: page, Limit: limit, Total: total, HasNext: page*limit < total}, rows.Err()
}

func (r *AutomationRepository) AcceptRecommendation(ctx context.Context, tenantID, userID, recID uuid.UUID) (domain.AutomationRecommendation, domain.PlaybookExecution, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
	}
	defer tx.Rollback(ctx)

	var rec domain.AutomationRecommendation
	row := tx.QueryRow(ctx, `
		SELECT id, tenant_id, rule_id, rule_version, playbook_id, playbook_version, playbook_version_id,
		       trigger_id, trigger_type, COALESCE(correlation_id,''), COALESCE(causation_id,''),
		       shipment_id, COALESCE(work_item_type,''), COALESCE(work_item_id,''), case_id,
		       COALESCE(risk_id,''), COALESCE(exception_id,''), status, match_explanation, idempotency_key,
		       created_at, expires_at, accepted_by_user_id, accepted_at, dismissed_by_user_id, dismissed_at,
		       COALESCE(dismiss_reason,''), completed_at
		FROM control_tower.automation_recommendation
		WHERE tenant_id = $1 AND id = $2 FOR UPDATE
	`, tenantID, recID)
	rec, err = scanRecommendation(row)
	if err == pgx.ErrNoRows {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, apperrors.NotFound("recommendation not found")
	}
	if err != nil {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
	}
	if rec.Status != domain.RecommendationStatusPending {
		if rec.Status == domain.RecommendationStatusAccepted {
			exec, err := r.getExecutionByRecommendationTx(ctx, tx, tenantID, recID)
			return rec, exec, err
		}
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, apperrors.Conflict("recommendation is not pending", map[string]any{"status": rec.Status})
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.automation_recommendation
		SET status='accepted', accepted_by_user_id=$3, accepted_at=$4
		WHERE tenant_id=$1 AND id=$2
	`, tenantID, recID, userID, now)
	if err != nil {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
	}
	rec.Status = domain.RecommendationStatusAccepted
	rec.AcceptedByUserID = &userID
	rec.AcceptedAt = &now

	execRow := tx.QueryRow(ctx, `
		INSERT INTO control_tower.playbook_execution
		    (tenant_id, recommendation_id, playbook_id, playbook_version, playbook_version_id,
		     shipment_id, work_item_type, work_item_id, case_id, owner_user_id, status, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'not_started',$10)
		ON CONFLICT (recommendation_id) DO NOTHING
		RETURNING id, tenant_id, recommendation_id, playbook_id, playbook_version, playbook_version_id,
		          shipment_id, COALESCE(work_item_type,''), COALESCE(work_item_id,''), case_id, owner_user_id,
		          status, started_at, completed_at, created_by_user_id, created_at, updated_at
	`, tenantID, recID, rec.PlaybookID, rec.PlaybookVersion, rec.PlaybookVersionID,
		rec.ShipmentID, nullIfEmpty(rec.WorkItemType), nullIfEmpty(rec.WorkItemID), rec.CaseID, userID)
	exec, err := scanExecution(execRow)
	if err == pgx.ErrNoRows {
		exec, err = r.getExecutionByRecommendationTx(ctx, tx, tenantID, recID)
	}
	if err != nil {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
	}
	steps, err := r.loadPlaybookStepsTx(ctx, tx, tenantID, rec.PlaybookVersionID)
	if err != nil {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
	}
	for _, step := range steps {
		_, err = tx.Exec(ctx, `
			INSERT INTO control_tower.playbook_execution_step
			    (tenant_id, execution_id, playbook_step_id, sequence, title, description, step_type, required, action_code, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending')
			ON CONFLICT (execution_id, sequence) DO NOTHING
		`, tenantID, exec.ID, step.ID, step.Sequence, step.Title, nullIfEmpty(step.Description),
			step.StepType, step.Required, nullIfEmpty(step.ActionCode))
		if err != nil {
			return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AutomationRecommendation{}, domain.PlaybookExecution{}, err
	}
	_ = r.insertAudit(ctx, tenantID, "recommendation_accepted", domain.ActorTypeUser, &userID, &rec.RuleID, &rec.PlaybookID, &recID, &exec.ID, nil)
	exec.Steps, _ = r.loadExecutionSteps(ctx, tenantID, exec.ID)
	return rec, exec, nil
}

func (r *AutomationRepository) DismissRecommendation(ctx context.Context, tenantID, userID, recID uuid.UUID, reason string) (domain.AutomationRecommendation, error) {
	rec, err := r.GetRecommendation(ctx, tenantID, recID)
	if err != nil {
		return domain.AutomationRecommendation{}, err
	}
	if rec.Status != domain.RecommendationStatusPending {
		return domain.AutomationRecommendation{}, apperrors.Conflict("recommendation is not pending", map[string]any{"status": rec.Status})
	}
	now := time.Now().UTC()
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.automation_recommendation
		SET status='dismissed', dismissed_by_user_id=$3, dismissed_at=$4, dismiss_reason=$5
		WHERE tenant_id=$1 AND id=$2 AND status='pending'
		RETURNING id
	`, tenantID, recID, userID, now, reason)
	if err := row.Scan(&recID); err == pgx.ErrNoRows {
		return domain.AutomationRecommendation{}, apperrors.Conflict("recommendation already handled", nil)
	}
	_ = r.insertAudit(ctx, tenantID, "recommendation_dismissed", domain.ActorTypeUser, &userID, &rec.RuleID, &rec.PlaybookID, &recID, nil, map[string]any{"reason": reason})
	return r.GetRecommendation(ctx, tenantID, recID)
}

func (r *AutomationRepository) GetKPI(ctx context.Context, tenantID uuid.UUID) (domain.AutomationKPI, error) {
	var kpi domain.AutomationKPI
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.automation_recommendation
		WHERE tenant_id=$1 AND status='pending'
	`, tenantID).Scan(&kpi.PendingRecommendations)
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.playbook_execution
		WHERE tenant_id=$1 AND status IN ('not_started','in_progress')
	`, tenantID).Scan(&kpi.ActivePlaybookExecutions)
	start := time.Now().UTC().Truncate(24 * time.Hour)
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.playbook_execution
		WHERE tenant_id=$1 AND status='completed' AND completed_at >= $2
	`, tenantID, start).Scan(&kpi.CompletedPlaybooksToday)
	return kpi, nil
}

// Playbook CRUD continues in automation_playbook_repository.go helpers below.

func defaultMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return domain.ExecutionModeRecommend
	}
	return mode
}

func normalizePage(page, limit int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return page, limit
}

func (r *AutomationRepository) snapshotRuleVersion(ctx context.Context, rule domain.AutomationRule, userID uuid.UUID) error {
	condJSON, _ := json.Marshal(rule.Conditions)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO control_tower.automation_rule_version
		    (tenant_id, rule_id, version, name, description, status, trigger_type, conditions,
		     condition_schema_version, playbook_id, execution_mode, priority, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (tenant_id, rule_id, version) DO NOTHING
	`, rule.TenantID, rule.ID, rule.Version, rule.Name, nullIfEmpty(rule.Description), rule.Status,
		rule.TriggerType, condJSON, rule.ConditionSchemaVersion, rule.PlaybookID, rule.ExecutionMode, rule.Priority, userID)
	return err
}

func (r *AutomationRepository) insertAudit(ctx context.Context, tenantID uuid.UUID, eventType, actorType string, actorID *uuid.UUID, ruleID, playbookID, recID, execID *uuid.UUID, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO control_tower.automation_audit_event
		    (tenant_id, event_type, actor_type, actor_user_id, rule_id, playbook_id, recommendation_id, execution_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, tenantID, eventType, actorType, actorID, ruleID, playbookID, recID, execID, body)
	return err
}

func scanRules(rows pgx.Rows) ([]domain.AutomationRule, error) {
	items := []domain.AutomationRule{}
	for rows.Next() {
		item, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRule(row pgx.Row) (domain.AutomationRule, error) {
	var rule domain.AutomationRule
	var condJSON []byte
	var playbookID *uuid.UUID
	err := row.Scan(&rule.ID, &rule.TenantID, &rule.Name, &rule.Description, &rule.Status, &rule.TriggerType, &condJSON,
		&rule.ConditionSchemaVersion, &playbookID, &rule.ExecutionMode, &rule.Priority, &rule.Version,
		&rule.CreatedByUserID, &rule.UpdatedByUserID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return domain.AutomationRule{}, err
	}
	rule.PlaybookID = playbookID
	if len(condJSON) > 0 {
		_ = json.Unmarshal(condJSON, &rule.Conditions)
	}
	return rule, nil
}

type automationScannable interface {
	Scan(dest ...any) error
}

func scanRecommendation(row automationScannable) (domain.AutomationRecommendation, error) {
	var rec domain.AutomationRecommendation
	var explanation []byte
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.RuleID, &rec.RuleVersion, &rec.PlaybookID, &rec.PlaybookVersion, &rec.PlaybookVersionID,
		&rec.TriggerID, &rec.TriggerType, &rec.CorrelationID, &rec.CausationID,
		&rec.ShipmentID, &rec.WorkItemType, &rec.WorkItemID, &rec.CaseID,
		&rec.RiskID, &rec.ExceptionID, &rec.Status, &explanation, &rec.IdempotencyKey,
		&rec.CreatedAt, &rec.ExpiresAt, &rec.AcceptedByUserID, &rec.AcceptedAt, &rec.DismissedByUserID, &rec.DismissedAt,
		&rec.DismissReason, &rec.CompletedAt)
	if err != nil {
		return domain.AutomationRecommendation{}, err
	}
	if len(explanation) > 0 {
		_ = json.Unmarshal(explanation, &rec.MatchExplanation)
	}
	return rec, nil
}

func scanRecommendationJoined(row automationScannable) (domain.AutomationRecommendation, error) {
	var rec domain.AutomationRecommendation
	var explanation []byte
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.RuleID, &rec.RuleVersion, &rec.PlaybookID, &rec.PlaybookVersion, &rec.PlaybookVersionID,
		&rec.TriggerID, &rec.TriggerType, &rec.CorrelationID, &rec.CausationID,
		&rec.ShipmentID, &rec.WorkItemType, &rec.WorkItemID, &rec.CaseID,
		&rec.RiskID, &rec.ExceptionID, &rec.Status, &explanation, &rec.IdempotencyKey,
		&rec.CreatedAt, &rec.ExpiresAt, &rec.AcceptedByUserID, &rec.AcceptedAt, &rec.DismissedByUserID, &rec.DismissedAt,
		&rec.DismissReason, &rec.CompletedAt, &rec.PlaybookName, &rec.RuleName)
	if err != nil {
		return domain.AutomationRecommendation{}, err
	}
	if len(explanation) > 0 {
		_ = json.Unmarshal(explanation, &rec.MatchExplanation)
	}
	return rec, nil
}

func scanExecution(row automationScannable) (domain.PlaybookExecution, error) {
	var exec domain.PlaybookExecution
	err := row.Scan(&exec.ID, &exec.TenantID, &exec.RecommendationID, &exec.PlaybookID, &exec.PlaybookVersion, &exec.PlaybookVersionID,
		&exec.ShipmentID, &exec.WorkItemType, &exec.WorkItemID, &exec.CaseID, &exec.OwnerUserID,
		&exec.Status, &exec.StartedAt, &exec.CompletedAt, &exec.CreatedByUserID, &exec.CreatedAt, &exec.UpdatedAt)
	return exec, err
}

func (r *AutomationRepository) getExecutionByRecommendationTx(ctx context.Context, tx pgx.Tx, tenantID, recID uuid.UUID) (domain.PlaybookExecution, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, tenant_id, recommendation_id, playbook_id, playbook_version, playbook_version_id,
		       shipment_id, COALESCE(work_item_type,''), COALESCE(work_item_id,''), case_id, owner_user_id,
		       status, started_at, completed_at, created_by_user_id, created_at, updated_at
		FROM control_tower.playbook_execution WHERE tenant_id=$1 AND recommendation_id=$2
	`, tenantID, recID)
	return scanExecution(row)
}

func (r *AutomationRepository) loadPlaybookStepsTx(ctx context.Context, tx pgx.Tx, tenantID, versionID uuid.UUID) ([]domain.PlaybookStep, error) {
	rows, err := tx.Query(ctx, `
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

func (r *AutomationRepository) loadExecutionSteps(ctx context.Context, tenantID, execID uuid.UUID) ([]domain.PlaybookExecutionStep, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, execution_id, playbook_step_id, sequence, title, COALESCE(description,''),
		       step_type, required, COALESCE(action_code,''), status, COALESCE(skip_reason,''),
		       started_at, completed_at, started_by_user_id, completed_by_user_id
		FROM control_tower.playbook_execution_step
		WHERE tenant_id=$1 AND execution_id=$2 ORDER BY sequence ASC
	`, tenantID, execID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.PlaybookExecutionStep{}
	for rows.Next() {
		var s domain.PlaybookExecutionStep
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ExecutionID, &s.PlaybookStepID, &s.Sequence, &s.Title, &s.Description,
			&s.StepType, &s.Required, &s.ActionCode, &s.Status, &s.SkipReason, &s.StartedAt, &s.CompletedAt,
			&s.StartedByUserID, &s.CompletedByUserID); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func scanPlaybookSteps(rows pgx.Rows) ([]domain.PlaybookStep, error) {
	items := []domain.PlaybookStep{}
	for rows.Next() {
		var s domain.PlaybookStep
		if err := rows.Scan(&s.ID, &s.TenantID, &s.PlaybookVersionID, &s.Sequence, &s.Title, &s.Description,
			&s.StepType, &s.Required, &s.EstimatedDurationMinutes, &s.ActionCode); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}
