//go:build integration

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

type benchmarkEnv struct {
	*laneCarrierEnv
	benchmarks    *repository.AnalyticsBenchmarkProjectionRepository
	opportunities *repository.AnalyticsOpportunityProjectionRepository
}

func setupBenchmarkEnv(t *testing.T) *benchmarkEnv {
	t.Helper()
	base := setupLaneCarrierEnv(t)
	return &benchmarkEnv{
		laneCarrierEnv: base,
		benchmarks:     repository.NewAnalyticsBenchmarkProjectionRepository(base.pool),
		opportunities:  repository.NewAnalyticsOpportunityProjectionRepository(base.pool),
	}
}

func seedFiveOrderLaneBenchmark(t *testing.T, env *benchmarkEnv, tenantID, buyerID, carrierID uuid.UUID, amounts []string, period time.Time) string {
	t.Helper()
	equipment := "TENT"
	var firstOrderID uuid.UUID
	for i, amount := range amounts {
		orderID := uuid.New()
		if i == 0 {
			firstOrderID = orderID
		}
		seedTransportOrderWithLocations(t, env.laneCarrierEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
		upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", amount, amount, period)
	}
	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("rebuild lane cohort: %v", err)
	}
	fact, err := repository.NewAnalyticsOrderFactRepository(env.pool).GetByTransportOrder(context.Background(), tenantID, firstOrderID, "RUB")
	if err != nil || fact.LaneKey == nil {
		t.Fatalf("lane key missing: err=%v", err)
	}
	return *fact.LaneKey
}

func TestFC22EBM001FiveValueMedian(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	laneKey := seedFiveOrderLaneBenchmark(t, env, tenantID, buyerID, carrierID,
		[]string{"1000.00", "2000.00", "3000.00", "4000.00", "5000.00"}, period)

	rows, err := env.benchmarks.List(context.Background(), tenantID, repository.AnalyticsBenchmarkListFilter{
		BuyerCompanyID: &buyerID, CurrencyCode: "RUB", LaneKey: laneKey, Limit: 10,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("benchmark rows: err=%v len=%d", err, len(rows))
	}
	if rows[0].DataQuality != domain.DataQualityAvailable {
		t.Fatalf("expected AVAILABLE, got %s", rows[0].DataQuality)
	}
	if rows[0].MedianAmount == nil || !decimal.RequireFromString("3000.00").Equal(*rows[0].MedianAmount) {
		t.Fatalf("expected median 3000, got %v", rows[0].MedianAmount)
	}
}

func TestFC22EBM008SampleThreshold(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	laneKey := seedFiveOrderLaneBenchmark(t, env, tenantID, buyerID, carrierID,
		[]string{"100.00", "200.00", "300.00", "400.00"}, period)

	rows, err := env.benchmarks.List(context.Background(), tenantID, repository.AnalyticsBenchmarkListFilter{
		BuyerCompanyID: &buyerID, CurrencyCode: "RUB", LaneKey: laneKey, Limit: 10,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("benchmark rows: err=%v len=%d", err, len(rows))
	}
	if rows[0].DataQuality != domain.DataQualityInsufficientSample {
		t.Fatalf("expected INSUFFICIENT_SAMPLE for n=4, got %s", rows[0].DataQuality)
	}
	if rows[0].MedianAmount != nil {
		t.Fatal("percentiles must be omitted below min sample")
	}
}

func TestFC22ECUR001CurrencyIsolation(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	equipment := "TENT"
	for _, spec := range []struct {
		currency string
		amount   string
	}{
		{"RUB", "1000.00"},
		{"RUB", "3000.00"},
		{"RUB", "5000.00"},
		{"EUR", "100.00"},
		{"EUR", "300.00"},
		{"EUR", "500.00"},
	} {
		orderID := uuid.New()
		seedTransportOrderWithLocations(t, env.laneCarrierEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
		upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, spec.currency, spec.amount, spec.amount, period)
	}
	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	rows, err := env.benchmarks.List(context.Background(), tenantID, repository.AnalyticsBenchmarkListFilter{Limit: 20})
	if err != nil || len(rows) != 2 {
		t.Fatalf("expected separate RUB/EUR benchmarks, got err=%v len=%d", err, len(rows))
	}
}

func TestFC22ESEC001CrossTenantIsolation(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	buyerA := uuid.New()
	buyerB := uuid.New()
	carrierA := uuid.New()
	carrierB := uuid.New()
	period := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seedFiveOrderLaneBenchmark(t, env, tenantA, buyerA, carrierA,
		[]string{"100.00", "100.00", "100.00", "100.00", "100.00"}, period)
	seedFiveOrderLaneBenchmark(t, env, tenantB, buyerB, carrierB,
		[]string{"900.00", "900.00", "900.00", "900.00", "900.00"}, period)

	rowsA, err := env.benchmarks.List(context.Background(), tenantA, repository.AnalyticsBenchmarkListFilter{Limit: 10})
	if err != nil || len(rowsA) != 1 {
		t.Fatalf("tenant A benchmark: err=%v len=%d", err, len(rowsA))
	}
	if rowsA[0].MedianAmount == nil || !decimal.RequireFromString("100.00").Equal(*rowsA[0].MedianAmount) {
		t.Fatalf("tenant A median must stay 100, got %v", rowsA[0].MedianAmount)
	}
}

func TestFC22EOPP004EstimatedDelta(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	seedFiveOrderLaneBenchmark(t, env, tenantID, buyerID, carrierID,
		[]string{"38000.00", "38000.00", "38000.00", "38000.00", "45000.00"}, period)

	opps, err := env.opportunities.List(context.Background(), tenantID, repository.AnalyticsOpportunityListFilter{
		OpportunityType: domain.OpportunityTypeCostAboveLaneMedian, Limit: 20,
	})
	if err != nil || len(opps) == 0 {
		t.Fatalf("expected opportunities, err=%v len=%d", err, len(opps))
	}
	found := false
	for _, opp := range opps {
		if opp.ObservedAmount.Equal(decimal.RequireFromString("45000.00")) &&
			opp.BaselineAmount.Equal(decimal.RequireFromString("38000.00")) &&
			opp.EstimatedDelta.Equal(decimal.RequireFromString("7000.00")) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected observed=45000 baseline=38000 delta=7000 opportunity")
	}
}

func TestFC22EOPP001DeterministicID(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	seedFiveOrderLaneBenchmark(t, env, tenantID, buyerID, carrierID,
		[]string{"38000.00", "38000.00", "38000.00", "38000.00", "45000.00"}, period)
	first, _ := env.opportunities.List(context.Background(), tenantID, repository.AnalyticsOpportunityListFilter{Limit: 50})
	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	second, _ := env.opportunities.List(context.Background(), tenantID, repository.AnalyticsOpportunityListFilter{Limit: 50})
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("opportunity count mismatch first=%d second=%d", len(first), len(second))
	}
	idsA := map[uuid.UUID]struct{}{}
	for _, opp := range first {
		idsA[opp.OpportunityID] = struct{}{}
	}
	for _, opp := range second {
		if _, ok := idsA[opp.OpportunityID]; !ok {
			t.Fatalf("rebuild changed opportunity id %s", opp.OpportunityID)
		}
	}
}

func TestFC22EBM003P25P75P90(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	laneKey := seedFiveOrderLaneBenchmark(t, env, tenantID, buyerID, carrierID,
		[]string{"1000.00", "2000.00", "3000.00", "4000.00", "5000.00"}, period)

	rows, err := env.benchmarks.List(context.Background(), tenantID, repository.AnalyticsBenchmarkListFilter{
		BuyerCompanyID: &buyerID, CurrencyCode: "RUB", LaneKey: laneKey, Limit: 10,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("benchmark rows: err=%v len=%d", err, len(rows))
	}
	row := rows[0]
	expect := map[string]string{
		"p25": "2000.00", "p75": "4000.00", "p90": "4600.00", "min": "1000.00", "max": "5000.00", "mean": "3000.00",
	}
	checks := map[string]*decimal.Decimal{
		"p25": row.P25Amount, "p75": row.P75Amount, "p90": row.P90Amount,
		"min": row.MinAmount, "max": row.MaxAmount, "mean": row.MeanAmount,
	}
	for name, ptr := range checks {
		if ptr == nil || !decimal.RequireFromString(expect[name]).Equal(*ptr) {
			t.Fatalf("%s expected %s got %v", name, expect[name], ptr)
		}
	}
}

func TestFC22ESEC002CrossCompanyIsolation(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	companyX := uuid.New()
	companyY := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	seedFiveOrderLaneBenchmark(t, env, tenantID, companyX, carrierID,
		[]string{"100.00", "100.00", "100.00", "100.00", "100.00"}, period)
	seedFiveOrderLaneBenchmark(t, env, tenantID, companyY, carrierID,
		[]string{"900.00", "900.00", "900.00", "900.00", "900.00"}, period)

	rowsX, err := env.benchmarks.List(context.Background(), tenantID, repository.AnalyticsBenchmarkListFilter{
		BuyerCompanyID: &companyX, Limit: 10,
	})
	if err != nil || len(rowsX) != 1 {
		t.Fatalf("company X benchmark: err=%v len=%d", err, len(rowsX))
	}
	if rowsX[0].MedianAmount == nil || !decimal.RequireFromString("100.00").Equal(*rowsX[0].MedianAmount) {
		t.Fatalf("company X median must stay 100, got %v", rowsX[0].MedianAmount)
	}
}

func TestFC22EEQV001RebuildMatchesIncremental(t *testing.T) {
	env := setupBenchmarkEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	period := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	amounts := []string{"38000.00", "38000.00", "38000.00", "38000.00", "45000.00"}
	equipment := "TENT"
	orderIDs := make([]uuid.UUID, len(amounts))
	for i, amount := range amounts {
		orderID := uuid.New()
		orderIDs[i] = orderID
		seedTransportOrderWithLocations(t, env.laneCarrierEnv, tenantID, buyerID, orderID, "Moscow", "SPB", "ROAD", &equipment)
		upsertSummary(t, env.analyticsEnv, tenantID, buyerID, carrierID, orderID, "RUB", amount, amount, period)
	}
	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("full rebuild: %v", err)
	}

	fullBenchmarks, err := env.benchmarks.List(context.Background(), tenantID, repository.AnalyticsBenchmarkListFilter{Limit: 20})
	if err != nil || len(fullBenchmarks) != 1 {
		t.Fatalf("full benchmarks: err=%v len=%d", err, len(fullBenchmarks))
	}
	fullOpps, err := env.opportunities.List(context.Background(), tenantID, repository.AnalyticsOpportunityListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("full opportunities: %v", err)
	}

	ctx := context.Background()
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_benchmark_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_opportunity_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_lane_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_carrier_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_order_fact WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_analytics_period_projection WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_state WHERE tenant_id = $1`, tenantID)
	_, _ = env.pool.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_coverage WHERE tenant_id = $1`, tenantID)

	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	for _, orderID := range orderIDs {
		if err := env.analytics.MarkCostSummaryChanged(ctx, tx, serviceAnalyticsChange(tenantID, buyerID, orderID, "RUB", period)); err != nil {
			t.Fatalf("mark dirty: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.analytics.ProcessDirtyBatch(ctx, 10); err != nil {
		t.Fatalf("incremental: %v", err)
	}

	incrBenchmarks, err := env.benchmarks.List(ctx, tenantID, repository.AnalyticsBenchmarkListFilter{Limit: 20})
	if err != nil || len(incrBenchmarks) != 1 {
		t.Fatalf("incr benchmarks: err=%v len=%d", err, len(incrBenchmarks))
	}
	incrOpps, err := env.opportunities.List(ctx, tenantID, repository.AnalyticsOpportunityListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("incr opportunities: %v", err)
	}
	if fullBenchmarks[0].SampleCount != incrBenchmarks[0].SampleCount ||
		fullBenchmarks[0].DataQuality != incrBenchmarks[0].DataQuality ||
		!fullBenchmarks[0].MedianAmount.Equal(*incrBenchmarks[0].MedianAmount) {
		t.Fatalf("benchmark equivalence failed full=%+v incr=%+v", fullBenchmarks[0], incrBenchmarks[0])
	}
	if len(fullOpps) != len(incrOpps) {
		t.Fatalf("opportunity count mismatch full=%d incr=%d", len(fullOpps), len(incrOpps))
	}
	fullIDs := map[uuid.UUID]struct{}{}
	for _, opp := range fullOpps {
		fullIDs[opp.OpportunityID] = struct{}{}
	}
	for _, opp := range incrOpps {
		if _, ok := fullIDs[opp.OpportunityID]; !ok {
			t.Fatalf("incremental changed opportunity id %s", opp.OpportunityID)
		}
	}
}
