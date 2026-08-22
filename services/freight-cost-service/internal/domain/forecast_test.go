package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFC_C_FOR_001_ForecastEqualsPlannedPlusProposed(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	proposed := Money{Amount: decimal.RequireFromString("75.00"), Currency: "RUB"}
	forecast, err := CalculateForecastExposure(planned, []Money{proposed})
	if err != nil {
		t.Fatal(err)
	}
	if forecast == nil || !forecast.Amount.Equal(decimal.RequireFromString("1075.00")) {
		t.Fatalf("forecast = %v", forecast)
	}
}

func TestFC_C_FOR_002_ForecastKnownEmptyEqualsPlanned(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	forecast, err := CalculateForecastExposure(planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if forecast == nil || !forecast.Amount.Equal(planned.Amount) {
		t.Fatalf("known-empty proposed set must equal planned, got %v", forecast)
	}
}

func TestFC_C_FOR_003_ForecastCurrencyMismatchNull(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	proposed := Money{Amount: decimal.RequireFromString("50.00"), Currency: "USD"}
	if _, err := CalculateForecastExposure(planned, []Money{proposed}); err != ErrCurrencyMismatch {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
}

func TestFC_C_FOR_004_ForecastNullWhenPlannedMissing(t *testing.T) {
	t.Parallel()
	forecast, err := CalculateForecastExposure(nil, []Money{{Amount: decimal.RequireFromString("10.00"), Currency: "RUB"}})
	if err != nil {
		t.Fatal(err)
	}
	if forecast != nil {
		t.Fatalf("expected nil forecast without planned, got %v", forecast)
	}
}

func TestFC_C_FOR_005_ProposedNeverAffectsAccrual(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	proposed := Money{Amount: decimal.RequireFromString("200.00"), Currency: "RUB"}
	accrual, err := CalculateAccrual(planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	forecast, err := CalculateForecastExposure(planned, []Money{proposed})
	if err != nil {
		t.Fatal(err)
	}
	if !accrual.Amount.Equal(decimal.RequireFromString("1000.00")) {
		t.Fatalf("accrual must exclude proposed-only exposure, got %v", accrual)
	}
	if forecast == nil || !forecast.Amount.Equal(decimal.RequireFromString("1200.00")) {
		t.Fatalf("forecast must include proposed exposure, got %v", forecast)
	}
}

func TestFC_C_FOR_006_KnownEmptyProposedSetForecastEqualsPlanned(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceKnown}); err != nil {
		t.Fatal(err)
	}
	if projection.ForecastExposure == nil || !projection.ForecastExposure.Equal(decimal.RequireFromString("1000.00")) {
		t.Fatalf("known-empty proposed set forecast = %v", projection.ForecastExposure)
	}
	if projection.ForecastSourceStatus != ForecastSourceKnown {
		t.Fatalf("forecast source = %q", projection.ForecastSourceStatus)
	}
}

func TestFC_C_FOR_007_UnknownProposedSourceNullForecast(t *testing.T) {
	t.Parallel()
	prior := decimal.RequireFromString("550.00")
	projection := &CostSummaryProjection{
		CurrencyCode:     "RUB",
		PlannedAmount:    moneyDecimalPtr("500.00"),
		ForecastExposure: &prior,
	}
	changed, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown})
	if err != nil {
		t.Fatal(err)
	}
	if projection.ForecastExposure != nil {
		t.Fatalf("forecast must be NULL on unknown source, got %v", projection.ForecastExposure)
	}
	if projection.ForecastSourceStatus != ForecastSourceUnknown {
		t.Fatalf("forecast source status = %q", projection.ForecastSourceStatus)
	}
	_ = changed
}

func TestFC_C_FOR_008_ProposedToApprovedDecreasesForecast(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	proposed := decimal.RequireFromString("100.00")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{
		SourceStatus: ProposedSourceKnown,
		TotalExVAT:   &proposed,
	}); err != nil {
		t.Fatal(err)
	}
	withProposed := projection.ForecastExposure
	projection.AccruedAmount = moneyDecimalPtr("1100.00")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{
		SourceStatus: ProposedSourceKnown,
	}); err != nil {
		t.Fatal(err)
	}
	if withProposed == nil || projection.ForecastExposure == nil {
		t.Fatal("expected forecast values before and after approval transition")
	}
	if !withProposed.GreaterThan(*projection.ForecastExposure) {
		t.Fatalf("approved transition must decrease forecast: before=%s after=%s", withProposed, projection.ForecastExposure)
	}
	if !projection.AccruedAmount.GreaterThan(decimal.RequireFromString("1000.00")) {
		t.Fatalf("accrual must increase after approval, got %v", projection.AccruedAmount)
	}
}
