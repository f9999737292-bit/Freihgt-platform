package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type QuestionnaireRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewQuestionnaireRepository(pool *pgxpool.Pool) *QuestionnaireRepository {
	return &QuestionnaireRepository{pool: pool}
}

func (r *QuestionnaireRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *QuestionnaireRepository) GetOrCreateDraftVersion(ctx context.Context, tenantID, eventID uuid.UUID) (*domain.RfxVersion, error) {
	const draftQuery = `SELECT draft_version_id FROM rfx.rfx_events WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var draftID *uuid.UUID
	if err := r.db().QueryRow(ctx, draftQuery, eventID, tenantID).Scan(&draftID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx event not found")
		}
		return nil, mapDBError(err)
	}
	if draftID != nil {
		version, err := r.GetVersionByID(ctx, *draftID, tenantID)
		if err != nil {
			return nil, err
		}
		if version.Status == domain.RfxVersionStatusDraft {
			return version, nil
		}
	}
	var maxNum int
	if err := r.db().QueryRow(ctx, `SELECT COALESCE(MAX(version_number),0) FROM rfx.rfx_versions WHERE rfx_event_id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, eventID, tenantID).Scan(&maxNum); err != nil {
		return nil, mapDBError(err)
	}
	const insert = `
		INSERT INTO rfx.rfx_versions (tenant_id, rfx_event_id, version_number, status, questionnaire_enabled)
		VALUES ($1,$2,$3,$4,FALSE)
		RETURNING id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at, published_by, created_at, updated_at, version`
	row := r.db().QueryRow(ctx, insert, tenantID, eventID, maxNum+1, domain.RfxVersionStatusDraft)
	ver, err := scanRfxVersion(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	_, err = r.db().Exec(ctx, `UPDATE rfx.rfx_events SET draft_version_id=$3, updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2`, eventID, tenantID, ver.ID)
	if err != nil {
		return nil, mapDBError(err)
	}
	return ver, nil
}

func (r *QuestionnaireRepository) GetVersionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at, published_by, created_at, updated_at, version
		FROM rfx.rfx_versions WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, id, tenantID)
	ver, err := scanRfxVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx version not found")
		}
		return nil, mapDBError(err)
	}
	return ver, nil
}

func (r *QuestionnaireRepository) TouchDraftVersion(ctx context.Context, versionID, tenantID uuid.UUID, expectedVersion int) (*domain.RfxVersion, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_versions SET updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND status=$3 AND version=$4
		RETURNING id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at, published_by, created_at, updated_at, version`,
		versionID, tenantID, domain.RfxVersionStatusDraft, expectedVersion)
	ver, err := scanRfxVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("draft version was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return ver, nil
}

func (r *QuestionnaireRepository) LoadQuestionnaire(ctx context.Context, versionID, tenantID uuid.UUID) (*domain.QuestionnaireDefinition, error) {
	ver, err := r.GetVersionByID(ctx, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	sections, err := r.ListSections(ctx, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	rules, err := r.ListRules(ctx, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	swq := make([]domain.SectionWithQuestions, 0, len(sections))
	for _, sec := range sections {
		questions, err := r.ListQuestionsBySection(ctx, sec.ID, tenantID)
		if err != nil {
			return nil, err
		}
		for i := range questions {
			opts, err := r.ListOptionsByQuestion(ctx, questions[i].ID, tenantID)
			if err != nil {
				return nil, err
			}
			questions[i].Options = opts
		}
		swq = append(swq, domain.SectionWithQuestions{Section: sec, Questions: questions})
	}
	return &domain.QuestionnaireDefinition{
		EventID:              ver.RfxEventID,
		RfxVersionID:         ver.ID,
		VersionNumber:        ver.VersionNumber,
		QuestionnaireEnabled: ver.QuestionnaireEnabled,
		VersionStatus:        ver.Status,
		Sections:             swq,
		Rules:                rules,
	}, nil
}

func (r *QuestionnaireRepository) CreateSection(ctx context.Context, tenantID, versionID uuid.UUID, in domain.CreateSectionInput) (*domain.Section, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_sections (tenant_id, rfx_version_id, section_code, title, description, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, tenant_id, rfx_version_id, section_code, title, description, sort_order, created_at, updated_at, version`,
		tenantID, versionID, strings.TrimSpace(in.SectionCode), strings.TrimSpace(in.Title), optionalString(in.Description), sortOrder)
	return scanSection(row)
}

func (r *QuestionnaireRepository) UpdateSection(ctx context.Context, sectionID, tenantID uuid.UUID, in domain.UpdateSectionInput) (*domain.Section, error) {
	sets := []string{"updated_at=now()", "version=version+1"}
	args := []any{sectionID, tenantID, in.ExpectedVersion}
	idx := 4
	if in.Title != nil {
		sets = append(sets, fmt.Sprintf("title=$%d", idx))
		args = append(args, strings.TrimSpace(*in.Title))
		idx++
	}
	if in.Description != nil {
		sets = append(sets, fmt.Sprintf("description=$%d", idx))
		args = append(args, optionalString(in.Description))
		idx++
	}
	if in.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order=$%d", idx))
		args = append(args, *in.SortOrder)
	}
	q := fmt.Sprintf(`UPDATE rfx.rfx_sections SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, rfx_version_id, section_code, title, description, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	sec, err := scanSection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("section was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return sec, nil
}

func (r *QuestionnaireRepository) DeleteSection(ctx context.Context, sectionID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_sections SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, sectionID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("section was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *QuestionnaireRepository) ReorderSections(ctx context.Context, tenantID, versionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_sections SET sort_order=$4, updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND rfx_version_id=$3 AND deleted_at IS NULL`, id, tenantID, versionID, i)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("section not found")
		}
	}
	return nil
}

func (r *QuestionnaireRepository) ListSections(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.Section, error) {
	rows, err := r.db().Query(ctx, `SELECT id, tenant_id, rfx_version_id, section_code, title, description, sort_order, created_at, updated_at, version FROM rfx.rfx_sections WHERE rfx_version_id=$1 AND tenant_id=$2 AND deleted_at IS NULL ORDER BY sort_order`, versionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.Section, 0)
	for rows.Next() {
		sec, err := scanSection(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *sec)
	}
	return out, rows.Err()
}

func (r *QuestionnaireRepository) GetSectionByID(ctx context.Context, sectionID, tenantID uuid.UUID) (*domain.Section, error) {
	row := r.db().QueryRow(ctx, `SELECT id, tenant_id, rfx_version_id, section_code, title, description, sort_order, created_at, updated_at, version FROM rfx.rfx_sections WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, sectionID, tenantID)
	sec, err := scanSection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("section not found")
		}
		return nil, mapDBError(err)
	}
	return sec, nil
}

func (r *QuestionnaireRepository) CountQuestionsInSection(ctx context.Context, sectionID, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.db().QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_questions WHERE section_id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, sectionID, tenantID).Scan(&count)
	return count, mapDBError(err)
}

func (r *QuestionnaireRepository) CreateQuestion(ctx context.Context, tenantID, sectionID uuid.UUID, in domain.CreateQuestionInput) (*domain.Question, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	val := in.ValidationRuleJSON
	if len(val) == 0 {
		val = json.RawMessage(`{}`)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_questions (tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9)
		RETURNING id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version`,
		tenantID, sectionID, strings.TrimSpace(in.QuestionCode), strings.TrimSpace(in.QuestionType), strings.TrimSpace(in.Label), optionalString(in.HelpText), in.Required, string(val), sortOrder)
	return scanQuestion(row)
}

func (r *QuestionnaireRepository) UpdateQuestion(ctx context.Context, questionID, tenantID uuid.UUID, in domain.UpdateQuestionInput) (*domain.Question, error) {
	sets := []string{"updated_at=now()", "version=version+1"}
	args := []any{questionID, tenantID, in.ExpectedVersion}
	idx := 4
	if in.QuestionType != nil {
		sets = append(sets, fmt.Sprintf("question_type=$%d", idx))
		args = append(args, strings.TrimSpace(*in.QuestionType))
		idx++
	}
	if in.Label != nil {
		sets = append(sets, fmt.Sprintf("label=$%d", idx))
		args = append(args, strings.TrimSpace(*in.Label))
		idx++
	}
	if in.HelpText != nil {
		sets = append(sets, fmt.Sprintf("help_text=$%d", idx))
		args = append(args, optionalString(in.HelpText))
		idx++
	}
	if in.Required != nil {
		sets = append(sets, fmt.Sprintf("required=$%d", idx))
		args = append(args, *in.Required)
		idx++
	}
	if len(in.ValidationRuleJSON) > 0 {
		sets = append(sets, fmt.Sprintf("validation_rule_json=$%d::jsonb", idx))
		args = append(args, string(in.ValidationRuleJSON))
		idx++
	}
	if in.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order=$%d", idx))
		args = append(args, *in.SortOrder)
	}
	q := fmt.Sprintf(`UPDATE rfx.rfx_questions SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	question, err := scanQuestion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("question was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return question, nil
}

func (r *QuestionnaireRepository) DeleteQuestion(ctx context.Context, questionID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_questions SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, questionID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("question was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *QuestionnaireRepository) GetQuestionByID(ctx context.Context, questionID, tenantID uuid.UUID) (*domain.Question, error) {
	row := r.db().QueryRow(ctx, `SELECT id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version FROM rfx.rfx_questions WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, questionID, tenantID)
	q, err := scanQuestion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("question not found")
		}
		return nil, mapDBError(err)
	}
	opts, err := r.ListOptionsByQuestion(ctx, questionID, tenantID)
	if err != nil {
		return nil, err
	}
	q.Options = opts
	return q, nil
}

func (r *QuestionnaireRepository) ListQuestionsBySection(ctx context.Context, sectionID, tenantID uuid.UUID) ([]domain.Question, error) {
	rows, err := r.db().Query(ctx, `SELECT id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version FROM rfx.rfx_questions WHERE section_id=$1 AND tenant_id=$2 AND deleted_at IS NULL ORDER BY sort_order`, sectionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.Question, 0)
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func (r *QuestionnaireRepository) ReorderQuestions(ctx context.Context, tenantID, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_questions SET sort_order=$4, updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND section_id=$3 AND deleted_at IS NULL`, id, tenantID, sectionID, i)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("question not found")
		}
	}
	return nil
}

func (r *QuestionnaireRepository) CreateOption(ctx context.Context, tenantID, questionID uuid.UUID, in domain.CreateQuestionOptionInput) (*domain.QuestionOption, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_question_options (tenant_id, question_id, option_code, label, sort_order)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version`,
		tenantID, questionID, strings.TrimSpace(in.OptionCode), strings.TrimSpace(in.Label), sortOrder)
	opt, err := scanQuestionOption(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return opt, nil
}

func (r *QuestionnaireRepository) UpdateOption(ctx context.Context, optionID, tenantID uuid.UUID, in domain.UpdateQuestionOptionInput) (*domain.QuestionOption, error) {
	sets := []string{"updated_at=now()", "version=version+1"}
	args := []any{optionID, tenantID, in.ExpectedVersion}
	idx := 4
	if in.Label != nil {
		sets = append(sets, fmt.Sprintf("label=$%d", idx))
		args = append(args, strings.TrimSpace(*in.Label))
		idx++
	}
	if in.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order=$%d", idx))
		args = append(args, *in.SortOrder)
	}
	q := fmt.Sprintf(`UPDATE rfx.rfx_question_options SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	opt, err := scanQuestionOption(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("option was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return opt, nil
}

func (r *QuestionnaireRepository) DeleteOption(ctx context.Context, optionID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_question_options SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, optionID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("option was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *QuestionnaireRepository) ListOptionsByQuestion(ctx context.Context, questionID, tenantID uuid.UUID) ([]domain.QuestionOption, error) {
	rows, err := r.db().Query(ctx, `SELECT id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version FROM rfx.rfx_question_options WHERE question_id=$1 AND tenant_id=$2 AND deleted_at IS NULL ORDER BY sort_order`, questionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.QuestionOption, 0)
	for rows.Next() {
		opt, err := scanQuestionOption(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *opt)
	}
	return out, rows.Err()
}

func (r *QuestionnaireRepository) CreateRule(ctx context.Context, tenantID, versionID uuid.UUID, in domain.CreateQuestionRuleInput, targetQuestionID uuid.UUID) (*domain.QuestionRule, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	cond := in.ConditionJSON
	if len(cond) == 0 {
		cond = json.RawMessage(`{}`)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_question_rules (tenant_id, rfx_version_id, target_question_id, rule_code, action, condition_json, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)
		RETURNING id, tenant_id, rfx_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version`,
		tenantID, versionID, targetQuestionID, strings.TrimSpace(in.RuleCode), strings.TrimSpace(in.Action), string(cond), sortOrder)
	return scanQuestionRule(row)
}

func (r *QuestionnaireRepository) UpdateRule(ctx context.Context, ruleID, tenantID uuid.UUID, in domain.UpdateQuestionRuleInput) (*domain.QuestionRule, error) {
	sets := []string{"updated_at=now()", "version=version+1"}
	args := []any{ruleID, tenantID, in.ExpectedVersion}
	idx := 4
	if in.Action != nil {
		sets = append(sets, fmt.Sprintf("action=$%d", idx))
		args = append(args, strings.TrimSpace(*in.Action))
		idx++
	}
	if len(in.ConditionJSON) > 0 {
		sets = append(sets, fmt.Sprintf("condition_json=$%d::jsonb", idx))
		args = append(args, string(in.ConditionJSON))
		idx++
	}
	if in.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order=$%d", idx))
		args = append(args, *in.SortOrder)
	}
	q := fmt.Sprintf(`UPDATE rfx.rfx_question_rules SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, rfx_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	rule, err := scanQuestionRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("rule was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return rule, nil
}

func (r *QuestionnaireRepository) DeleteRule(ctx context.Context, ruleID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_question_rules SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, ruleID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("rule was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *QuestionnaireRepository) ListRules(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.QuestionRule, error) {
	rows, err := r.db().Query(ctx, `SELECT id, tenant_id, rfx_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version FROM rfx.rfx_question_rules WHERE rfx_version_id=$1 AND tenant_id=$2 AND deleted_at IS NULL ORDER BY sort_order`, versionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.QuestionRule, 0)
	for rows.Next() {
		rule, err := scanQuestionRule(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *rule)
	}
	return out, rows.Err()
}

type scannable interface{ Scan(dest ...any) error }

func scanRfxVersion(row scannable) (*domain.RfxVersion, error) {
	var v domain.RfxVersion
	if err := row.Scan(&v.ID, &v.TenantID, &v.RfxEventID, &v.VersionNumber, &v.Status, &v.QuestionnaireEnabled, &v.PublishedAt, &v.PublishedBy, &v.CreatedAt, &v.UpdatedAt, &v.Version); err != nil {
		return nil, err
	}
	return &v, nil
}

func scanSection(row scannable) (*domain.Section, error) {
	var s domain.Section
	if err := row.Scan(&s.ID, &s.TenantID, &s.RfxVersionID, &s.SectionCode, &s.Title, &s.Description, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt, &s.Version); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanQuestion(row scannable) (*domain.Question, error) {
	var q domain.Question
	if err := row.Scan(&q.ID, &q.TenantID, &q.SectionID, &q.QuestionCode, &q.QuestionType, &q.Label, &q.HelpText, &q.Required, &q.ValidationRuleJSON, &q.SortOrder, &q.CreatedAt, &q.UpdatedAt, &q.Version); err != nil {
		return nil, err
	}
	if len(q.ValidationRuleJSON) == 0 {
		q.ValidationRuleJSON = json.RawMessage(`{}`)
	}
	return &q, nil
}

func scanQuestionOption(row scannable) (*domain.QuestionOption, error) {
	var o domain.QuestionOption
	if err := row.Scan(&o.ID, &o.TenantID, &o.QuestionID, &o.OptionCode, &o.Label, &o.SortOrder, &o.CreatedAt, &o.UpdatedAt, &o.Version); err != nil {
		return nil, err
	}
	return &o, nil
}

func scanQuestionRule(row scannable) (*domain.QuestionRule, error) {
	var rule domain.QuestionRule
	if err := row.Scan(&rule.ID, &rule.TenantID, &rule.RfxVersionID, &rule.TargetQuestionID, &rule.RuleCode, &rule.Action, &rule.ConditionJSON, &rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt, &rule.Version); err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *QuestionnaireRepository) TouchVersion(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.RfxVersion, error) {
	return r.TouchDraftVersion(ctx, id, tenantID, expectedVersion)
}

func (r *QuestionnaireRepository) ListRulesByVersion(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.QuestionRule, error) {
	return r.ListRules(ctx, versionID, tenantID)
}

func (r *QuestionnaireRepository) LoadQuestionnaireTree(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.SectionWithQuestions, error) {
	def, err := r.LoadQuestionnaire(ctx, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	return def.Sections, nil
}

func (r *QuestionnaireRepository) CreateQuestionOption(ctx context.Context, tenantID, questionID uuid.UUID, in domain.CreateQuestionOptionInput) (*domain.QuestionOption, error) {
	return r.CreateOption(ctx, tenantID, questionID, in)
}

func (r *QuestionnaireRepository) UpdateQuestionOption(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateQuestionOptionInput) (*domain.QuestionOption, error) {
	return r.UpdateOption(ctx, id, tenantID, in)
}

func (r *QuestionnaireRepository) DeleteQuestionOption(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) error {
	return r.DeleteOption(ctx, id, tenantID, expectedVersion)
}

func (r *QuestionnaireRepository) GetQuestionOptionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.QuestionOption, error) {
	row := r.db().QueryRow(ctx, `SELECT id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version FROM rfx.rfx_question_options WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, id, tenantID)
	opt, err := scanQuestionOption(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("question option not found")
		}
		return nil, mapDBError(err)
	}
	return opt, nil
}

func (r *QuestionnaireRepository) GetQuestionRuleByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.QuestionRule, error) {
	row := r.db().QueryRow(ctx, `SELECT id, tenant_id, rfx_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version FROM rfx.rfx_question_rules WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, id, tenantID)
	rule, err := scanQuestionRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("question rule not found")
		}
		return nil, mapDBError(err)
	}
	return rule, nil
}

func (r *QuestionnaireRepository) CreateQuestionRule(ctx context.Context, tenantID, versionID uuid.UUID, targetQuestionID *uuid.UUID, in domain.CreateQuestionRuleInput) (*domain.QuestionRule, error) {
	if targetQuestionID == nil {
		return nil, apperrors.Validation("target question is required for rules", map[string]any{"field": "target_question_code"})
	}
	return r.CreateRule(ctx, tenantID, versionID, in, *targetQuestionID)
}

func (r *QuestionnaireRepository) UpdateQuestionRule(ctx context.Context, id, tenantID uuid.UUID, targetQuestionID *uuid.UUID, in domain.UpdateQuestionRuleInput) (*domain.QuestionRule, error) {
	rule, err := r.UpdateRule(ctx, id, tenantID, in)
	if err != nil {
		return nil, err
	}
	if targetQuestionID != nil {
		row := r.db().QueryRow(ctx, `UPDATE rfx.rfx_question_rules SET target_question_id=$4, updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3 RETURNING id, tenant_id, rfx_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version`, id, tenantID, rule.Version, *targetQuestionID)
		updated, err := scanQuestionRule(row)
		if err != nil {
			return nil, mapDBError(err)
		}
		return updated, nil
	}
	return rule, nil
}

func (r *QuestionnaireRepository) DeleteQuestionRule(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) error {
	return r.DeleteRule(ctx, id, tenantID, expectedVersion)
}

func (r *QuestionnaireRepository) GetQuestionIDByCodeInVersion(ctx context.Context, versionID, tenantID uuid.UUID, questionCode string) (*uuid.UUID, error) {
	row := r.db().QueryRow(ctx, `
		SELECT q.id FROM rfx.rfx_questions q
		INNER JOIN rfx.rfx_sections s ON s.id = q.section_id
		WHERE s.rfx_version_id = $1 AND q.tenant_id = $2 AND q.question_code = $3
			AND q.deleted_at IS NULL AND s.deleted_at IS NULL`, versionID, tenantID, strings.TrimSpace(questionCode))
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("question not found")
		}
		return nil, mapDBError(err)
	}
	return &id, nil
}

func (r *QuestionnaireRepository) AssertSectionBelongsToVersion(ctx context.Context, sectionID, versionID, tenantID uuid.UUID) error {
	section, err := r.GetSectionByID(ctx, sectionID, tenantID)
	if err != nil {
		return err
	}
	if section.RfxVersionID != versionID {
		return apperrors.NotFound("section not found")
	}
	return nil
}

func (r *QuestionnaireRepository) AssertQuestionBelongsToVersion(ctx context.Context, questionID, versionID, tenantID uuid.UUID) error {
	question, err := r.GetQuestionByID(ctx, questionID, tenantID)
	if err != nil {
		return err
	}
	section, err := r.GetSectionByID(ctx, question.SectionID, tenantID)
	if err != nil {
		return err
	}
	if section.RfxVersionID != versionID {
		return apperrors.NotFound("question not found")
	}
	return nil
}

func (r *QuestionnaireRepository) DuplicateQuestion(ctx context.Context, tenantID, questionID uuid.UUID, newCode string) (*domain.Question, error) {
	source, err := r.GetQuestionByID(ctx, questionID, tenantID)
	if err != nil {
		return nil, err
	}
	created, err := r.CreateQuestion(ctx, tenantID, source.SectionID, domain.CreateQuestionInput{
		QuestionCode:       newCode,
		QuestionType:       source.QuestionType,
		Label:              source.Label + " (copy)",
		HelpText:           source.HelpText,
		Required:           source.Required,
		ValidationRuleJSON: source.ValidationRuleJSON,
		SortOrder:          intPtr(source.SortOrder + 1),
	})
	if err != nil {
		return nil, err
	}
	for _, opt := range source.Options {
		if _, err := r.CreateOption(ctx, tenantID, created.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.OptionCode,
			Label:      opt.Label,
			SortOrder:  intPtr(opt.SortOrder),
		}); err != nil {
			return nil, err
		}
	}
	return r.GetQuestionByID(ctx, created.ID, tenantID)
}

func intPtr(v int) *int { return &v }

func (r *QuestionnaireRepository) GetPublishedVersionForEvent(ctx context.Context, eventID, tenantID uuid.UUID) (*domain.RfxVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at, published_by, created_at, updated_at, version
		FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND status = $3 AND deleted_at IS NULL
		ORDER BY version_number DESC
		LIMIT 1`, eventID, tenantID, domain.RfxVersionStatusPublished)
	ver, err := scanRfxVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("published questionnaire version not found")
		}
		return nil, mapDBError(err)
	}
	return ver, nil
}

