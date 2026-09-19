package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ReferenceMappingRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewReferenceMappingRepository(pool *pgxpool.Pool) *ReferenceMappingRepository {
	return &ReferenceMappingRepository{pool: pool}
}

func (r *ReferenceMappingRepository) WithTx(tx pgx.Tx) *ReferenceMappingRepository {
	return &ReferenceMappingRepository{pool: r.pool, exec: tx}
}

func (r *ReferenceMappingRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *ReferenceMappingRepository) CreateSet(ctx context.Context, in domain.ReferenceMappingSet) (*domain.ReferenceMappingSet, error) {
	if err := validateReferenceMappingSet(in); err != nil {
		return nil, err
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_reference_mapping_sets (tenant_id, mapping_type, version, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, mapping_type, version, status
	`, in.TenantID, strings.TrimSpace(in.MappingType), in.Version, in.Status)
	return scanReferenceMappingSet(row)
}

func (r *ReferenceMappingRepository) CreateEntry(ctx context.Context, in domain.ReferenceMappingEntry) (*domain.ReferenceMappingEntry, error) {
	if err := validateReferenceMappingEntry(in); err != nil {
		return nil, err
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_reference_mapping_entries (
			tenant_id, mapping_set_id, external_code, canonical_code
		) VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, mapping_set_id, external_code, canonical_code
	`, in.TenantID, in.MappingSetID, strings.TrimSpace(in.ExternalCode), strings.TrimSpace(in.CanonicalCode))
	return scanReferenceMappingEntry(row)
}

func (r *ReferenceMappingRepository) GetSetByID(ctx context.Context, id uuid.UUID) (*domain.ReferenceMappingSet, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("mapping_set_id is required", map[string]any{"field": "mapping_set_id"})
	}
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, mapping_type, version, status
		FROM rfx.rfx_reference_mapping_sets
		WHERE id = $1
	`, id)
	set, err := scanReferenceMappingSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("reference mapping set not found")
		}
		return nil, mapDBError(err)
	}
	return set, nil
}

func (r *ReferenceMappingRepository) GetLatestActiveSetForTenant(
	ctx context.Context,
	tenantID uuid.UUID,
	mappingType string,
) (*domain.ReferenceMappingSet, error) {
	mappingType = strings.TrimSpace(mappingType)
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, mapping_type, version, status
		FROM rfx.rfx_reference_mapping_sets
		WHERE mapping_type = $1
		  AND status = $2
		  AND tenant_id = $3
		ORDER BY version DESC
		LIMIT 1
	`, mappingType, domain.ReferenceMappingSetStatusActive, tenantID)
	set, err := scanReferenceMappingSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("reference mapping set not found")
		}
		return nil, mapDBError(err)
	}
	return set, nil
}

func (r *ReferenceMappingRepository) GetLatestActivePlatformSet(
	ctx context.Context,
	mappingType string,
) (*domain.ReferenceMappingSet, error) {
	mappingType = strings.TrimSpace(mappingType)
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, mapping_type, version, status
		FROM rfx.rfx_reference_mapping_sets
		WHERE mapping_type = $1
		  AND status = $2
		  AND tenant_id IS NULL
		ORDER BY version DESC
		LIMIT 1
	`, mappingType, domain.ReferenceMappingSetStatusActive)
	set, err := scanReferenceMappingSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("reference mapping set not found")
		}
		return nil, mapDBError(err)
	}
	return set, nil
}

func (r *ReferenceMappingRepository) GetSetsAtMaxVersion(
	ctx context.Context,
	tenantID *uuid.UUID,
	mappingType string,
) ([]domain.ReferenceMappingSet, error) {
	mappingType = strings.TrimSpace(mappingType)
	rows, err := r.db().Query(ctx, `
		WITH latest AS (
			SELECT MAX(version) AS max_version
			FROM rfx.rfx_reference_mapping_sets
			WHERE mapping_type = $1
			  AND (
				($2::uuid IS NULL AND tenant_id IS NULL)
				OR tenant_id = $2
			  )
		)
		SELECT s.id, s.tenant_id, s.mapping_type, s.version, s.status
		FROM rfx.rfx_reference_mapping_sets s
		JOIN latest l ON s.version = l.max_version
		WHERE s.mapping_type = $1
		  AND (
			($2::uuid IS NULL AND s.tenant_id IS NULL)
			OR s.tenant_id = $2
		  )
	`, mappingType, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var sets []domain.ReferenceMappingSet
	for rows.Next() {
		set, scanErr := scanReferenceMappingSet(rows)
		if scanErr != nil {
			return nil, mapDBError(scanErr)
		}
		sets = append(sets, *set)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return sets, nil
}

func (r *ReferenceMappingRepository) CountActiveSetsAtMaxVersion(
	ctx context.Context,
	tenantID *uuid.UUID,
	mappingType string,
) (int, error) {
	var count int
	err := r.db().QueryRow(ctx, `
		WITH latest AS (
			SELECT MAX(version) AS max_version
			FROM rfx.rfx_reference_mapping_sets
			WHERE mapping_type = $1
			  AND status = $2
			  AND (
				($3::uuid IS NULL AND tenant_id IS NULL)
				OR tenant_id = $3
			  )
		)
		SELECT COUNT(*)
		FROM rfx.rfx_reference_mapping_sets s
		JOIN latest l ON s.version = l.max_version
		WHERE s.mapping_type = $1
		  AND s.status = $2
		  AND (
			($3::uuid IS NULL AND s.tenant_id IS NULL)
			OR s.tenant_id = $3
		  )
	`, mappingType, domain.ReferenceMappingSetStatusActive, tenantID).Scan(&count)
	return count, err
}

func (r *ReferenceMappingRepository) GetActiveSet(
	ctx context.Context,
	tenantID *uuid.UUID,
	mappingType string,
	version int,
) (*domain.ReferenceMappingSet, error) {
	mappingType = strings.TrimSpace(mappingType)
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, mapping_type, version, status
		FROM rfx.rfx_reference_mapping_sets
		WHERE mapping_type = $1
		  AND version = $2
		  AND status = $3
		  AND (
			($4::uuid IS NULL AND tenant_id IS NULL)
			OR tenant_id = $4
		  )
	`, mappingType, version, domain.ReferenceMappingSetStatusActive, tenantID)
	set, err := scanReferenceMappingSet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("reference mapping set not found")
		}
		return nil, mapDBError(err)
	}
	return set, nil
}

func (r *ReferenceMappingRepository) ResolveCanonicalCode(
	ctx context.Context,
	setID uuid.UUID,
	externalCode string,
) (string, error) {
	externalCode = strings.TrimSpace(externalCode)
	var canonical string
	err := r.db().QueryRow(ctx, `
		SELECT canonical_code
		FROM rfx.rfx_reference_mapping_entries
		WHERE mapping_set_id = $1 AND external_code = $2
	`, setID, externalCode).Scan(&canonical)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NotFound("reference mapping entry not found")
		}
		return "", mapDBError(err)
	}
	return canonical, nil
}

func (r *ReferenceMappingRepository) CountEntriesForExternalCode(
	ctx context.Context,
	setID uuid.UUID,
	externalCode string,
) (int, error) {
	var count int
	err := r.db().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM rfx.rfx_reference_mapping_entries
		WHERE mapping_set_id = $1 AND external_code = $2
	`, setID, strings.TrimSpace(externalCode)).Scan(&count)
	if err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}

func validateReferenceMappingSet(in domain.ReferenceMappingSet) error {
	if strings.TrimSpace(in.MappingType) == "" {
		return apperrors.Validation("mapping_type is required", map[string]any{"field": "mapping_type"})
	}
	if in.Version <= 0 {
		return apperrors.Validation("version must be positive", map[string]any{"field": "version"})
	}
	switch strings.TrimSpace(in.Status) {
	case domain.ReferenceMappingSetStatusActive, domain.ReferenceMappingSetStatusRetired:
	default:
		return apperrors.Validation("invalid mapping set status", map[string]any{"field": "status"})
	}
	return nil
}

func validateReferenceMappingEntry(in domain.ReferenceMappingEntry) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.MappingSetID == uuid.Nil {
		return apperrors.Validation("mapping_set_id is required", map[string]any{"field": "mapping_set_id"})
	}
	if strings.TrimSpace(in.ExternalCode) == "" {
		return apperrors.Validation("external_code is required", map[string]any{"field": "external_code"})
	}
	if strings.TrimSpace(in.CanonicalCode) == "" {
		return apperrors.Validation("canonical_code is required", map[string]any{"field": "canonical_code"})
	}
	return nil
}

func scanReferenceMappingSet(row pgx.Row) (*domain.ReferenceMappingSet, error) {
	var out domain.ReferenceMappingSet
	var tenantID *uuid.UUID
	err := row.Scan(&out.ID, &tenantID, &out.MappingType, &out.Version, &out.Status)
	if err != nil {
		return nil, err
	}
	out.TenantID = tenantID
	return &out, nil
}

func scanReferenceMappingEntry(row pgx.Row) (*domain.ReferenceMappingEntry, error) {
	var out domain.ReferenceMappingEntry
	err := row.Scan(&out.ID, &out.TenantID, &out.MappingSetID, &out.ExternalCode, &out.CanonicalCode)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
