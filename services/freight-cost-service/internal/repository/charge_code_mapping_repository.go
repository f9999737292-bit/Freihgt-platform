package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type ChargeCodeMappingRepository struct {
	pool *pgxpool.Pool
}

func NewChargeCodeMappingRepository(pool *pgxpool.Pool) *ChargeCodeMappingRepository {
	return &ChargeCodeMappingRepository{pool: pool}
}

func (r *ChargeCodeMappingRepository) LoadActiveMappings(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	evaluationTime time.Time,
) (platform []domain.ChargeCodeMapping, tenant []domain.ChargeCodeMapping, maxVersion int64, err error) {
	return r.loadMappingsAtVersion(ctx, tx, tenantID, evaluationTime)
}

func (r *ChargeCodeMappingRepository) LoadPinnedMappings(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	pinnedVersion int64,
) (platform []domain.ChargeCodeMapping, tenant []domain.ChargeCodeMapping, maxVersion int64, err error) {
	return r.loadPinnedMappingsAtVersion(ctx, tx, tenantID, pinnedVersion)
}

func (r *ChargeCodeMappingRepository) loadPinnedMappingsAtVersion(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	pinnedVersion int64,
) (platform []domain.ChargeCodeMapping, tenant []domain.ChargeCodeMapping, maxLoaded int64, err error) {
	query := `
WITH eligible AS (
	SELECT
		mapping_scope,
		tenant_id,
		source_charge_code_normalized,
		normalized_category,
		mapping_version
	FROM freight_cost.charge_code_mapping
	WHERE (
		(mapping_scope = 'PLATFORM' AND tenant_id IS NULL)
		OR (mapping_scope = 'TENANT' AND tenant_id = $1)
	)
	AND mapping_version <= $2
),
ranked AS (
	SELECT
		*,
		ROW_NUMBER() OVER (
			PARTITION BY source_charge_code_normalized
			ORDER BY
				CASE mapping_scope WHEN 'TENANT' THEN 0 ELSE 1 END,
				mapping_version DESC
		) AS rn
	FROM eligible
)
SELECT mapping_scope, tenant_id, source_charge_code_normalized, normalized_category, mapping_version
FROM ranked
WHERE rn = 1
ORDER BY source_charge_code_normalized`

	var rows pgx.Rows
	if tx != nil {
		rows, err = tx.Query(ctx, query, tenantID, pinnedVersion)
	} else {
		rows, err = r.pool.Query(ctx, query, tenantID, pinnedVersion)
	}
	if err != nil {
		return nil, nil, 0, mapDBError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var mapping domain.ChargeCodeMapping
		var scope string
		var tenantIDPtr *uuid.UUID
		if scanErr := rows.Scan(
			&scope,
			&tenantIDPtr,
			&mapping.SourceChargeCodeNormalized,
			&mapping.NormalizedCategory,
			&mapping.MappingVersion,
		); scanErr != nil {
			return nil, nil, 0, mapDBError(scanErr)
		}
		mapping.MappingScope = scope
		mapping.TenantID = tenantIDPtr
		if mapping.MappingVersion > maxLoaded {
			maxLoaded = mapping.MappingVersion
		}
		if scope == domain.MappingScopePlatform {
			platform = append(platform, mapping)
		} else {
			tenant = append(tenant, mapping)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, mapDBError(err)
	}
	if maxLoaded == 0 {
		maxLoaded = pinnedVersion
	}
	return platform, tenant, maxLoaded, nil
}

func (r *ChargeCodeMappingRepository) loadMappingsAtVersion(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	evaluationTime time.Time,
) (platform []domain.ChargeCodeMapping, tenant []domain.ChargeCodeMapping, maxLoaded int64, err error) {
	query := `
WITH eligible AS (
	SELECT
		mapping_scope,
		tenant_id,
		source_charge_code_normalized,
		normalized_category,
		mapping_version,
		effective_from,
		effective_to
	FROM freight_cost.charge_code_mapping
	WHERE (
		(mapping_scope = 'PLATFORM' AND tenant_id IS NULL)
		OR (mapping_scope = 'TENANT' AND tenant_id = $1)
	)
	AND effective_from <= $2
	AND (effective_to IS NULL OR effective_to > $2)
),
ranked AS (
	SELECT
		*,
		ROW_NUMBER() OVER (
			PARTITION BY source_charge_code_normalized
			ORDER BY
				CASE mapping_scope WHEN 'TENANT' THEN 0 ELSE 1 END,
				mapping_version DESC,
				effective_from DESC
		) AS rn
	FROM eligible
)
SELECT mapping_scope, tenant_id, source_charge_code_normalized, normalized_category, mapping_version
FROM ranked
WHERE rn = 1
ORDER BY source_charge_code_normalized`

	var rows pgx.Rows
	if tx != nil {
		rows, err = tx.Query(ctx, query, tenantID, evaluationTime)
	} else {
		rows, err = r.pool.Query(ctx, query, tenantID, evaluationTime)
	}
	if err != nil {
		return nil, nil, 0, mapDBError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var mapping domain.ChargeCodeMapping
		var scope string
		var tenantIDPtr *uuid.UUID
		if scanErr := rows.Scan(
			&scope,
			&tenantIDPtr,
			&mapping.SourceChargeCodeNormalized,
			&mapping.NormalizedCategory,
			&mapping.MappingVersion,
		); scanErr != nil {
			return nil, nil, 0, mapDBError(scanErr)
		}
		mapping.MappingScope = scope
		mapping.TenantID = tenantIDPtr
		if mapping.MappingVersion > maxLoaded {
			maxLoaded = mapping.MappingVersion
		}
		if scope == domain.MappingScopePlatform {
			platform = append(platform, mapping)
		} else {
			tenant = append(tenant, mapping)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, mapDBError(err)
	}
	if maxLoaded == 0 {
		maxLoaded = 1
	}
	return platform, tenant, maxLoaded, nil
}

type UpsertChargeCodeMappingInput struct {
	MappingScope     string
	TenantID         *uuid.UUID
	SourceCode       string
	TargetCategory   string
	EffectiveFrom    time.Time
	EffectiveTo      *time.Time
	ActorID          string
}

func (r *ChargeCodeMappingRepository) UpsertMapping(
	ctx context.Context,
	input UpsertChargeCodeMappingInput,
) (domain.ChargeCodeMapping, error) {
	normalized, err := domain.NormalizeChargeCode(input.SourceCode)
	if err != nil {
		return domain.ChargeCodeMapping{}, err
	}
	target, err := domain.NormalizeMappingCategory(input.TargetCategory)
	if err != nil {
		return domain.ChargeCodeMapping{}, err
	}
	if input.EffectiveTo != nil && !input.EffectiveTo.After(input.EffectiveFrom) {
		return domain.ChargeCodeMapping{}, fmt.Errorf("effective_to must be after effective_from")
	}
	scope := strings.ToUpper(strings.TrimSpace(input.MappingScope))
	if scope != domain.MappingScopeTenant {
		return domain.ChargeCodeMapping{}, fmt.Errorf("platform mapping mutation is disabled until verified platform-admin authorization is available")
	}
	if input.TenantID == nil {
		return domain.ChargeCodeMapping{}, fmt.Errorf("tenant mapping requires tenant_id")
	}

	var version int64
	if err := r.pool.QueryRow(ctx, `SELECT nextval('freight_cost.charge_code_mapping_version_seq')`).Scan(&version); err != nil {
		return domain.ChargeCodeMapping{}, mapDBError(err)
	}

	const insertSQL = `
INSERT INTO freight_cost.charge_code_mapping (
	mapping_scope,
	tenant_id,
	source_charge_code_normalized,
	normalized_category,
	mapping_version,
	effective_from,
	effective_to
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING mapping_version`

	if err := r.pool.QueryRow(
		ctx,
		insertSQL,
		scope,
		input.TenantID,
		normalized,
		target,
		version,
		input.EffectiveFrom,
		input.EffectiveTo,
	).Scan(&version); err != nil {
		return domain.ChargeCodeMapping{}, mapDBError(err)
	}

	return domain.ChargeCodeMapping{
		MappingScope:               scope,
		TenantID:                   input.TenantID,
		SourceChargeCodeNormalized: normalized,
		NormalizedCategory:         target,
		MappingVersion:             version,
	}, nil
}

func IsOverlapConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "ex_charge_code_mapping_no_overlap") ||
		strings.Contains(msg, "charge_code_mapping_no_overlap") ||
		strings.Contains(msg, "exclusion constraint")
}

func (r *ChargeCodeMappingRepository) CurrentPlatformMappingVersion(ctx context.Context) (int64, error) {
	const query = `
		SELECT COALESCE(MAX(mapping_version), 1)
		FROM freight_cost.charge_code_mapping
		WHERE mapping_scope = 'PLATFORM' AND tenant_id IS NULL`
	var version int64
	if err := r.pool.QueryRow(ctx, query).Scan(&version); err != nil {
		return 1, mapDBError(err)
	}
	return version, nil
}
