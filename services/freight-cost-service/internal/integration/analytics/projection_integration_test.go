//go:build integration

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC22BRebuildTenantCurrencySeparated(t *testing.T) {
	env := setupAnalyticsEnv(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	buyerA := uuid.New()
	buyerB := uuid.New()
	carrier := uuid.New()
	orderRUB := uuid.New()
	orderEUR := uuid.New()
	orderOtherTenant := uuid.New()
	period := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	upsertSummary(t, env, tenantA, buyerA, carrier, orderRUB, "RUB", "1000.00", "1100.00", period)
	upsertSummary(t, env, tenantA, buyerA, carrier, orderEUR, "EUR", "200.00", "250.00", period)
	upsertSummary(t, env, tenantB, buyerB, carrier, orderOtherTenant, "RUB", "500.00", "600.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantA); err != nil {
		t.Fatalf("rebuild tenant A: %v", err)
	}

	keyRUB := domainPeriodKey(tenantA, buyerA, period, "RUB")
	keyEUR := domainPeriodKey(tenantA, buyerA, period, "EUR")
	projRUB, err := env.periods.GetByKey(context.Background(), tenantA, keyRUB)
	if err != nil {
		t.Fatalf("get rub projection: %v", err)
	}
	projEUR, err := env.periods.GetByKey(context.Background(), tenantA, keyEUR)
	if err != nil {
		t.Fatalf("get eur projection: %v", err)
	}
	if projRUB.OrderCount != 1 || projEUR.OrderCount != 1 {
		t.Fatalf("expected separate order counts, got rub=%d eur=%d", projRUB.OrderCount, projEUR.OrderCount)
	}
	if !decimal.RequireFromString("1100.00").Equal(*projRUB.CurrentActualTotal) {
		t.Fatalf("unexpected rub total: %v", projRUB.CurrentActualTotal)
	}
	if !decimal.RequireFromString("250.00").Equal(*projEUR.CurrentActualTotal) {
		t.Fatalf("unexpected eur total: %v", projEUR.CurrentActualTotal)
	}

	_, err = env.periods.GetByKey(context.Background(), tenantB, domainPeriodKey(tenantB, buyerB, period, "RUB"))
	if err == nil {
		t.Fatal("tenant B must not be visible when querying tenant A scope through tenant-scoped rebuild only")
	}
}

func TestFC22BRebuildIdempotent(t *testing.T) {
	env := setupAnalyticsEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	period := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	upsertSummary(t, env, tenantID, buyerID, carrierID, orderID, "RUB", "100.00", "120.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("first rebuild: %v", err)
	}
	first, err := env.periods.GetByKey(context.Background(), tenantID, domainPeriodKey(tenantID, buyerID, period, "RUB"))
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("second rebuild: %v", err)
	}
	second, err := env.periods.GetByKey(context.Background(), tenantID, domainPeriodKey(tenantID, buyerID, period, "RUB"))
	if err != nil {
		t.Fatalf("get second: %v", err)
	}
	if first.OrderCount != second.OrderCount ||
		!first.CurrentActualTotal.Equal(*second.CurrentActualTotal) ||
		first.ProjectionVersion != second.ProjectionVersion {
		t.Fatalf("rebuild not idempotent: first=%+v second=%+v", first, second)
	}
}

func TestFC22BEqvRebuildMatchesIncremental(t *testing.T) {
	env := setupAnalyticsEnv(t)
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	period := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	upsertSummary(t, env, tenantID, buyerID, carrierID, orderID, "RUB", "300.00", "330.00", period)

	if err := env.analytics.RebuildTenant(context.Background(), tenantID); err != nil {
		t.Fatalf("full rebuild: %v", err)
	}
	full, err := env.periods.GetByKey(context.Background(), tenantID, domainPeriodKey(tenantID, buyerID, period, "RUB"))
	if err != nil {
		t.Fatalf("get full: %v", err)
	}

	// Reset analytics tables and simulate incremental path via dirty processing.
	if _, err := env.pool.Exec(context.Background(), `
		DELETE FROM freight_cost.cost_analytics_period_projection WHERE tenant_id = $1;
		DELETE FROM freight_cost.cost_analytics_order_fact WHERE tenant_id = $1;
		DELETE FROM freight_cost.analytics_projection_state WHERE tenant_id = $1`, tenantID); err != nil {
		t.Fatalf("reset analytics: %v", err)
	}
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.analytics.MarkCostSummaryChanged(context.Background(), tx, serviceAnalyticsChange(tenantID, buyerID, orderID, "RUB", period)); err != nil {
		t.Fatalf("mark dirty: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit dirty: %v", err)
	}
	if _, err := env.analytics.ProcessDirtyBatch(context.Background(), 10); err != nil {
		t.Fatalf("incremental batch: %v", err)
	}
	incr, err := env.periods.GetByKey(context.Background(), tenantID, domainPeriodKey(tenantID, buyerID, period, "RUB"))
	if err != nil {
		t.Fatalf("get incremental: %v", err)
	}
	if full.OrderCount != incr.OrderCount ||
		!full.CurrentActualTotal.Equal(*incr.CurrentActualTotal) ||
		!full.PlannedTotal.Equal(*incr.PlannedTotal) {
		t.Fatalf("equivalence failed full=%+v incr=%+v", full, incr)
	}
}

func domainPeriodKey(tenantID, buyerID uuid.UUID, period time.Time, currency string) domain.AnalyticsPeriodKey {
	return domain.AnalyticsPeriodKey{
		TenantID:       tenantID,
		BuyerCompanyID: buyerID,
		PeriodStart:    domain.PeriodStartFromSummaryUpdatedAt(period),
		PeriodGrain:    domain.AnalyticsPeriodGrainMonth,
		CurrencyCode:   currency,
	}
}

func serviceAnalyticsChange(tenantID, buyerID, orderID uuid.UUID, currency string, updatedAt time.Time) service.AnalyticsChangeInput {
	return service.AnalyticsChangeInput{
		TenantID:         tenantID,
		TransportOrderID: orderID,
		BuyerCompanyID:   buyerID,
		CurrencyCode:     currency,
		SummaryUpdatedAt: updatedAt,
		SourceEventID:    uuid.New(),
	}
}
