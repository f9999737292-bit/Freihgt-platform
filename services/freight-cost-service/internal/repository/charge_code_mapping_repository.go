package repository

import (
	"context"
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
) (platform []domain.ChargeCodeMapping, tenant []domain.ChargeCodeMapping, maxVersion int64, err error) {
	query := `
		SELECT mapping_scope, tenant_id, source_charge_code_normalized, normalized_category, mapping_version
		FROM freight_cost.charge_code_mapping
		WHERE (mapping_scope = 'PLATFORM' AND tenant_id IS NULL)
		   OR (mapping_scope = 'TENANT' AND tenant_id = $1)
		  AND (effective_to IS NULL OR effective_to > NOW())
		ORDER BY mapping_version DESC`
	var rows pgx.Rows
	if tx != nil {
		rows, err = tx.Query(ctx, query, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, query, tenantID)
	}
	if err != nil {
		return nil, nil, 0, mapDBError(err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var mapping domain.ChargeCodeMapping
		var scope string
		var tenantIDPtr *uuid.UUID
		if scanErr := rows.Scan(&scope, &tenantIDPtr, &mapping.SourceChargeCodeNormalized, &mapping.NormalizedCategory, &mapping.MappingVersion); scanErr != nil {
			return nil, nil, 0, mapDBError(scanErr)
		}
		mapping.MappingScope = scope
		mapping.TenantID = tenantIDPtr
		if mapping.MappingVersion > maxVersion {
			maxVersion = mapping.MappingVersion
		}
		_ = now
		if scope == domain.MappingScopePlatform {
			platform = append(platform, mapping)
		} else {
			tenant = append(tenant, mapping)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, mapDBError(err)
	}
	return platform, tenant, maxVersion, nil
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
