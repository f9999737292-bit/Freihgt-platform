package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestFC_C_SEC_001_CarrierMaskHidesForecast(t *testing.T) {
	t.Parallel()
	filtered := ApplyViewScope(CostViewScopeCarrierReceivable, populateBuyerOnlyFixture(&CostSummary{
		TenantID: uuid.New(), TransportOrderID: uuid.New(),
	}))
	if filtered.ForecastExposure != nil {
		t.Fatal("carrier view must hide forecast_exposure")
	}
}

func TestFC_C_SEC_002_CarrierMaskHidesVariance(t *testing.T) {
	t.Parallel()
	filtered := ApplyViewScope(CostViewScopeCarrierReceivable, populateBuyerOnlyFixture(&CostSummary{
		TenantID: uuid.New(), TransportOrderID: uuid.New(),
	}))
	if filtered.CurrentVarianceAmount != nil || filtered.FinalVarianceAmount != nil {
		t.Fatal("carrier view must hide variance fields")
	}
}

func TestFC_C_SEC_003_CarrierMaskHidesAccrual(t *testing.T) {
	t.Parallel()
	filtered := ApplyViewScope(CostViewScopeCarrierReceivable, populateBuyerOnlyFixture(&CostSummary{
		TenantID: uuid.New(), TransportOrderID: uuid.New(),
	}))
	if filtered.AccruedAmount != nil {
		t.Fatal("carrier view must hide accrued_amount")
	}
}

func TestFC_C_SEC_004_BuyerViewShowsVariance(t *testing.T) {
	t.Parallel()
	summary := populateBuyerOnlyFixture(&CostSummary{
		TenantID: uuid.New(), TransportOrderID: uuid.New(),
	})
	filtered := ApplyViewScope(CostViewScopeBuyerCost, summary)
	if filtered.CurrentVarianceAmount == nil || filtered.ForecastExposure == nil {
		t.Fatal("buyer view must retain variance and forecast")
	}
}
