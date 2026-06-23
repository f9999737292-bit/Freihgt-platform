package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type AdminFormTemplateRepository struct {
	pool      *pgxpool.Pool
	auditRepo *ConfigurationAuditRepository
}

func NewAdminFormTemplateRepository(pool *pgxpool.Pool, auditRepo *ConfigurationAuditRepository) *AdminFormTemplateRepository {
	return &AdminFormTemplateRepository{pool: pool, auditRepo: auditRepo}
}

type CreateDraftResult struct {
	ID            uuid.UUID
	ConfigurationID uuid.UUID
	Status        string
	Version       int
}

type AdminListFilter struct {
	TenantID   uuid.UUID
	EntityType string
	Status     string
	Limit      int
}

type UpdateDraftInput struct {
	TenantID    uuid.UUID
	TemplateID  uuid.UUID
	Name        string
	Description string
	Sections    []domain.DraftFormSectionInput
	Audit       domain.AuditContext
}

type CreateDraftInput struct {
	TenantID    uuid.UUID
	EntityType  string
	Code        string
	Name        string
	Description string
	Sections    []domain.DraftFormSectionInput
	Audit       domain.AuditContext
}

func (r *AdminFormTemplateRepository) CreateDraft(
	ctx context.Context,
	input CreateDraftInput,
) (*CreateDraftResult, error) {
	var result *CreateDraftResult
	err := measureDB("admin_form_template_repository", "create_draft", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		configurationID := uuid.New()
		templateID := uuid.New()
		configCode := fmt.Sprintf("cfg_%s", input.Code)

		const configQuery = `
			INSERT INTO lowcode.low_code_configurations (
				id, tenant_id, code, name, description, config_type, status, version, created_by_user_id, updated_by_user_id
			) VALUES ($1, $2, $3, $4, $5, 'FORM_TEMPLATE', $6, 1, $7, $7)
		`
		if _, err := tx.Exec(ctx, configQuery,
			configurationID,
			input.TenantID,
			configCode,
			input.Name,
			nullIfEmptyString(input.Description),
			domain.DraftStatus,
			input.Audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		const templateQuery = `
			INSERT INTO lowcode.form_templates (
				id, tenant_id, configuration_id, entity_type, code, name, description, status, version,
				created_by_user_id, updated_by_user_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, $9)
		`
		if _, err := tx.Exec(ctx, templateQuery,
			templateID,
			input.TenantID,
			configurationID,
			input.EntityType,
			input.Code,
			input.Name,
			nullIfEmptyString(input.Description),
			domain.DraftStatus,
			input.Audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		if err := r.insertSectionsAndFields(ctx, tx, input.TenantID, templateID, input.Sections); err != nil {
			return err
		}

		if err := r.insertTemplateAudit(ctx, tx, domain.AuditDBActionCreate, domain.AuditEventKindFormTemplateDraftCreated, nil,
			input.TenantID, configurationID, templateID, input.EntityType, input.Code, input.Sections, input.Audit); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		result = &CreateDraftResult{
			ID:              templateID,
			ConfigurationID: configurationID,
			Status:          domain.DraftStatus,
			Version:         1,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *AdminFormTemplateRepository) ListAdmin(
	ctx context.Context,
	filter AdminListFilter,
) ([]domain.FormTemplateSummary, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var items []domain.FormTemplateSummary
	err := measureDB("admin_form_template_repository", "list_admin", func() error {
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
		`
		args := []any{filter.TenantID}
		argIndex := 2
		if filter.EntityType != "" {
			query += fmt.Sprintf(" AND ft.entity_type = $%d", argIndex)
			args = append(args, filter.EntityType)
			argIndex++
		}
		if filter.Status != "" {
			query += fmt.Sprintf(" AND ft.status = $%d", argIndex)
			args = append(args, filter.Status)
			argIndex++
		}
		query += fmt.Sprintf(" ORDER BY ft.updated_at DESC, ft.code LIMIT $%d", argIndex)
		args = append(args, limit)

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
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AdminFormTemplateRepository) GetByID(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*domain.FormTemplateDetail, error) {
	var detail *domain.FormTemplateDetail
	err := measureDB("admin_form_template_repository", "get_by_id", func() error {
		var err error
		detail, err = r.loadTemplateDetail(ctx, tenantID, templateID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *AdminFormTemplateRepository) UpdateDraft(
	ctx context.Context,
	input UpdateDraftInput,
) error {
	return measureDB("admin_form_template_repository", "update_draft", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		meta, err := r.loadTemplateMetaInTx(ctx, tx, input.TenantID, input.TemplateID)
		if err != nil {
			return err
		}
		if meta.Status != domain.DraftStatus {
			return apperrors.FormTemplateNotDraft(meta.Status)
		}

		const updateQuery = `
			UPDATE lowcode.form_templates
			SET name = $3,
			    description = $4,
			    updated_by_user_id = $5,
			    updated_at = now()
			WHERE id = $1 AND tenant_id = $2
		`
		if _, err := tx.Exec(ctx, updateQuery,
			input.TemplateID,
			input.TenantID,
			input.Name,
			nullIfEmptyString(input.Description),
			input.Audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		const updateConfigQuery = `
			UPDATE lowcode.low_code_configurations
			SET name = $3,
			    description = $4,
			    updated_by_user_id = $5,
			    updated_at = now()
			WHERE id = $1 AND tenant_id = $2
		`
		if _, err := tx.Exec(ctx, updateConfigQuery,
			meta.ConfigurationID,
			input.TenantID,
			input.Name,
			nullIfEmptyString(input.Description),
			input.Audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		if _, err := tx.Exec(ctx, `
			DELETE FROM lowcode.form_fields
			WHERE tenant_id = $1 AND form_template_id = $2
		`, input.TenantID, input.TemplateID); err != nil {
			return mapDBError(err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM lowcode.form_sections
			WHERE tenant_id = $1 AND form_template_id = $2
		`, input.TenantID, input.TemplateID); err != nil {
			return mapDBError(err)
		}

		if err := r.insertSectionsAndFields(ctx, tx, input.TenantID, input.TemplateID, input.Sections); err != nil {
			return err
		}

		if err := r.insertTemplateAudit(ctx, tx, domain.AuditDBActionUpdate, domain.AuditEventKindFormTemplateDraftUpdated, nil,
			input.TenantID, meta.ConfigurationID, input.TemplateID, meta.EntityType, meta.Code, input.Sections, input.Audit); err != nil {
			return err
		}

		return mapDBError(tx.Commit(ctx))
	})
}

func (r *AdminFormTemplateRepository) PublishDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	audit domain.AuditContext,
) (*domain.FormTemplateDetail, error) {
	var detail *domain.FormTemplateDetail
	err := measureDB("admin_form_template_repository", "publish_draft", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		meta, err := r.loadTemplateMetaInTx(ctx, tx, tenantID, templateID)
		if err != nil {
			return err
		}
		if meta.Status != domain.DraftStatus {
			return apperrors.FormTemplateNotDraft(meta.Status)
		}

		const publishTemplateQuery = `
			UPDATE lowcode.form_templates
			SET status = $3,
			    published_at = now(),
			    updated_by_user_id = $4,
			    updated_at = now()
			WHERE id = $1 AND tenant_id = $2
		`
		if _, err := tx.Exec(ctx, publishTemplateQuery,
			templateID, tenantID, domain.PublishedStatus, audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		const publishConfigQuery = `
			UPDATE lowcode.low_code_configurations
			SET status = $3,
			    published_at = now(),
			    updated_by_user_id = $4,
			    updated_at = now()
			WHERE id = $1 AND tenant_id = $2
		`
		if _, err := tx.Exec(ctx, publishConfigQuery,
			meta.ConfigurationID, tenantID, domain.PublishedStatus, audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		sections, err := r.loadSectionsForAudit(ctx, tx, tenantID, templateID)
		if err != nil {
			return err
		}
		if err := r.insertTemplateAudit(ctx, tx, domain.AuditDBActionPublish, domain.AuditEventKindFormTemplateDraftPublished, nil,
			tenantID, meta.ConfigurationID, templateID, meta.EntityType, meta.Code, sections, audit); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		detail, err = r.loadTemplateDetail(ctx, tenantID, templateID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

type templateMeta struct {
	ConfigurationID uuid.UUID
	EntityType      string
	Code            string
	Status          string
}

func (r *AdminFormTemplateRepository) loadTemplateMetaInTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*templateMeta, error) {
	const query = `
		SELECT configuration_id, entity_type, code, status
		FROM lowcode.form_templates
		WHERE id = $1 AND tenant_id = $2
	`
	var meta templateMeta
	if err := tx.QueryRow(ctx, query, templateID, tenantID).Scan(
		&meta.ConfigurationID, &meta.EntityType, &meta.Code, &meta.Status,
	); err != nil {
		return nil, mapDBError(err)
	}
	return &meta, nil
}

func (r *AdminFormTemplateRepository) loadTemplateDetail(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*domain.FormTemplateDetail, error) {
	const query = `
		SELECT id, tenant_id, entity_type, code, name, description, status, version, published_at
		FROM lowcode.form_templates
		WHERE id = $1 AND tenant_id = $2
	`
	var detail domain.FormTemplateDetail
	var description *string
	if err := r.pool.QueryRow(ctx, query, templateID, tenantID).Scan(
		&detail.ID,
		&detail.TenantID,
		&detail.EntityType,
		&detail.Code,
		&detail.Name,
		&description,
		&detail.Status,
		&detail.Version,
		&detail.PublishedAt,
	); err != nil {
		return nil, mapDBError(err)
	}
	if description != nil {
		detail.Description = *description
	}

	baseRepo := &FormTemplateRepository{pool: r.pool}
	sections, err := baseRepo.loadSections(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	detail.Sections = sections
	return &detail, nil
}

func (r *AdminFormTemplateRepository) insertSectionsAndFields(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	sections []domain.DraftFormSectionInput,
) error {
	const sectionQuery = `
		INSERT INTO lowcode.form_sections (
			id, tenant_id, form_template_id, code, title, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	const fieldQuery = `
		INSERT INTO lowcode.form_fields (
			id, tenant_id, form_template_id, section_id, code, label, field_type,
			required, read_only, system_field, options_json, validation_rule_json, visibility_rule_json, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	for _, section := range sections {
		sectionID := uuid.New()
		if _, err := tx.Exec(ctx, sectionQuery,
			sectionID,
			tenantID,
			templateID,
			strings.TrimSpace(section.Code),
			strings.TrimSpace(section.Title),
			section.SortOrder,
		); err != nil {
			return mapDBError(err)
		}

		for _, field := range section.Fields {
			if _, err := tx.Exec(ctx, fieldQuery,
				uuid.New(),
				tenantID,
				templateID,
				sectionID,
				strings.TrimSpace(field.Code),
				strings.TrimSpace(field.Label),
				strings.TrimSpace(field.FieldType),
				field.Required,
				field.ReadOnly,
				field.SystemField,
				nullJSON(field.OptionsJSON),
				nullJSON(field.ValidationRuleJSON),
				nullJSON(field.VisibilityRuleJSON),
				field.SortOrder,
			); err != nil {
				return mapDBError(err)
			}
		}
	}
	return nil
}

func (r *AdminFormTemplateRepository) loadSectionsForAudit(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) ([]domain.DraftFormSectionInput, error) {
	const sectionQuery = `
		SELECT code, title, sort_order
		FROM lowcode.form_sections
		WHERE tenant_id = $1 AND form_template_id = $2
		ORDER BY sort_order, code
	`
	rows, err := tx.Query(ctx, sectionQuery, tenantID, templateID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	sections := make([]domain.DraftFormSectionInput, 0)
	for rows.Next() {
		var section domain.DraftFormSectionInput
		if err := rows.Scan(&section.Code, &section.Title, &section.SortOrder); err != nil {
			return nil, mapDBError(err)
		}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}

	for i := range sections {
		const fieldQuery = `
			SELECT code, label, field_type, required, read_only, system_field, sort_order
			FROM lowcode.form_fields
			WHERE tenant_id = $1 AND form_template_id = $2
			  AND section_id IN (
			    SELECT id FROM lowcode.form_sections
			    WHERE tenant_id = $1 AND form_template_id = $2 AND code = $3
			  )
			ORDER BY sort_order, code
		`
		fieldRows, err := tx.Query(ctx, fieldQuery, tenantID, templateID, sections[i].Code)
		if err != nil {
			return nil, mapDBError(err)
		}
		fields := make([]domain.DraftFormFieldInput, 0)
		for fieldRows.Next() {
			var field domain.DraftFormFieldInput
			if err := fieldRows.Scan(
				&field.Code, &field.Label, &field.FieldType,
				&field.Required, &field.ReadOnly, &field.SystemField, &field.SortOrder,
			); err != nil {
				fieldRows.Close()
				return nil, mapDBError(err)
			}
			fields = append(fields, field)
		}
		fieldRows.Close()
		if err := fieldRows.Err(); err != nil {
			return nil, mapDBError(err)
		}
		sections[i].Fields = fields
	}

	return sections, nil
}

func (r *AdminFormTemplateRepository) insertTemplateAudit(
	ctx context.Context,
	tx pgx.Tx,
	dbAction string,
	eventKind string,
	oldJSON []byte,
	tenantID uuid.UUID,
	configurationID uuid.UUID,
	templateID uuid.UUID,
	entityType string,
	code string,
	sections []domain.DraftFormSectionInput,
	audit domain.AuditContext,
) error {
	if r.auditRepo == nil {
		return nil
	}

	sectionCodes := domain.CollectSectionCodes(sections)
	fieldCodes := domain.CollectFieldCodes(sections)
	newJSON, err := domain.BuildFormTemplateDraftAuditPayload(
		eventKind, templateID, entityType, code, sectionCodes, fieldCodes,
	)
	if err != nil {
		return apperrors.Internal("failed to build audit payload", err)
	}

	configID := configurationID
	entry := domain.ConfigurationAuditEntry{
		TenantID:        tenantID,
		ConfigurationID: &configID,
		EntityType:      entityType,
		EntityID:        templateID,
		Action:          dbAction,
		OldValueJSON:    oldJSON,
		NewValueJSON:    newJSON,
		ChangedByUserID: audit.ChangedByUserID,
		RequestID:       audit.RequestID,
		IPAddress:       audit.IPAddress,
		UserAgent:       audit.UserAgent,
	}
	return r.auditRepo.InsertInTx(ctx, tx, entry)
}

type ClonePublishedToDraftResult struct {
	ID               uuid.UUID
	SourceTemplateID uuid.UUID
	Status           string
	Version          int
	Code             string
}

func (r *AdminFormTemplateRepository) ClonePublishedToDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	sourceTemplateID uuid.UUID,
	audit domain.AuditContext,
) (*ClonePublishedToDraftResult, error) {
	var result *ClonePublishedToDraftResult
	err := measureDB("admin_form_template_repository", "clone_published_to_draft", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		source, err := r.loadTemplateDetailInTx(ctx, tx, tenantID, sourceTemplateID)
		if err != nil {
			return err
		}
		if source.Status != domain.PublishedStatus {
			return apperrors.FormTemplateCloneSourceNotPublished(source.Status)
		}

		draftCode, draftVersion, err := r.resolveCloneDraftCodeAndVersion(ctx, tx, tenantID, source.EntityType, source.Code)
		if err != nil {
			return err
		}

		configurationID := uuid.New()
		draftTemplateID := uuid.New()
		configCode := fmt.Sprintf("cfg_%s", draftCode)

		const configQuery = `
			INSERT INTO lowcode.low_code_configurations (
				id, tenant_id, code, name, description, config_type, status, version, created_by_user_id, updated_by_user_id
			) VALUES ($1, $2, $3, $4, $5, 'FORM_TEMPLATE', $6, $7, $8, $8)
		`
		if _, err := tx.Exec(ctx, configQuery,
			configurationID,
			tenantID,
			configCode,
			source.Name,
			nullIfEmptyString(source.Description),
			domain.DraftStatus,
			draftVersion,
			audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		const templateQuery = `
			INSERT INTO lowcode.form_templates (
				id, tenant_id, configuration_id, entity_type, code, name, description, status, version,
				created_by_user_id, updated_by_user_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		`
		if _, err := tx.Exec(ctx, templateQuery,
			draftTemplateID,
			tenantID,
			configurationID,
			source.EntityType,
			draftCode,
			source.Name,
			nullIfEmptyString(source.Description),
			domain.DraftStatus,
			draftVersion,
			audit.ChangedByUserID,
		); err != nil {
			return mapDBError(err)
		}

		if err := r.insertSectionsAndFieldsFromTemplate(ctx, tx, tenantID, draftTemplateID, source.Sections); err != nil {
			return err
		}

		sectionsCount := len(source.Sections)
		fieldsCount := 0
		for _, section := range source.Sections {
			fieldsCount += len(section.Fields)
		}

		if err := r.insertCloneAudit(ctx, tx, tenantID, configurationID, source, draftTemplateID, draftCode, draftVersion, sectionsCount, fieldsCount, audit); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		result = &ClonePublishedToDraftResult{
			ID:               draftTemplateID,
			SourceTemplateID: sourceTemplateID,
			Status:           domain.DraftStatus,
			Version:          draftVersion,
			Code:             draftCode,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *AdminFormTemplateRepository) resolveCloneDraftCodeAndVersion(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	entityType string,
	sourceCode string,
) (string, int, error) {
	const versionQuery = `
		SELECT COALESCE(MAX(version), 0)
		FROM lowcode.form_templates
		WHERE tenant_id = $1 AND entity_type = $2 AND code = $3
	`
	var maxVersion int
	if err := tx.QueryRow(ctx, versionQuery, tenantID, entityType, sourceCode).Scan(&maxVersion); err != nil {
		return "", 0, mapDBError(err)
	}
	nextVersion := maxVersion + 1
	return sourceCode, nextVersion, nil
}

func (r *AdminFormTemplateRepository) loadTemplateDetailInTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*domain.FormTemplateDetail, error) {
	const query = `
		SELECT id, tenant_id, entity_type, code, name, description, status, version, published_at
		FROM lowcode.form_templates
		WHERE id = $1 AND tenant_id = $2
	`
	var detail domain.FormTemplateDetail
	var description *string
	if err := tx.QueryRow(ctx, query, templateID, tenantID).Scan(
		&detail.ID,
		&detail.TenantID,
		&detail.EntityType,
		&detail.Code,
		&detail.Name,
		&description,
		&detail.Status,
		&detail.Version,
		&detail.PublishedAt,
	); err != nil {
		return nil, mapDBError(err)
	}
	if description != nil {
		detail.Description = *description
	}

	sections, err := r.loadSectionsInTx(ctx, tx, tenantID, templateID)
	if err != nil {
		return nil, err
	}
	detail.Sections = sections
	return &detail, nil
}

func (r *AdminFormTemplateRepository) loadSectionsInTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) ([]domain.FormSection, error) {
	const sectionQuery = `
		SELECT id, code, title, sort_order
		FROM lowcode.form_sections
		WHERE tenant_id = $1 AND form_template_id = $2
		ORDER BY sort_order, code
	`
	rows, err := tx.Query(ctx, sectionQuery, tenantID, templateID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	sections := make([]domain.FormSection, 0)
	for rows.Next() {
		var section domain.FormSection
		if err := rows.Scan(&section.ID, &section.Code, &section.Title, &section.SortOrder); err != nil {
			return nil, mapDBError(err)
		}
		section.Fields = []domain.FormField{}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}

	const fieldQuery = `
		SELECT
			section_id, code, label, field_type, required, read_only, system_field,
			options_json, validation_rule_json, visibility_rule_json, sort_order
		FROM lowcode.form_fields
		WHERE tenant_id = $1 AND form_template_id = $2
		ORDER BY sort_order, code
	`
	fieldRows, err := tx.Query(ctx, fieldQuery, tenantID, templateID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer fieldRows.Close()

	fieldsBySection := make(map[uuid.UUID][]domain.FormField)
	for fieldRows.Next() {
		var sectionID uuid.UUID
		var field domain.FormField
		var optionsJSON, validationJSON, visibilityJSON []byte
		if err := fieldRows.Scan(
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
		fieldsBySection[sectionID] = append(fieldsBySection[sectionID], field)
	}
	if err := fieldRows.Err(); err != nil {
		return nil, mapDBError(err)
	}

	for i := range sections {
		sections[i].Fields = fieldsBySection[sections[i].ID]
		if sections[i].Fields == nil {
			sections[i].Fields = []domain.FormField{}
		}
	}
	return sections, nil
}

func (r *AdminFormTemplateRepository) insertSectionsAndFieldsFromTemplate(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	sections []domain.FormSection,
) error {
	const sectionQuery = `
		INSERT INTO lowcode.form_sections (
			id, tenant_id, form_template_id, code, title, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	const fieldQuery = `
		INSERT INTO lowcode.form_fields (
			id, tenant_id, form_template_id, section_id, code, label, field_type,
			required, read_only, system_field, options_json, validation_rule_json, visibility_rule_json, sort_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	for _, section := range sections {
		sectionID := uuid.New()
		if _, err := tx.Exec(ctx, sectionQuery,
			sectionID,
			tenantID,
			templateID,
			section.Code,
			section.Title,
			section.SortOrder,
		); err != nil {
			return mapDBError(err)
		}

		for _, field := range section.Fields {
			if _, err := tx.Exec(ctx, fieldQuery,
				uuid.New(),
				tenantID,
				templateID,
				sectionID,
				field.Code,
				field.Label,
				field.FieldType,
				field.Required,
				field.ReadOnly,
				field.SystemField,
				nullJSON(field.OptionsJSON),
				nullJSON(field.ValidationRuleJSON),
				nullJSON(field.VisibilityRuleJSON),
				field.SortOrder,
			); err != nil {
				return mapDBError(err)
			}
		}
	}
	return nil
}

func (r *AdminFormTemplateRepository) insertCloneAudit(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	configurationID uuid.UUID,
	source *domain.FormTemplateDetail,
	draftTemplateID uuid.UUID,
	draftCode string,
	draftVersion int,
	sectionsCount int,
	fieldsCount int,
	audit domain.AuditContext,
) error {
	if r.auditRepo == nil {
		return nil
	}

	newJSON, err := domain.BuildFormTemplateClonedAuditPayload(
		source.ID,
		draftTemplateID,
		source.EntityType,
		source.Code,
		draftCode,
		source.Version,
		draftVersion,
		sectionsCount,
		fieldsCount,
	)
	if err != nil {
		return apperrors.Internal("failed to build clone audit payload", err)
	}

	configID := configurationID
	entry := domain.ConfigurationAuditEntry{
		TenantID:        tenantID,
		ConfigurationID: &configID,
		EntityType:      source.EntityType,
		EntityID:        draftTemplateID,
		Action:          domain.AuditDBActionCreate,
		NewValueJSON:    newJSON,
		ChangedByUserID: audit.ChangedByUserID,
		RequestID:       audit.RequestID,
		IPAddress:       audit.IPAddress,
		UserAgent:       audit.UserAgent,
	}
	return r.auditRepo.InsertInTx(ctx, tx, entry)
}

func (r *AdminFormTemplateRepository) RecordTemplateExport(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	detail domain.FormTemplateDetail,
	audit domain.AuditContext,
	schemaVersion string,
) error {
	return measureDB("admin_form_template_repository", "record_template_export", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		meta, err := r.loadTemplateMetaInTx(ctx, tx, tenantID, templateID)
		if err != nil {
			return err
		}

		if r.auditRepo != nil {
			newJSON, err := domain.BuildFormTemplateExportedAuditPayload(
				templateID,
				detail.Code,
				detail.Version,
				detail.Status,
				schemaVersion,
			)
			if err != nil {
				return apperrors.Internal("failed to build export audit payload", err)
			}

			configID := meta.ConfigurationID
			entry := domain.ConfigurationAuditEntry{
				TenantID:        tenantID,
				ConfigurationID: &configID,
				EntityType:      detail.EntityType,
				EntityID:        templateID,
				Action:          domain.AuditDBActionTest,
				NewValueJSON:    newJSON,
				ChangedByUserID: audit.ChangedByUserID,
				RequestID:       audit.RequestID,
				IPAddress:       audit.IPAddress,
				UserAgent:       audit.UserAgent,
			}
			if err := r.auditRepo.InsertInTx(ctx, tx, entry); err != nil {
				return err
			}
		}

		return mapDBError(tx.Commit(ctx))
	})
}

func nullIfEmptyString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
