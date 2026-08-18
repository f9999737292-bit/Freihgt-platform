package domain

import "testing"

func TestCalculateSettlementTotals(t *testing.T) {
	vat := 20.0
	without, vatAmt, with := CalculateSettlementTotals(100000, 5000, &vat)
	if without != 105000 {
		t.Fatalf("without=%v", without)
	}
	if vatAmt != 21000 {
		t.Fatalf("vat=%v", vatAmt)
	}
	if with != 126000 {
		t.Fatalf("with=%v", with)
	}
}

func TestValidateSettlementTransitionApprovedToDocumentsReady(t *testing.T) {
	if err := ValidateSettlementTransition(SettlementStatusApproved, SettlementStatusDocumentsReady); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSettlementTransitionDraftToPaidDenied(t *testing.T) {
	if err := ValidateSettlementTransition(SettlementStatusDraft, SettlementStatusReadyForPayment); err == nil {
		t.Fatal("expected denied transition")
	}
}
