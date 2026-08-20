package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestRoundMoneyHalfUp(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.005", "1.01"},
		{"1.004", "1.00"},
		{"2.675", "2.68"},
		{"0.001", "0.00"},
	}
	for _, tc := range cases {
		got := RoundMoney(decimal.RequireFromString(tc.in))
		if got.StringFixed(MoneyScale) != tc.want {
			t.Fatalf("RoundMoney(%s) = %s, want %s", tc.in, got.StringFixed(MoneyScale), tc.want)
		}
	}
}

func TestValidateMoneyScaleRejectsExtraPrecision(t *testing.T) {
	err := ValidateMoneyScale(decimal.RequireFromString("1.001"), "amount")
	if err == nil {
		t.Fatal("expected precision validation error")
	}
}

func TestFuelSurchargeRoundingOrder(t *testing.T) {
	base := decimal.RequireFromString("1000.00")
	pct := decimal.RequireFromString("12.5")
	components := []RateComponent{
		{ComponentType: ComponentTypeBaseFreight, CalculationMethod: CalcMethodFlat, Amount: &base},
		{ComponentType: ComponentTypeFuelSurcharge, CalculationMethod: CalcMethodPercent, PercentValue: &pct},
	}
	calc, err := CalculatePreExecutionTotal(components, "RUB")
	if err != nil {
		t.Fatalf("calc: %v", err)
	}
	if calc.TotalAmount.StringFixed(MoneyScale) != "1125.00" {
		t.Fatalf("expected 1125.00, got %s", calc.TotalAmount.StringFixed(MoneyScale))
	}
}

func TestNormalizeCurrencyCode(t *testing.T) {
	if NormalizeCurrencyCode(" rub ") != "RUB" {
		t.Fatal("expected uppercase trim")
	}
}
