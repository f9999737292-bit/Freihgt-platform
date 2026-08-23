package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsAccessorialPeriodProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsAccessorialPeriodProjectionRepository(pool *pgxpool.Pool) *AnalyticsAccessorialPeriodProjectionRepository {
	return &AnalyticsAccessorialPeriodProjectionRepository{pool: pool}
}

func (r *AnalyticsAccessorialPeriodProjectionRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_accessorial_period_projection WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsAccessorialPeriodProjectionRepository) DeleteByKey(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsAccessorialPeriodKey,
) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.cost_analytics_accessorial_period_projection
		WHERE tenant_id = $1 AND buyer_company_id = $2 AND normalized_category = $3
		  AND period_start = $4 AND period_grain = $5 AND currency_code = $6`,
		key.TenantID, key.BuyerCompanyID, key.NormalizedCategory,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	return mapDBError(err)
}

func (r *AnalyticsAccessorialPeriodProjectionRepository) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.AnalyticsAccessorialPeriodProjection,
) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_accessorial_period_projection (
			tenant_id, buyer_company_id, normalized_category,
			period_start, period_grain, currency_code,
			total_amount, order_count, line_count,
			share_of_spend, accessorial_order_rate, freight_spend_total,
			calculated_at, data_through, projection_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (
			tenant_id, buyer_company_id, normalized_category,
			period_start, period_grain, currency_code
		) DO UPDATE SET
			total_amount = EXCLUDED.total_amount,
			order_count = EXCLUDED.order_count,
			line_count = EXCLUDED.line_count,
			share_of_spend = EXCLUDED.share_of_spend,
			accessorial_order_rate = EXCLUDED.accessorial_order_rate,
			freight_spend_total = EXCLUDED.freight_spend_total,
			calculated_at = EXCLUDED.calculated_at,
			data_through = EXCLUDED.data_through,
			projection_version = EXCLUDED.projection_version`
	_, err := tx.Exec(ctx, query,
		projection.TenantID, projection.BuyerCompanyID, projection.NormalizedCategory,
		projection.PeriodStart, projection.PeriodGrain, projection.CurrencyCode,
		decimalArg(projection.TotalAmount), projection.OrderCount, projection.LineCount,
		ratioArg(projection.ShareOfSpend), ratioArg(projection.AccessorialOrderRate),
		decimalArg(projection.FreightSpendTotal),
		projection.CalculatedAt, projection.DataThrough, projection.ProjectionVersion,
	)
	return mapDBError(err)
}

func ratioArg(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return value.StringFixed(6)
}

func (r *AnalyticsAccessorialPeriodProjectionRepository) ListDistinctKeysForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]domain.AnalyticsAccessorialPeriodKey, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT tenant_id, buyer_company_id, normalized_category,
			period_start, period_grain, currency_code
		FROM freight_cost.cost_analytics_accessorial_fact
		WHERE tenant_id = $1 AND eligible = TRUE
		ORDER BY buyer_company_id, normalized_category, period_start, currency_code`, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var keys []domain.AnalyticsAccessorialPeriodKey
	for rows.Next() {
		var key domain.AnalyticsAccessorialPeriodKey
		if err := rows.Scan(
			&key.TenantID, &key.BuyerCompanyID, &key.NormalizedCategory,
			&key.PeriodStart, &key.PeriodGrain, &key.CurrencyCode,
		); err != nil {
			return nil, mapDBError(err)
		}
		keys = append(keys, key)
	}
	return keys, mapDBError(rows.Err())
}

func (r *AnalyticsAccessorialPeriodProjectionRepository) List(
	ctx context.Context,
	tenantID uuid.UUID,
	filter AnalyticsAccessorialListFilter,
) ([]domain.AnalyticsAccessorialPeriodProjection, error) {
	query := `
		SELECT tenant_id, buyer_company_id, normalized_category,
			period_start, period_grain, currency_code,
			total_amount, order_count, line_count,
			share_of_spend, accessorial_order_rate, freight_spend_total,
			calculated_at, data_through, projection_version
		FROM freight_cost.cost_analytics_accessorial_period_projection
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
	if filter.NormalizedCategory != "" {
		query += ` AND normalized_category = $` + strconv.Itoa(argIdx)
		args = append(args, filter.NormalizedCategory)
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
	query += ` ORDER BY period_start DESC, normalized_category ASC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
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
	return scanAccessorialPeriodProjections(rows)
}

type AnalyticsAccessorialListFilter struct {
	BuyerCompanyID     *uuid.UUID
	CurrencyCode       string
	NormalizedCategory string
	PeriodFrom         *time.Time
	PeriodTo           *time.Time
	Limit              int
	Offset             int
}

func scanAccessorialPeriodProjections(rows pgx.Rows) ([]domain.AnalyticsAccessorialPeriodProjection, error) {
	var items []domain.AnalyticsAccessorialPeriodProjection
	for rows.Next() {
		var p domain.AnalyticsAccessorialPeriodProjection
		if err := rows.Scan(
			&p.TenantID, &p.BuyerCompanyID, &p.NormalizedCategory,
			&p.PeriodStart, &p.PeriodGrain, &p.CurrencyCode,
			&p.TotalAmount, &p.OrderCount, &p.LineCount,
			&p.ShareOfSpend, &p.AccessorialOrderRate, &p.FreightSpendTotal,
			&p.CalculatedAt, &p.DataThrough, &p.ProjectionVersion,
		); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, p)
	}
	return items, mapDBError(rows.Err())
}
