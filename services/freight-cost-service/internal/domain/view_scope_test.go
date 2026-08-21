package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestFC_A_SEC_003_CarrierViewMasksBuyerOnlyFields(t *testing.T) {
	t.Parallel()

	base := &CostSummary{
		TenantID:         uuid.New(),
		TransportOrderID: uuid.New(),
	}
	populated := PopulateBuyerOnlyFixture(base)
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
