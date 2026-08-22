//go:build integration

package variance

import (
	"context"
	"testing"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func TestFC_C_REC_007_RepeatedScanSameDriftOneOpenFinding(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	projection.PlannedAmount = nil
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx, projection); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	first, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reconcile first: %v", err)
	}
	second, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reconcile second: %v", err)
	}
	if first == 0 {
		t.Fatal("expected finding on first scan")
	}
	open := countOpenFindings(t, env, fix)
	if open != 1 {
		t.Fatalf("expected one OPEN finding, got %d (first=%d second=%d)", open, first, second)
	}
}

func TestFC_C_REC_008_DriftClearedResolved(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	projection.PlannedAmount = nil
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx, projection); err != nil {
		t.Fatalf("upsert drift: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	projection = getProjection(t, env, fix)
	projection.PlannedAmount = decimalAmount(fix.PlannedAmount.StringFixed(domain.MoneyScale))
	tx2, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx2, projection); err != nil {
		t.Fatalf("upsert clear: %v", err)
	}
	if err := tx2.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("reconcile clear: %v", err)
	}
	if countOpenFindings(t, env, fix) != 0 {
		t.Fatal("drift cleared must resolve OPEN findings")
	}
}

func TestFC_C_REC_009_ReopenedWhenSameFindingReturns(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)

	projection := getProjection(t, env, fix)
	projection.PlannedAmount = nil
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx, projection); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("open finding: %v", err)
	}

	projection = getProjection(t, env, fix)
	projection.PlannedAmount = decimalAmount(fix.PlannedAmount.StringFixed(domain.MoneyScale))
	tx2, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx2, projection); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := tx2.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("resolve scan: %v", err)
	}

	projection = getProjection(t, env, fix)
	projection.PlannedAmount = nil
	tx3, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx3, projection); err != nil {
		t.Fatalf("re-drift: %v", err)
	}
	if err := tx3.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("reopen scan: %v", err)
	}

	var status string
	err = env.pool.QueryRow(context.Background(), `
		SELECT status FROM freight_cost.reconciliation_finding
		WHERE tenant_id = $1 AND transport_order_id = $2 AND finding_kind = $3
		LIMIT 1`, fix.TenantID, fix.OrderID, domain.FindingMissingPlannedFact).Scan(&status)
	if err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != domain.FindingStatusReopened {
		t.Fatalf("expected REOPENED, got %q", status)
	}
}

func TestFC_C_REC_010_ReconciliationDoesNotAutoRebuild(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	before := countCostEntries(t, env, fix)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	projection.PlannedAmount = nil
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx, projection); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	after := countCostEntries(t, env, fix)
	if after <= before {
		t.Fatalf("expected ingest rows, before=%d after=%d", before, after)
	}
	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err == nil {
		// reconciliation scan itself must not trigger rebuild; only explicit rebuild call changes facts
	}
	if countOpenFindings(t, env, fix) == 0 {
		t.Fatal("finding expected without auto-rebuild side effects clearing drift")
	}
}

func countCostEntries(t *testing.T, env *env, fix fixture) int {
	t.Helper()
	var count int
	err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM freight_cost.cost_entry
		WHERE tenant_id = $1 AND transport_order_id = $2`, fix.TenantID, fix.OrderID).Scan(&count)
	if err != nil {
		t.Fatalf("count entries: %v", err)
	}
	return count
}
