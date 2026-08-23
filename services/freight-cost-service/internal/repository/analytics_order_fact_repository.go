package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsOrderFactRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsOrderFactRepository(pool *pgxpool.Pool) *AnalyticsOrderFactRepository {
	return &AnalyticsOrderFactRepository{pool: pool}
}

func (r *AnalyticsOrderFactRepository) Upsert(ctx context.Context, tx pgx.Tx, fact *domain.AnalyticsOrderFact) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_order_fact (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
			period_start, period_grain,
			planned_amount, accrued_amount, current_actual_amount, final_actual_amount,
			current_variance_amount, final_variance_amount,
			data_stage, financial_finality,
			source_summary_revision, source_summary_updated_at, calculated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9, $10, $11,
			$12, $13,
			$14, $15,
			$16, $17, $18
		)
		ON CONFLICT (tenant_id, transport_order_id, currency_code) DO UPDATE SET
			buyer_company_id = EXCLUDED.buyer_company_id,
			carrier_company_id = EXCLUDED.carrier_company_id,
			period_start = EXCLUDED.period_start,
			period_grain = EXCLUDED.period_grain,
			planned_amount = EXCLUDED.planned_amount,
			accrued_amount = EXCLUDED.accrued_amount,
			current_actual_amount = EXCLUDED.current_actual_amount,
			final_actual_amount = EXCLUDED.final_actual_amount,
			current_variance_amount = EXCLUDED.current_variance_amount,
			final_variance_amount = EXCLUDED.final_variance_amount,
			data_stage = EXCLUDED.data_stage,
			financial_finality = EXCLUDED.financial_finality,
			source_summary_revision = EXCLUDED.source_summary_revision,
			source_summary_updated_at = EXCLUDED.source_summary_updated_at,
			calculated_at = EXCLUDED.calculated_at`
	_, err := tx.Exec(ctx, query,
		fact.TenantID, fact.TransportOrderID, fact.BuyerCompanyID, fact.CarrierCompanyID, fact.CurrencyCode,
		fact.PeriodStart, fact.PeriodGrain,
		decimalArg(fact.PlannedAmount), decimalArg(fact.AccruedAmount),
		decimalArg(fact.CurrentActualAmount), decimalArg(fact.FinalActualAmount),
		decimalArg(fact.CurrentVarianceAmount), decimalArg(fact.FinalVarianceAmount),
		string(fact.DataStage), string(fact.FinancialFinality),
		fact.SourceSummaryRevision, fact.SourceSummaryUpdatedAt, fact.CalculatedAt,
	)
	return mapDBError(err)
}

func (r *AnalyticsOrderFactRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_order_fact WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

type analyticsPeriodAggregateRow struct {
	OrderCount           int
	PlannedTotal         *decimal.Decimal
	AccruedTotal         *decimal.Decimal
	CurrentActualTotal   *decimal.Decimal
	FinalActualTotal     *decimal.Decimal
	CurrentVarianceTotal *decimal.Decimal
	FinalVarianceTotal   *decimal.Decimal
	MaxSourceUpdatedAt   time.Time
}

func (r *AnalyticsOrderFactRepository) AggregatePeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsPeriodKey,
) (*analyticsPeriodAggregateRow, error) {
	query := `
		SELECT
			COUNT(*)::int,
			SUM(planned_amount),
			SUM(accrued_amount),
			SUM(current_actual_amount),
			SUM(final_actual_amount),
			SUM(current_variance_amount),
			SUM(final_variance_amount),
			COALESCE(MAX(source_summary_updated_at), NOW())
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1
		  AND buyer_company_id = $2
		  AND period_start = $3
		  AND period_grain = $4
		  AND currency_code = $5`
	row := tx.QueryRow(ctx, query, key.TenantID, key.BuyerCompanyID, key.PeriodStart, key.PeriodGrain, key.CurrencyCode)
	var result analyticsPeriodAggregateRow
	if err := row.Scan(
		&result.OrderCount,
		&result.PlannedTotal,
		&result.AccruedTotal,
		&result.CurrentActualTotal,
		&result.FinalActualTotal,
		&result.CurrentVarianceTotal,
		&result.FinalVarianceTotal,
		&result.MaxSourceUpdatedAt,
	); err != nil {
		return nil, mapDBError(err)
	}
	return &result, nil
}

func (r *AnalyticsOrderFactRepository) ListDistinctPeriodKeysForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]domain.AnalyticsPeriodKey, error) {
	query := `
		SELECT DISTINCT tenant_id, buyer_company_id, period_start, period_grain, currency_code
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1
		ORDER BY buyer_company_id, period_start, currency_code`
	rows, err := tx.Query(ctx, query, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	var keys []domain.AnalyticsPeriodKey
	for rows.Next() {
		var key domain.AnalyticsPeriodKey
		if err := rows.Scan(&key.TenantID, &key.BuyerCompanyID, &key.PeriodStart, &key.PeriodGrain, &key.CurrencyCode); err != nil {
			return nil, mapDBError(err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
