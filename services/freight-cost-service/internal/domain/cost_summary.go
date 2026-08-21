package domain

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type FinancialFinality string

const (
	FinancialFinalityNotEvaluated  FinancialFinality = "NOT_EVALUATED"
	FinancialFinalityDraft         FinancialFinality = "DRAFT"
	FinancialFinalityCurrentActual FinancialFinality = "CURRENT_ACTUAL"
	FinancialFinalityFinalActual   FinancialFinality = "FINAL_ACTUAL"
	FinancialFinalityCancelled     FinancialFinality = "CANCELLED"
)

const (
	SettlementStatusDraft           = "DRAFT"
	SettlementStatusUnderReview     = "UNDER_REVIEW"
	SettlementStatusDisputed        = "DISPUTED"
	SettlementStatusApproved        = "APPROVED"
	SettlementStatusDocumentsReady  = "DOCUMENTS_READY"
	SettlementStatusReadyForPayment = "READY_FOR_PAYMENT"
	SettlementStatusCancelled       = "CANCELLED"
)

type SettlementFinancialInput struct {
	Status           string
	OpenDisputeCount int
	TotalWithoutVAT  *decimal.Decimal
}

func IsCurrentActualAvailable(in SettlementFinancialInput) bool {
	if in.TotalWithoutVAT == nil {
		return false
	}
	if in.OpenDisputeCount > 0 {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(in.Status)) {
	case SettlementStatusApproved, SettlementStatusDocumentsReady, SettlementStatusReadyForPayment:
		return true
	default:
		return false
	}
}

func IsFinalActual(in SettlementFinancialInput) bool {
	if in.TotalWithoutVAT == nil {
		return false
	}
	if in.OpenDisputeCount > 0 {
		return false
	}
	return strings.ToUpper(strings.TrimSpace(in.Status)) == SettlementStatusReadyForPayment
}

func NormalizeSettlementFinancialState(in SettlementFinancialInput) FinancialFinality {
	if in.TotalWithoutVAT == nil {
		return FinancialFinalityNotEvaluated
	}
	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if status == SettlementStatusCancelled {
		return FinancialFinalityCancelled
	}
	if IsFinalActual(in) {
		return FinancialFinalityFinalActual
	}
	if IsCurrentActualAvailable(in) {
		return FinancialFinalityCurrentActual
	}
	return FinancialFinalityDraft
}

func CurrentActualAmount(in SettlementFinancialInput) *decimal.Decimal {
	if !IsCurrentActualAvailable(in) {
		return nil
	}
	amount := in.TotalWithoutVAT.Round(MoneyScale)
	return &amount
}

func FinalActualAmount(in SettlementFinancialInput) *decimal.Decimal {
	if !IsFinalActual(in) {
		return nil
	}
	amount := in.TotalWithoutVAT.Round(MoneyScale)
	return &amount
}

type DataStage string

const DataStagePlannedOnly DataStage = "PLANNED_ONLY"

type CostViewScope string

const (
	CostViewScopeBuyerCost        CostViewScope = "BUYER_COST_VIEW"
	CostViewScopeCarrierReceivable CostViewScope = "CARRIER_RECEIVABLE_VIEW"
)

type CostSummary struct {
	TenantID                    uuid.UUID
	TransportOrderID            uuid.UUID
	ShipmentID                  *uuid.UUID
	BuyerCompanyID              uuid.UUID
	CarrierCompanyID            uuid.UUID
	CurrencyCode                string
	PlannedAmount               *Money
	PlannedSourceRef            *CanonicalSourceRef
	DataStage                   DataStage
	FinancialFinality           FinancialFinality
	AccruedAmount               *Money
	ForecastExposure            *Money
	CurrentActualAmount         *Money
	FinalActualAmount           *Money
	BillingRegisterAmount       *Money
	PaidAmount                  *Money
	CurrentVarianceAmount       *Money
	FinalVarianceAmount         *Money
	BillingReconciliationStatus *BillingReconciliationStatus
	SourcesAvailable            []string
}
