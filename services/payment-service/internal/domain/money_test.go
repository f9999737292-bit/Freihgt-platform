package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidateMoneyScale(t *testing.T) {
	t.Parallel()
	pass := []string{"1.23", "1.230", "0.01", "100000.99"}
	for _, raw := range pass {
		value, err := decimal.NewFromString(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if err := ValidateMoneyScale(value, "amount"); err != nil {
			t.Fatalf("ValidateMoneyScale(%q) expected pass, got %v", raw, err)
		}
	}
	reject := []string{"1.234", "0.001", "100000.999"}
	for _, raw := range reject {
		value := decimal.RequireFromString(raw)
		if err := ValidateMoneyScale(value, "amount"); err == nil {
			t.Fatalf("ValidateMoneyScale(%q) expected reject", raw)
		}
	}
}

func TestParseMoneyPrecisionValidation(t *testing.T) {
	t.Parallel()
	pass := []string{"1", "1.2", "1.20", "1.230", "0.01", "100000.99"}
	for _, raw := range pass {
		if _, err := ParseMoney(raw, "amount"); err != nil {
			t.Fatalf("ParseMoney(%q) expected pass, got %v", raw, err)
		}
	}
	reject := []string{"1.234", "0.001", "100000.999"}
	for _, raw := range reject {
		if _, err := ParseMoney(raw, "amount"); err == nil {
			t.Fatalf("ParseMoney(%q) expected reject", raw)
		}
	}
}

func TestParseMoneyNormalizesToTwoDecimals(t *testing.T) {
	t.Parallel()
	value, err := ParseMoney("1.230", "amount")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !value.Equal(decimal.RequireFromString("1.23")) {
		t.Fatalf("expected 1.23, got %s", value)
	}
}
