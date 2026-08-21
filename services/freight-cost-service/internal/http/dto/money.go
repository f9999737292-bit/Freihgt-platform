package dto

import (
	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func MoneyAmountToDTO(m *domain.Money) *string {
	if m == nil {
		return nil
	}
	formatted := domain.FormatMoneyAmount(m.Amount)
	return &formatted
}

func MoneyAmountFromString(value *string, currency string) (*domain.Money, error) {
	if value == nil {
		return nil, nil
	}
	amount, err := domain.ParseMoneyAmount(*value)
	if err != nil {
		return nil, err
	}
	return domain.NewMoney(amount, currency)
}
