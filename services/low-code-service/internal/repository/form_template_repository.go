package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type FormTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewFormTemplateRepository(pool *pgxpool.Pool) *FormTemplateRepository {
	return &FormTemplateRepository{pool: pool}
}

func (r *FormTemplateRepository) ListPublished(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
) ([]domain.FormTemplateSummary, error) {
	var items []domain.FormTemplateSummary
	err := measureDB("form_template_repository", "list_published", func() error {
		query := `
			SELECT
				ft.id,
				ft.tenant_id,
				ft.entity_type,
				ft.code,
				ft.name,
				ft.status,
				ft.version,
				ft.published_at,
				(
					SELECT COUNT(*)
					FROM lowcode.form_sections fs
					WHERE fs.form_template_id = ft.id AND fs.tenant_id = ft.tenant_id
				) AS sections_count,
				(
					SELECT COUNT(*)
					FROM lowcode.form_fields ff
					WHERE ff.form_template_id = ft.id AND ff.tenant_id = ft.tenant_id
				) AS fields_count
			FROM lowcode.form_templates ft
			WHERE ft.tenant_id = $1
				AND ft.status = $2
		`
		args := []any{tenantID, domain.PublishedStatus}
		if entityType != "" {
			query += " AND ft.entity_type = $3"
			args = append(args, entityType)
		}
		query += " ORDER BY ft.entity_type, ft.code, ft.version DESC"

		rows, err := r.pool.Query(ctx, query, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items = make([]domain.FormTemplateSummary, 0)
		for rows.Next() {
			var item domain.FormTemplateSummary
			if err := rows.Scan(
				&item.ID,
				&item.TenantID,
				&item.EntityType,
				&item.Code,
				&item.Name,
				&item.Status,
				&item.Version,
				&item.PublishedAt,
				&item.SectionsCount,
				&item.FieldsCount,
			); err != nil {
				return mapDBError(err)
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *FormTemplateRepository) GetPublishedByID(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*domain.FormTemplateDetail, error) {
	var detail *domain.FormTemplateDetail
	err := measureDB("form_template_repository", "get_published_by_id", func() error {
		var err error
		detail, err = r.loadPublishedTemplate(ctx, tenantID, "ft.id = $2", templateID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *FormTemplateRepository) GetPublishedByCode(
	ctx context.Context,
	tenantID uuid.UUID,
	code string,
) (*domain.FormTemplateDetail, error) {
	var detail *domain.FormTemplateDetail
	err := measureDB("form_template_repository", "get_published_by_code", func() error {
		var err error
		detail, err = r.loadPublishedTemplate(ctx, tenantID, "ft.code = $2", code)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *FormTemplateRepository) loadPublishedTemplate(
	ctx context.Context,
	tenantID uuid.UUID,
	matchClause string,
	matchValue any,
) (*domain.FormTemplateDetail, error) {
	query := fmt.Sprintf(`
		SELECT
			ft.id,
			ft.tenant_id,
			ft.entity_type,
			ft.code,
			ft.name,
			ft.status,
			ft.version,
			ft.published_at
		FROM lowcode.form_templates ft
		WHERE ft.tenant_id = $1
			AND ft.status = '%s'
			AND %s
		ORDER BY ft.version DESC
		LIMIT 1
	`, domain.PublishedStatus, matchClause)

	var detail domain.FormTemplateDetail
	err := r.pool.QueryRow(ctx, query, tenantID, matchValue).Scan(
		&detail.ID,
		&detail.TenantID,
		&detail.EntityType,
		&detail.Code,
		&detail.Name,
		&detail.Status,
		&detail.Version,
		&detail.PublishedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}

	sections, err := r.loadSections(ctx, tenantID, detail.ID)
	if err != nil {
		return nil, err
	}
	detail.Sections = sections
	return &detail, nil
}

func (r *FormTemplateRepository) loadSections(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) ([]domain.FormSection, error) {
	const sectionQuery = `
		SELECT id, code, title, sort_order
		FROM lowcode.form_sections
		WHERE tenant_id = $1 AND form_template_id = $2
		ORDER BY sort_order, code
	`
	rows, err := r.pool.Query(ctx, sectionQuery, tenantID, templateID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	sections := make([]domain.FormSection, 0)
	sectionIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var section domain.FormSection
		if err := rows.Scan(&section.ID, &section.Code, &section.Title, &section.SortOrder); err != nil {
			return nil, mapDBError(err)
		}
		section.Fields = []domain.FormField{}
		sections = append(sections, section)
		sectionIDs = append(sectionIDs, section.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}

	fieldsBySection, err := r.loadFieldsBySection(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}

	for i := range sections {
		sections[i].Fields = fieldsBySection[sections[i].ID]
		if sections[i].Fields == nil {
			sections[i].Fields = []domain.FormField{}
		}
	}

	return sections, nil
}

func (r *FormTemplateRepository) loadFieldsBySection(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (map[uuid.UUID][]domain.FormField, error) {
	const fieldQuery = `
		SELECT
			id,
			section_id,
			code,
			label,
			field_type,
			required,
			read_only,
			system_field,
			options_json,
			validation_rule_json,
			visibility_rule_json,
			sort_order
		FROM lowcode.form_fields
		WHERE tenant_id = $1 AND form_template_id = $2
		ORDER BY sort_order, code
	`
	rows, err := r.pool.Query(ctx, fieldQuery, tenantID, templateID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]domain.FormField)
	for rows.Next() {
		var field domain.FormField
		var sectionID *uuid.UUID
		var optionsJSON, validationJSON, visibilityJSON []byte
		if err := rows.Scan(
			&field.ID,
			&sectionID,
			&field.Code,
			&field.Label,
			&field.FieldType,
			&field.Required,
			&field.ReadOnly,
			&field.SystemField,
			&optionsJSON,
			&validationJSON,
			&visibilityJSON,
			&field.SortOrder,
		); err != nil {
			return nil, mapDBError(err)
		}
		field.OptionsJSON = normalizeJSON(optionsJSON)
		field.ValidationRuleJSON = normalizeJSON(validationJSON)
		field.VisibilityRuleJSON = normalizeJSON(visibilityJSON)
		if sectionID != nil {
			result[*sectionID] = append(result[*sectionID], field)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return result, nil
}

func normalizeJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func (r *FormTemplateRepository) GetPublishedTemplateContext(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*domain.PublishedTemplateContext, error) {
	var result *domain.PublishedTemplateContext
	err := measureDB("form_template_repository", "get_published_template_context", func() error {
		const query = `
			SELECT id, tenant_id, entity_type, status
			FROM lowcode.form_templates
			WHERE id = $1 AND tenant_id = $2
		`
		var tmpl domain.PublishedTemplateContext
		if err := r.pool.QueryRow(ctx, query, templateID, tenantID).Scan(
			&tmpl.ID, &tmpl.TenantID, &tmpl.EntityType, &tmpl.Status,
		); err != nil {
			return mapDBError(err)
		}
		if tmpl.TenantID != tenantID {
			return apperrors.TenantMismatch()
		}
		if tmpl.Status != domain.PublishedStatus {
			return apperrors.FormTemplateNotPublished()
		}

		fields, err := r.loadFieldDefinitions(ctx, tenantID, templateID)
		if err != nil {
			return err
		}
		tmpl.Fields = fields
		result = &tmpl
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *FormTemplateRepository) loadFieldDefinitions(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (map[string]domain.FieldDefinition, error) {
	const query = `
		SELECT id, code, field_type, required, system_field, options_json, validation_rule_json
		FROM lowcode.form_fields
		WHERE tenant_id = $1 AND form_template_id = $2
	`
	rows, err := r.pool.Query(ctx, query, tenantID, templateID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	fields := make(map[string]domain.FieldDefinition)
	for rows.Next() {
		var field domain.FieldDefinition
		var optionsJSON, validationJSON []byte
		if err := rows.Scan(
			&field.ID, &field.Code, &field.FieldType, &field.Required, &field.SystemField,
			&optionsJSON, &validationJSON,
		); err != nil {
			return nil, mapDBError(err)
		}
		field.OptionsJSON = normalizeJSON(optionsJSON)
		field.ValidationRuleJSON = normalizeJSON(validationJSON)
		fields[field.Code] = field
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return fields, nil
}
