package domain

import (
	"strings"

	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

func NormalizeCurrencyCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func ValidateCurrencyCode(code string) error {
	code = NormalizeCurrencyCode(code)
	if len(code) != 3 {
		return apperrors.Validation("currency_code must be 3 characters", map[string]any{"field": "currency_code"})
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return apperrors.Validation("currency_code must be ISO 4217 uppercase", map[string]any{"field": "currency_code"})
		}
	}
	return nil
}

func RoundMoney(value decimal.Decimal) decimal.Decimal {
	return value.Round(MoneyScale)
}

func ValidateMoneyScale(value decimal.Decimal, field string) error {
	normalized := value.Round(MoneyScale)
	if !value.Equal(normalized) {
		return apperrors.Validation("amount exceeds allowed precision of two decimal places", map[string]any{"field": field})
	}
	return nil
}
