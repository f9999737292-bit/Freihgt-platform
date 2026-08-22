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

func (r *CostSummaryProjectionRepository) GetByTransportOrder(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
) (*domain.CostSummaryProjection, error) {
	query := `
		SELECT tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
		       planned_amount, accrued_amount, current_actual_amount, final_actual_amount,
		       billing_register_amount, payable_amount, paid_amount,
		       billing_reconciliation_status, financial_finality, data_stage, sources_available
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
	query := `
		SELECT tenant_id, transport_order_id, buyer_company_id, carrier_company_id, currency_code,
		       planned_amount, accrued_amount, current_actual_amount, final_actual_amount,
		       billing_register_amount, payable_amount, paid_amount,
		       billing_reconciliation_status, financial_finality, data_stage, sources_available
		FROM freight_cost.cost_summary_projection
		WHERE tenant_id = $1 AND transport_order_id = $2`
	row := tx.QueryRow(ctx, query, tenantID, transportOrderID)
	projection, err := scanProjection(row)
	if err != nil {
		return nil, err
	}
	return projection, nil
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
			billing_reconciliation_status, financial_finality, data_stage, sources_available, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15, $16, $17
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
			updated_at = EXCLUDED.updated_at`
	args := []any{
		projection.TenantID, projection.TransportOrderID, projection.BuyerCompanyID, projection.CarrierCompanyID,
		nullIfEmpty(projection.CurrencyCode),
		decimalArg(projection.PlannedAmount), decimalArg(projection.AccruedAmount),
		decimalArg(projection.CurrentActualAmount), decimalArg(projection.FinalActualAmount),
		decimalArg(projection.BillingRegisterAmount), decimalArg(projection.PayableAmount), decimalArg(projection.PaidAmount),
		string(projection.BillingReconciliationStatus), string(projection.FinancialFinality), string(projection.DataStage),
		sourcesRaw, time.Now().UTC(),
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
		}, nil
	}
	return nil, err
}

func scanProjection(row pgx.Row) (*domain.CostSummaryProjection, error) {
	var projection domain.CostSummaryProjection
	var currencyCode *string
	var planned, accrued, currentActual, finalActual, billed, payable, paid *string
	var reconciliationStatus, finality, dataStage string
	var sourcesRaw []byte

	err := row.Scan(
		&projection.TenantID, &projection.TransportOrderID, &projection.BuyerCompanyID, &projection.CarrierCompanyID,
		&currencyCode,
		&planned, &accrued, &currentActual, &finalActual,
		&billed, &payable, &paid,
		&reconciliationStatus, &finality, &dataStage, &sourcesRaw,
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
