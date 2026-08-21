package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func decimalPtr(value string) *decimal.Decimal {
	d := decimal.RequireFromString(value)
	return &d
}

func TestFC_A_DOM_003_CurrentActualApprovedNoDisputes(t *testing.T) {
	t.Parallel()

	in := SettlementFinancialInput{
		Status:           SettlementStatusApproved,
		OpenDisputeCount: 0,
		TotalWithoutVAT:  decimalPtr("1500.00"),
	}
	if !IsCurrentActualAvailable(in) {
		t.Fatal("expected current actual available")
	}
	if got := CurrentActualAmount(in); got == nil || !got.Equal(decimal.RequireFromString("1500.00")) {
		t.Fatalf("unexpected current actual: %v", got)
	}
	if NormalizeSettlementFinancialState(in) != FinancialFinalityCurrentActual {
		t.Fatalf("finality = %s", NormalizeSettlementFinancialState(in))
	}
}

func TestFC_A_DOM_004_CurrentActualDisputedIsNull(t *testing.T) {
	t.Parallel()

	in := SettlementFinancialInput{
		Status:           SettlementStatusDisputed,
		OpenDisputeCount: 1,
		TotalWithoutVAT:  decimalPtr("1500.00"),
	}
	if IsCurrentActualAvailable(in) {
		t.Fatal("expected current actual unavailable for disputed settlement")
	}
	if CurrentActualAmount(in) != nil {
		t.Fatal("expected nil current actual")
	}
}

func TestFC_A_DOM_005_FinalActualReadyForPayment(t *testing.T) {
	t.Parallel()

	in := SettlementFinancialInput{
		Status:           SettlementStatusReadyForPayment,
		OpenDisputeCount: 0,
		TotalWithoutVAT:  decimalPtr("1500.00"),
	}
	if !IsFinalActual(in) {
		t.Fatal("expected final actual available")
	}
	if got := FinalActualAmount(in); got == nil || !got.Equal(decimal.RequireFromString("1500.00")) {
		t.Fatalf("unexpected final actual: %v", got)
	}
	if NormalizeSettlementFinancialState(in) != FinancialFinalityFinalActual {
		t.Fatalf("finality = %s", NormalizeSettlementFinancialState(in))
	}
}

func TestFC_A_DOM_006_ApprovedIsNotFinal(t *testing.T) {
	t.Parallel()

	in := SettlementFinancialInput{
		Status:           SettlementStatusApproved,
		OpenDisputeCount: 0,
		TotalWithoutVAT:  decimalPtr("1500.00"),
	}
	if IsFinalActual(in) {
		t.Fatal("approved settlement must not be final actual")
	}
	if FinalActualAmount(in) != nil {
		t.Fatal("expected nil final actual")
	}
}
