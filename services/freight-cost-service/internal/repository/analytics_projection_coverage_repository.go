package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsProjectionCoverageRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsProjectionCoverageRepository(pool *pgxpool.Pool) *AnalyticsProjectionCoverageRepository {
	return &AnalyticsProjectionCoverageRepository{pool: pool}
}

func (r *AnalyticsProjectionCoverageRepository) Upsert(ctx context.Context, tx pgx.Tx, coverage *domain.AnalyticsProjectionCoverage) error {
	query := `
		INSERT INTO freight_cost.analytics_projection_coverage (
			projection_name, tenant_id, calculated_at,
			source_order_count, eligible_order_count, excluded_order_count,
			excluded_missing_origin_city, excluded_missing_destination_city,
			excluded_missing_country, excluded_missing_mode, excluded_missing_carrier_id,
			data_quality
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (projection_name, tenant_id) DO UPDATE SET
			calculated_at = EXCLUDED.calculated_at,
			source_order_count = EXCLUDED.source_order_count,
			eligible_order_count = EXCLUDED.eligible_order_count,
			excluded_order_count = EXCLUDED.excluded_order_count,
			excluded_missing_origin_city = EXCLUDED.excluded_missing_origin_city,
			excluded_missing_destination_city = EXCLUDED.excluded_missing_destination_city,
			excluded_missing_country = EXCLUDED.excluded_missing_country,
			excluded_missing_mode = EXCLUDED.excluded_missing_mode,
			excluded_missing_carrier_id = EXCLUDED.excluded_missing_carrier_id,
			data_quality = EXCLUDED.data_quality`
	_, err := tx.Exec(ctx, query,
		coverage.ProjectionName, coverage.TenantID, coverage.CalculatedAt.UTC(),
		coverage.SourceOrderCount, coverage.EligibleOrderCount, coverage.ExcludedOrderCount,
		coverage.ExcludedMissingOriginCity, coverage.ExcludedMissingDestinationCity,
		coverage.ExcludedMissingCountry, coverage.ExcludedMissingMode, coverage.ExcludedMissingCarrierID,
		coverage.DataQuality,
	)
	return mapDBError(err)
}

func (r *AnalyticsProjectionCoverageRepository) Get(
	ctx context.Context,
	projectionName string,
	tenantID uuid.UUID,
) (*domain.AnalyticsProjectionCoverage, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT projection_name, tenant_id, calculated_at,
			source_order_count, eligible_order_count, excluded_order_count,
			excluded_missing_origin_city, excluded_missing_destination_city,
			excluded_missing_country, excluded_missing_mode, excluded_missing_carrier_id,
			data_quality
		FROM freight_cost.analytics_projection_coverage
		WHERE projection_name = $1 AND tenant_id = $2`, projectionName, tenantID)
	var c domain.AnalyticsProjectionCoverage
	if err := row.Scan(
		&c.ProjectionName, &c.TenantID, &c.CalculatedAt,
		&c.SourceOrderCount, &c.EligibleOrderCount, &c.ExcludedOrderCount,
		&c.ExcludedMissingOriginCity, &c.ExcludedMissingDestinationCity,
		&c.ExcludedMissingCountry, &c.ExcludedMissingMode, &c.ExcludedMissingCarrierID,
		&c.DataQuality,
	); err != nil {
		return nil, mapDBError(err)
	}
	return &c, nil
}

func (r *AnalyticsProjectionCoverageRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_coverage WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}
