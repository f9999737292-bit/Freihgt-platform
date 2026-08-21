package dto

import (
	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type CanonicalSourceRefDTO struct {
	SourceService       string  `json:"source_service"`
	SourceType          string  `json:"source_type"`
	SourceID            string  `json:"source_id"`
	SourceVersion       *int    `json:"source_version"`
	PricingModelVersion *string `json:"pricing_model_version"`
}

type CostSummaryResponse struct {
	TenantID                    string                 `json:"tenant_id"`
	TransportOrderID            string                 `json:"transport_order_id"`
	ShipmentID                  *string                `json:"shipment_id"`
	BuyerCompanyID              string                 `json:"buyer_company_id"`
	CarrierCompanyID            string                 `json:"carrier_company_id"`
	CurrencyCode                string                 `json:"currency_code"`
	DataStage                   string                 `json:"data_stage"`
	FinancialFinality           string                 `json:"financial_finality"`
	SourcesAvailable            []string               `json:"sources_available"`
	PlannedAmount               *string                `json:"planned_amount"`
	PlannedSource               *CanonicalSourceRefDTO `json:"planned_source"`
	AccruedAmount               *string                `json:"accrued_amount"`
	ForecastExposure            *string                `json:"forecast_exposure"`
	CurrentActualAmount         *string                `json:"current_actual_amount"`
	FinalActualAmount           *string                `json:"final_actual_amount"`
	BillingRegisterAmount       *string                `json:"billing_register_amount"`
	PaidAmount                  *string                `json:"paid_amount"`
	CurrentVarianceAmount       *string                `json:"current_variance_amount"`
	FinalVarianceAmount         *string                `json:"final_variance_amount"`
	BillingReconciliationStatus *string                `json:"billing_reconciliation_status"`
}

func ToCostSummaryResponse(summary *domain.CostSummary) CostSummaryResponse {
	resp := CostSummaryResponse{
		TenantID:              summary.TenantID.String(),
		TransportOrderID:      summary.TransportOrderID.String(),
		BuyerCompanyID:        summary.BuyerCompanyID.String(),
		CarrierCompanyID:      summary.CarrierCompanyID.String(),
		CurrencyCode:          summary.CurrencyCode,
		DataStage:             string(summary.DataStage),
		FinancialFinality:     string(summary.FinancialFinality),
		SourcesAvailable:      summary.SourcesAvailable,
		PlannedAmount:         MoneyAmountToDTO(summary.PlannedAmount),
		AccruedAmount:         MoneyAmountToDTO(summary.AccruedAmount),
		ForecastExposure:      MoneyAmountToDTO(summary.ForecastExposure),
		CurrentActualAmount:   MoneyAmountToDTO(summary.CurrentActualAmount),
		FinalActualAmount:     MoneyAmountToDTO(summary.FinalActualAmount),
		BillingRegisterAmount: MoneyAmountToDTO(summary.BillingRegisterAmount),
		PaidAmount:            MoneyAmountToDTO(summary.PaidAmount),
		CurrentVarianceAmount: MoneyAmountToDTO(summary.CurrentVarianceAmount),
		FinalVarianceAmount:   MoneyAmountToDTO(summary.FinalVarianceAmount),
	}
	if summary.ShipmentID != nil {
		value := summary.ShipmentID.String()
		resp.ShipmentID = &value
	}
	if summary.PlannedSourceRef != nil {
		resp.PlannedSource = &CanonicalSourceRefDTO{
			SourceService:       summary.PlannedSourceRef.SourceService,
			SourceType:          summary.PlannedSourceRef.SourceType,
			SourceID:            summary.PlannedSourceRef.SourceID.String(),
			SourceVersion:       summary.PlannedSourceRef.SourceVersion,
			PricingModelVersion: summary.PlannedSourceRef.PricingModelVersion,
		}
	}
	if summary.BillingReconciliationStatus != nil {
		value := string(*summary.BillingReconciliationStatus)
		resp.BillingReconciliationStatus = &value
	}
	return resp
}
