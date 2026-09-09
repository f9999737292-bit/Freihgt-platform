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

type TemplateQuestionnaireRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewTemplateQuestionnaireRepository(pool *pgxpool.Pool) *TemplateQuestionnaireRepository {
	return &TemplateQuestionnaireRepository{pool: pool}
}

func (r *TemplateQuestionnaireRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *TemplateQuestionnaireRepository) WithTx(tx pgx.Tx) *TemplateQuestionnaireRepository {
	return &TemplateQuestionnaireRepository{pool: r.pool, exec: tx}
}

func (r *TemplateQuestionnaireRepository) LoadQuestionnaire(ctx context.Context, templateID, versionID, tenantID uuid.UUID) (*domain.TemplateQuestionnaireDefinition, error) {
	ver, err := r.GetVersionByID(ctx, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	if ver.TemplateID != templateID {
		return nil, apperrors.NotFound("rfx template version not found")
	}
	sections, err := r.ListSections(ctx, templateID, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	rules, err := r.ListRules(ctx, templateID, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	swq := make([]domain.TemplateSectionWithQuestions, 0, len(sections))
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
		swq = append(swq, domain.TemplateSectionWithQuestions{Section: sec, Questions: questions})
	}
	return &domain.TemplateQuestionnaireDefinition{
		TemplateID:           templateID,
		RfxTemplateVersionID: versionID,
		VersionNumber:        ver.VersionNumber,
		VersionStatus:        ver.Status,
		Sections:             swq,
		Rules:                rules,
	}, nil
}

func (r *TemplateQuestionnaireRepository) GetVersionByID(ctx context.Context, versionID, tenantID uuid.UUID) (*domain.RfxTemplateVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxTemplateVersionSelectColumns+`
		FROM rfx.rfx_template_versions
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, versionID, tenantID)
	ver, err := scanRfxTemplateVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx template version not found")
		}
		return nil, mapDBError(err)
	}
	return ver, nil
}

func (r *TemplateQuestionnaireRepository) TouchDraftVersion(ctx context.Context, versionID, tenantID uuid.UUID, expectedVersion int) (*domain.RfxTemplateVersion, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_template_versions SET updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND status=$3 AND version=$4
		RETURNING `+rfxTemplateVersionSelectColumns,
		versionID, tenantID, domain.RfxVersionStatusDraft, expectedVersion)
	ver, err := scanRfxTemplateVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("draft version was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return ver, nil
}

func (r *TemplateQuestionnaireRepository) CreateSection(ctx context.Context, tenantID, templateID, versionID uuid.UUID, in domain.CreateSectionInput) (*domain.TemplateSection, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_template_sections (tenant_id, template_id, rfx_template_version_id, section_code, title, description, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant_id, template_id, rfx_template_version_id, section_code, title, description, sort_order, created_at, updated_at, version`,
		tenantID, templateID, versionID, strings.TrimSpace(in.SectionCode), strings.TrimSpace(in.Title), optionalString(in.Description), sortOrder)
	sec, err := scanTemplateSection(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return sec, nil
}

func (r *TemplateQuestionnaireRepository) UpdateSection(ctx context.Context, sectionID, tenantID uuid.UUID, in domain.UpdateSectionInput) (*domain.TemplateSection, error) {
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
	q := fmt.Sprintf(`UPDATE rfx.rfx_template_sections SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, template_id, rfx_template_version_id, section_code, title, description, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	sec, err := scanTemplateSection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("section was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return sec, nil
}

func (r *TemplateQuestionnaireRepository) DeleteSection(ctx context.Context, sectionID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_template_sections SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, sectionID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("section was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) ReorderSections(ctx context.Context, tenantID, templateID, versionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_template_sections SET sort_order=$5, updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND template_id=$3 AND rfx_template_version_id=$4 AND deleted_at IS NULL`, id, tenantID, templateID, versionID, i)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("section not found")
		}
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) ListSections(ctx context.Context, templateID, versionID, tenantID uuid.UUID) ([]domain.TemplateSection, error) {
	rows, err := r.db().Query(ctx, `
		SELECT id, tenant_id, template_id, rfx_template_version_id, section_code, title, description, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_sections
		WHERE template_id=$1 AND rfx_template_version_id=$2 AND tenant_id=$3 AND deleted_at IS NULL
		ORDER BY sort_order`, templateID, versionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.TemplateSection, 0)
	for rows.Next() {
		sec, err := scanTemplateSection(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *sec)
	}
	return out, rows.Err()
}

func (r *TemplateQuestionnaireRepository) GetSectionByID(ctx context.Context, sectionID, tenantID uuid.UUID) (*domain.TemplateSection, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, template_id, rfx_template_version_id, section_code, title, description, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_sections WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, sectionID, tenantID)
	sec, err := scanTemplateSection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("section not found")
		}
		return nil, mapDBError(err)
	}
	return sec, nil
}

func (r *TemplateQuestionnaireRepository) AssertSectionBelongsToVersion(ctx context.Context, sectionID, templateID, versionID, tenantID uuid.UUID) error {
	var id uuid.UUID
	err := r.db().QueryRow(ctx, `
		SELECT id FROM rfx.rfx_template_sections
		WHERE id=$1 AND tenant_id=$2 AND template_id=$3 AND rfx_template_version_id=$4 AND deleted_at IS NULL`,
		sectionID, tenantID, templateID, versionID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("section not found")
		}
		return mapDBError(err)
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) CreateQuestion(ctx context.Context, tenantID, sectionID uuid.UUID, in domain.CreateQuestionInput) (*domain.TemplateQuestion, error) {
	sec, err := r.GetSectionByID(ctx, sectionID, tenantID)
	if err != nil {
		return nil, err
	}
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	val := in.ValidationRuleJSON
	if len(val) == 0 {
		val = json.RawMessage(`{}`)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_template_questions (
			tenant_id, template_id, rfx_template_version_id, section_id,
			question_code, question_type, label, help_text, required, validation_rule_json, sort_order
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11)
		RETURNING id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version`,
		tenantID, sec.TemplateID, sec.RfxTemplateVersionID, sectionID,
		strings.TrimSpace(in.QuestionCode), strings.TrimSpace(in.QuestionType), strings.TrimSpace(in.Label),
		optionalString(in.HelpText), in.Required, string(val), sortOrder)
	q, err := scanTemplateQuestion(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return q, nil
}

func (r *TemplateQuestionnaireRepository) UpdateQuestion(ctx context.Context, questionID, tenantID uuid.UUID, in domain.UpdateQuestionInput) (*domain.TemplateQuestion, error) {
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
	q := fmt.Sprintf(`UPDATE rfx.rfx_template_questions SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	question, err := scanTemplateQuestion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("question was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return question, nil
}

func (r *TemplateQuestionnaireRepository) DeleteQuestion(ctx context.Context, questionID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_template_questions SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, questionID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("question was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) GetQuestionByID(ctx context.Context, questionID, tenantID uuid.UUID) (*domain.TemplateQuestion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_questions WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, questionID, tenantID)
	q, err := scanTemplateQuestion(row)
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

func (r *TemplateQuestionnaireRepository) ListQuestionsBySection(ctx context.Context, sectionID, tenantID uuid.UUID) ([]domain.TemplateQuestion, error) {
	rows, err := r.db().Query(ctx, `
		SELECT id, tenant_id, section_id, question_code, question_type, label, help_text, required, validation_rule_json, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_questions WHERE section_id=$1 AND tenant_id=$2 AND deleted_at IS NULL ORDER BY sort_order`, sectionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.TemplateQuestion, 0)
	for rows.Next() {
		q, err := scanTemplateQuestion(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func (r *TemplateQuestionnaireRepository) ReorderQuestions(ctx context.Context, tenantID, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_template_questions SET sort_order=$4, updated_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND section_id=$3 AND deleted_at IS NULL`, id, tenantID, sectionID, i)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("question not found")
		}
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) DuplicateQuestion(ctx context.Context, tenantID, questionID uuid.UUID, newCode string) (*domain.TemplateQuestion, error) {
	source, err := r.GetQuestionByID(ctx, questionID, tenantID)
	if err != nil {
		return nil, err
	}
	created, err := r.CreateQuestion(ctx, tenantID, source.SectionID, domain.CreateQuestionInput{
		QuestionCode: newCode, QuestionType: source.QuestionType, Label: source.Label,
		HelpText: source.HelpText, Required: source.Required, ValidationRuleJSON: source.ValidationRuleJSON,
		SortOrder: intPtr(source.SortOrder + 1),
	})
	if err != nil {
		return nil, err
	}
	for _, opt := range source.Options {
		if _, err := r.CreateOption(ctx, tenantID, created.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.OptionCode, Label: opt.Label, SortOrder: intPtr(opt.SortOrder),
		}); err != nil {
			return nil, err
		}
	}
	return r.GetQuestionByID(ctx, created.ID, tenantID)
}

func (r *TemplateQuestionnaireRepository) AssertQuestionBelongsToVersion(ctx context.Context, questionID, templateID, versionID, tenantID uuid.UUID) error {
	var id uuid.UUID
	err := r.db().QueryRow(ctx, `
		SELECT q.id FROM rfx.rfx_template_questions q
		INNER JOIN rfx.rfx_template_sections s ON s.id = q.section_id AND s.tenant_id = q.tenant_id
		WHERE q.id=$1 AND q.tenant_id=$2 AND s.template_id=$3 AND s.rfx_template_version_id=$4 AND q.deleted_at IS NULL AND s.deleted_at IS NULL`,
		questionID, tenantID, templateID, versionID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("question not found")
		}
		return mapDBError(err)
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) CreateOption(ctx context.Context, tenantID, questionID uuid.UUID, in domain.CreateQuestionOptionInput) (*domain.TemplateQuestionOption, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_template_question_options (tenant_id, question_id, option_code, label, sort_order)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version`,
		tenantID, questionID, strings.TrimSpace(in.OptionCode), strings.TrimSpace(in.Label), sortOrder)
	opt, err := scanTemplateQuestionOption(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return opt, nil
}

func (r *TemplateQuestionnaireRepository) UpdateOption(ctx context.Context, optionID, tenantID uuid.UUID, in domain.UpdateQuestionOptionInput) (*domain.TemplateQuestionOption, error) {
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
	q := fmt.Sprintf(`UPDATE rfx.rfx_template_question_options SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	opt, err := scanTemplateQuestionOption(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("option was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return opt, nil
}

func (r *TemplateQuestionnaireRepository) DeleteOption(ctx context.Context, optionID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_template_question_options SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, optionID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("option was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) GetOptionByID(ctx context.Context, optionID, tenantID uuid.UUID) (*domain.TemplateQuestionOption, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_question_options WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, optionID, tenantID)
	opt, err := scanTemplateQuestionOption(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("option not found")
		}
		return nil, mapDBError(err)
	}
	return opt, nil
}

func (r *TemplateQuestionnaireRepository) ListOptionsByQuestion(ctx context.Context, questionID, tenantID uuid.UUID) ([]domain.TemplateQuestionOption, error) {
	rows, err := r.db().Query(ctx, `
		SELECT id, tenant_id, question_id, option_code, label, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_question_options WHERE question_id=$1 AND tenant_id=$2 AND deleted_at IS NULL ORDER BY sort_order`, questionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.TemplateQuestionOption, 0)
	for rows.Next() {
		opt, err := scanTemplateQuestionOption(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *opt)
	}
	return out, rows.Err()
}

func (r *TemplateQuestionnaireRepository) CreateRule(ctx context.Context, tenantID, templateID, versionID uuid.UUID, targetQuestionID *uuid.UUID, in domain.CreateQuestionRuleInput) (*domain.TemplateQuestionRule, error) {
	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}
	cond := in.ConditionJSON
	if len(cond) == 0 {
		cond = json.RawMessage(`{}`)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_template_question_rules (tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action, condition_json, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8)
		RETURNING id, tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version`,
		tenantID, templateID, versionID, targetQuestionID, strings.TrimSpace(in.RuleCode), strings.TrimSpace(in.Action), string(cond), sortOrder)
	rule, err := scanTemplateQuestionRule(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return rule, nil
}

func (r *TemplateQuestionnaireRepository) UpdateRule(ctx context.Context, ruleID, tenantID uuid.UUID, targetQuestionID *uuid.UUID, in domain.UpdateQuestionRuleInput) (*domain.TemplateQuestionRule, error) {
	sets := []string{"updated_at=now()", "version=version+1"}
	args := []any{ruleID, tenantID, in.ExpectedVersion}
	idx := 4
	if in.Action != nil {
		sets = append(sets, fmt.Sprintf("action=$%d", idx))
		args = append(args, strings.TrimSpace(*in.Action))
		idx++
	}
	sets = append(sets, fmt.Sprintf("target_question_id=$%d", idx))
	args = append(args, targetQuestionID)
	idx++
	if len(in.ConditionJSON) > 0 {
		sets = append(sets, fmt.Sprintf("condition_json=$%d::jsonb", idx))
		args = append(args, string(in.ConditionJSON))
		idx++
	}
	if in.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order=$%d", idx))
		args = append(args, *in.SortOrder)
	}
	q := fmt.Sprintf(`UPDATE rfx.rfx_template_question_rules SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3
		RETURNING id, tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version`, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	rule, err := scanTemplateQuestionRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("rule was modified", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return rule, nil
}

func (r *TemplateQuestionnaireRepository) DeleteRule(ctx context.Context, ruleID, tenantID uuid.UUID, expectedVersion int) error {
	tag, err := r.db().Exec(ctx, `UPDATE rfx.rfx_template_question_rules SET deleted_at=now(), version=version+1 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3`, ruleID, tenantID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("rule was modified or not found", map[string]any{"field": "version"})
	}
	return nil
}

func (r *TemplateQuestionnaireRepository) GetRuleByID(ctx context.Context, ruleID, tenantID uuid.UUID) (*domain.TemplateQuestionRule, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_question_rules WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, ruleID, tenantID)
	rule, err := scanTemplateQuestionRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rule not found")
		}
		return nil, mapDBError(err)
	}
	return rule, nil
}

func (r *TemplateQuestionnaireRepository) ListRules(ctx context.Context, templateID, versionID, tenantID uuid.UUID) ([]domain.TemplateQuestionRule, error) {
	rows, err := r.db().Query(ctx, `
		SELECT id, tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action, condition_json, sort_order, created_at, updated_at, version
		FROM rfx.rfx_template_question_rules
		WHERE template_id=$1 AND rfx_template_version_id=$2 AND tenant_id=$3 AND deleted_at IS NULL ORDER BY sort_order`,
		templateID, versionID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.TemplateQuestionRule, 0)
	for rows.Next() {
		rule, err := scanTemplateQuestionRule(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *rule)
	}
	return out, rows.Err()
}

func (r *TemplateQuestionnaireRepository) GetQuestionIDByCodeInVersion(ctx context.Context, templateID, versionID, tenantID uuid.UUID, questionCode string) (*uuid.UUID, error) {
	var id uuid.UUID
	err := r.db().QueryRow(ctx, `
		SELECT q.id FROM rfx.rfx_template_questions q
		INNER JOIN rfx.rfx_template_sections s ON s.id = q.section_id AND s.tenant_id = q.tenant_id
		WHERE s.template_id=$1 AND s.rfx_template_version_id=$2 AND q.tenant_id=$3 AND q.question_code=$4
			AND q.deleted_at IS NULL AND s.deleted_at IS NULL`,
		templateID, versionID, tenantID, strings.TrimSpace(questionCode)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("question not found")
		}
		return nil, mapDBError(err)
	}
	return &id, nil
}

func scanTemplateSection(row pgx.Row) (*domain.TemplateSection, error) {
	var sec domain.TemplateSection
	if err := row.Scan(&sec.ID, &sec.TenantID, &sec.TemplateID, &sec.RfxTemplateVersionID, &sec.SectionCode, &sec.Title, &sec.Description, &sec.SortOrder, &sec.CreatedAt, &sec.UpdatedAt, &sec.Version); err != nil {
		return nil, err
	}
	return &sec, nil
}

func scanTemplateQuestion(row pgx.Row) (*domain.TemplateQuestion, error) {
	var q domain.TemplateQuestion
	var val []byte
	if err := row.Scan(&q.ID, &q.TenantID, &q.SectionID, &q.QuestionCode, &q.QuestionType, &q.Label, &q.HelpText, &q.Required, &val, &q.SortOrder, &q.CreatedAt, &q.UpdatedAt, &q.Version); err != nil {
		return nil, err
	}
	q.ValidationRuleJSON = val
	return &q, nil
}

func scanTemplateQuestionOption(row pgx.Row) (*domain.TemplateQuestionOption, error) {
	var opt domain.TemplateQuestionOption
	if err := row.Scan(&opt.ID, &opt.TenantID, &opt.QuestionID, &opt.OptionCode, &opt.Label, &opt.SortOrder, &opt.CreatedAt, &opt.UpdatedAt, &opt.Version); err != nil {
		return nil, err
	}
	return &opt, nil
}

func scanTemplateQuestionRule(row pgx.Row) (*domain.TemplateQuestionRule, error) {
	var rule domain.TemplateQuestionRule
	var cond []byte
	if err := row.Scan(&rule.ID, &rule.TenantID, &rule.TemplateID, &rule.RfxTemplateVersionID, &rule.TargetQuestionID, &rule.RuleCode, &rule.Action, &cond, &rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt, &rule.Version); err != nil {
		return nil, err
	}
	rule.ConditionJSON = cond
	return &rule, nil
}
