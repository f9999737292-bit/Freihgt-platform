package domain

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const MoneyScale = 2

var ZeroMoney = decimal.Zero

func ParseMoney(raw string, field string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Decimal{}, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Decimal{}, apperrors.Validation("invalid monetary amount", map[string]any{"field": field})
	}
	if value.LessThanOrEqual(decimal.Zero) {
		return decimal.Decimal{}, apperrors.Validation("amount must be greater than zero", map[string]any{"field": field})
	}
	if err := ValidateMoneyScale(value, field); err != nil {
		return decimal.Decimal{}, err
	}
	return value.Round(MoneyScale), nil
}

func ValidateMoneyScale(value decimal.Decimal, field string) error {
	normalized := value.Round(MoneyScale)
	if !value.Equal(normalized) {
		return apperrors.Validation("amount exceeds allowed precision of two decimal places", map[string]any{"field": field})
	}
	return nil
}

func RoundMoney(value decimal.Decimal) decimal.Decimal {
	return value.Round(MoneyScale)
}

func MoneyEqual(a, b decimal.Decimal) bool {
	return RoundMoney(a).Equal(RoundMoney(b))
}

func NormalizeCurrencyCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func ValidateCurrencyCode(code string) error {
	code = NormalizeCurrencyCode(code)
	if len(code) != 3 {
		return apperrors.Validation("currency_code must be 3 characters", map[string]any{"field": "currency_code"})
	}
	return nil
}

func ParseUUID(raw, field string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid "+field, map[string]any{"field": field})
	}
	return id, nil
}
