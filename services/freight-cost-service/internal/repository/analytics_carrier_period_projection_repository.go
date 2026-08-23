package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsCarrierPeriodProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsCarrierPeriodProjectionRepository(pool *pgxpool.Pool) *AnalyticsCarrierPeriodProjectionRepository {
	return &AnalyticsCarrierPeriodProjectionRepository{pool: pool}
}

func (r *AnalyticsCarrierPeriodProjectionRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_carrier_period_projection WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsCarrierPeriodProjectionRepository) DeleteByKey(ctx context.Context, tx pgx.Tx, key domain.AnalyticsCarrierPeriodKey) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.cost_analytics_carrier_period_projection
		WHERE tenant_id = $1 AND buyer_company_id = $2 AND carrier_company_id = $3
		  AND period_start = $4 AND period_grain = $5 AND currency_code = $6`,
		key.TenantID, key.BuyerCompanyID, key.CarrierCompanyID,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	return mapDBError(err)
}

func (r *AnalyticsCarrierPeriodProjectionRepository) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.AnalyticsCarrierPeriodProjection,
) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_carrier_period_projection (
			tenant_id, buyer_company_id, carrier_company_id,
			period_start, period_grain, currency_code,
			order_count, lane_count,
			planned_total, accrued_total, current_actual_total, final_actual_total,
			current_variance_total, final_variance_total,
			calculated_at, data_through, projection_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (
			tenant_id, buyer_company_id, carrier_company_id,
			period_start, period_grain, currency_code
		) DO UPDATE SET
			order_count = EXCLUDED.order_count,
			lane_count = EXCLUDED.lane_count,
			planned_total = EXCLUDED.planned_total,
			accrued_total = EXCLUDED.accrued_total,
			current_actual_total = EXCLUDED.current_actual_total,
			final_actual_total = EXCLUDED.final_actual_total,
			current_variance_total = EXCLUDED.current_variance_total,
			final_variance_total = EXCLUDED.final_variance_total,
			calculated_at = EXCLUDED.calculated_at,
			data_through = EXCLUDED.data_through,
			projection_version = EXCLUDED.projection_version`
	_, err := tx.Exec(ctx, query,
		projection.TenantID, projection.BuyerCompanyID, projection.CarrierCompanyID,
		projection.PeriodStart, projection.PeriodGrain, projection.CurrencyCode,
		projection.OrderCount, projection.LaneCount,
		decimalArg(projection.PlannedTotal), decimalArg(projection.AccruedTotal),
		decimalArg(projection.CurrentActualTotal), decimalArg(projection.FinalActualTotal),
		decimalArg(projection.CurrentVarianceTotal), decimalArg(projection.FinalVarianceTotal),
		projection.CalculatedAt, projection.DataThrough, projection.ProjectionVersion,
	)
	return mapDBError(err)
}

func (r *AnalyticsCarrierPeriodProjectionRepository) ListDistinctKeysForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]domain.AnalyticsCarrierPeriodKey, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT tenant_id, buyer_company_id, carrier_company_id,
			period_start, period_grain, currency_code
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND carrier_company_id IS NOT NULL
		  AND carrier_company_id <> '00000000-0000-0000-0000-000000000000'
		ORDER BY buyer_company_id, carrier_company_id, period_start, currency_code`, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var keys []domain.AnalyticsCarrierPeriodKey
	for rows.Next() {
		var key domain.AnalyticsCarrierPeriodKey
		if err := rows.Scan(
			&key.TenantID, &key.BuyerCompanyID, &key.CarrierCompanyID,
			&key.PeriodStart, &key.PeriodGrain, &key.CurrencyCode,
		); err != nil {
			return nil, mapDBError(err)
		}
		keys = append(keys, key)
	}
	return keys, mapDBError(rows.Err())
}

type AnalyticsCarrierListFilter struct {
	BuyerCompanyID   *uuid.UUID
	CurrencyCode     string
	CarrierCompanyID *uuid.UUID
	PeriodFrom       *time.Time
	PeriodTo         *time.Time
	Limit            int
	Offset           int
}

func (r *AnalyticsCarrierPeriodProjectionRepository) List(
	ctx context.Context,
	tenantID uuid.UUID,
	filter AnalyticsCarrierListFilter,
) ([]domain.AnalyticsCarrierPeriodProjection, error) {
	query := `
		SELECT tenant_id, buyer_company_id, carrier_company_id,
			period_start, period_grain, currency_code,
			order_count, lane_count,
			planned_total, accrued_total, current_actual_total, final_actual_total,
			current_variance_total, final_variance_total,
			calculated_at, data_through, projection_version
		FROM freight_cost.cost_analytics_carrier_period_projection
		WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2
	if filter.BuyerCompanyID != nil {
		query += ` AND buyer_company_id = $` + strconv.Itoa(argIdx)
		args = append(args, *filter.BuyerCompanyID)
		argIdx++
	}
	if filter.CurrencyCode != "" {
		query += ` AND currency_code = $` + strconv.Itoa(argIdx)
		args = append(args, filter.CurrencyCode)
		argIdx++
	}
	if filter.CarrierCompanyID != nil {
		query += ` AND carrier_company_id = $` + strconv.Itoa(argIdx)
		args = append(args, *filter.CarrierCompanyID)
		argIdx++
	}
	if filter.PeriodFrom != nil {
		query += ` AND period_start >= $` + strconv.Itoa(argIdx)
		args = append(args, *filter.PeriodFrom)
		argIdx++
	}
	if filter.PeriodTo != nil {
		query += ` AND period_start <= $` + strconv.Itoa(argIdx)
		args = append(args, *filter.PeriodTo)
		argIdx++
	}
	query += ` ORDER BY period_start DESC, carrier_company_id ASC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var items []domain.AnalyticsCarrierPeriodProjection
	for rows.Next() {
		var p domain.AnalyticsCarrierPeriodProjection
		if err := rows.Scan(
			&p.TenantID, &p.BuyerCompanyID, &p.CarrierCompanyID,
			&p.PeriodStart, &p.PeriodGrain, &p.CurrencyCode,
			&p.OrderCount, &p.LaneCount,
			&p.PlannedTotal, &p.AccruedTotal, &p.CurrentActualTotal, &p.FinalActualTotal,
			&p.CurrentVarianceTotal, &p.FinalVarianceTotal,
			&p.CalculatedAt, &p.DataThrough, &p.ProjectionVersion,
		); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, p)
	}
	return items, mapDBError(rows.Err())
}
