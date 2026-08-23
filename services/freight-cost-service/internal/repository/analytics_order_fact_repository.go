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

const analyticsDimensionBatchSize = 500

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
			source_summary_revision, source_summary_updated_at, calculated_at,
			lane_key, origin_country, origin_city, destination_country, destination_city,
			transport_mode, equipment_type, lane_eligible,
			order_reference, carrier_display_name, lane_label
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29
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
			calculated_at = EXCLUDED.calculated_at,
			lane_key = EXCLUDED.lane_key,
			origin_country = EXCLUDED.origin_country,
			origin_city = EXCLUDED.origin_city,
			destination_country = EXCLUDED.destination_country,
			destination_city = EXCLUDED.destination_city,
			transport_mode = EXCLUDED.transport_mode,
			equipment_type = EXCLUDED.equipment_type,
			lane_eligible = EXCLUDED.lane_eligible,
			order_reference = EXCLUDED.order_reference,
			carrier_display_name = EXCLUDED.carrier_display_name,
			lane_label = EXCLUDED.lane_label`
	_, err := tx.Exec(ctx, query,
		fact.TenantID, fact.TransportOrderID, fact.BuyerCompanyID, fact.CarrierCompanyID, fact.CurrencyCode,
		fact.PeriodStart, fact.PeriodGrain,
		decimalArg(fact.PlannedAmount), decimalArg(fact.AccruedAmount),
		decimalArg(fact.CurrentActualAmount), decimalArg(fact.FinalActualAmount),
		decimalArg(fact.CurrentVarianceAmount), decimalArg(fact.FinalVarianceAmount),
		string(fact.DataStage), string(fact.FinancialFinality),
		fact.SourceSummaryRevision, fact.SourceSummaryUpdatedAt, fact.CalculatedAt,
		nullableString(fact.LaneKey), nullableString(fact.OriginCountry), nullableString(fact.OriginCity),
		nullableString(fact.DestinationCountry), nullableString(fact.DestinationCity),
		nullableString(fact.TransportMode), nullableString(fact.EquipmentType), fact.LaneEligible,
		nullableString(fact.OrderReference), nullableString(fact.CarrierDisplayName), nullableString(fact.LaneLabel),
	)
	return mapDBError(err)
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

const analyticsOrderFactSelectColumns = `
		tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
		period_start, period_grain,
		planned_amount, accrued_amount, current_actual_amount, final_actual_amount,
		current_variance_amount, final_variance_amount,
		data_stage, financial_finality,
		source_summary_revision, source_summary_updated_at, calculated_at,
		lane_key, origin_country, origin_city, destination_country, destination_city,
		transport_mode, equipment_type, lane_eligible,
		order_reference, carrier_display_name, lane_label`

func (r *AnalyticsOrderFactRepository) GetByKey(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
	currencyCode string,
) (*domain.AnalyticsOrderFact, error) {
	query := `SELECT ` + analyticsOrderFactSelectColumns + `
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND transport_order_id = $2 AND currency_code = $3`
	row := tx.QueryRow(ctx, query, tenantID, transportOrderID, currencyCode)
	return scanAnalyticsOrderFact(row)
}

func (r *AnalyticsOrderFactRepository) GetByTransportOrder(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
	currencyCode string,
) (*domain.AnalyticsOrderFact, error) {
	query := `SELECT ` + analyticsOrderFactSelectColumns + `
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND transport_order_id = $2 AND currency_code = $3`
	row := r.pool.QueryRow(ctx, query, tenantID, transportOrderID, currencyCode)
	return scanAnalyticsOrderFact(row)
}

func (r *AnalyticsOrderFactRepository) ListTransportOrderIDsForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT transport_order_id
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapDBError(err)
		}
		ids = append(ids, id)
	}
	return ids, mapDBError(rows.Err())
}

func (r *AnalyticsOrderFactRepository) ListForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]*domain.AnalyticsOrderFact, error) {
	rows, err := tx.Query(ctx, `SELECT `+analyticsOrderFactSelectColumns+`
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var facts []*domain.AnalyticsOrderFact
	for rows.Next() {
		fact, err := scanAnalyticsOrderFact(rows)
		if err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, mapDBError(rows.Err())
}

func (r *AnalyticsOrderFactRepository) ListLaneLabels(
	ctx context.Context,
	tenantID, buyerCompanyID uuid.UUID,
) (map[string]*string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (lane_key) lane_key, lane_label
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND buyer_company_id = $2 AND lane_key IS NOT NULL
		ORDER BY lane_key, source_summary_updated_at DESC`, tenantID, buyerCompanyID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := map[string]*string{}
	for rows.Next() {
		var laneKey string
		var label *string
		if err := rows.Scan(&laneKey, &label); err != nil {
			return nil, mapDBError(err)
		}
		out[laneKey] = label
	}
	return out, mapDBError(rows.Err())
}

func (r *AnalyticsOrderFactRepository) ListCarrierDisplayNames(
	ctx context.Context,
	tenantID, buyerCompanyID uuid.UUID,
) (map[uuid.UUID]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (carrier_company_id) carrier_company_id, carrier_display_name
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND buyer_company_id = $2
		ORDER BY carrier_company_id, source_summary_updated_at DESC`, tenantID, buyerCompanyID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]string{}
	for rows.Next() {
		var carrierID uuid.UUID
		var name *string
		if err := rows.Scan(&carrierID, &name); err != nil {
			return nil, mapDBError(err)
		}
		if name != nil {
			out[carrierID] = *name
		}
	}
	return out, mapDBError(rows.Err())
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

type analyticsLaneAggregateRow struct {
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

func (r *AnalyticsOrderFactRepository) AggregateLanePeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsLanePeriodKey,
) (*analyticsLaneAggregateRow, error) {
	query := `
		SELECT
			COUNT(*)::int,
			COUNT(DISTINCT carrier_company_id) FILTER (
				WHERE carrier_company_id IS NOT NULL
				  AND carrier_company_id <> '00000000-0000-0000-0000-000000000000'
			)::int,
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
		  AND lane_key = $3
		  AND transport_mode = $4
		  AND equipment_type = $5
		  AND period_start = $6
		  AND period_grain = $7
		  AND currency_code = $8
		  AND lane_eligible = TRUE`
	row := tx.QueryRow(ctx, query,
		key.TenantID, key.BuyerCompanyID, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	var result analyticsLaneAggregateRow
	if err := row.Scan(
		&result.OrderCount,
		&result.CarrierCount,
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

type analyticsCarrierAggregateRow struct {
	OrderCount           int
	LaneCount            int
	PlannedTotal         *decimal.Decimal
	AccruedTotal         *decimal.Decimal
	CurrentActualTotal   *decimal.Decimal
	FinalActualTotal     *decimal.Decimal
	CurrentVarianceTotal *decimal.Decimal
	FinalVarianceTotal   *decimal.Decimal
	MaxSourceUpdatedAt   time.Time
}

func (r *AnalyticsOrderFactRepository) AggregateCarrierPeriod(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsCarrierPeriodKey,
) (*analyticsCarrierAggregateRow, error) {
	query := `
		SELECT
			COUNT(*)::int,
			COUNT(DISTINCT lane_key) FILTER (WHERE lane_eligible = TRUE AND lane_key IS NOT NULL)::int,
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
		  AND carrier_company_id = $3
		  AND period_start = $4
		  AND period_grain = $5
		  AND currency_code = $6
		  AND carrier_company_id IS NOT NULL
		  AND carrier_company_id <> '00000000-0000-0000-0000-000000000000'`
	row := tx.QueryRow(ctx, query,
		key.TenantID, key.BuyerCompanyID, key.CarrierCompanyID,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	var result analyticsCarrierAggregateRow
	if err := row.Scan(
		&result.OrderCount,
		&result.LaneCount,
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

func scanAnalyticsOrderFact(row pgx.Row) (*domain.AnalyticsOrderFact, error) {
	var fact domain.AnalyticsOrderFact
	var dataStage, finality string
	var laneKey, originCountry, originCity, destCountry, destCity, mode, equipment *string
	var orderReference, carrierDisplayName, laneLabel *string
	err := row.Scan(
		&fact.TenantID, &fact.TransportOrderID, &fact.BuyerCompanyID, &fact.CarrierCompanyID, &fact.CurrencyCode,
		&fact.PeriodStart, &fact.PeriodGrain,
		&fact.PlannedAmount, &fact.AccruedAmount, &fact.CurrentActualAmount, &fact.FinalActualAmount,
		&fact.CurrentVarianceAmount, &fact.FinalVarianceAmount,
		&dataStage, &finality,
		&fact.SourceSummaryRevision, &fact.SourceSummaryUpdatedAt, &fact.CalculatedAt,
		&laneKey, &originCountry, &originCity, &destCountry, &destCity, &mode, &equipment, &fact.LaneEligible,
		&orderReference, &carrierDisplayName, &laneLabel,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	fact.DataStage = domain.DataStage(dataStage)
	fact.FinancialFinality = domain.FinancialFinality(finality)
	fact.LaneKey = laneKey
	fact.OriginCountry = originCountry
	fact.OriginCity = originCity
	fact.DestinationCountry = destCountry
	fact.DestinationCity = destCity
	fact.TransportMode = mode
	fact.EquipmentType = equipment
	fact.OrderReference = orderReference
	fact.CarrierDisplayName = carrierDisplayName
	fact.LaneLabel = laneLabel
	return &fact, nil
}

func chunkUUIDs(ids []uuid.UUID, size int) [][]uuid.UUID {
	if size <= 0 {
		size = analyticsDimensionBatchSize
	}
	var chunks [][]uuid.UUID
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunk := make([]uuid.UUID, end-start)
		copy(chunk, ids[start:end])
		chunks = append(chunks, chunk)
	}
	return chunks
}

func (r *AnalyticsOrderFactRepository) ListForLaneBenchmarkKey(
	ctx context.Context,
	tx pgx.Tx,
	key domain.AnalyticsBenchmarkKey,
) ([]*domain.AnalyticsOrderFact, error) {
	rows, err := tx.Query(ctx, `SELECT `+analyticsOrderFactSelectColumns+`
		FROM freight_cost.cost_analytics_order_fact
		WHERE tenant_id = $1 AND buyer_company_id = $2
		  AND lane_key = $3 AND transport_mode = $4 AND equipment_type = $5
		  AND period_start = $6 AND period_grain = $7 AND currency_code = $8
		  AND lane_eligible = TRUE
		  AND (
			(financial_finality = 'FINAL_ACTUAL' AND final_actual_amount IS NOT NULL)
			OR (financial_finality = 'CURRENT_ACTUAL' AND current_actual_amount IS NOT NULL)
		  )`,
		key.TenantID, key.BuyerCompanyID, key.LaneKey, key.TransportMode, key.EquipmentType,
		key.PeriodStart, key.PeriodGrain, key.CurrencyCode,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var facts []*domain.AnalyticsOrderFact
	for rows.Next() {
		fact, err := scanAnalyticsOrderFact(rows)
		if err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, mapDBError(rows.Err())
}
