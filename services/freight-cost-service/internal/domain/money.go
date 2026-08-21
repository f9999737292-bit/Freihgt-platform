package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

const MoneyScale = 2

var (
	ErrCurrencyMismatch = errors.New("CURRENCY_MISMATCH")
	currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)
	scientificPattern   = regexp.MustCompile(`(?i)[eE]`)
)

type Money struct {
	Amount   decimal.Decimal
	Currency string
}

func (m Money) IsZero() bool {
	return m.Amount.Equal(decimal.Zero)
}

func FormatMoneyAmount(d decimal.Decimal) string {
	return d.Round(MoneyScale).StringFixed(MoneyScale)
}

func ParseMoneyAmount(s string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return decimal.Decimal{}, fmt.Errorf("empty amount")
	}
	if scientificPattern.MatchString(trimmed) {
		return decimal.Decimal{}, fmt.Errorf("scientific notation is not allowed")
	}
	d, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Decimal{}, err
	}
	if !d.Equal(d.Round(MoneyScale)) {
		return decimal.Decimal{}, fmt.Errorf("amount must have at most %d fractional digits", MoneyScale)
	}
	return d.Round(MoneyScale), nil
}

func ValidateCurrencyCode(code string) error {
	trimmed := strings.TrimSpace(code)
	if !currencyCodePattern.MatchString(trimmed) {
		return fmt.Errorf("invalid currency code")
	}
	return nil
}

func NewMoney(amount decimal.Decimal, currency string) (*Money, error) {
	if err := ValidateCurrencyCode(currency); err != nil {
		return nil, err
	}
	return &Money{Amount: amount.Round(MoneyScale), Currency: strings.ToUpper(strings.TrimSpace(currency))}, nil
}

func SumMoney(base Money, additions []Money) (*Money, error) {
	total := base.Amount
	currency := base.Currency
	for _, item := range additions {
		if item.Currency != currency {
			return nil, ErrCurrencyMismatch
		}
		total = total.Add(item.Amount)
	}
	return &Money{Amount: total.Round(MoneyScale), Currency: currency}, nil
}
