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

type AnalyticsAccessorialFactRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsAccessorialFactRepository(pool *pgxpool.Pool) *AnalyticsAccessorialFactRepository {
	return &AnalyticsAccessorialFactRepository{pool: pool}
}

func (r *AnalyticsAccessorialFactRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_accessorial_fact WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsAccessorialFactRepository) DeleteByTransportOrder(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.cost_analytics_accessorial_fact
		WHERE tenant_id = $1 AND transport_order_id = $2`, tenantID, transportOrderID)
	return mapDBError(err)
}

func (r *AnalyticsAccessorialFactRepository) Upsert(ctx context.Context, tx pgx.Tx, fact *domain.AnalyticsAccessorialFact) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_accessorial_fact (
			tenant_id, accessorial_id, currency_code, transport_order_id, buyer_company_id,
			settlement_id, charge_code, normalized_category, amount, status,
			mapping_version, mapping_evaluated_at, period_start, period_grain, eligible, calculated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		ON CONFLICT (tenant_id, accessorial_id, currency_code) DO UPDATE SET
			transport_order_id = EXCLUDED.transport_order_id,
			buyer_company_id = EXCLUDED.buyer_company_id,
			settlement_id = EXCLUDED.settlement_id,
			charge_code = EXCLUDED.charge_code,
			normalized_category = EXCLUDED.normalized_category,
			amount = EXCLUDED.amount,
			status = EXCLUDED.status,
			mapping_version = EXCLUDED.mapping_version,
			mapping_evaluated_at = EXCLUDED.mapping_evaluated_at,
			period_start = EXCLUDED.period_start,
			period_grain = EXCLUDED.period_grain,
			eligible = EXCLUDED.eligible,
			calculated_at = EXCLUDED.calculated_at`
	_, err := tx.Exec(ctx, query,
		fact.TenantID, fact.AccessorialID, fact.CurrencyCode, fact.TransportOrderID, fact.BuyerCompanyID,
		fact.SettlementID, fact.ChargeCode, fact.NormalizedCategory, fact.Amount.StringFixed(domain.MoneyScale),
		fact.Status, fact.MappingVersion, fact.MappingEvaluatedAt, fact.PeriodStart, fact.PeriodGrain,
		fact.Eligible, fact.CalculatedAt,
	)
	return mapDBError(err)
}

func (r *AnalyticsAccessorialFactRepository) ListByTransportOrder(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) ([]domain.AnalyticsAccessorialFact, error) {
	rows, err := tx.Query(ctx, `
		SELECT tenant_id, accessorial_id, currency_code, transport_order_id, buyer_company_id,
			settlement_id, charge_code, normalized_category, amount, status,
			mapping_version, mapping_evaluated_at, period_start, period_grain, eligible, calculated_at
		FROM freight_cost.cost_analytics_accessorial_fact
		WHERE tenant_id = $1 AND transport_order_id = $2`, tenantID, transportOrderID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanAccessorialFacts(rows)
}

func (r *AnalyticsAccessorialFactRepository) ListDistinctPeriodKeysForTenant(
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

type analyticsAccessorialAggregateRow struct {
	TotalAmount        *decimal.Decimal
	OrderCount         int
	LineCount          int
	MaxCalculatedAt    time.Time
}

func (r *AnalyticsAccessorialFactRepository) AggregatePeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsAccessorialPeriodKey,
) (*analyticsAccessorialAggregateRow, error) {
	row := tx.QueryRow(ctx, `
		SELECT
			SUM(amount),
			COUNT(DISTINCT transport_order_id)::int,
			COUNT(*)::int,
			COALESCE(MAX(calculated_at), NOW())
		FROM freight_cost.cost_analytics_accessorial_fact
		WHERE tenant_id = $1
		  AND buyer_company_id = $2
		  AND normalized_category = $3
		  AND period_start = $4
		  AND period_grain = $5
		  AND currency_code = $6
		  AND eligible = TRUE`,
		key.TenantID, key.BuyerCompanyID, key.NormalizedCategory,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	var result analyticsAccessorialAggregateRow
	if err := row.Scan(&result.TotalAmount, &result.OrderCount, &result.LineCount, &result.MaxCalculatedAt); err != nil {
		return nil, mapDBError(err)
	}
	return &result, nil
}

func scanAccessorialFacts(rows pgx.Rows) ([]domain.AnalyticsAccessorialFact, error) {
	var facts []domain.AnalyticsAccessorialFact
	for rows.Next() {
		var fact domain.AnalyticsAccessorialFact
		var amount decimal.Decimal
		if err := rows.Scan(
			&fact.TenantID, &fact.AccessorialID, &fact.CurrencyCode, &fact.TransportOrderID, &fact.BuyerCompanyID,
			&fact.SettlementID, &fact.ChargeCode, &fact.NormalizedCategory, &amount, &fact.Status,
			&fact.MappingVersion, &fact.MappingEvaluatedAt, &fact.PeriodStart, &fact.PeriodGrain,
			&fact.Eligible, &fact.CalculatedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		fact.Amount = amount
		facts = append(facts, fact)
	}
	return facts, mapDBError(rows.Err())
}
