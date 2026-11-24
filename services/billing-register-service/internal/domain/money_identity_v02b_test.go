package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestStoredKopeckAmountsSurviveFloatRound2 checks the fixed two-decimal
// amounts used by the money-identity proof. NUMERIC(18,2) values are parsed
// as decimal text, passed through the existing float64 round2 path, and
// compared again as decimal text.
func TestStoredKopeckAmountsSurviveFloatRound2(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"1234.56", "0.01", "0.10", "10.01", "8.10", "0.29", "100000.10"} {
		stored := decimal.RequireFromString(raw)
		asFloat, _ := stored.Float64()
		got := decimal.NewFromFloat(round2(asFloat)).StringFixed(MoneyDecimalPlaces)
		if got != stored.StringFixed(MoneyDecimalPlaces) {
			t.Fatalf("round2 changed stored amount %s to %s", raw, got)
		}
		amounts := CalculateItemAmounts(asFloat, 0, 0, nil)
		if decimal.NewFromFloat(amounts.AmountWithoutVAT).StringFixed(MoneyDecimalPlaces) != stored.StringFixed(MoneyDecimalPlaces) {
			t.Fatalf("CalculateItemAmounts changed %s to %s", raw, decimal.NewFromFloat(amounts.AmountWithoutVAT).StringFixed(MoneyDecimalPlaces))
		}
		if amounts.VATAmount != 0 || decimal.NewFromFloat(amounts.AmountWithVAT).StringFixed(MoneyDecimalPlaces) != stored.StringFixed(MoneyDecimalPlaces) {
			t.Fatalf("nil VAT rate changed %s to vat=%v with=%v", raw, amounts.VATAmount, amounts.AmountWithVAT)
		}
	}
}

// TestNilVATRateObservedZero records the current calculation when no rate is supplied.
// A missing rate is not a confirmed tax exemption.
func TestNilVATRateObservedZero(t *testing.T) {
	t.Parallel()
	base := decimal.RequireFromString("1234.56")
	without, vat, with := CalculateSettlementTotalsDecimal(base, decimal.Zero, nil)
	if without.StringFixed(MoneyDecimalPlaces) != "1234.56" || !vat.Equal(decimal.Zero) || with.StringFixed(MoneyDecimalPlaces) != "1234.56" {
		t.Fatalf("nil rate totals without=%s vat=%s with=%s", without, vat, with)
	}
	floatWithout, floatVAT, floatWith := CalculateSettlementTotals(1234.56, 0, nil)
	if floatVAT != 0 || decimal.NewFromFloat(floatWithout).StringFixed(MoneyDecimalPlaces) != "1234.56" || decimal.NewFromFloat(floatWith).StringFixed(MoneyDecimalPlaces) != "1234.56" {
		t.Fatalf("float nil rate totals without=%v vat=%v with=%v", floatWithout, floatVAT, floatWith)
	}
}

// TestLegacyFloatRound2DivergesOnThreeDecimalHalf reproduces the existing
// float64 rounding path. NUMERIC(18,2) cannot store 1.255, so this is not a
// snapshot-row identity failure. The production helper is left unchanged.
func TestLegacyFloatRound2DivergesOnThreeDecimalHalf(t *testing.T) {
	t.Parallel()
	const raw = "1.255"
	exact := decimal.RequireFromString(raw).Round(MoneyDecimalPlaces)
	asFloat, _ := decimal.RequireFromString(raw).Float64()
	observed := decimal.NewFromFloat(round2(asFloat)).StringFixed(MoneyDecimalPlaces)
	if exact.StringFixed(MoneyDecimalPlaces) != "1.26" {
		t.Fatalf("decimal policy for %s is %s", raw, exact)
	}
	if observed == exact.StringFixed(MoneyDecimalPlaces) {
		t.Fatalf("float64 round2(%s) now matches decimal %s; refresh the finding", raw, exact)
	}
	if observed != "1.25" {
		t.Fatalf("float64 round2(%s)=%s, previously observed 1.25", raw, observed)
	}
}
