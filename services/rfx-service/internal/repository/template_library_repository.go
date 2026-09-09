package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type TemplateLibraryRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

const rfxTemplateSelectColumns = `
	id, tenant_id, template_code, name_i18n_json, description_i18n_json, rfx_type,
	owner_company_id, status, version, created_by, created_at, updated_at
`

const rfxTemplateVersionSelectColumns = `
	id, tenant_id, template_id, version_number, status, published_at, published_by,
	change_summary, version, created_by, created_at, updated_at
`

func NewTemplateLibraryRepository(pool *pgxpool.Pool) *TemplateLibraryRepository {
	return &TemplateLibraryRepository{pool: pool}
}

func (r *TemplateLibraryRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *TemplateLibraryRepository) WithTx(tx pgx.Tx) *TemplateLibraryRepository {
	return &TemplateLibraryRepository{pool: r.pool, exec: tx}
}

func (r *TemplateLibraryRepository) CreateTemplateWithDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	in domain.CreateTemplateInput,
	ownerCompanyID *uuid.UUID,
	createdBy uuid.UUID,
	nameI18n, descriptionI18n []byte,
) (*domain.RfxTemplate, *domain.RfxTemplateVersion, error) {
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_templates (
			tenant_id, template_code, name_i18n_json, description_i18n_json, rfx_type,
			owner_company_id, status, created_by
		) VALUES ($1,$2,$3::jsonb,$4::jsonb,$5,$6,$7,$8)
		RETURNING `+rfxTemplateSelectColumns,
		tenantID, strings.TrimSpace(in.TemplateCode), string(nameI18n), templateNullableJSON(descriptionI18n),
		optionalString(in.RfxType), ownerCompanyID, domain.RfxTemplateStatusActive, createdBy)
	tmpl, err := scanRfxTemplate(row)
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	verRow := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_template_versions (
			tenant_id, template_id, version_number, status, created_by
		) VALUES ($1,$2,1,$3,$4)
		RETURNING `+rfxTemplateVersionSelectColumns,
		tenantID, tmpl.ID, domain.RfxVersionStatusDraft, createdBy)
	ver, err := scanRfxTemplateVersion(verRow)
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	ver.IsActiveDraft = true
	return tmpl, ver, nil
}

func (r *TemplateLibraryRepository) GetTemplateByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxTemplate, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxTemplateSelectColumns+`
		FROM rfx.rfx_templates
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, id, tenantID)
	tmpl, err := scanRfxTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx template not found")
		}
		return nil, mapDBError(err)
	}
	return tmpl, nil
}

func (r *TemplateLibraryRepository) LockTemplateByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxTemplate, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxTemplateSelectColumns+`
		FROM rfx.rfx_templates
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
		FOR UPDATE`, id, tenantID)
	tmpl, err := scanRfxTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx template not found")
		}
		return nil, mapDBError(err)
	}
	return tmpl, nil
}

func (r *TemplateLibraryRepository) UpdateTemplate(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateTemplateInput, nameI18n, descriptionI18n []byte) (*domain.RfxTemplate, error) {
	sets := []string{"updated_at=now()", "version=version+1"}
	args := []any{id, tenantID, in.ExpectedVersion, domain.RfxTemplateStatusActive}
	idx := 5
	if len(nameI18n) > 0 {
		sets = append(sets, fmt.Sprintf("name_i18n_json=$%d::jsonb", idx))
		args = append(args, string(nameI18n))
		idx++
	}
	if len(descriptionI18n) > 0 {
		sets = append(sets, fmt.Sprintf("description_i18n_json=$%d::jsonb", idx))
		args = append(args, string(descriptionI18n))
		idx++
	}
	if in.RfxType != nil {
		sets = append(sets, fmt.Sprintf("rfx_type=$%d", idx))
		args = append(args, optionalString(in.RfxType))
	}
	q := fmt.Sprintf(`UPDATE rfx.rfx_templates SET %s WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND version=$3 AND status=$4
		RETURNING `+rfxTemplateSelectColumns, strings.Join(sets, ","))
	row := r.db().QueryRow(ctx, q, args...)
	tmpl, err := scanRfxTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("template was modified or is not editable", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return tmpl, nil
}

func (r *TemplateLibraryRepository) ArchiveTemplate(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxTemplate, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_templates
		SET status=$3, updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND status=$4
		RETURNING `+rfxTemplateSelectColumns,
		id, tenantID, domain.RfxTemplateStatusArchived, domain.RfxTemplateStatusActive)
	tmpl, err := scanRfxTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("template is not archivable", map[string]any{"field": "status"})
		}
		return nil, mapDBError(err)
	}
	return tmpl, nil
}

func (r *TemplateLibraryRepository) SoftDeleteTemplate(ctx context.Context, id, tenantID uuid.UUID) error {
	var publishedCount int
	if err := r.db().QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_versions
		WHERE template_id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			AND status = ANY($3::text[])`, id, tenantID, []string{domain.RfxVersionStatusPublished, domain.RfxVersionStatusSuperseded}).Scan(&publishedCount); err != nil {
		return mapDBError(err)
	}
	if publishedCount > 0 {
		return apperrors.Conflict("template with published versions cannot be deleted", map[string]any{"field": "template_id"})
	}
	tag, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_templates SET deleted_at=now(), updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("rfx template not found")
	}
	_, err = r.db().Exec(ctx, `
		UPDATE rfx.rfx_template_versions SET deleted_at=now(), updated_at=now()
		WHERE template_id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, id, tenantID)
	return mapDBError(err)
}

func (r *TemplateLibraryRepository) ListTemplates(ctx context.Context, tenantID uuid.UUID, filter domain.TemplateListFilter) ([]domain.RfxTemplate, int, error) {
	where := []string{"tenant_id=$1", "deleted_at IS NULL"}
	args := []any{tenantID}
	idx := 2
	if filter.DenyAll {
		where = append(where, "1=0")
	} else if filter.OwnerCompanyID != nil {
		where = append(where, fmt.Sprintf("owner_company_id=$%d", idx))
		args = append(args, *filter.OwnerCompanyID)
		idx++
	} else if len(filter.AccessibleOwnerCompanyIDs) > 0 {
		if filter.IncludeTenantWide {
			where = append(where, fmt.Sprintf("(owner_company_id IS NULL OR owner_company_id = ANY($%d))", idx))
		} else {
			where = append(where, fmt.Sprintf("owner_company_id = ANY($%d)", idx))
		}
		args = append(args, filter.AccessibleOwnerCompanyIDs)
		idx++
	} else {
		where = append(where, "1=0")
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status=$%d", idx))
		args = append(args, strings.TrimSpace(*filter.Status))
		idx++
	}
	if filter.RfxType != nil {
		where = append(where, fmt.Sprintf("rfx_type=$%d", idx))
		args = append(args, strings.TrimSpace(*filter.RfxType))
		idx++
	}
	if filter.Search != nil {
		where = append(where, fmt.Sprintf("(template_code ILIKE $%d OR name_i18n_json::text ILIKE $%d)", idx, idx))
		args = append(args, "%"+strings.TrimSpace(*filter.Search)+"%")
		idx++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db().QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_templates WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, mapDBError(err)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db().Query(ctx, `
		SELECT `+rfxTemplateSelectColumns+`
		FROM rfx.rfx_templates
		WHERE `+whereSQL+`
		ORDER BY template_code ASC, created_at ASC
		LIMIT $`+fmt.Sprint(idx)+` OFFSET $`+fmt.Sprint(idx+1), listArgs...)
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.RfxTemplate, 0)
	for rows.Next() {
		tmpl, err := scanRfxTemplate(rows)
		if err != nil {
			return nil, 0, mapDBError(err)
		}
		items = append(items, *tmpl)
	}
	return items, total, rows.Err()
}

func (r *TemplateLibraryRepository) ListVersionsForTemplate(ctx context.Context, templateID, tenantID uuid.UUID) ([]domain.RfxTemplateVersion, error) {
	rows, err := r.db().Query(ctx, `
		SELECT `+rfxTemplateVersionSelectColumns+`
		FROM rfx.rfx_template_versions
		WHERE template_id=$1 AND tenant_id=$2 AND deleted_at IS NULL
		ORDER BY version_number DESC`, templateID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.RfxTemplateVersion, 0)
	for rows.Next() {
		ver, err := scanRfxTemplateVersion(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *ver)
	}
	return out, rows.Err()
}

func (r *TemplateLibraryRepository) GetDraftVersion(ctx context.Context, templateID, tenantID uuid.UUID) (*domain.RfxTemplateVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxTemplateVersionSelectColumns+`
		FROM rfx.rfx_template_versions
		WHERE template_id=$1 AND tenant_id=$2 AND status=$3 AND deleted_at IS NULL`, templateID, tenantID, domain.RfxVersionStatusDraft)
	ver, err := scanRfxTemplateVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	ver.IsActiveDraft = true
	return ver, nil
}

func (r *TemplateLibraryRepository) GetPublishedVersion(ctx context.Context, templateID, tenantID uuid.UUID) (*domain.RfxTemplateVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxTemplateVersionSelectColumns+`
		FROM rfx.rfx_template_versions
		WHERE template_id=$1 AND tenant_id=$2 AND status=$3 AND deleted_at IS NULL`, templateID, tenantID, domain.RfxVersionStatusPublished)
	ver, err := scanRfxTemplateVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	ver.IsPublished = true
	return ver, nil
}

func (r *TemplateLibraryRepository) GetVersionByID(ctx context.Context, versionID, tenantID uuid.UUID) (*domain.RfxTemplateVersion, error) {
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

func (r *TemplateLibraryRepository) LockVersionByID(ctx context.Context, versionID, tenantID uuid.UUID) (*domain.RfxTemplateVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxTemplateVersionSelectColumns+`
		FROM rfx.rfx_template_versions
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
		FOR UPDATE`, versionID, tenantID)
	ver, err := scanRfxTemplateVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx template version not found")
		}
		return nil, mapDBError(err)
	}
	return ver, nil
}

func (r *TemplateLibraryRepository) HasEverPublished(ctx context.Context, templateID, tenantID uuid.UUID) (bool, error) {
	var count int
	err := r.db().QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_versions
		WHERE template_id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			AND status = ANY($3::text[])`, templateID, tenantID, []string{domain.RfxVersionStatusPublished, domain.RfxVersionStatusSuperseded}).Scan(&count)
	return count > 0, mapDBError(err)
}

func scanRfxTemplate(row pgx.Row) (*domain.RfxTemplate, error) {
	var tmpl domain.RfxTemplate
	var rfxType *string
	var ownerCompanyID *uuid.UUID
	var nameI18n, descriptionI18n []byte
	if err := row.Scan(
		&tmpl.ID, &tmpl.TenantID, &tmpl.TemplateCode, &nameI18n, &descriptionI18n, &rfxType,
		&ownerCompanyID, &tmpl.Status, &tmpl.Version, &tmpl.CreatedBy, &tmpl.CreatedAt, &tmpl.UpdatedAt,
	); err != nil {
		return nil, err
	}
	tmpl.NameI18nJSON = nameI18n
	if len(descriptionI18n) > 0 {
		tmpl.DescriptionI18nJSON = descriptionI18n
	}
	if rfxType != nil {
		tmpl.RfxType = rfxType
	}
	tmpl.OwnerCompanyID = ownerCompanyID
	return &tmpl, nil
}

func scanRfxTemplateVersion(row pgx.Row) (*domain.RfxTemplateVersion, error) {
	var ver domain.RfxTemplateVersion
	if err := row.Scan(
		&ver.ID, &ver.TenantID, &ver.TemplateID, &ver.VersionNumber, &ver.Status,
		&ver.PublishedAt, &ver.PublishedBy, &ver.ChangeSummary, &ver.Version,
		&ver.CreatedBy, &ver.CreatedAt, &ver.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &ver, nil
}

func templateNullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
