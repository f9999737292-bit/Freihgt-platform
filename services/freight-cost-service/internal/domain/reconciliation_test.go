package domain

import (
	"testing"
)

func TestFC_A_DOM_008_BillingReconciliationMatch(t *testing.T) {
	t.Parallel()

	status := DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked:      true,
		SettlementTotalExVAT:  decimalPtr("100.00"),
		SettlementCurrency:    "RUB",
		SettlementStatus:      SettlementStatusApproved,
		OpenDisputeCount:      0,
		BilledLineAmountExVAT: decimalPtr("100.00"),
		BilledLineCurrency:    "RUB",
	})
	if status != BillingReconciliationMatch {
		t.Fatalf("status = %s", status)
	}
}

func TestFC_A_DOM_009_BillingReconciliationMismatch(t *testing.T) {
	t.Parallel()

	status := DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked:      true,
		SettlementTotalExVAT:  decimalPtr("100.00"),
		SettlementCurrency:    "RUB",
		SettlementStatus:      SettlementStatusDisputed,
		OpenDisputeCount:      1,
		BilledLineAmountExVAT: decimalPtr("100.00"),
		BilledLineCurrency:    "RUB",
	})
	if status != BillingReconciliationMismatch {
		t.Fatalf("status = %s", status)
	}
}

func TestFC_A_DOM_010_BillingReconciliationUnlinked(t *testing.T) {
	t.Parallel()

	status := DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked: false,
	})
	if status != BillingReconciliationUnlinked {
		t.Fatalf("status = %s", status)
	}
}
