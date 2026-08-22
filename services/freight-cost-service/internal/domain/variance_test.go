package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFC_C_VAR_001_CurrentVariancePositiveOverPlan(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	actual := mustNewMoney(t, "1100.00", "RUB")
	variance, err := CalculateCurrentVariance(planned, actual)
	if err != nil {
		t.Fatal(err)
	}
	if variance == nil || !variance.Amount.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("variance = %v", variance)
	}
}

func TestFC_C_VAR_002_CurrentVarianceNegativeUnderPlan(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	actual := mustNewMoney(t, "900.00", "RUB")
	variance, err := CalculateCurrentVariance(planned, actual)
	if err != nil {
		t.Fatal(err)
	}
	if variance == nil || !variance.Amount.Equal(decimal.RequireFromString("-100.00")) {
		t.Fatalf("variance = %v", variance)
	}
}

func TestFC_C_VAR_003_CurrentVarianceNullWhenPlannedMissing(t *testing.T) {
	t.Parallel()
	actual := mustNewMoney(t, "1000.00", "RUB")
	variance, err := CalculateCurrentVariance(nil, actual)
	if err != nil {
		t.Fatal(err)
	}
	if variance != nil {
		t.Fatalf("expected nil variance, got %v", variance)
	}
}

func TestFC_C_VAR_004_CurrentVarianceNullWhenActualMissing(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	variance, err := CalculateCurrentVariance(planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if variance != nil {
		t.Fatalf("expected nil variance, got %v", variance)
	}
}

func TestFC_C_VAR_005_FinalVarianceWhenReadyForPayment(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	final := mustNewMoney(t, "1050.00", "RUB")
	variance, err := CalculateFinalVariance(planned, final)
	if err != nil {
		t.Fatal(err)
	}
	if variance == nil || !variance.Amount.Equal(decimal.RequireFromString("50.00")) {
		t.Fatalf("final variance = %v", variance)
	}
}

func TestFC_C_VAR_006_FinalVarianceNullWhenNotFinal(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	variance, err := CalculateFinalVariance(planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if variance != nil {
		t.Fatalf("expected nil final variance, got %v", variance)
	}
}

func TestFC_C_VAR_007_VariancePercentWhenPlannedPositive(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	variance := &Money{Amount: decimal.RequireFromString("50.00"), Currency: "RUB"}
	percent, err := CalculateVariancePercent(variance, planned)
	if err != nil {
		t.Fatal(err)
	}
	if percent == nil || !percent.Equal(decimal.RequireFromString("5.0000")) {
		t.Fatalf("percent = %v", percent)
	}
}

func TestFC_C_VAR_008_VariancePercentNullWhenPlannedZero(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "0.00", "RUB")
	variance := &Money{Amount: decimal.RequireFromString("10.00"), Currency: "RUB"}
	percent, err := CalculateVariancePercent(variance, planned)
	if err != nil {
		t.Fatal(err)
	}
	if percent != nil {
		t.Fatalf("expected nil percent, got %v", percent)
	}
}

func TestFC_C_VAR_009_CurrencyMismatchYieldsNullVariance(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	actual := &Money{Amount: decimal.RequireFromString("1000.00"), Currency: "USD"}
	variance, err := CalculateCurrentVariance(planned, actual)
	if err != nil {
		t.Fatal(err)
	}
	if variance != nil {
		t.Fatalf("expected nil on currency mismatch, got %v", variance)
	}
}

func TestFC_C_VAR_010_DisputeNullifiesCurrentVariance(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	projection.OpenDisputeCount = 1
	projection.SettlementStatus = SettlementStatusDisputed
	projection.CurrentActualAmount = nil
	changed, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown})
	if err != nil {
		t.Fatal(err)
	}
	if projection.CurrentVarianceAmount != nil {
		t.Fatalf("dispute must nullify variance, got %v", projection.CurrentVarianceAmount)
	}
	_ = changed
}

func TestFC_C_VAR_011_RecomputeDerivedProjectionUpdatesVariance(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1150.00", "")
	changed, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown})
	if err != nil {
		t.Fatal(err)
	}
	if projection.CurrentVarianceAmount == nil || !projection.CurrentVarianceAmount.Equal(decimal.RequireFromString("150.00")) {
		t.Fatalf("current variance = %v", projection.CurrentVarianceAmount)
	}
	if !changed {
		t.Fatal("expected fingerprint change on first compute")
	}
}

func TestFC_C_VAR_012_VarianceSignPositiveIsOverPlan(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("500.00", "550.00", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	if projection.CurrentVarianceAmount == nil || !projection.CurrentVarianceAmount.IsPositive() {
		t.Fatalf("positive variance means over plan, got %v", projection.CurrentVarianceAmount)
	}
}
