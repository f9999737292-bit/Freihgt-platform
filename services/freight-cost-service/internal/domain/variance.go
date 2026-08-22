package domain

import "github.com/shopspring/decimal"

func CalculateCurrentVariance(planned, currentActual *Money) (*Money, error) {
	return calculateVariance(planned, currentActual)
}

func CalculateFinalVariance(planned, finalActual *Money) (*Money, error) {
	return calculateVariance(planned, finalActual)
}

func calculateVariance(planned, actual *Money) (*Money, error) {
	if planned == nil || actual == nil {
		return nil, nil
	}
	if !MoneyCurrencyCompatible(planned, actual) {
		return nil, nil
	}
	diff := actual.Amount.Sub(planned.Amount).Round(MoneyScale)
	return &Money{Amount: diff, Currency: planned.Currency}, nil
}

func CalculateVariancePercent(variance, planned *Money) (*decimal.Decimal, error) {
	if variance == nil || planned == nil {
		return nil, nil
	}
	if planned.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}
	percent := variance.Amount.Div(planned.Amount).Mul(decimal.NewFromInt(100)).Round(4)
	return &percent, nil
}

func MoneyCurrencyCompatible(a, b *Money) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Currency != "" && a.Currency == b.Currency
}
