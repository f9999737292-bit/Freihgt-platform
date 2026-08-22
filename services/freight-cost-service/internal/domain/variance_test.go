package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func mustMoney(t *testing.T, amount string) *Money {
	t.Helper()
	d, err := ParseMoneyAmount(amount)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewMoney(d, "RUB")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFC_C_VAR_001_CurrentVarianceOverPlan(t *testing.T) {
	t.Parallel()
	planned := mustMoney(t, "100.00")
	actual := mustMoney(t, "120.00")
	v, err := CalculateCurrentVariance(planned, actual)
	if err != nil || v == nil || !v.Amount.Equal(decimal.RequireFromString("20.00")) {
		t.Fatalf("variance = %v err = %v", v, err)
	}
}

func TestFC_C_VAR_002_CurrentVarianceUnderPlan(t *testing.T) {
	t.Parallel()
	planned := mustMoney(t, "100.00")
	actual := mustMoney(t, "80.00")
	v, err := CalculateCurrentVariance(planned, actual)
	if err != nil || v == nil || !v.Amount.Equal(decimal.RequireFromString("-20.00")) {
		t.Fatalf("variance = %v", v)
	}
}

func TestFC_C_VAR_003_NullActualNullVariance(t *testing.T) {
	t.Parallel()
	v, err := CalculateCurrentVariance(mustMoney(t, "100.00"), nil)
	if err != nil || v != nil {
		t.Fatalf("expected nil variance, got %v", v)
	}
}

func TestFC_C_VAR_004_CurrencyMismatchNullVariance(t *testing.T) {
	t.Parallel()
	planned := mustMoney(t, "100.00")
	actual, _ := NewMoney(decimal.RequireFromString("100.00"), "USD")
	v, err := CalculateCurrentVariance(planned, actual)
	if err != nil || v != nil {
		t.Fatalf("expected nil variance on currency mismatch")
	}
}

func TestFC_C_VAR_005_VariancePercentNullWhenPlannedZero(t *testing.T) {
	t.Parallel()
	planned := mustMoney(t, "0.00")
	variance := mustMoney(t, "10.00")
	p, err := CalculateVariancePercent(variance, planned)
	if err != nil || p != nil {
		t.Fatalf("expected nil percent")
	}
}

func TestFC_C_FOR_006_KnownEmptyProposedEqualsPlanned(t *testing.T) {
	t.Parallel()
	projection := &CostSummaryProjection{
		CurrencyCode:  "RUB",
		PlannedAmount: moneyDecimalPtr("500.00"),
	}
	planned, _ := NewMoney(*projection.PlannedAmount, "RUB")
	forecast, err := CalculateForecastExposure(planned, nil)
	if err != nil || forecast == nil || !forecast.Amount.Equal(decimal.RequireFromString("500.00")) {
		t.Fatalf("forecast = %v", forecast)
	}
}

func TestFC_C_FOR_007_UnknownProposedRetainsPrior(t *testing.T) {
	t.Parallel()
	prior := decimal.RequireFromString("550.00")
	projection := &CostSummaryProjection{
		CurrencyCode:     "RUB",
		PlannedAmount:    moneyDecimalPtr("500.00"),
		ForecastExposure: &prior,
	}
	err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}, &prior)
	if err != nil {
		t.Fatal(err)
	}
	if projection.ForecastExposure == nil || !projection.ForecastExposure.Equal(prior) {
		t.Fatalf("forecast changed on unknown source")
	}
}

func TestFC_C_REA_009_SnapshotFuelAloneNoDriverWithoutAccessorial(t *testing.T) {
	t.Parallel()
	projection := &CostSummaryProjection{
		TenantID:              uuidNew(),
		TransportOrderID:      uuidNew(),
		PlannedAmount:         moneyDecimalPtr("500.00"),
		CurrentVarianceAmount: moneyDecimalPtr("50.00"),
		ProjectionRevision:    1,
	}
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	if len(drivers) != 1 || drivers[0].ReasonCode != ReasonUnattributed {
		t.Fatalf("expected UNATTRIBUTED without accessorial evidence, got %+v", drivers)
	}
}

func moneyDecimalPtr(v string) *decimal.Decimal {
	d := decimal.RequireFromString(v)
	return &d
}

func uuidNew() uuid.UUID {
	return uuid.MustParse("11111111-1111-1111-1111-111111111111")
}
