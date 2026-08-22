package domain

import (
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	DataStageAccrualAvailable        DataStage = "ACCRUAL_AVAILABLE"
	DataStageCurrentActualAvailable  DataStage = "CURRENT_ACTUAL_AVAILABLE"
	DataStageFinalActualAvailable    DataStage = "FINAL_ACTUAL_AVAILABLE"
	DataStageBillingLinked           DataStage = "BILLING_LINKED"
	DataStagePaid                    DataStage = "PAID"
)

func ApplyCostEntryToProjection(projection *CostSummaryProjection, entry *CostEntry) error {
	if projection == nil || entry == nil {
		return nil
	}
	if projection.CurrencyCode == "" {
		projection.CurrencyCode = entry.CurrencyCode
	} else if !strings.EqualFold(projection.CurrencyCode, entry.CurrencyCode) {
		return ErrCurrencyMismatch
	}

	amount := entryAmount(entry)
	switch entry.EntryKind {
	case EntryKindPlannedCostSnapshot:
		projection.PlannedAmount = amount
		addSource(projection, SourceTypeTORateSnapshot)
	case EntryKindAccrualCostSnapshot:
		projection.AccruedAmount = amount
		addSource(projection, SourceTypeFreightSettlement)
	case EntryKindCurrentActualCostSnapshot:
		projection.CurrentActualAmount = amount
		addSource(projection, SourceTypeFreightSettlement)
	case EntryKindFinalActualCostSnapshot:
		projection.FinalActualAmount = amount
		addSource(projection, SourceTypeFreightSettlement)
	case EntryKindBilledCostSnapshot:
		projection.BillingRegisterAmount = amount
		if entry.AmountAvailability == AmountAvailabilityUnavailable {
			projection.SettlementLinked = false
		} else {
			projection.SettlementLinked = true
		}
		addSource(projection, SourceTypeFreightSettlementBillingLink)
	case EntryKindPayableAmountSnapshot:
		projection.PayableAmount = amount
		addSource(projection, SourceTypeBillingRegister)
	case EntryKindPaidAmountSnapshot:
		projection.PaidAmount = amount
		addSource(projection, SourceTypePaymentObligation)
	}

	projection.BuyerCompanyID = entry.BuyerCompanyID
	projection.CarrierCompanyID = entry.CarrierCompanyID
	projection.DataStage = ComputeDataStage(projection)
	projection.BillingReconciliationStatus = RecomputeBillingReconciliation(projection)
	projection.FinancialFinality = RecomputeFinancialFinality(projection)
	return nil
}

func entryAmount(entry *CostEntry) *decimal.Decimal {
	if entry.AmountAvailability == AmountAvailabilityUnavailable || entry.Amount == nil {
		return nil
	}
	value := entry.Amount.Round(MoneyScale)
	return &value
}

func addSource(projection *CostSummaryProjection, sourceType string) {
	if projection.SourcesAvailable == nil {
		projection.SourcesAvailable = []string{}
	}
	if !slices.Contains(projection.SourcesAvailable, sourceType) {
		projection.SourcesAvailable = append(projection.SourcesAvailable, sourceType)
	}
}

func ComputeDataStage(projection *CostSummaryProjection) DataStage {
	if projection == nil {
		return DataStagePlannedOnly
	}
	if projection.PaidAmount != nil {
		return DataStagePaid
	}
	if projection.SettlementLinked && projection.BillingRegisterAmount != nil {
		return DataStageBillingLinked
	}
	if projection.FinalActualAmount != nil {
		return DataStageFinalActualAvailable
	}
	if projection.CurrentActualAmount != nil {
		return DataStageCurrentActualAvailable
	}
	if projection.AccruedAmount != nil {
		return DataStageAccrualAvailable
	}
	if projection.PlannedAmount != nil {
		return DataStagePlannedOnly
	}
	return DataStagePlannedOnly
}

func RecomputeBillingReconciliation(projection *CostSummaryProjection) BillingReconciliationStatus {
	if projection == nil {
		return BillingReconciliationUnlinked
	}
	return DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked:      projection.SettlementLinked,
		SettlementTotalExVAT:  projection.CurrentActualAmount,
		SettlementCurrency:    projection.CurrencyCode,
		SettlementStatus:      projection.SettlementStatus,
		OpenDisputeCount:      projection.OpenDisputeCount,
		BilledLineAmountExVAT: projection.BillingRegisterAmount,
		BilledLineCurrency:    projection.CurrencyCode,
	})
}

func RecomputeFinancialFinality(projection *CostSummaryProjection) FinancialFinality {
	if projection == nil {
		return FinancialFinalityNotEvaluated
	}
	return NormalizeSettlementFinancialState(SettlementFinancialInput{
		Status:           projection.SettlementStatus,
		OpenDisputeCount: projection.OpenDisputeCount,
		TotalWithoutVAT:  projection.CurrentActualAmount,
	})
}

func ProjectionToCostSummary(projection *CostSummaryProjection) (*CostSummary, error) {
	if projection == nil {
		return nil, nil
	}
	summary := &CostSummary{
		TenantID:          projection.TenantID,
		TransportOrderID:  projection.TransportOrderID,
		BuyerCompanyID:    projection.BuyerCompanyID,
		CarrierCompanyID:  projection.CarrierCompanyID,
		CurrencyCode:      projection.CurrencyCode,
		DataStage:         projection.DataStage,
		FinancialFinality: projection.FinancialFinality,
		SourcesAvailable:  projection.SourcesAvailable,
	}
	if projection.PlannedAmount != nil {
		m, err := NewMoney(*projection.PlannedAmount, projection.CurrencyCode)
		if err != nil {
			return nil, err
		}
		summary.PlannedAmount = m
	}
	if projection.AccruedAmount != nil {
		m, err := NewMoney(*projection.AccruedAmount, projection.CurrencyCode)
		if err != nil {
			return nil, err
		}
		summary.AccruedAmount = m
	}
	if projection.CurrentActualAmount != nil {
		m, err := NewMoney(*projection.CurrentActualAmount, projection.CurrencyCode)
		if err != nil {
			return nil, err
		}
		summary.CurrentActualAmount = m
	}
	if projection.FinalActualAmount != nil {
		m, err := NewMoney(*projection.FinalActualAmount, projection.CurrencyCode)
		if err != nil {
			return nil, err
		}
		summary.FinalActualAmount = m
	}
	if projection.BillingRegisterAmount != nil {
		m, err := NewMoney(*projection.BillingRegisterAmount, projection.CurrencyCode)
		if err != nil {
			return nil, err
		}
		summary.BillingRegisterAmount = m
	}
	if projection.PaidAmount != nil {
		m, err := NewMoney(*projection.PaidAmount, projection.CurrencyCode)
		if err != nil {
			return nil, err
		}
		summary.PaidAmount = m
	}
	status := projection.BillingReconciliationStatus
	summary.BillingReconciliationStatus = &status
	return summary, nil
}
