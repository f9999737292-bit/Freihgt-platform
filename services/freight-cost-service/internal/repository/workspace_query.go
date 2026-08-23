package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/security"
)

const workspaceProjectionColumns = projectionSelectColumns + `, updated_at`

type WorkspaceListFilter struct {
	TenantID             uuid.UUID
	CompanyID            uuid.UUID
	ActorKind            string
	Currency             string
	CarrierID            *uuid.UUID
	ReconciliationState  string
	Limit                int
	Offset               int
}

type WorkspaceAggregate struct {
	CurrencyCode              string
	MixedCurrency             bool
	PlannedTotal              *decimal.Decimal
	AccruedTotal              *decimal.Decimal
	ForecastExposureTotal     *decimal.Decimal
	CurrentActualTotal        *decimal.Decimal
	FinalActualTotal          *decimal.Decimal
	CurrentVarianceTotal      *decimal.Decimal
	FinalVarianceTotal        *decimal.Decimal
	ReconciliationMismatchCnt int
}

type CarrierPerformanceRow struct {
	CarrierCompanyID    uuid.UUID
	OrderCount          int
	PlannedTotal        *decimal.Decimal
	CurrentActualTotal  *decimal.Decimal
	FinalActualTotal    *decimal.Decimal
	CurrentVarianceTotal *decimal.Decimal
	CurrencyCode        string
}

func (r *CostSummaryProjectionRepository) ListForWorkspace(
	ctx context.Context,
	filter WorkspaceListFilter,
) ([]*domain.CostSummaryProjection, int, error) {
	where, args := buildWorkspaceWhere(filter)
	countQuery := `SELECT COUNT(*) FROM freight_cost.cost_summary_projection WHERE ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, mapDBError(err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	listQuery := fmt.Sprintf(
		`SELECT %s FROM freight_cost.cost_summary_projection WHERE %s ORDER BY updated_at DESC LIMIT $%d OFFSET $%d`,
		workspaceProjectionColumns,
		where,
		len(args)+1,
		len(args)+2,
	)
	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	defer rows.Close()

	var items []*domain.CostSummaryProjection
	for rows.Next() {
		projection, err := scanProjectionWithUpdatedAt(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, projection)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapDBError(err)
	}
	return items, total, nil
}

func (r *CostSummaryProjectionRepository) AggregateForWorkspace(
	ctx context.Context,
	filter WorkspaceListFilter,
) (*WorkspaceAggregate, error) {
	where, args := buildWorkspaceWhere(filter)
	query := fmt.Sprintf(`
		SELECT
			COALESCE(MIN(currency_code), '') AS currency_code,
			COUNT(DISTINCT currency_code) AS currency_count,
			SUM(planned_amount),
			SUM(accrued_amount),
			SUM(forecast_exposure),
			SUM(current_actual_amount),
			SUM(final_actual_amount),
			SUM(current_variance_amount),
			SUM(final_variance_amount),
			SUM(CASE WHEN billing_reconciliation_status = 'MISMATCH' THEN 1 ELSE 0 END)
		FROM freight_cost.cost_summary_projection
		WHERE %s`, where)

	var currencyCode string
	var currencyCount int
	var planned, accrued, forecast, currentActual, finalActual, currentVar, finalVar *string
	var mismatchCount int
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&currencyCode,
		&currencyCount,
		&planned,
		&accrued,
		&forecast,
		&currentActual,
		&finalActual,
		&currentVar,
		&finalVar,
		&mismatchCount,
	)
	if err != nil {
		return nil, mapDBError(err)
	}

	return &WorkspaceAggregate{
		CurrencyCode:              currencyCode,
		MixedCurrency:             currencyCount > 1,
		PlannedTotal:              parseDecimalPtr(planned),
		AccruedTotal:              parseDecimalPtr(accrued),
		ForecastExposureTotal:     parseDecimalPtr(forecast),
		CurrentActualTotal:        parseDecimalPtr(currentActual),
		FinalActualTotal:          parseDecimalPtr(finalActual),
		CurrentVarianceTotal:      parseDecimalPtr(currentVar),
		FinalVarianceTotal:        parseDecimalPtr(finalVar),
		ReconciliationMismatchCnt: mismatchCount,
	}, nil
}

func (r *CostSummaryProjectionRepository) CarrierPerformance(
	ctx context.Context,
	filter WorkspaceListFilter,
) ([]CarrierPerformanceRow, error) {
	where, args := buildWorkspaceWhere(filter)
	query := fmt.Sprintf(`
		SELECT
			carrier_company_id,
			COUNT(*) AS order_count,
			SUM(planned_amount),
			SUM(current_actual_amount),
			SUM(final_actual_amount),
			SUM(current_variance_amount),
			MIN(currency_code)
		FROM freight_cost.cost_summary_projection
		WHERE %s
		GROUP BY carrier_company_id
		ORDER BY order_count DESC
		LIMIT 50`, where)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	var out []CarrierPerformanceRow
	for rows.Next() {
		var row CarrierPerformanceRow
		var planned, currentActual, finalActual, currentVar *string
		var currency *string
		if err := rows.Scan(
			&row.CarrierCompanyID,
			&row.OrderCount,
			&planned,
			&currentActual,
			&finalActual,
			&currentVar,
			&currency,
		); err != nil {
			return nil, mapDBError(err)
		}
		row.PlannedTotal = parseDecimalPtr(planned)
		row.CurrentActualTotal = parseDecimalPtr(currentActual)
		row.FinalActualTotal = parseDecimalPtr(finalActual)
		row.CurrentVarianceTotal = parseDecimalPtr(currentVar)
		if currency != nil {
			row.CurrencyCode = *currency
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func buildWorkspaceWhere(filter WorkspaceListFilter) (string, []any) {
	parts := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	idx := 2

	switch filter.ActorKind {
	case security.ActorKindCarrier:
		parts = append(parts, fmt.Sprintf("carrier_company_id = $%d", idx))
		args = append(args, filter.CompanyID)
		idx++
	default:
		parts = append(parts, fmt.Sprintf("buyer_company_id = $%d", idx))
		args = append(args, filter.CompanyID)
		idx++
	}

	if filter.Currency != "" {
		parts = append(parts, fmt.Sprintf("currency_code = $%d", idx))
		args = append(args, strings.ToUpper(filter.Currency))
		idx++
	}
	if filter.CarrierID != nil {
		parts = append(parts, fmt.Sprintf("carrier_company_id = $%d", idx))
		args = append(args, *filter.CarrierID)
		idx++
	}
	if filter.ReconciliationState != "" {
		parts = append(parts, fmt.Sprintf("billing_reconciliation_status = $%d", idx))
		args = append(args, filter.ReconciliationState)
		idx++
	}
	_ = idx
	return strings.Join(parts, " AND "), args
}

func scanProjectionWithUpdatedAt(row pgx.Row) (*domain.CostSummaryProjection, error) {
	var projection domain.CostSummaryProjection
	var currencyCode *string
	var planned, accrued, currentActual, finalActual, billed, payable, paid *string
	var currentVariance, finalVariance, forecast *string
	var currentPercent, finalPercent *string
	var forecastSourceStatus, derivedFingerprint *string
	var mappingEvaluatedAt *time.Time
	var reconciliationStatus, finality, dataStage string
	var sourcesRaw []byte
	var updatedAt time.Time

	err := row.Scan(
		&projection.TenantID, &projection.TransportOrderID, &projection.BuyerCompanyID, &projection.CarrierCompanyID,
		&currencyCode,
		&planned, &accrued, &currentActual, &finalActual,
		&billed, &payable, &paid,
		&reconciliationStatus, &finality, &dataStage, &sourcesRaw,
		&currentVariance, &finalVariance, &currentPercent, &finalPercent,
		&forecast, &forecastSourceStatus, &derivedFingerprint,
		&projection.AttributionMappingVersion, &mappingEvaluatedAt, &projection.ProjectionRevision,
		&updatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	if currencyCode != nil {
		projection.CurrencyCode = *currencyCode
	}
	projection.PlannedAmount = parseDecimalPtr(planned)
	projection.AccruedAmount = parseDecimalPtr(accrued)
	projection.CurrentActualAmount = parseDecimalPtr(currentActual)
	projection.FinalActualAmount = parseDecimalPtr(finalActual)
	projection.BillingRegisterAmount = parseDecimalPtr(billed)
	projection.PayableAmount = parseDecimalPtr(payable)
	projection.PaidAmount = parseDecimalPtr(paid)
	projection.CurrentVarianceAmount = parseDecimalPtr(currentVariance)
	projection.FinalVarianceAmount = parseDecimalPtr(finalVariance)
	projection.CurrentVariancePercent = parsePercentPtr(currentPercent)
	projection.FinalVariancePercent = parsePercentPtr(finalPercent)
	projection.ForecastExposure = parseDecimalPtr(forecast)
	if forecastSourceStatus != nil {
		projection.ForecastSourceStatus = *forecastSourceStatus
	}
	projection.DerivedStateFingerprint = derivedFingerprint
	projection.AttributionMappingEvaluatedAt = mappingEvaluatedAt
	projection.BillingReconciliationStatus = domain.BillingReconciliationStatus(reconciliationStatus)
	projection.FinancialFinality = domain.FinancialFinality(finality)
	projection.DataStage = domain.DataStage(dataStage)
	if len(sourcesRaw) > 0 {
		_ = json.Unmarshal(sourcesRaw, &projection.SourcesAvailable)
	}
	if projection.BillingRegisterAmount != nil {
		projection.SettlementLinked = true
	}
	return &projection, nil
}
