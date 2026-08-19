package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestDeriveObligationStatus(t *testing.T) {
	original := decimal.RequireFromString("100.00")

	cases := []struct {
		name    string
		paid    string
		want    string
		wantErr bool
	}{
		{"open", "0", ObligationStatusOpen, false},
		{"partial", "40.00", ObligationStatusPartiallyPaid, false},
		{"paid", "100.00", ObligationStatusPaid, false},
		{"overpaid forbidden", "100.01", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paid := decimal.RequireFromString(tc.paid)
			got, err := DeriveObligationStatus(original, paid)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestDerivePaymentAllocationStatus(t *testing.T) {
	amount := decimal.RequireFromString("100.00")
	cases := []struct {
		name      string
		allocated string
		want      string
	}{
		{"received", "0", PaymentStatusReceived},
		{"partial", "25.00", PaymentStatusPartiallyAllocated},
		{"full", "100.00", PaymentStatusFullyAllocated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocated := decimal.RequireFromString(tc.allocated)
			got, err := DerivePaymentAllocationStatus(amount, allocated)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
