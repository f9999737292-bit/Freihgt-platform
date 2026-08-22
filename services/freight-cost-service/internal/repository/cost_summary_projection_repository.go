package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

type CostSummaryProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewCostSummaryProjectionRepository(pool *pgxpool.Pool) *CostSummaryProjectionRepository {
	return &CostSummaryProjectionRepository{pool: pool}
}

const projectionSelectColumns = `
		tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
		planned_amount, accrued_amount, current_actual_amount, final_actual_amount,
		billing_register_amount, payable_amount, paid_amount,
		billing_reconciliation_status, financial_finality, data_stage, sources_available,
		current_variance_amount, final_variance_amount, current_variance_percent, final_variance_percent,
		forecast_exposure, forecast_source_status, derived_state_fingerprint,
		attribution_mapping_version, projection_revision`

func (r *CostSummaryProjectionRepository) GetByTransportOrder(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) (*domain.CostSummaryProjection, error) {
	query := `SELECT ` + projectionSelectColumns + `
		FROM freight_cost.cost_summary_projection
		WHERE tenant_id = $1 AND transport_order_id = $2`
	row := r.pool.QueryRow(ctx, query, tenantID, transportOrderID)
	return scanProjection(row)
}

func (r *CostSummaryProjectionRepository) GetByTransportOrderTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) (*domain.CostSummaryProjection, error) {
	query := `SELECT ` + projectionSelectColumns + `
		FROM freight_cost.cost_summary_projection
		WHERE tenant_id = $1 AND transport_order_id = $2`
	row := tx.QueryRow(ctx, query, tenantID, transportOrderID)
	return scanProjection(row)
}

func (r *CostSummaryProjectionRepository) Upsert(ctx context.Context, tx pgx.Tx, projection *domain.CostSummaryProjection) error {
	sourcesRaw, err := json.Marshal(projection.SourcesAvailable)
	if err != nil {
		return apperrors.Internal("marshal sources_available", err)
	}
	query := `
		INSERT INTO freight_cost.cost_summary_projection (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
			planned_amount, accrued_amount, current_actual_amount, final_actual_amount,
			billing_register_amount, payable_amount, paid_amount,
			billing_reconciliation_status, financial_finality, data_stage, sources_available,
			current_variance_amount, final_variance_amount, current_variance_percent, final_variance_percent,
			forecast_exposure, forecast_source_status, derived_state_fingerprint,
			attribution_mapping_version, projection_revision, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26
		)
		ON CONFLICT (tenant_id, transport_order_id) DO UPDATE SET
			buyer_company_id = EXCLUDED.buyer_company_id,
			carrier_company_id = EXCLUDED.carrier_company_id,
			currency_code = EXCLUDED.currency_code,
			planned_amount = EXCLUDED.planned_amount,
			accrued_amount = EXCLUDED.accrued_amount,
			current_actual_amount = EXCLUDED.current_actual_amount,
			final_actual_amount = EXCLUDED.final_actual_amount,
			billing_register_amount = EXCLUDED.billing_register_amount,
			payable_amount = EXCLUDED.payable_amount,
			paid_amount = EXCLUDED.paid_amount,
			billing_reconciliation_status = EXCLUDED.billing_reconciliation_status,
			financial_finality = EXCLUDED.financial_finality,
			data_stage = EXCLUDED.data_stage,
			sources_available = EXCLUDED.sources_available,
			current_variance_amount = EXCLUDED.current_variance_amount,
			final_variance_amount = EXCLUDED.final_variance_amount,
			current_variance_percent = EXCLUDED.current_variance_percent,
			final_variance_percent = EXCLUDED.final_variance_percent,
			forecast_exposure = EXCLUDED.forecast_exposure,
			forecast_source_status = EXCLUDED.forecast_source_status,
			derived_state_fingerprint = EXCLUDED.derived_state_fingerprint,
			attribution_mapping_version = EXCLUDED.attribution_mapping_version,
			projection_revision = EXCLUDED.projection_revision,
			updated_at = EXCLUDED.updated_at`
	args := []any{
		projection.TenantID, projection.TransportOrderID, projection.BuyerCompanyID, projection.CarrierCompanyID,
		nullIfEmpty(projection.CurrencyCode),
		decimalArg(projection.PlannedAmount), decimalArg(projection.AccruedAmount),
		decimalArg(projection.CurrentActualAmount), decimalArg(projection.FinalActualAmount),
		decimalArg(projection.BillingRegisterAmount), decimalArg(projection.PayableAmount), decimalArg(projection.PaidAmount),
		string(projection.BillingReconciliationStatus), string(projection.FinancialFinality), string(projection.DataStage),
		sourcesRaw,
		decimalArg(projection.CurrentVarianceAmount), decimalArg(projection.FinalVarianceAmount),
		percentArg(projection.CurrentVariancePercent), percentArg(projection.FinalVariancePercent),
		decimalArg(projection.ForecastExposure), nullIfEmpty(projection.ForecastSourceStatus), projection.DerivedStateFingerprint,
		projection.AttributionMappingVersion, projection.ProjectionRevision,
		time.Now().UTC(),
	}
	if tx != nil {
		_, err = tx.Exec(ctx, query, args...)
	} else {
		_, err = r.pool.Exec(ctx, query, args...)
	}
	return mapDBError(err)
}

func (r *CostSummaryProjectionRepository) GetOrInit(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID, buyerCompanyID, carrierCompanyID uuid.UUID,
) (*domain.CostSummaryProjection, error) {
	projection, err := r.GetByTransportOrderTx(ctx, tx, tenantID, transportOrderID)
	if err == nil {
		return projection, nil
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
		return &domain.CostSummaryProjection{
			TenantID:                    tenantID,
			TransportOrderID:            transportOrderID,
			BuyerCompanyID:              buyerCompanyID,
			CarrierCompanyID:            carrierCompanyID,
			BillingReconciliationStatus: domain.BillingReconciliationUnlinked,
			FinancialFinality:           domain.FinancialFinalityNotEvaluated,
			DataStage:                   domain.DataStagePlannedOnly,
			SourcesAvailable:            []string{},
			ProjectionRevision:          0,
		}, nil
	}
	return nil, err
}

func scanProjection(row pgx.Row) (*domain.CostSummaryProjection, error) {
	var projection domain.CostSummaryProjection
	var currencyCode *string
	var planned, accrued, currentActual, finalActual, billed, payable, paid *string
	var currentVariance, finalVariance, forecast *string
	var currentPercent, finalPercent *string
	var forecastSourceStatus, derivedFingerprint *string
	var reconciliationStatus, finality, dataStage string
	var sourcesRaw []byte

	err := row.Scan(
		&projection.TenantID, &projection.TransportOrderID, &projection.BuyerCompanyID, &projection.CarrierCompanyID,
		&currencyCode,
		&planned, &accrued, &currentActual, &finalActual,
		&billed, &payable, &paid,
		&reconciliationStatus, &finality, &dataStage, &sourcesRaw,
		&currentVariance, &finalVariance, &currentPercent, &finalPercent,
		&forecast, &forecastSourceStatus, &derivedFingerprint,
		&projection.AttributionMappingVersion, &projection.ProjectionRevision,
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

func percentArg(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return value.StringFixed(4)
}

func parsePercentPtr(raw *string) *decimal.Decimal {
	if raw == nil {
		return nil
	}
	parsed, err := decimal.NewFromString(*raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func decimalArg(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return value.StringFixed(domain.MoneyScale)
}

func parseDecimalPtr(raw *string) *decimal.Decimal {
	if raw == nil {
		return nil
	}
	parsed, err := domain.ParseMoneyAmount(*raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
