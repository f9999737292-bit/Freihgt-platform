package domain

import "testing"

func TestValidatePaymentListQuery(t *testing.T) {
	if err := ValidatePaymentListQuery(PaymentListQuery{Status: "RECONCILED"}); err != nil {
		t.Fatalf("valid status: %v", err)
	}
	if err := ValidatePaymentListQuery(PaymentListQuery{Status: "NOT_A_STATUS"}); err == nil {
		t.Fatal("invalid status should fail")
	}
}

func TestNormalizePaymentListQueryLimit(t *testing.T) {
	q := NormalizePaymentListQuery(PaymentListQuery{Limit: 500})
	if q.Limit != MaxPaymentListLimit {
		t.Fatalf("expected cap %d, got %d", MaxPaymentListLimit, q.Limit)
	}
}
