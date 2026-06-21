package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestCalculateBidTotals(t *testing.T) {
	t.Parallel()
	vat := 20.0
	totals := CalculateBidTotals([]CreateBidItemInput{{
		BaseAmount: 100000, FuelSurcharge: 5000, TollAmount: 3000, ExtraCharges: 0, VATRate: &vat,
	}}, &vat)

	if totals.TotalAmount != 108000 {
		t.Fatalf("expected total 108000, got %v", totals.TotalAmount)
	}
	if totals.VATAmount != 21600 {
		t.Fatalf("expected vat 21600, got %v", totals.VATAmount)
	}
	if totals.TotalAmountWithVAT != 129600 {
		t.Fatalf("expected total with vat 129600, got %v", totals.TotalAmountWithVAT)
	}
}

func TestValidateSubmitBid(t *testing.T) {
	t.Parallel()
	if err := ValidateSubmitBid(BidStatusDraft); err != nil {
		t.Fatalf("expected submit from DRAFT")
	}
	if err := ValidateSubmitBid(BidStatusSubmitted); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateAcceptBid(t *testing.T) {
	t.Parallel()
	if err := ValidateAcceptBid(BidStatusSubmitted); err != nil {
		t.Fatalf("expected accept from SUBMITTED")
	}
	if err := ValidateAcceptBid(BidStatusDraft); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateCreateBidInputNegativeAmount(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if err := ValidateCreateBidInput(CreateBidInput{
		TenantID: tenantID, FreightRequestID: uuid.New(), CarrierCompanyID: uuid.New(),
		BidNumber: "BID-1", Items: []CreateBidItemInput{{BaseAmount: -1}},
	}); err == nil {
		t.Fatalf("expected validation error for negative amount")
	}
}
