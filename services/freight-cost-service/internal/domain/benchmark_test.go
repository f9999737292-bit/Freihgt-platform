package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestBenchmarkDataQualityForSample(t *testing.T) {
	if BenchmarkDataQualityForSample(4, 5) != DataQualityInsufficientSample {
		t.Fatal("expected INSUFFICIENT_SAMPLE below min")
	}
	if BenchmarkDataQualityForSample(5, 5) != DataQualityAvailable {
		t.Fatal("expected AVAILABLE at min")
	}
}

func TestBenchmarkEligibleCostAmountUsesFinality(t *testing.T) {
	final := decimal.RequireFromString("100.00")
	current := decimal.RequireFromString("80.00")
	lane := "RU:MOSCOW->RU:SPB|ROAD|TENT"
	fact := &AnalyticsOrderFact{
		LaneEligible:      true,
		LaneKey:           &lane,
		FinancialFinality: FinancialFinalityFinalActual,
		FinalActualAmount: &final,
		CurrentActualAmount: &current,
	}
	got := BenchmarkEligibleCostAmount(fact)
	if got == nil || !got.Equal(final) {
		t.Fatalf("expected final actual, got %v", got)
	}
	fact.FinancialFinality = FinancialFinalityCurrentActual
	got = BenchmarkEligibleCostAmount(fact)
	if got == nil || !got.Equal(current) {
		t.Fatalf("expected current actual, got %v", got)
	}
	fact.FinancialFinality = FinancialFinalityDraft
	if BenchmarkEligibleCostAmount(fact) != nil {
		t.Fatal("draft must be excluded")
	}
}

func TestDeriveOpportunityIDDeterministic(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	buyerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	period := mustDate(t, "2026-06-01")
	a := DeriveOpportunityID(tenantID, buyerID, OpportunityTypeLaneCostOutlier, OpportunityScopeOrder, "entity", "RUB", period, OpportunityRuleVersion)
	b := DeriveOpportunityID(tenantID, buyerID, OpportunityTypeLaneCostOutlier, OpportunityScopeOrder, "entity", "RUB", period, OpportunityRuleVersion)
	if a != b {
		t.Fatalf("expected deterministic id, got %s vs %s", a, b)
	}
}

func mustDate(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return parsed
}
