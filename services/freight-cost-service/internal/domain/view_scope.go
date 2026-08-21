package domain

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func ViewScopeForActorKind(actorKind string) CostViewScope {
	switch strings.ToUpper(strings.TrimSpace(actorKind)) {
	case "CARRIER":
		return CostViewScopeCarrierReceivable
	default:
		return CostViewScopeBuyerCost
	}
}

func ApplyViewScope(scope CostViewScope, summary *CostSummary) *CostSummary {
	if summary == nil {
		return nil
	}
	if scope == CostViewScopeBuyerCost {
		return summary
	}
	filtered := *summary
	filtered.AccruedAmount = nil
	filtered.ForecastExposure = nil
	filtered.CurrentVarianceAmount = nil
	filtered.FinalVarianceAmount = nil
	return &filtered
}

func PopulateBuyerOnlyFixture(summary *CostSummary) *CostSummary {
	if summary == nil {
		return nil
	}
	populated := *summary
	rub := "RUB"
	planned, _ := NewMoney(decimalFromString("1000.00"), rub)
	accrual, _ := NewMoney(decimalFromString("1100.00"), rub)
	forecast, _ := NewMoney(decimalFromString("1200.00"), rub)
	currentVar, _ := NewMoney(decimalFromString("50.00"), rub)
	finalVar, _ := NewMoney(decimalFromString("25.00"), rub)
	populated.PlannedAmount = planned
	populated.AccruedAmount = accrual
	populated.ForecastExposure = forecast
	populated.CurrentVarianceAmount = currentVar
	populated.FinalVarianceAmount = finalVar
	populated.BuyerCompanyID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	populated.CarrierCompanyID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return &populated
}

func decimalFromString(value string) decimal.Decimal {
	d, _ := ParseMoneyAmount(value)
	return d
}
