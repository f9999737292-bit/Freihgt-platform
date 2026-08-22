package domain

import "github.com/shopspring/decimal"

const (
	ProposedSourceKnown   = "KNOWN"
	ProposedSourceUnknown = "UNKNOWN"

	ForecastSourceKnown   = ProposedSourceKnown
	ForecastSourceUnknown = ProposedSourceUnknown
)

type ProposedAccessorialInput struct {
	SourceStatus string
	TotalExVAT   *decimal.Decimal
}

// RecomputeDerivedProjection recalculates variance/forecast fields.
// Returns true when canonical derived input fingerprint changed (attribution refresh required).
func RecomputeDerivedProjection(projection *CostSummaryProjection, proposed ProposedAccessorialInput) (bool, error) {
	if projection == nil {
		return false, nil
	}

	plannedMoney, err := moneyFromDecimal(projection.PlannedAmount, projection.CurrencyCode)
	if err != nil {
		return false, err
	}
	currentMoney, err := moneyFromDecimal(projection.CurrentActualAmount, projection.CurrencyCode)
	if err != nil {
		return false, err
	}
	finalMoney, err := moneyFromDecimal(projection.FinalActualAmount, projection.CurrencyCode)
	if err != nil {
		return false, err
	}

	currentVariance, err := CalculateCurrentVariance(plannedMoney, currentMoney)
	if err != nil {
		return false, err
	}
	finalVariance, err := CalculateFinalVariance(plannedMoney, finalMoney)
	if err != nil {
		return false, err
	}
	projection.CurrentVarianceAmount = moneyToDecimal(currentVariance)
	projection.FinalVarianceAmount = moneyToDecimal(finalVariance)

	currentPercent, err := CalculateVariancePercent(currentVariance, plannedMoney)
	if err != nil {
		return false, err
	}
	finalPercent, err := CalculateVariancePercent(finalVariance, plannedMoney)
	if err != nil {
		return false, err
	}
	projection.CurrentVariancePercent = currentPercent
	projection.FinalVariancePercent = finalPercent

	projection.ForecastExposure, projection.ForecastSourceStatus = computeForecastExposure(plannedMoney, proposed)
	return ApplyDerivedStateRevision(projection, proposed), nil
}

func computeForecastExposure(planned *Money, proposed ProposedAccessorialInput) (*decimal.Decimal, string) {
	if planned == nil {
		return nil, ForecastSourceUnknown
	}
	if proposed.SourceStatus != ProposedSourceKnown {
		return nil, ForecastSourceUnknown
	}
	var proposedMonies []Money
	if proposed.TotalExVAT != nil {
		proposedMonies = append(proposedMonies, Money{
			Amount:   proposed.TotalExVAT.Round(MoneyScale),
			Currency: planned.Currency,
		})
	}
	forecast, err := CalculateForecastExposure(planned, proposedMonies)
	if err != nil {
		return nil, ForecastSourceUnknown
	}
	return moneyToDecimal(forecast), ForecastSourceKnown
}

func moneyFromDecimal(amount *decimal.Decimal, currency string) (*Money, error) {
	if amount == nil {
		return nil, nil
	}
	return NewMoney(*amount, currency)
}

func moneyToDecimal(m *Money) *decimal.Decimal {
	if m == nil {
		return nil
	}
	v := m.Amount.Round(MoneyScale)
	return &v
}
