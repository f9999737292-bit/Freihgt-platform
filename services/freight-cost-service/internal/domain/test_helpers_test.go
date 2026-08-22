package domain

import "github.com/shopspring/decimal"

func moneyDecimalPtr(value string) *decimal.Decimal {
	return decimalPtr(value)
}

func mustNewMoney(t testingTB, value, currency string) *Money {
	t.Helper()
	m, err := NewMoney(decimal.RequireFromString(value), currency)
	if err != nil {
		t.Fatalf("NewMoney(%q, %q): %v", value, currency, err)
	}
	return m
}

func projectionWithAmounts(planned, current, final string) *CostSummaryProjection {
	p := &CostSummaryProjection{CurrencyCode: "RUB"}
	if planned != "" {
		p.PlannedAmount = moneyDecimalPtr(planned)
	}
	if current != "" {
		p.CurrentActualAmount = moneyDecimalPtr(current)
	}
	if final != "" {
		p.FinalActualAmount = moneyDecimalPtr(final)
	}
	return p
}

type testingTB interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}
