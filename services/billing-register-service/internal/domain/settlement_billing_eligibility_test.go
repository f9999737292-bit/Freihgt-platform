package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestEvaluateSettlementBillingEligibilityApproved(t *testing.T) {
	t.Parallel()
	s := &FreightSettlement{Status: SettlementStatusApproved}
	el := EvaluateSettlementBillingEligibility(s, false, "")
	if !el.Eligible {
		t.Fatalf("expected eligible, reason=%s", el.BlockReason)
	}
}

func TestEvaluateSettlementBillingEligibilityAlreadyIncluded(t *testing.T) {
	t.Parallel()
	regID := uuid.New()
	s := &FreightSettlement{Status: SettlementStatusApproved, BillingRegisterID: &regID}
	el := EvaluateSettlementBillingEligibility(s, false, "")
	if el.Eligible || el.BlockReason != BillingBlockAlreadyIncluded {
		t.Fatalf("got eligible=%v reason=%s", el.Eligible, el.BlockReason)
	}
}

func TestEvaluateSettlementBillingEligibilityOpenDispute(t *testing.T) {
	t.Parallel()
	s := &FreightSettlement{Status: SettlementStatusApproved}
	el := EvaluateSettlementBillingEligibility(s, true, "")
	if el.Eligible || el.BlockReason != BillingBlockOpenDispute {
		t.Fatalf("got eligible=%v reason=%s", el.Eligible, el.BlockReason)
	}
}

func TestEvaluateSettlementBillingEligibilityCurrencyMismatch(t *testing.T) {
	t.Parallel()
	s := &FreightSettlement{Status: SettlementStatusApproved, CurrencyCode: "RUB"}
	el := EvaluateSettlementBillingEligibility(s, false, "USD")
	if el.Eligible || el.BlockReason != BillingBlockCurrencyMismatch {
		t.Fatalf("got eligible=%v reason=%s", el.Eligible, el.BlockReason)
	}
}
