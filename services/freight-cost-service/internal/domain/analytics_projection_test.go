package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestSumDecimalPtrExact(t *testing.T) {
	a := decimal.RequireFromString("0.10")
	b := decimal.RequireFromString("0.20")
	sum := SumDecimalPtr(&a, &b)
	if sum == nil || !sum.Equal(decimal.RequireFromString("0.30")) {
		t.Fatalf("expected 0.30, got %v", sum)
	}
}

func TestSumDecimalPtrNullSafe(t *testing.T) {
	a := decimal.RequireFromString("100.00")
	sum := SumDecimalPtr(nil, &a, nil)
	if sum == nil || !sum.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("expected 100.00, got %v", sum)
	}
	if SumDecimalPtr(nil, nil) != nil {
		t.Fatal("expected nil sum for all nil inputs")
	}
}

func TestPeriodStartFromSummaryUpdatedAt(t *testing.T) {
	ts := time.Date(2026, 8, 23, 15, 30, 0, 0, time.UTC)
	start := PeriodStartFromSummaryUpdatedAt(ts)
	want := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(want) {
		t.Fatalf("expected %v, got %v", want, start)
	}
}

func TestOrderFactFromCostSummaryUsesProjectionNotLedger(t *testing.T) {
	planned := decimal.RequireFromString("1000.00")
	current := decimal.RequireFromString("1100.00")
	projection := &CostSummaryProjection{
		TenantID:          uuid.New(),
		TransportOrderID:  uuid.New(),
		BuyerCompanyID:    uuid.New(),
		CarrierCompanyID:  uuid.New(),
		CurrencyCode:      "RUB",
		PlannedAmount:     &planned,
		CurrentActualAmount: &current,
		DataStage:         DataStageCurrentActualAvailable,
		FinancialFinality: FinancialFinalityCurrentActual,
		ProjectionRevision: 3,
	}
	updatedAt := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	fact := OrderFactFromCostSummary(projection, updatedAt, time.Now().UTC())
	if fact == nil {
		t.Fatal("expected order fact")
	}
	if !fact.PlannedAmount.Equal(planned) || !fact.CurrentActualAmount.Equal(current) {
		t.Fatal("order fact must mirror active cost summary projection")
	}
	if fact.PeriodStart.Month() != time.March {
		t.Fatalf("expected March period, got %v", fact.PeriodStart)
	}
}
