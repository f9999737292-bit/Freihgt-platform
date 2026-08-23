package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsPeriodProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsPeriodProjectionRepository(pool *pgxpool.Pool) *AnalyticsPeriodProjectionRepository {
	return &AnalyticsPeriodProjectionRepository{pool: pool}
}

func (r *AnalyticsPeriodProjectionRepository) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.AnalyticsPeriodProjection,
) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_period_projection (
			tenant_id, buyer_company_id, period_start, period_grain, currency_code,
			order_count,
			planned_total, accrued_total, current_actual_total, final_actual_total,
			current_variance_total, final_variance_total,
			reconciliation_open_count,
			calculated_at, data_through, projection_version
		) VALUES (
			$1, $2, $3, $4, $5,
			$6,
			$7, $8, $9, $10,
			$11, $12,
			$13,
			$14, $15, $16
		)
		ON CONFLICT (tenant_id, buyer_company_id, period_start, period_grain, currency_code) DO UPDATE SET
			order_count = EXCLUDED.order_count,
			planned_total = EXCLUDED.planned_total,
			accrued_total = EXCLUDED.accrued_total,
			current_actual_total = EXCLUDED.current_actual_total,
			final_actual_total = EXCLUDED.final_actual_total,
			current_variance_total = EXCLUDED.current_variance_total,
			final_variance_total = EXCLUDED.final_variance_total,
			reconciliation_open_count = EXCLUDED.reconciliation_open_count,
			calculated_at = EXCLUDED.calculated_at,
			data_through = EXCLUDED.data_through,
			projection_version = EXCLUDED.projection_version`
	_, err := tx.Exec(ctx, query,
		projection.TenantID, projection.BuyerCompanyID, projection.PeriodStart, projection.PeriodGrain, projection.CurrencyCode,
		projection.OrderCount,
		decimalArg(projection.PlannedTotal), decimalArg(projection.AccruedTotal),
		decimalArg(projection.CurrentActualTotal), decimalArg(projection.FinalActualTotal),
		decimalArg(projection.CurrentVarianceTotal), decimalArg(projection.FinalVarianceTotal),
		projection.ReconciliationOpenCount,
		projection.CalculatedAt, projection.DataThrough, projection.ProjectionVersion,
	)
	return mapDBError(err)
}

func (r *AnalyticsPeriodProjectionRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_period_projection WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsPeriodProjectionRepository) CountOpenReconciliations(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsPeriodKey,
) (int, error) {
	query := `
		SELECT COUNT(DISTINCT rf.finding_id)::int
		FROM freight_cost.reconciliation_finding rf
		INNER JOIN freight_cost.cost_analytics_order_fact af
			ON af.tenant_id = rf.tenant_id
		   AND af.transport_order_id = rf.transport_order_id
		WHERE rf.tenant_id = $1
		  AND rf.status IN ('OPEN', 'REOPENED')
		  AND af.buyer_company_id = $2
		  AND af.period_start = $3
		  AND af.period_grain = $4
		  AND af.currency_code = $5`
	var count int
	if err := tx.QueryRow(ctx, query, key.TenantID, key.BuyerCompanyID, key.PeriodStart, key.PeriodGrain, key.CurrencyCode).Scan(&count); err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}

func (r *AnalyticsPeriodProjectionRepository) GetByKey(
	ctx context.Context,
	tenantID uuid.UUID,
	key domain.AnalyticsPeriodKey,
) (*domain.AnalyticsPeriodProjection, error) {
	query := `
		SELECT tenant_id, buyer_company_id, period_start, period_grain, currency_code,
			order_count, planned_total, accrued_total, current_actual_total, final_actual_total,
			current_variance_total, final_variance_total, reconciliation_open_count,
			calculated_at, data_through, projection_version
		FROM freight_cost.cost_analytics_period_projection
		WHERE tenant_id = $1 AND buyer_company_id = $2 AND period_start = $3
		  AND period_grain = $4 AND currency_code = $5`
	row := r.pool.QueryRow(ctx, query, tenantID, key.BuyerCompanyID, key.PeriodStart, key.PeriodGrain, key.CurrencyCode)
	var projection domain.AnalyticsPeriodProjection
	if err := row.Scan(
		&projection.TenantID, &projection.BuyerCompanyID, &projection.PeriodStart, &projection.PeriodGrain, &projection.CurrencyCode,
		&projection.OrderCount, &projection.PlannedTotal, &projection.AccruedTotal, &projection.CurrentActualTotal, &projection.FinalActualTotal,
		&projection.CurrentVarianceTotal, &projection.FinalVarianceTotal, &projection.ReconciliationOpenCount,
		&projection.CalculatedAt, &projection.DataThrough, &projection.ProjectionVersion,
	); err != nil {
		return nil, mapDBError(err)
	}
	return &projection, nil
}
