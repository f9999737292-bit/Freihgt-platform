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

type AnalyticsLanePeriodProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsLanePeriodProjectionRepository(pool *pgxpool.Pool) *AnalyticsLanePeriodProjectionRepository {
	return &AnalyticsLanePeriodProjectionRepository{pool: pool}
}

func (r *AnalyticsLanePeriodProjectionRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_lane_period_projection WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsLanePeriodProjectionRepository) DeleteByKey(ctx context.Context, tx pgx.Tx, key domain.AnalyticsLanePeriodKey) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.cost_analytics_lane_period_projection
		WHERE tenant_id = $1 AND buyer_company_id = $2 AND lane_key = $3
		  AND transport_mode = $4 AND equipment_type = $5
		  AND period_start = $6 AND period_grain = $7 AND currency_code = $8`,
		key.TenantID, key.BuyerCompanyID, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	return mapDBError(err)
}

func (r *AnalyticsLanePeriodProjectionRepository) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.AnalyticsLanePeriodProjection,
) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_lane_period_projection (
			tenant_id, buyer_company_id, lane_key, transport_mode, equipment_type,
			period_start, period_grain, currency_code,
			order_count, carrier_count,
			planned_total, accrued_total, current_actual_total, final_actual_total,
			current_variance_total, final_variance_total,
			calculated_at, data_through, projection_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19
		)
		ON CONFLICT (
			tenant_id, buyer_company_id, lane_key, transport_mode, equipment_type,
			period_start, period_grain, currency_code
		) DO UPDATE SET
			order_count = EXCLUDED.order_count,
			carrier_count = EXCLUDED.carrier_count,
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
		projection.TenantID, projection.BuyerCompanyID, projection.LaneKey, projection.TransportMode, projection.EquipmentType,
		projection.PeriodStart, projection.PeriodGrain, projection.CurrencyCode,
		projection.OrderCount, projection.CarrierCount,
		decimalArg(projection.PlannedTotal), decimalArg(projection.AccruedTotal),
		decimalArg(projection.CurrentActualTotal), decimalArg(projection.FinalActualTotal),
		decimalArg(projection.CurrentVarianceTotal), decimalArg(projection.FinalVarianceTotal),
		projection.CalculatedAt, projection.DataThrough, projection.ProjectionVersion,
	)
	return mapDBError(err)
}

type AnalyticsLaneAggregateRow struct {
	OrderCount           int
	CarrierCount         int
	PlannedTotal         *decimal.Decimal
	AccruedTotal         *decimal.Decimal
	CurrentActualTotal   *decimal.Decimal
	FinalActualTotal     *decimal.Decimal
	CurrentVarianceTotal *decimal.Decimal
	FinalVarianceTotal   *decimal.Decimal
	MaxSourceUpdatedAt   time.Time
}

func (r *AnalyticsLanePeriodProjectionRepository) ListDistinctKeysForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]domain.AnalyticsLanePeriodKey, error) {
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
	var keys []domain.AnalyticsLanePeriodKey
	for rows.Next() {
		var key domain.AnalyticsLanePeriodKey
		if err := rows.Scan(
			&key.TenantID, &key.BuyerCompanyID, &key.LaneKey, &key.TransportMode, &key.EquipmentType,
			&key.PeriodStart, &key.PeriodGrain, &key.CurrencyCode,
		); err != nil {
			return nil, mapDBError(err)
		}
		keys = append(keys, key)
	}
	return keys, mapDBError(rows.Err())
}

func (r *AnalyticsLanePeriodProjectionRepository) List(
	ctx context.Context,
	tenantID uuid.UUID,
	filter AnalyticsLaneListFilter,
) ([]domain.AnalyticsLanePeriodProjection, error) {
	query := `
		SELECT tenant_id, buyer_company_id, lane_key, transport_mode, equipment_type,
			period_start, period_grain, currency_code,
			order_count, carrier_count,
			planned_total, accrued_total, current_actual_total, final_actual_total,
			current_variance_total, final_variance_total,
			calculated_at, data_through, projection_version
		FROM freight_cost.cost_analytics_lane_period_projection
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
	if filter.TransportMode != "" {
		query += ` AND transport_mode = $` + strconv.Itoa(argIdx)
		args = append(args, filter.TransportMode)
		argIdx++
	}
	if filter.EquipmentType != "" {
		query += ` AND equipment_type = $` + strconv.Itoa(argIdx)
		args = append(args, filter.EquipmentType)
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
	return scanLaneProjections(rows)
}

type AnalyticsLaneListFilter struct {
	BuyerCompanyID *uuid.UUID
	CurrencyCode   string
	LaneKey        string
	TransportMode  string
	EquipmentType  string
	PeriodFrom     *time.Time
	PeriodTo       *time.Time
	Limit          int
	Offset         int
}

func scanLaneProjections(rows pgx.Rows) ([]domain.AnalyticsLanePeriodProjection, error) {
	var items []domain.AnalyticsLanePeriodProjection
	for rows.Next() {
		var p domain.AnalyticsLanePeriodProjection
		if err := rows.Scan(
			&p.TenantID, &p.BuyerCompanyID, &p.LaneKey, &p.TransportMode, &p.EquipmentType,
			&p.PeriodStart, &p.PeriodGrain, &p.CurrencyCode,
			&p.OrderCount, &p.CarrierCount,
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
