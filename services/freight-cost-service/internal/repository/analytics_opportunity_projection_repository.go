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

type AnalyticsOpportunityProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsOpportunityProjectionRepository(pool *pgxpool.Pool) *AnalyticsOpportunityProjectionRepository {
	return &AnalyticsOpportunityProjectionRepository{pool: pool}
}

func (r *AnalyticsOpportunityProjectionRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_opportunity_projection WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}

func (r *AnalyticsOpportunityProjectionRepository) DeleteExceptIDs(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	keepIDs []uuid.UUID,
) error {
	if len(keepIDs) == 0 {
		_, err := tx.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_opportunity_projection WHERE tenant_id = $1`, tenantID)
		return mapDBError(err)
	}
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.cost_analytics_opportunity_projection
		WHERE tenant_id = $1 AND NOT (opportunity_id = ANY($2))`, tenantID, keepIDs)
	return mapDBError(err)
}

func (r *AnalyticsOpportunityProjectionRepository) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	projection *domain.AnalyticsOpportunityProjection,
) error {
	query := `
		INSERT INTO freight_cost.cost_analytics_opportunity_projection (
			tenant_id, buyer_company_id, opportunity_id, opportunity_type, scope, entity_key,
			currency_code, transport_order_id, carrier_company_id, lane_key,
			period_start, period_grain,
			observed_amount, baseline_amount, estimated_delta, sample_size,
			data_quality, rule_version, evidence_json,
			calculated_at, data_through, projection_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		ON CONFLICT (tenant_id, opportunity_id) DO UPDATE SET
			opportunity_type = EXCLUDED.opportunity_type,
			scope = EXCLUDED.scope,
			entity_key = EXCLUDED.entity_key,
			currency_code = EXCLUDED.currency_code,
			transport_order_id = EXCLUDED.transport_order_id,
			carrier_company_id = EXCLUDED.carrier_company_id,
			lane_key = EXCLUDED.lane_key,
			period_start = EXCLUDED.period_start,
			period_grain = EXCLUDED.period_grain,
			observed_amount = EXCLUDED.observed_amount,
			baseline_amount = EXCLUDED.baseline_amount,
			estimated_delta = EXCLUDED.estimated_delta,
			sample_size = EXCLUDED.sample_size,
			data_quality = EXCLUDED.data_quality,
			rule_version = EXCLUDED.rule_version,
			evidence_json = EXCLUDED.evidence_json,
			calculated_at = EXCLUDED.calculated_at,
			data_through = EXCLUDED.data_through,
			projection_version = EXCLUDED.projection_version`
	_, err := tx.Exec(ctx, query,
		projection.TenantID, projection.BuyerCompanyID, projection.OpportunityID,
		projection.OpportunityType, projection.Scope, projection.EntityKey,
		projection.CurrencyCode,
		uuidArg(projection.TransportOrderID), uuidArg(projection.CarrierCompanyID), nullableString(projection.LaneKey),
		projection.PeriodStart, projection.PeriodGrain,
		projection.ObservedAmount.StringFixed(domain.MoneyScale),
		projection.BaselineAmount.StringFixed(domain.MoneyScale),
		projection.EstimatedDelta.StringFixed(domain.MoneyScale),
		projection.SampleSize, projection.DataQuality, projection.RuleVersion, projection.EvidenceJSON,
		projection.CalculatedAt, projection.DataThrough, projection.ProjectionVersion,
	)
	return mapDBError(err)
}

type AnalyticsOpportunityListFilter struct {
	BuyerCompanyID  *uuid.UUID
	CurrencyCode    string
	OpportunityType string
	PeriodFrom      *time.Time
	PeriodTo        *time.Time
	Limit           int
	Offset          int
}

func (r *AnalyticsOpportunityProjectionRepository) List(
	ctx context.Context,
	tenantID uuid.UUID,
	filter AnalyticsOpportunityListFilter,
) ([]domain.AnalyticsOpportunityProjection, error) {
	query := `
		SELECT tenant_id, buyer_company_id, opportunity_id, opportunity_type, scope, entity_key,
			currency_code, transport_order_id, carrier_company_id, lane_key,
			period_start, period_grain,
			observed_amount, baseline_amount, estimated_delta, sample_size,
			data_quality, rule_version, evidence_json,
			calculated_at, data_through, projection_version
		FROM freight_cost.cost_analytics_opportunity_projection
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
	if filter.OpportunityType != "" {
		query += ` AND opportunity_type = $` + strconv.Itoa(argIdx)
		args = append(args, filter.OpportunityType)
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
	query += ` ORDER BY estimated_delta DESC, calculated_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
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
	return scanOpportunityProjections(rows)
}

func scanOpportunityProjections(rows pgx.Rows) ([]domain.AnalyticsOpportunityProjection, error) {
	var items []domain.AnalyticsOpportunityProjection
	for rows.Next() {
		var p domain.AnalyticsOpportunityProjection
		var transportOrderID, carrierCompanyID *uuid.UUID
		var laneKey *string
		if err := rows.Scan(
			&p.TenantID, &p.BuyerCompanyID, &p.OpportunityID, &p.OpportunityType, &p.Scope, &p.EntityKey,
			&p.CurrencyCode, &transportOrderID, &carrierCompanyID, &laneKey,
			&p.PeriodStart, &p.PeriodGrain,
			&p.ObservedAmount, &p.BaselineAmount, &p.EstimatedDelta, &p.SampleSize,
			&p.DataQuality, &p.RuleVersion, &p.EvidenceJSON,
			&p.CalculatedAt, &p.DataThrough, &p.ProjectionVersion,
		); err != nil {
			return nil, mapDBError(err)
		}
		p.TransportOrderID = transportOrderID
		p.CarrierCompanyID = carrierCompanyID
		p.LaneKey = laneKey
		items = append(items, p)
	}
	return items, mapDBError(rows.Err())
}
