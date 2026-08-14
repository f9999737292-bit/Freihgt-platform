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

type CaseRepository struct {
	pool *pgxpool.Pool
}

func NewCaseRepository(pool *pgxpool.Pool) *CaseRepository {
	return &CaseRepository{pool: pool}
}

func (r *CaseRepository) CreateCase(ctx context.Context, tenantID, userID uuid.UUID, input domain.CreateCaseInput) (domain.OperationalCase, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.OperationalCase{}, err
	}
	defer tx.Rollback(ctx)

	ref, err := r.nextReference(ctx, tx, tenantID)
	if err != nil {
		return domain.OperationalCase{}, err
	}

	severity := strings.TrimSpace(input.Severity)
	if severity == "" {
		severity = domain.CaseSeverityMedium
	}
	if !isValidSeverity(severity) {
		return domain.OperationalCase{}, apperrors.Validation("invalid severity", map[string]any{"field": "severity"})
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return domain.OperationalCase{}, apperrors.Validation("title is required", map[string]any{"field": "title"})
	}

	ownerID := input.OwnerUserID
	var c domain.OperationalCase
	row := tx.QueryRow(ctx, `
		INSERT INTO control_tower.operational_case
		    (tenant_id, reference, title, summary, status, derived_severity, effective_severity,
		     severity_override, owner_user_id, created_by_user_id)
		VALUES ($1,$2,$3,$4,'open',$5,$5,$6,$7,$8)
		RETURNING id, tenant_id, reference, title, COALESCE(summary,''), status,
		          derived_severity, effective_severity, severity_override, owner_user_id,
		          created_by_user_id, resolution_code, resolution_summary, version,
		          last_activity_at, created_at, updated_at, resolved_at, closed_at
	`, tenantID, ref, title, nullIfEmpty(input.Summary), severity, severity, severity != domain.CaseSeverityMedium && input.Severity != "",
		ownerID, userID)
	c, err = scanCase(row)
	if err != nil {
		return domain.OperationalCase{}, err
	}

	if err := r.insertEvent(ctx, tx, tenantID, c.ID, domain.CaseEventSourceCase, "case_created", &userID, map[string]any{
		"reference": ref, "title": title,
	}); err != nil {
		return domain.OperationalCase{}, err
	}

	for _, shipmentID := range input.ShipmentIDs {
		if err := r.addLinkTx(ctx, tx, tenantID, c.ID, userID, domain.CaseLinkShipment, shipmentID.String()); err != nil {
			return domain.OperationalCase{}, err
		}
	}

	for _, wi := range input.WorkItems {
		entityType := wi.ItemType
		if entityType == domain.WorkItemTypeException || entityType == domain.WorkItemTypeRisk {
			if err := r.addWorkLinkTx(ctx, tx, tenantID, c.ID, userID, entityType, wi.ItemID); err != nil {
				return domain.OperationalCase{}, err
			}
		}
	}

	for _, pid := range input.ParticipantUserIDs {
		if pid == userID {
			continue
		}
		if err := r.addParticipantTx(ctx, tx, tenantID, c.ID, userID, pid, domain.ParticipantRoleCollaborator); err != nil {
			return domain.OperationalCase{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.OperationalCase{}, err
	}
	return r.refreshDerivedSeverity(ctx, tenantID, c.ID)
}

func (r *CaseRepository) GetCase(ctx context.Context, tenantID, caseID uuid.UUID) (domain.OperationalCase, error) {
	row := r.pool.QueryRow(ctx, caseSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, caseID)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.NotFound("case not found")
		}
		return domain.OperationalCase{}, err
	}
	return c, nil
}

func (r *CaseRepository) ListCases(ctx context.Context, tenantID, userID uuid.UUID, filter domain.CaseListFilter) (domain.CasePage, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 50
	}
	if filter.Limit > domain.CasesMaxPageLimit {
		filter.Limit = domain.CasesMaxPageLimit
	}

	where := []string{"c.tenant_id = $1"}
	args := []any{tenantID}
	argN := 2

	applyPreset := filter.Preset
	if applyPreset == "" && filter.Status == "" && !filter.IncludeClosed {
		where = append(where, fmt.Sprintf("c.status NOT IN ('closed')"))
	}
	switch applyPreset {
	case "my_cases":
		where = append(where, fmt.Sprintf("(c.owner_user_id = $%d OR EXISTS (SELECT 1 FROM control_tower.operational_case_participant p WHERE p.case_id = c.id AND p.user_id = $%d))", argN, argN))
		args = append(args, userID)
		argN++
	case "unassigned":
		where = append(where, "c.owner_user_id IS NULL")
		where = append(where, "c.status NOT IN ('resolved','closed')")
	case "critical":
		where = append(where, "c.effective_severity = 'critical'")
		where = append(where, "c.status NOT IN ('resolved','closed')")
	case "action_required":
		where = append(where, "c.status = 'action_required'")
	case "monitoring":
		where = append(where, "c.status = 'monitoring'")
	case "resolved":
		where = append(where, "c.status = 'resolved'")
	case "closed":
		where = append(where, "c.status = 'closed'")
	}

	if filter.MyCases {
		where = append(where, fmt.Sprintf("(c.owner_user_id = $%d OR EXISTS (SELECT 1 FROM control_tower.operational_case_participant p WHERE p.case_id = c.id AND p.user_id = $%d))", argN, argN))
		args = append(args, userID)
		argN++
	}
	if filter.Unassigned {
		where = append(where, "c.owner_user_id IS NULL")
	}
	if filter.Status != "" {
		where = append(where, fmt.Sprintf("c.status = $%d", argN))
		args = append(args, filter.Status)
		argN++
	}
	if filter.Severity != "" {
		where = append(where, fmt.Sprintf("c.effective_severity = $%d", argN))
		args = append(args, filter.Severity)
		argN++
	}
	if filter.OwnerUserID != nil {
		where = append(where, fmt.Sprintf("c.owner_user_id = $%d", argN))
		args = append(args, *filter.OwnerUserID)
		argN++
	}
	if filter.Search != "" {
		search := "%" + strings.TrimSpace(filter.Search) + "%"
		where = append(where, fmt.Sprintf("(c.reference ILIKE $%d OR c.title ILIKE $%d)", argN, argN))
		args = append(args, search)
		argN++
	}
	if filter.ShipmentID != nil {
		where = append(where, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM control_tower.operational_case_link l
			WHERE l.case_id = c.id AND l.entity_type = 'shipment' AND l.entity_id = $%d
		)`, argN))
		args = append(args, filter.ShipmentID.String())
		argN++
	}

	whereSQL := strings.Join(where, " AND ")
	countQuery := `SELECT COUNT(*) FROM control_tower.operational_case c WHERE ` + whereSQL
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return domain.CasePage{}, err
	}

	offset := (filter.Page - 1) * filter.Limit
	listQuery := caseSelectSQL + ` FROM control_tower.operational_case c WHERE ` + whereSQL +
		fmt.Sprintf(` ORDER BY c.last_activity_at DESC LIMIT $%d OFFSET $%d`, argN, argN+1)
	args = append(args, filter.Limit, offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return domain.CasePage{}, err
	}
	defer rows.Close()

	items := make([]domain.OperationalCase, 0)
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return domain.CasePage{}, err
		}
		items = append(items, c)
	}
	hasNext := filter.Page*filter.Limit < total
	return domain.CasePage{Items: items, Page: filter.Page, Limit: filter.Limit, Total: total, HasNext: hasNext}, nil
}

func (r *CaseRepository) UpdateCase(ctx context.Context, tenantID, userID, caseID uuid.UUID, expectedVersion int64, patch map[string]any) (domain.OperationalCase, error) {
	existing, err := r.GetCase(ctx, tenantID, caseID)
	if err != nil {
		return domain.OperationalCase{}, err
	}
	if existing.Version != expectedVersion {
		return domain.OperationalCase{}, apperrors.Conflict("case was modified by another operator", map[string]any{
			"caseId": caseID.String(), "reference": existing.Reference,
		})
	}

	title := existing.Title
	if v, ok := patch["title"].(string); ok && strings.TrimSpace(v) != "" {
		title = strings.TrimSpace(v)
	}
	summary := existing.Summary
	if v, ok := patch["summary"].(string); ok {
		summary = strings.TrimSpace(v)
	}
	status := existing.Status
	if v, ok := patch["status"].(string); ok && v != "" {
		if !isValidCaseStatus(v) {
			return domain.OperationalCase{}, apperrors.Validation("invalid status", map[string]any{"field": "status"})
		}
		status = v
	}
	effectiveSeverity := existing.EffectiveSeverity
	severityOverride := existing.SeverityOverride
	if v, ok := patch["severity"].(string); ok && v != "" {
		if !isValidSeverity(v) {
			return domain.OperationalCase{}, apperrors.Validation("invalid severity", map[string]any{"field": "severity"})
		}
		effectiveSeverity = v
		severityOverride = true
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case
		SET title = $4, summary = $5, status = $6, effective_severity = $7, severity_override = $8,
		    version = version + 1, updated_at = NOW(), last_activity_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND version = $3
		RETURNING `+caseSelectColumns,
		tenantID, caseID, expectedVersion, title, nullIfEmpty(summary), status, effectiveSeverity, severityOverride)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.Conflict("case was modified by another operator", nil)
		}
		return domain.OperationalCase{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "case_updated", &userID, map[string]any{"status": status})
	return c, nil
}

func (r *CaseRepository) ClaimCase(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
	return r.setOwner(ctx, tenantID, userID, caseID, &userID, "case_claimed")
}

func (r *CaseRepository) AssignCase(ctx context.Context, tenantID, actorID, caseID, targetID uuid.UUID) (domain.OperationalCase, error) {
	return r.setOwner(ctx, tenantID, actorID, caseID, &targetID, "case_assigned")
}

func (r *CaseRepository) UnassignCase(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
	return r.setOwner(ctx, tenantID, userID, caseID, nil, "case_unassigned")
}

func (r *CaseRepository) setOwner(ctx context.Context, tenantID, actorID, caseID uuid.UUID, ownerID *uuid.UUID, action string) (domain.OperationalCase, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case
		SET owner_user_id = $3, version = version + 1, updated_at = NOW(), last_activity_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND status NOT IN ('closed')
		RETURNING `+caseSelectColumns,
		tenantID, caseID, ownerID)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.NotFound("case not found or closed")
		}
		return domain.OperationalCase{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, action, &actorID, nil)
	return c, nil
}

func (r *CaseRepository) AddLink(ctx context.Context, tenantID, userID, caseID uuid.UUID, entityType, entityID string) (domain.CaseLink, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CaseLink{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := r.getCaseTx(ctx, tx, tenantID, caseID); err != nil {
		return domain.CaseLink{}, err
	}

	if entityType == domain.CaseLinkException || entityType == domain.CaseLinkRisk {
		if err := r.addWorkLinkTx(ctx, tx, tenantID, caseID, userID, entityType, entityID); err != nil {
			return domain.CaseLink{}, err
		}
	} else {
		if err := r.addLinkTx(ctx, tx, tenantID, caseID, userID, entityType, entityID); err != nil {
			return domain.CaseLink{}, err
		}
	}

	if err := r.touchCaseTx(ctx, tx, tenantID, caseID); err != nil {
		return domain.CaseLink{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.CaseLink{}, err
	}
	_, _ = r.refreshDerivedSeverity(ctx, tenantID, caseID)
	return r.getLink(ctx, tenantID, caseID, entityType, entityID)
}

func (r *CaseRepository) RemoveLink(ctx context.Context, tenantID, userID, caseID, linkID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var entityType, entityID string
	err = tx.QueryRow(ctx, `
		DELETE FROM control_tower.operational_case_link
		WHERE tenant_id = $1 AND case_id = $2 AND id = $3
		RETURNING entity_type, entity_id
	`, tenantID, caseID, linkID).Scan(&entityType, &entityID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NotFound("link not found")
		}
		return err
	}

	if entityType == domain.CaseLinkException || entityType == domain.CaseLinkRisk {
		_, _ = tx.Exec(ctx, `
			DELETE FROM control_tower.operational_case_active_work_link
			WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3 AND case_id = $4
		`, tenantID, entityType, entityID, caseID)
	}

	_ = r.insertEvent(ctx, tx, tenantID, caseID, domain.CaseEventSourceCase, "link_removed", &userID, map[string]any{
		"entityType": entityType, "entityId": entityID,
	})
	_ = r.touchCaseTx(ctx, tx, tenantID, caseID)
	return tx.Commit(ctx)
}

func (r *CaseRepository) CreateNote(ctx context.Context, tenantID, userID, caseID uuid.UUID, body string, mentionIDs []uuid.UUID) (domain.CaseNote, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.CaseNote{}, apperrors.Validation("body is required", map[string]any{"field": "body"})
	}
	if _, err := r.GetCase(ctx, tenantID, caseID); err != nil {
		return domain.CaseNote{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CaseNote{}, err
	}
	defer tx.Rollback(ctx)

	var note domain.CaseNote
	row := tx.QueryRow(ctx, `
		INSERT INTO control_tower.operational_case_note
		    (tenant_id, case_id, author_user_id, body, visibility)
		VALUES ($1,$2,$3,$4,'internal')
		RETURNING id, tenant_id, case_id, author_user_id, body, visibility, created_at, updated_at, edited_at, deleted_at
	`, tenantID, caseID, userID, body)
	note, err = scanNote(row)
	if err != nil {
		return domain.CaseNote{}, err
	}

	for _, mid := range mentionIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO control_tower.operational_case_note_mention (note_id, mentioned_user_id)
			VALUES ($1,$2) ON CONFLICT DO NOTHING
		`, note.ID, mid)
		if err != nil {
			return domain.CaseNote{}, err
		}
	}
	note.MentionedIDs = mentionIDs

	_ = r.insertEvent(ctx, tx, tenantID, caseID, domain.CaseEventSourceNote, "note_added", &userID, map[string]any{"noteId": note.ID.String()})
	_ = r.touchCaseTx(ctx, tx, tenantID, caseID)
	if err := tx.Commit(ctx); err != nil {
		return domain.CaseNote{}, err
	}
	return note, nil
}

func (r *CaseRepository) UpdateNote(ctx context.Context, tenantID, userID, caseID, noteID uuid.UUID, body string) (domain.CaseNote, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.CaseNote{}, apperrors.Validation("body is required", map[string]any{"field": "body"})
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case_note
		SET body = $5, edited_at = NOW(), updated_at = NOW()
		WHERE tenant_id = $1 AND case_id = $2 AND id = $3 AND author_user_id = $4 AND deleted_at IS NULL
		RETURNING id, tenant_id, case_id, author_user_id, body, visibility, created_at, updated_at, edited_at, deleted_at
	`, tenantID, caseID, noteID, userID, body)
	note, err := scanNote(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.CaseNote{}, apperrors.NotFound("note not found")
		}
		return domain.CaseNote{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceNote, "note_edited", &userID, map[string]any{"noteId": noteID.String()})
	_ = r.touchCase(ctx, tenantID, caseID)
	return note, nil
}

func (r *CaseRepository) CreateActionItem(ctx context.Context, tenantID, userID, caseID uuid.UUID, title, description string, assigneeID *uuid.UUID, dueAt *time.Time) (domain.CaseActionItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.CaseActionItem{}, apperrors.Validation("title is required", map[string]any{"field": "title"})
	}
	if _, err := r.GetCase(ctx, tenantID, caseID); err != nil {
		return domain.CaseActionItem{}, err
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.operational_case_action_item
		    (tenant_id, case_id, title, description, assignee_user_id, due_at, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant_id, case_id, title, COALESCE(description,''), status, assignee_user_id,
		          due_at, created_by_user_id, created_at, updated_at, completed_at
	`, tenantID, caseID, title, nullIfEmpty(description), assigneeID, dueAt, userID)
	item, err := scanActionItem(row)
	if err != nil {
		return domain.CaseActionItem{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceActionItem, "action_created", &userID, map[string]any{"actionId": item.ID.String()})
	_ = r.touchCase(ctx, tenantID, caseID)
	return item, nil
}

func (r *CaseRepository) UpdateActionItem(ctx context.Context, tenantID, userID, caseID, actionID uuid.UUID, patch map[string]any) (domain.CaseActionItem, error) {
	existing, err := r.getActionItem(ctx, tenantID, caseID, actionID)
	if err != nil {
		return domain.CaseActionItem{}, err
	}
	title := existing.Title
	if v, ok := patch["title"].(string); ok && strings.TrimSpace(v) != "" {
		title = strings.TrimSpace(v)
	}
	desc := existing.Description
	if v, ok := patch["description"].(string); ok {
		desc = strings.TrimSpace(v)
	}
	status := existing.Status
	if v, ok := patch["status"].(string); ok && v != "" {
		status = v
	}
	var assigneeID *uuid.UUID = existing.AssigneeUserID
	if v, ok := patch["assigneeUserId"].(string); ok {
		if v == "" {
			assigneeID = nil
		} else if id, err := uuid.Parse(v); err == nil {
			assigneeID = &id
		}
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case_action_item
		SET title = $4, description = $5, status = $6, assignee_user_id = $7, updated_at = NOW(),
		    completed_at = CASE WHEN $6 = 'done' THEN COALESCE(completed_at, NOW()) ELSE completed_at END
		WHERE tenant_id = $1 AND case_id = $2 AND id = $3
		RETURNING id, tenant_id, case_id, title, COALESCE(description,''), status, assignee_user_id,
		          due_at, created_by_user_id, created_at, updated_at, completed_at
	`, tenantID, caseID, actionID, title, nullIfEmpty(desc), status, assigneeID)
	item, err := scanActionItem(row)
	if err != nil {
		return domain.CaseActionItem{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceActionItem, "action_updated", &userID, map[string]any{"actionId": actionID.String()})
	_ = r.touchCase(ctx, tenantID, caseID)
	return item, nil
}

func (r *CaseRepository) CompleteActionItem(ctx context.Context, tenantID, userID, caseID, actionID uuid.UUID) (domain.CaseActionItem, error) {
	return r.UpdateActionItem(ctx, tenantID, userID, caseID, actionID, map[string]any{"status": domain.ActionItemStatusDone})
}

func (r *CaseRepository) CreateDecision(ctx context.Context, tenantID, userID, caseID uuid.UUID, decision, rationale string) (domain.CaseDecision, error) {
	decision = strings.TrimSpace(decision)
	if decision == "" {
		return domain.CaseDecision{}, apperrors.Validation("decision is required", map[string]any{"field": "decision"})
	}
	if _, err := r.GetCase(ctx, tenantID, caseID); err != nil {
		return domain.CaseDecision{}, err
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO control_tower.operational_case_decision
		    (tenant_id, case_id, decision, rationale, decided_by_user_id)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, case_id, decision, COALESCE(rationale,''), decided_by_user_id, decided_at
	`, tenantID, caseID, decision, nullIfEmpty(rationale), userID)
	d, err := scanDecision(row)
	if err != nil {
		return domain.CaseDecision{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceDecision, "decision_recorded", &userID, map[string]any{"decisionId": d.ID.String()})
	_ = r.touchCase(ctx, tenantID, caseID)
	return d, nil
}

func (r *CaseRepository) ResolveCase(ctx context.Context, tenantID, userID, caseID uuid.UUID, code, summary string) (domain.OperationalCase, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return domain.OperationalCase{}, apperrors.Validation("resolutionCode is required", map[string]any{"field": "resolutionCode"})
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case
		SET status = 'resolved', resolution_code = $3, resolution_summary = $4,
		    resolved_at = NOW(), version = version + 1, updated_at = NOW(), last_activity_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND status NOT IN ('closed')
		RETURNING `+caseSelectColumns,
		tenantID, caseID, code, nullIfEmpty(summary))
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.NotFound("case not found")
		}
		return domain.OperationalCase{}, err
	}
	r.clearActiveWorkLinks(ctx, tenantID, caseID)
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "case_resolved", &userID, map[string]any{"code": code})
	return c, nil
}

func (r *CaseRepository) CloseCase(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case
		SET status = 'closed', closed_at = NOW(), version = version + 1, updated_at = NOW(), last_activity_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND status = 'resolved'
		RETURNING `+caseSelectColumns,
		tenantID, caseID)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.Validation("case must be resolved before closing", nil)
		}
		return domain.OperationalCase{}, err
	}
	r.clearActiveWorkLinks(ctx, tenantID, caseID)
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "case_closed", &userID, nil)
	return c, nil
}

func (r *CaseRepository) ReopenCase(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE control_tower.operational_case
		SET status = 'open', resolved_at = NULL, closed_at = NULL, resolution_code = NULL, resolution_summary = NULL,
		    version = version + 1, updated_at = NOW(), last_activity_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND status IN ('resolved','closed')
		RETURNING `+caseSelectColumns,
		tenantID, caseID)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.NotFound("case not found or not reopenable")
		}
		return domain.OperationalCase{}, err
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "case_reopened", &userID, nil)
	return c, nil
}

func (r *CaseRepository) ListLinks(ctx context.Context, tenantID, caseID uuid.UUID) ([]domain.CaseLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, case_id, entity_type, entity_id, linked_at, linked_by_user_id
		FROM control_tower.operational_case_link
		WHERE tenant_id = $1 AND case_id = $2 ORDER BY linked_at ASC
	`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLinks(rows)
}

func (r *CaseRepository) ListParticipants(ctx context.Context, tenantID, caseID uuid.UUID) ([]domain.CaseParticipant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT case_id, tenant_id, user_id, role, added_at, added_by_user_id
		FROM control_tower.operational_case_participant
		WHERE tenant_id = $1 AND case_id = $2
	`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.CaseParticipant, 0)
	for rows.Next() {
		var p domain.CaseParticipant
		if err := rows.Scan(&p.CaseID, &p.TenantID, &p.UserID, &p.Role, &p.AddedAt, &p.AddedByUserID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *CaseRepository) AddParticipant(ctx context.Context, tenantID, actorID, caseID, targetID uuid.UUID, role string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := r.addParticipantTx(ctx, tx, tenantID, caseID, actorID, targetID, role); err != nil {
		return err
	}
	_ = r.insertEvent(ctx, tx, tenantID, caseID, domain.CaseEventSourceCase, "participant_added", &actorID, map[string]any{"userId": targetID.String()})
	_ = r.touchCaseTx(ctx, tx, tenantID, caseID)
	return tx.Commit(ctx)
}

func (r *CaseRepository) RemoveParticipant(ctx context.Context, tenantID, actorID, caseID, targetID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM control_tower.operational_case_participant
		WHERE tenant_id = $1 AND case_id = $2 AND user_id = $3
	`, tenantID, caseID, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("participant not found")
	}
	_ = r.recordEvent(ctx, tenantID, caseID, domain.CaseEventSourceCase, "participant_removed", &actorID, map[string]any{"userId": targetID.String()})
	return r.touchCase(ctx, tenantID, caseID)
}

func (r *CaseRepository) ListNotes(ctx context.Context, tenantID, caseID uuid.UUID, limit int) ([]domain.CaseNote, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, case_id, author_user_id, body, visibility, created_at, updated_at, edited_at, deleted_at
		FROM control_tower.operational_case_note
		WHERE tenant_id = $1 AND case_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $3
	`, tenantID, caseID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.CaseNote, 0)
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (r *CaseRepository) ListActionItems(ctx context.Context, tenantID, caseID uuid.UUID) ([]domain.CaseActionItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, case_id, title, COALESCE(description,''), status, assignee_user_id,
		       due_at, created_by_user_id, created_at, updated_at, completed_at
		FROM control_tower.operational_case_action_item
		WHERE tenant_id = $1 AND case_id = $2 ORDER BY created_at ASC
	`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.CaseActionItem, 0)
	for rows.Next() {
		item, err := scanActionItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *CaseRepository) ListDecisions(ctx context.Context, tenantID, caseID uuid.UUID) ([]domain.CaseDecision, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, case_id, decision, COALESCE(rationale,''), decided_by_user_id, decided_at
		FROM control_tower.operational_case_decision
		WHERE tenant_id = $1 AND case_id = $2 ORDER BY decided_at DESC
	`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.CaseDecision, 0)
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *CaseRepository) ListTimeline(ctx context.Context, tenantID, caseID uuid.UUID, page, limit int) ([]domain.CaseEvent, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.operational_case_event WHERE tenant_id = $1 AND case_id = $2
	`, tenantID, caseID).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, case_id, source, action_type, actor_user_id, occurred_at, metadata
		FROM control_tower.operational_case_event
		WHERE tenant_id = $1 AND case_id = $2
		ORDER BY occurred_at DESC, id DESC LIMIT $3 OFFSET $4
	`, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.CaseEvent, 0)
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}

func (r *CaseRepository) GetKPI(ctx context.Context, tenantID, userID uuid.UUID) (domain.CaseKPI, error) {
	var kpi domain.CaseKPI
	err := r.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status NOT IN ('resolved','closed')),
		  COUNT(*) FILTER (WHERE status NOT IN ('resolved','closed') AND (owner_user_id = $2 OR EXISTS (
		    SELECT 1 FROM control_tower.operational_case_participant p WHERE p.case_id = c.id AND p.user_id = $2
		  ))),
		  COUNT(*) FILTER (WHERE status NOT IN ('resolved','closed') AND effective_severity = 'critical'),
		  COUNT(*) FILTER (WHERE status NOT IN ('resolved','closed') AND owner_user_id IS NULL),
		  COUNT(*) FILTER (WHERE status = 'resolved')
		FROM control_tower.operational_case c WHERE tenant_id = $1
	`, tenantID, userID).Scan(&kpi.OpenCases, &kpi.MyOpenCases, &kpi.CriticalCases, &kpi.UnassignedCases, &kpi.ResolvedCases)
	return kpi, err
}

func (r *CaseRepository) LookupActiveCaseForWorkItem(ctx context.Context, tenantID uuid.UUID, itemType, itemID string) (*domain.ActiveCaseRef, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.reference, c.title, c.status
		FROM control_tower.operational_case_active_work_link aw
		JOIN control_tower.operational_case c ON c.id = aw.case_id
		WHERE aw.tenant_id = $1 AND aw.entity_type = $2 AND aw.entity_id = $3
	`, tenantID, itemType, itemID)
	var ref domain.ActiveCaseRef
	if err := row.Scan(&ref.CaseID, &ref.Reference, &ref.Title, &ref.Status); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ref, nil
}

func (r *CaseRepository) FindDuplicateCandidates(ctx context.Context, tenantID uuid.UUID, itemType, itemID string, shipmentID *uuid.UUID) ([]domain.ActiveCaseRef, error) {
	refs := make([]domain.ActiveCaseRef, 0)
	if itemType != "" && itemID != "" {
		if existing, err := r.LookupActiveCaseForWorkItem(ctx, tenantID, itemType, itemID); err != nil {
			return nil, err
		} else if existing != nil {
			return []domain.ActiveCaseRef{*existing}, nil
		}
	}
	if shipmentID != nil {
		rows, err := r.pool.Query(ctx, `
			SELECT DISTINCT c.id, c.reference, c.title, c.status
			FROM control_tower.operational_case c
			JOIN control_tower.operational_case_link l ON l.case_id = c.id
			WHERE c.tenant_id = $1 AND l.entity_type = 'shipment' AND l.entity_id = $2
			  AND c.status NOT IN ('resolved','closed')
			LIMIT 5
		`, tenantID, shipmentID.String())
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var ref domain.ActiveCaseRef
			if err := rows.Scan(&ref.CaseID, &ref.Reference, &ref.Title, &ref.Status); err != nil {
				return nil, err
			}
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func (r *CaseRepository) GetCaseHealth(ctx context.Context, tenantID, caseID uuid.UUID) (domain.CaseHealth, error) {
	var health domain.CaseHealth
	// Simplified derived health from linked exceptions/risks and open actions
	err := r.pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM control_tower.operational_case_action_item ai
		   WHERE ai.case_id = $2 AND ai.status IN ('open','in_progress')),
		  (SELECT COUNT(*) FROM control_tower.operational_case_action_item ai
		   WHERE ai.case_id = $2 AND ai.status IN ('open','in_progress') AND ai.due_at IS NOT NULL AND ai.due_at < NOW())
	`, tenantID, caseID).Scan(&health.OpenActionCount, &health.OverdueActionCount)
	if err != nil {
		return health, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT entity_type, entity_id FROM control_tower.operational_case_link
		WHERE tenant_id = $1 AND case_id = $2 AND entity_type IN ('exception','risk')
	`, tenantID, caseID)
	if err != nil {
		return health, err
	}
	defer rows.Close()
	for rows.Next() {
		var entityType, entityID string
		if err := rows.Scan(&entityType, &entityID); err != nil {
			return health, err
		}
		health.ActiveWorkItemCount++
	}
	return health, nil
}

// --- helpers ---

const caseSelectColumns = `id, tenant_id, reference, title, COALESCE(summary,''), status,
	derived_severity, effective_severity, severity_override, owner_user_id,
	created_by_user_id, resolution_code, resolution_summary, version,
	last_activity_at, created_at, updated_at, resolved_at, closed_at`

const caseSelectSQL = `SELECT ` + caseSelectColumns

func (r *CaseRepository) nextReference(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (string, error) {
	year := time.Now().UTC().Year()
	var last int64
	err := tx.QueryRow(ctx, `
		INSERT INTO control_tower.operational_case_reference_counter (tenant_id, year, last_value)
		VALUES ($1, $2, 1)
		ON CONFLICT (tenant_id, year) DO UPDATE SET last_value = operational_case_reference_counter.last_value + 1
		RETURNING last_value
	`, tenantID, year).Scan(&last)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CT-%d-%06d", year, last), nil
}

func (r *CaseRepository) addWorkLinkTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, userID uuid.UUID, entityType, entityID string) error {
	if err := r.validateWorkEntity(ctx, tx, tenantID, entityType, entityID); err != nil {
		return err
	}
	var existingCaseID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT case_id FROM control_tower.operational_case_active_work_link
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3
	`, tenantID, entityType, entityID).Scan(&existingCaseID)
	if err == nil {
		var ref string
		_ = tx.QueryRow(ctx, `SELECT reference FROM control_tower.operational_case WHERE id = $1`, existingCaseID).Scan(&ref)
		return apperrors.Conflict("work item already linked to active case", map[string]any{
			"caseId": existingCaseID.String(), "reference": ref,
		})
	}
	if err != pgx.ErrNoRows {
		return err
	}
	if err := r.addLinkTx(ctx, tx, tenantID, caseID, userID, entityType, entityID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.operational_case_active_work_link (tenant_id, entity_type, entity_id, case_id)
		VALUES ($1,$2,$3,$4)
	`, tenantID, entityType, entityID, caseID)
	return err
}

func (r *CaseRepository) addLinkTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, userID uuid.UUID, entityType, entityID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO control_tower.operational_case_link
		    (tenant_id, case_id, entity_type, entity_id, linked_by_user_id)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (tenant_id, case_id, entity_type, entity_id) DO NOTHING
	`, tenantID, caseID, entityType, entityID, userID)
	if err != nil {
		return err
	}
	return r.insertEvent(ctx, tx, tenantID, caseID, domain.CaseEventSourceCase, "link_added", &userID, map[string]any{
		"entityType": entityType, "entityId": entityID,
	})
}

func (r *CaseRepository) validateWorkEntity(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, entityType, entityID string) error {
	switch entityType {
	case domain.CaseLinkException:
		var ok bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM control_tower.critical_event_workflow WHERE tenant_id = $1 AND event_id = $2)
		`, tenantID, entityID).Scan(&ok)
		if err != nil || !ok {
			return apperrors.NotFound("exception not found")
		}
	case domain.CaseLinkRisk:
		var ok bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM control_tower.shipment_risk WHERE tenant_id = $1 AND risk_key = $2)
		`, tenantID, entityID).Scan(&ok)
		if err != nil || !ok {
			return apperrors.NotFound("risk not found")
		}
	}
	return nil
}

func (r *CaseRepository) addParticipantTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, actorID, targetID uuid.UUID, role string) error {
	if role == "" {
		role = domain.ParticipantRoleCollaborator
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO control_tower.operational_case_participant
		    (case_id, tenant_id, user_id, role, added_by_user_id)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (case_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, caseID, tenantID, targetID, role, actorID)
	return err
}

func (r *CaseRepository) clearActiveWorkLinks(ctx context.Context, tenantID, caseID uuid.UUID) {
	_, _ = r.pool.Exec(ctx, `
		DELETE FROM control_tower.operational_case_active_work_link WHERE case_id = $1 AND tenant_id = $2
	`, caseID, tenantID)
}

func (r *CaseRepository) refreshDerivedSeverity(ctx context.Context, tenantID, caseID uuid.UUID) (domain.OperationalCase, error) {
	c, err := r.GetCase(ctx, tenantID, caseID)
	if err != nil {
		return c, err
	}
	if c.SeverityOverride {
		return c, nil
	}
	derived := domain.CaseSeverityMedium
	// keep medium default for v0.6; linked item severity enrichment follow-up
	_, _ = r.pool.Exec(ctx, `
		UPDATE control_tower.operational_case SET derived_severity = $3,
		    effective_severity = CASE WHEN severity_override THEN effective_severity ELSE $3 END
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, caseID, derived)
	return r.GetCase(ctx, tenantID, caseID)
}

func (r *CaseRepository) getCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID uuid.UUID) (domain.OperationalCase, error) {
	row := tx.QueryRow(ctx, caseSelectSQL+` FROM control_tower.operational_case c WHERE tenant_id = $1 AND id = $2`, tenantID, caseID)
	c, err := scanCase(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.OperationalCase{}, apperrors.NotFound("case not found")
		}
		return domain.OperationalCase{}, err
	}
	return c, nil
}

func (r *CaseRepository) getLink(ctx context.Context, tenantID, caseID uuid.UUID, entityType, entityID string) (domain.CaseLink, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, case_id, entity_type, entity_id, linked_at, linked_by_user_id
		FROM control_tower.operational_case_link
		WHERE tenant_id = $1 AND case_id = $2 AND entity_type = $3 AND entity_id = $4
	`, tenantID, caseID, entityType, entityID)
	var l domain.CaseLink
	err := row.Scan(&l.ID, &l.TenantID, &l.CaseID, &l.EntityType, &l.EntityID, &l.LinkedAt, &l.LinkedByUserID)
	return l, err
}

func (r *CaseRepository) getActionItem(ctx context.Context, tenantID, caseID, actionID uuid.UUID) (domain.CaseActionItem, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, case_id, title, COALESCE(description,''), status, assignee_user_id,
		       due_at, created_by_user_id, created_at, updated_at, completed_at
		FROM control_tower.operational_case_action_item
		WHERE tenant_id = $1 AND case_id = $2 AND id = $3
	`, tenantID, caseID, actionID)
	item, err := scanActionItem(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.CaseActionItem{}, apperrors.NotFound("action item not found")
		}
		return domain.CaseActionItem{}, err
	}
	return item, nil
}

func (r *CaseRepository) touchCase(ctx context.Context, tenantID, caseID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE control_tower.operational_case SET last_activity_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND id = $2
	`, tenantID, caseID)
	return err
}

func (r *CaseRepository) touchCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE control_tower.operational_case SET last_activity_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND id = $2
	`, tenantID, caseID)
	return err
}

func (r *CaseRepository) recordEvent(ctx context.Context, tenantID, caseID uuid.UUID, source, action string, actor *uuid.UUID, meta map[string]any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := r.insertEvent(ctx, tx, tenantID, caseID, source, action, actor, meta); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *CaseRepository) insertEvent(ctx context.Context, tx pgx.Tx, tenantID, caseID uuid.UUID, source, action string, actor *uuid.UUID, meta map[string]any) error {
	raw, _ := json.Marshal(meta)
	_, err := tx.Exec(ctx, `
		INSERT INTO control_tower.operational_case_event (tenant_id, case_id, source, action_type, actor_user_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
	`, tenantID, caseID, source, action, actor, string(raw))
	return err
}

func scanCase(row pgx.Row) (domain.OperationalCase, error) {
	var c domain.OperationalCase
	var summary, resolutionCode, resolutionSummary *string
	err := row.Scan(&c.ID, &c.TenantID, &c.Reference, &c.Title, &summary, &c.Status,
		&c.DerivedSeverity, &c.EffectiveSeverity, &c.SeverityOverride, &c.OwnerUserID,
		&c.CreatedByUserID, &resolutionCode, &resolutionSummary, &c.Version,
		&c.LastActivityAt, &c.CreatedAt, &c.UpdatedAt, &c.ResolvedAt, &c.ClosedAt)
	if summary != nil {
		c.Summary = *summary
	}
	if resolutionCode != nil {
		c.ResolutionCode = resolutionCode
	}
	if resolutionSummary != nil {
		c.ResolutionSummary = resolutionSummary
	}
	return c, err
}

func scanNote(row pgx.Row) (domain.CaseNote, error) {
	var n domain.CaseNote
	err := row.Scan(&n.ID, &n.TenantID, &n.CaseID, &n.AuthorUserID, &n.Body, &n.Visibility,
		&n.CreatedAt, &n.UpdatedAt, &n.EditedAt, &n.DeletedAt)
	return n, err
}

func scanActionItem(row pgx.Row) (domain.CaseActionItem, error) {
	var item domain.CaseActionItem
	var desc string
	err := row.Scan(&item.ID, &item.TenantID, &item.CaseID, &item.Title, &desc, &item.Status,
		&item.AssigneeUserID, &item.DueAt, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt, &item.CompletedAt)
	item.Description = desc
	return item, err
}

func scanDecision(row pgx.Row) (domain.CaseDecision, error) {
	var d domain.CaseDecision
	var rationale string
	err := row.Scan(&d.ID, &d.TenantID, &d.CaseID, &d.Decision, &rationale, &d.DecidedByUserID, &d.DecidedAt)
	d.Rationale = rationale
	return d, err
}

func scanEvent(row pgx.Row) (domain.CaseEvent, error) {
	var e domain.CaseEvent
	var raw []byte
	err := row.Scan(&e.ID, &e.TenantID, &e.CaseID, &e.Source, &e.ActionType, &e.ActorUserID, &e.OccurredAt, &raw)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &e.Metadata)
	}
	return e, err
}

func scanLinks(rows pgx.Rows) ([]domain.CaseLink, error) {
	out := make([]domain.CaseLink, 0)
	for rows.Next() {
		var l domain.CaseLink
		if err := rows.Scan(&l.ID, &l.TenantID, &l.CaseID, &l.EntityType, &l.EntityID, &l.LinkedAt, &l.LinkedByUserID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func isValidCaseStatus(s string) bool {
	switch s {
	case domain.CaseStatusOpen, domain.CaseStatusInvestigating, domain.CaseStatusActionRequired,
		domain.CaseStatusMonitoring, domain.CaseStatusResolved, domain.CaseStatusClosed:
		return true
	}
	return false
}

func isValidSeverity(s string) bool {
	switch s {
	case domain.CaseSeverityCritical, domain.CaseSeverityHigh, domain.CaseSeverityMedium, domain.CaseSeverityLow:
		return true
	}
	return false
}

func nullIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
