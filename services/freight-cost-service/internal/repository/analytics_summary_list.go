package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsSummaryRow struct {
	Projection *domain.CostSummaryProjection
	UpdatedAt  time.Time
}

// ListSummariesForTenant returns cost_summary_projection rows for tenant rebuild.
func (r *CostSummaryProjectionRepository) ListSummariesForTenant(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
) ([]AnalyticsSummaryRow, error) {
	query := fmt.Sprintf(`
		SELECT %s, updated_at
		FROM freight_cost.cost_summary_projection
		WHERE tenant_id = $1
		  AND currency_code IS NOT NULL
		  AND TRIM(currency_code) <> ''
		ORDER BY transport_order_id`, projectionSelectColumns)
	var rows pgx.Rows
	var err error
	if tx != nil {
		rows, err = tx.Query(ctx, query, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, query, tenantID)
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	var items []AnalyticsSummaryRow
	for rows.Next() {
		projection, updatedAt, err := scanProjectionRowWithUpdatedAt(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, AnalyticsSummaryRow{Projection: projection, UpdatedAt: updatedAt})
	}
	return items, mapDBError(rows.Err())
}

func scanProjectionRowWithUpdatedAt(row pgx.Row) (*domain.CostSummaryProjection, time.Time, error) {
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
		return nil, time.Time{}, mapDBError(err)
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
	return &projection, updatedAt, nil
}
