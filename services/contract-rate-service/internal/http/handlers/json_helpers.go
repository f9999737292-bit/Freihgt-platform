package handlers

import (
	"github.com/shopspring/decimal"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

func parseOptionalDecimal(raw *string, field string) (*decimal.Decimal, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := decimal.NewFromString(*raw)
	if err != nil {
		return nil, apperrors.Validation("invalid decimal value", map[string]any{"field": field})
	}
	if err := domain.ValidateMoneyScale(value, field); err != nil {
		return nil, err
	}
	return &value, nil
}

func parseRequiredDecimal(raw string, field string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, apperrors.Validation("invalid decimal value", map[string]any{"field": field})
	}
	if err := domain.ValidateMoneyScale(value, field); err != nil {
		return decimal.Zero, err
	}
	return value, nil
}

func decimalStringPtr(v *decimal.Decimal) *string {
	if v == nil {
		return nil
	}
	s := v.StringFixed(domain.MoneyScale)
	return &s
}
