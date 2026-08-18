package domain

import (
	"github.com/google/uuid"
)

func ResponseCommercialSummary(lines []RfxResponseOfferLine, eventCurrency string) (total float64, currency string, complete bool) {
	if len(lines) == 0 {
		return 0, "", false
	}
	currency = NormalizeCurrencyCode(lines[0].CurrencyCode)
	total = SumOfferLineAmounts(lines)
	for _, line := range lines {
		if NormalizeCurrencyCode(line.CurrencyCode) != currency {
			return total, currency, false
		}
	}
	if eventCurrency != "" && currency != NormalizeCurrencyCode(eventCurrency) {
		return total, currency, false
	}
	return total, currency, true
}

func ResponseOfferComplete(lotCount int, lines []RfxResponseOfferLine) bool {
	if len(lines) == 0 {
		return false
	}
	if lotCount == 0 {
		for _, line := range lines {
			if line.RfxLotID == uuid.Nil {
				return line.Amount >= 0
			}
		}
		return false
	}
	covered := map[uuid.UUID]struct{}{}
	for _, line := range lines {
		if line.RfxLotID == uuid.Nil {
			continue
		}
		covered[line.RfxLotID] = struct{}{}
	}
	return len(covered) == lotCount
}
