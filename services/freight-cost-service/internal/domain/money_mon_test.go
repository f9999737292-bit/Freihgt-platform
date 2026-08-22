package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFC_C_MON_001_VarianceDecimalTwoScale(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1000.333", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	if projection.CurrentVarianceAmount == nil {
		t.Fatal("expected variance")
	}
	if projection.CurrentVarianceAmount.StringFixed(MoneyScale) != "0.33" {
		t.Fatalf("variance scale = %s", projection.CurrentVarianceAmount.StringFixed(MoneyScale))
	}
}

func TestFC_C_MON_002_ForecastDecimalTwoScale(t *testing.T) {
	t.Parallel()
	proposed := decimal.RequireFromString("10.556")
	projection := projectionWithAmounts("1000.00", "", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{
		SourceStatus: ProposedSourceKnown,
		TotalExVAT:   &proposed,
	}); err != nil {
		t.Fatal(err)
	}
	if projection.ForecastExposure == nil || projection.ForecastExposure.StringFixed(MoneyScale) != "1010.56" {
		t.Fatalf("forecast = %v", projection.ForecastExposure)
	}
}

func TestFC_C_MON_003_VariancePercentFourScale(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1033.33", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	if projection.CurrentVariancePercent == nil || projection.CurrentVariancePercent.StringFixed(4) != "3.3330" {
		t.Fatalf("percent = %v", projection.CurrentVariancePercent)
	}
}

func TestFC_C_MON_004_NullNotZeroInVariance(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	if projection.CurrentVarianceAmount != nil {
		t.Fatalf("NULL must not become zero, got %v", projection.CurrentVarianceAmount)
	}
}
