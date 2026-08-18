package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type RfxResponseOfferLine struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	RfxResponseID uuid.UUID
	RfxLotID      uuid.UUID
	Amount        float64
	CurrencyCode  string
	Comment       *string
	Version       int
}

type UpsertOfferLineInput struct {
	RfxLotID     uuid.UUID
	Amount       float64
	CurrencyCode string
	Comment      *string
}

func ValidateOfferLineInput(in UpsertOfferLineInput, lotCount int) error {
	if lotCount > 0 && in.RfxLotID == uuid.Nil {
		return apperrors.Validation("rfx_lot_id is required when tender has lots", map[string]any{"field": "rfx_lot_id"})
	}
	if in.Amount < 0 {
		return apperrors.Validation("amount must be >= 0", map[string]any{"field": "amount"})
	}
	code := strings.TrimSpace(in.CurrencyCode)
	if len(code) != 3 {
		return apperrors.Validation("currency_code must be 3 characters", map[string]any{"field": "currency_code"})
	}
	return nil
}

func ValidateOfferLinesForEventCurrency(lines []UpsertOfferLineInput, eventCurrency string) error {
	eventCurrency = strings.TrimSpace(strings.ToUpper(eventCurrency))
	if eventCurrency == "" {
		return nil
	}
	for _, line := range lines {
		if strings.ToUpper(strings.TrimSpace(line.CurrencyCode)) != eventCurrency {
			return apperrors.Validation("offer line currency must match tender currency", map[string]any{
				"field":           "currency_code",
				"expected_currency": eventCurrency,
			})
		}
	}
	return nil
}

func SumOfferLineAmounts(lines []RfxResponseOfferLine) float64 {
	var total float64
	for _, line := range lines {
		total += line.Amount
	}
	return roundMoney(total)
}

func roundMoney(value float64) float64 {
	return float64(int64(value*100+0.5)) / 100
}
