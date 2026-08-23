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

type AnalyticsBenchmarkProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsBenchmarkProjectionRepository(pool *pgxpool.Pool) *AnalyticsBenchmarkProjectionRepository {
	return &AnalyticsBenchmarkProjectionRepository{pool: pool}
}

func (r *AnalyticsBenchmarkProjectionRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_benchmark_projection WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsBenchmarkProjectionRepository) DeleteByKey(ctx context.Context, tx pgx.Tx, key domain.AnalyticsBenchmarkKey) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.cost_analytics_benchmark_projection
		WHERE tenant_id = $1 AND buyer_company_id = $2 AND cohort_type = $3
		  AND lane_key = $4 AND transport_mode = $5 AND equipment_type = $6
		  AND period_start = $7 AND period_grain = $8 AND currency_code = $9`,
		key.TenantID, key.BuyerCompanyID, key.CohortType, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	return mapDBError(err)
}

func (r *AnalyticsBenchmarkProjectionRepository) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.AnalyticsBenchmarkProjection,
) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_benchmark_projection (
			tenant_id, buyer_company_id, cohort_type, lane_key, transport_mode, equipment_type,
			period_start, period_grain, currency_code,
			sample_count, mean_amount, median_amount, p25_amount, p75_amount, p90_amount,
			min_amount, max_amount, data_quality, rule_version,
			calculated_at, data_through, projection_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		ON CONFLICT (
			tenant_id, buyer_company_id, cohort_type, lane_key,
			transport_mode, equipment_type, period_start, period_grain, currency_code
		) DO UPDATE SET
			sample_count = EXCLUDED.sample_count,
			mean_amount = EXCLUDED.mean_amount,
			median_amount = EXCLUDED.median_amount,
			p25_amount = EXCLUDED.p25_amount,
			p75_amount = EXCLUDED.p75_amount,
			p90_amount = EXCLUDED.p90_amount,
			min_amount = EXCLUDED.min_amount,
			max_amount = EXCLUDED.max_amount,
			data_quality = EXCLUDED.data_quality,
			rule_version = EXCLUDED.rule_version,
			calculated_at = EXCLUDED.calculated_at,
			data_through = EXCLUDED.data_through,
			projection_version = EXCLUDED.projection_version`
	_, err := tx.Exec(ctx, query,
		projection.TenantID, projection.BuyerCompanyID, projection.CohortType,
		projection.LaneKey, projection.TransportMode, projection.EquipmentType,
		projection.PeriodStart, projection.PeriodGrain, projection.CurrencyCode,
		projection.SampleCount,
		decimalArg(projection.MeanAmount), decimalArg(projection.MedianAmount),
		decimalArg(projection.P25Amount), decimalArg(projection.P75Amount), decimalArg(projection.P90Amount),
		decimalArg(projection.MinAmount), decimalArg(projection.MaxAmount),
		projection.DataQuality, projection.RuleVersion,
		projection.CalculatedAt, projection.DataThrough, projection.ProjectionVersion,
	)
	return mapDBError(err)
}

type LaneBenchmarkStats struct {
	SampleCount  int
	MeanAmount   *decimal.Decimal
	MedianAmount *decimal.Decimal
	P25Amount    *decimal.Decimal
	P75Amount    *decimal.Decimal
	P90Amount    *decimal.Decimal
	MinAmount    *decimal.Decimal
	MaxAmount    *decimal.Decimal
	MaxUpdated   time.Time
}

func (r *AnalyticsBenchmarkProjectionRepository) ComputeLaneBenchmarkStats(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsBenchmarkKey,
) (*LaneBenchmarkStats, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			AVG(amount),
			percentile_cont(0.50) WITHIN GROUP (ORDER BY amount),
			percentile_cont(0.25) WITHIN GROUP (ORDER BY amount),
			percentile_cont(0.75) WITHIN GROUP (ORDER BY amount),
			percentile_cont(0.90) WITHIN GROUP (ORDER BY amount),
			MIN(amount),
			MAX(amount),
			COALESCE(MAX(source_summary_updated_at), NOW())
		FROM (
			SELECT
				CASE
					WHEN financial_finality = 'FINAL_ACTUAL' THEN final_actual_amount
					WHEN financial_finality = 'CURRENT_ACTUAL' THEN current_actual_amount
				END AS amount,
				source_summary_updated_at
			FROM freight_cost.cost_analytics_order_fact
			WHERE tenant_id = $1
			  AND buyer_company_id = $2
			  AND lane_key = $3
			  AND transport_mode = $4
			  AND equipment_type = $5
			  AND period_start = $6
			  AND period_grain = $7
			  AND currency_code = $8
			  AND lane_eligible = TRUE
			  AND (
				(financial_finality = 'FINAL_ACTUAL' AND final_actual_amount IS NOT NULL)
				OR (financial_finality = 'CURRENT_ACTUAL' AND current_actual_amount IS NOT NULL)
			  )
		) samples`,
		key.TenantID, key.BuyerCompanyID, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	var stats LaneBenchmarkStats
	if err := row.Scan(
		&stats.SampleCount, &stats.MeanAmount, &stats.MedianAmount,
		&stats.P25Amount, &stats.P75Amount, &stats.P90Amount,
		&stats.MinAmount, &stats.MaxAmount, &stats.MaxUpdated,
	); err != nil {
		return nil, mapDBError(err)
	}
	return &stats, nil
}

func (r *AnalyticsBenchmarkProjectionRepository) ListDistinctLaneKeysForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]domain.AnalyticsBenchmarkKey, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT tenant_id, buyer_company_id, lane_key, transport_mode, equipment_type,
			period_start, period_grain, currency_code
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND lane_eligible = TRUE AND lane_key IS NOT NULL
		ORDER BY buyer_company_id, lane_key, period_start, currency_code`, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var keys []domain.AnalyticsBenchmarkKey
	for rows.Next() {
		var key domain.AnalyticsBenchmarkKey
		if err := rows.Scan(
			&key.TenantID, &key.BuyerCompanyID, &key.LaneKey, &key.TransportMode, &key.EquipmentType,
			&key.PeriodStart, &key.PeriodGrain, &key.CurrencyCode,
		); err != nil {
			return nil, mapDBError(err)
		}
		key.CohortType = domain.BenchmarkCohortTypeLane
		keys = append(keys, key)
	}
	return keys, mapDBError(rows.Err())
}

type AnalyticsBenchmarkListFilter struct {
	BuyerCompanyID *uuid.UUID
	CurrencyCode   string
	LaneKey        string
	PeriodFrom     *time.Time
	PeriodTo       *time.Time
	Limit          int
	Offset         int
}

func (r *AnalyticsBenchmarkProjectionRepository) List(
	ctx context.Context,
	tenantID uuid.UUID,
	filter AnalyticsBenchmarkListFilter,
) ([]domain.AnalyticsBenchmarkProjection, error) {
	query := `
		SELECT tenant_id, buyer_company_id, cohort_type, lane_key, transport_mode, equipment_type,
			period_start, period_grain, currency_code,
			sample_count, mean_amount, median_amount, p25_amount, p75_amount, p90_amount,
			min_amount, max_amount, data_quality, rule_version,
			calculated_at, data_through, projection_version
		FROM freight_cost.cost_analytics_benchmark_projection
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
	if filter.LaneKey != "" {
		query += ` AND lane_key = $` + strconv.Itoa(argIdx)
		args = append(args, filter.LaneKey)
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
	query += ` ORDER BY period_start DESC, lane_key ASC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
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
	return scanBenchmarkProjections(rows)
}

func scanBenchmarkProjections(rows pgx.Rows) ([]domain.AnalyticsBenchmarkProjection, error) {
	var items []domain.AnalyticsBenchmarkProjection
	for rows.Next() {
		var p domain.AnalyticsBenchmarkProjection
		if err := rows.Scan(
			&p.TenantID, &p.BuyerCompanyID, &p.CohortType, &p.LaneKey, &p.TransportMode, &p.EquipmentType,
			&p.PeriodStart, &p.PeriodGrain, &p.CurrencyCode,
			&p.SampleCount, &p.MeanAmount, &p.MedianAmount, &p.P25Amount, &p.P75Amount, &p.P90Amount,
			&p.MinAmount, &p.MaxAmount, &p.DataQuality, &p.RuleVersion,
			&p.CalculatedAt, &p.DataThrough, &p.ProjectionVersion,
		); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, p)
	}
	return items, mapDBError(rows.Err())
}
