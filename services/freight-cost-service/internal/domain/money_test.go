package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFC_A_DOM_001_FormatAndParseMoneyAmount(t *testing.T) {
	t.Parallel()

	amount, err := ParseMoneyAmount("1250.00")
	if err != nil {
		t.Fatalf("parse valid amount: %v", err)
	}
	if got := FormatMoneyAmount(amount); got != "1250.00" {
		t.Fatalf("format = %q, want 1250.00", got)
	}

	if _, err := ParseMoneyAmount("1250.001"); err == nil {
		t.Fatal("expected rejection for >2 fractional digits")
	}
	if _, err := ParseMoneyAmount("1e3"); err == nil {
		t.Fatal("expected rejection for scientific notation")
	}
	if _, err := ParseMoneyAmount("1E+2"); err == nil {
		t.Fatal("expected rejection for scientific notation")
	}
}

func TestFC_A_DOM_002_NilMoneyIsNotKnownZero(t *testing.T) {
	t.Parallel()

	var unknown *Money
	if unknown != nil {
		t.Fatal("expected nil unknown money")
	}

	zero, err := NewMoney(decimal.Zero, "RUB")
	if err != nil {
		t.Fatalf("new zero money: %v", err)
	}
	if !zero.IsZero() {
		t.Fatal("expected known zero")
	}
	if unknown == zero {
		t.Fatal("nil and known zero must differ")
	}
}

func TestFC_A_DOM_007_CurrencyMismatchDeny(t *testing.T) {
	t.Parallel()

	planned, err := NewMoney(decimal.RequireFromString("100.00"), "RUB")
	if err != nil {
		t.Fatalf("planned: %v", err)
	}
	accessorial := Money{Amount: decimal.RequireFromString("10.00"), Currency: "USD"}

	if _, err := CalculateAccrual(planned, []Money{accessorial}); err != ErrCurrencyMismatch {
		t.Fatalf("expected CURRENCY_MISMATCH, got %v", err)
	}
	if err := ValidateCurrencyCode("rub"); err == nil {
		t.Fatal("expected invalid lowercase currency")
	}
}
