package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ImportAnalysisRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewImportAnalysisRepository(pool *pgxpool.Pool) *ImportAnalysisRepository {
	return &ImportAnalysisRepository{pool: pool}
}

func (r *ImportAnalysisRepository) WithTx(tx pgx.Tx) *ImportAnalysisRepository {
	return &ImportAnalysisRepository{pool: r.pool, exec: tx}
}

func (r *ImportAnalysisRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

const importAnalysisSelectColumns = `
	id, tenant_id, actor_id, actor_company_id, workbook_type, schema_version,
	target_type, target_id, target_version, canonical_payload_json, canonical_hash,
	status, validation_summary, expires_at, consumed_at, result_reference_type,
	result_reference_id, created_at
`

func (r *ImportAnalysisRepository) CreatePreview(ctx context.Context, in domain.ImportAnalysis) (*domain.ImportAnalysis, error) {
	if err := domain.ValidateImportAnalysisPreviewInput(in); err != nil {
		return nil, err
	}
	createdAt := in.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	expiresAt := in.ExpiresAt.UTC()
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_import_analyses (
			tenant_id, actor_id, actor_company_id, workbook_type, schema_version,
			target_type, target_id, target_version, canonical_payload_json, canonical_hash,
			status, validation_summary, created_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		RETURNING `+importAnalysisSelectColumns,
		in.TenantID, in.ActorID, in.ActorCompanyID, in.WorkbookType, in.SchemaVersion,
		in.TargetType, in.TargetID, in.TargetVersion, in.CanonicalPayloadJSON, in.CanonicalHash,
		domain.ImportAnalysisStatusPreviewed, in.ValidationSummary, createdAt, expiresAt,
	)
	return scanImportAnalysis(row)
}

func (r *ImportAnalysisRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ImportAnalysis, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+importAnalysisSelectColumns+`
		FROM rfx.rfx_import_analyses
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	analysis, err := scanImportAnalysis(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("import analysis not found")
		}
		return nil, mapDBError(err)
	}
	return analysis, nil
}

func (r *ImportAnalysisRepository) MarkConsumed(
	ctx context.Context,
	id, tenantID uuid.UUID,
	resultReferenceType string,
	resultReferenceID uuid.UUID,
	consumedAt time.Time,
) (*domain.ImportAnalysis, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_import_analyses
		SET status = $4,
		    consumed_at = $5,
		    result_reference_type = $6,
		    result_reference_id = $7
		WHERE id = $1 AND tenant_id = $2 AND status = $3
		RETURNING `+importAnalysisSelectColumns,
		id, tenantID, domain.ImportAnalysisStatusPreviewed, domain.ImportAnalysisStatusConsumed,
		consumedAt.UTC(), resultReferenceType, resultReferenceID,
	)
	analysis, err := scanImportAnalysis(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("import analysis is not consumable", map[string]any{"id": id.String()})
		}
		return nil, mapDBError(err)
	}
	return analysis, nil
}

func scanImportAnalysis(row pgx.Row) (*domain.ImportAnalysis, error) {
	var out domain.ImportAnalysis
	var targetID *uuid.UUID
	var targetVersion *int
	var consumedAt *time.Time
	var resultReferenceType *string
	var resultReferenceID *uuid.UUID
	err := row.Scan(
		&out.ID, &out.TenantID, &out.ActorID, &out.ActorCompanyID, &out.WorkbookType, &out.SchemaVersion,
		&out.TargetType, &targetID, &targetVersion, &out.CanonicalPayloadJSON, &out.CanonicalHash,
		&out.Status, &out.ValidationSummary, &out.ExpiresAt, &consumedAt, &resultReferenceType,
		&resultReferenceID, &out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	out.TargetID = targetID
	out.TargetVersion = targetVersion
	out.ConsumedAt = consumedAt
	out.ResultReferenceType = resultReferenceType
	out.ResultReferenceID = resultReferenceID
	return &out, nil
}
