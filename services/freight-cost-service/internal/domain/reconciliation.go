package domain

import (
	"strings"

	"github.com/shopspring/decimal"
)

type BillingReconciliationStatus string

const (
	BillingReconciliationMatch    BillingReconciliationStatus = "MATCH"
	BillingReconciliationMismatch BillingReconciliationStatus = "MISMATCH"
	BillingReconciliationUnlinked BillingReconciliationStatus = "UNLINKED"
)

type BillingReconciliationInput struct {
	SettlementLinked      bool
	SettlementTotalExVAT  *decimal.Decimal
	SettlementCurrency    string
	SettlementStatus      string
	OpenDisputeCount      int
	BilledLineAmountExVAT *decimal.Decimal
	BilledLineCurrency    string
}

func DetermineBillingReconciliation(in BillingReconciliationInput) BillingReconciliationStatus {
	if !in.SettlementLinked {
		return BillingReconciliationUnlinked
	}
	status := strings.ToUpper(strings.TrimSpace(in.SettlementStatus))
	if in.OpenDisputeCount > 0 || status == SettlementStatusDisputed {
		return BillingReconciliationMismatch
	}
	if status == SettlementStatusCancelled {
		return BillingReconciliationMismatch
	}
	if in.SettlementTotalExVAT == nil || in.BilledLineAmountExVAT == nil {
		return BillingReconciliationMismatch
	}
	if strings.ToUpper(strings.TrimSpace(in.SettlementCurrency)) != strings.ToUpper(strings.TrimSpace(in.BilledLineCurrency)) {
		return BillingReconciliationMismatch
	}
	if !in.SettlementTotalExVAT.Round(MoneyScale).Equal(in.BilledLineAmountExVAT.Round(MoneyScale)) {
		return BillingReconciliationMismatch
	}
	return BillingReconciliationMatch
}
