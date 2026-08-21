package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func populateBuyerOnlyFixture(summary *CostSummary) *CostSummary {
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

func TestFC_A_SEC_003_CarrierViewMasksBuyerOnlyFields(t *testing.T) {
	t.Parallel()

	base := &CostSummary{
		TenantID:         uuid.New(),
		TransportOrderID: uuid.New(),
	}
	populated := populateBuyerOnlyFixture(base)
	filtered := ApplyViewScope(CostViewScopeCarrierReceivable, populated)

	if filtered.AccruedAmount != nil {
		t.Fatal("carrier view must hide accrued_amount")
	}
	if filtered.ForecastExposure != nil {
		t.Fatal("carrier view must hide forecast_exposure")
	}
	if filtered.CurrentVarianceAmount != nil {
		t.Fatal("carrier view must hide current_variance_amount")
	}
	if filtered.FinalVarianceAmount != nil {
		t.Fatal("carrier view must hide final_variance_amount")
	}
	if filtered.PlannedAmount == nil {
		t.Fatal("carrier view should retain planned_amount for same-company reads")
	}
}
