package domain

import "github.com/shopspring/decimal"

const (
	ProposedSourceKnown   = "KNOWN"
	ProposedSourceUnknown = "UNKNOWN"
)

type ProposedAccessorialInput struct {
	SourceStatus string
	TotalExVAT   *decimal.Decimal
}

func RecomputeDerivedProjection(projection *CostSummaryProjection, proposed ProposedAccessorialInput, priorForecast *decimal.Decimal) error {
	if projection == nil {
		return nil
	}
	projection.ProjectionRevision++

	plannedMoney, err := moneyFromDecimal(projection.PlannedAmount, projection.CurrencyCode)
	if err != nil {
		return err
	}
	currentMoney, err := moneyFromDecimal(projection.CurrentActualAmount, projection.CurrencyCode)
	if err != nil {
		return err
	}
	finalMoney, err := moneyFromDecimal(projection.FinalActualAmount, projection.CurrencyCode)
	if err != nil {
		return err
	}

	currentVariance, err := CalculateCurrentVariance(plannedMoney, currentMoney)
	if err != nil {
		return err
	}
	finalVariance, err := CalculateFinalVariance(plannedMoney, finalMoney)
	if err != nil {
		return err
	}
	projection.CurrentVarianceAmount = moneyToDecimal(currentVariance)
	projection.FinalVarianceAmount = moneyToDecimal(finalVariance)

	currentPercent, err := CalculateVariancePercent(currentVariance, plannedMoney)
	if err != nil {
		return err
	}
	finalPercent, err := CalculateVariancePercent(finalVariance, plannedMoney)
	if err != nil {
		return err
	}
	projection.CurrentVariancePercent = currentPercent
	projection.FinalVariancePercent = finalPercent

	projection.ForecastExposure = computeForecastExposure(projection, plannedMoney, proposed, priorForecast)
	return nil
}

func computeForecastExposure(projection *CostSummaryProjection, planned *Money, proposed ProposedAccessorialInput, priorForecast *decimal.Decimal) *decimal.Decimal {
	if planned == nil {
		return nil
	}
	if proposed.SourceStatus != ProposedSourceKnown {
		if priorForecast != nil {
			return priorForecast
		}
		return projection.ForecastExposure
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
		return priorForecast
	}
	return moneyToDecimal(forecast)
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
