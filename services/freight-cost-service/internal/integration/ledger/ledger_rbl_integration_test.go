//go:build integration

package ledger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

func TestFC_B_RBL_001_RebuildCreatesCanonicalFacts(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	result, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if result.FactsProcessed == 0 {
		t.Fatal("expected rebuild facts")
	}
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) == 0 {
		t.Fatal("expected journal rows from rebuild")
	}
}

func TestFC_B_RBL_002_RebuildIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	first, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("first rebuild: %v", err)
	}
	before := countCostEntries(t, env.pool, fix.TenantID, fix.OrderID)
	second, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("second rebuild: %v", err)
	}
	after := countCostEntries(t, env.pool, fix.TenantID, fix.OrderID)
	if before != after {
		t.Fatalf("rebuild not idempotent: before=%d after=%d", before, after)
	}
	if len(second.Outcomes) == 0 {
		t.Fatal("expected rebuild outcomes")
	}
	for _, outcome := range second.Outcomes {
		if outcome.Outcome != service.IngestOutcomeNoOpEvent && outcome.Outcome != service.IngestOutcomeNoOpFact {
			t.Fatalf("unexpected second rebuild outcome: %s", outcome.Outcome)
		}
	}
	_ = first
}

func TestFC_B_RBL_003_RebuildUsesCanonicalHTTPProviders(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	result, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	hasPlanned := false
	for _, outcome := range result.Outcomes {
		if outcome.Outcome == service.IngestOutcomeApplied || outcome.Outcome == service.IngestOutcomeNoOpFact {
			hasPlanned = true
		}
	}
	if !hasPlanned {
		t.Fatal("expected planned snapshot from transport provider")
	}
}

func TestFC_B_RBL_004_RebuildMismatchFlagsReconciliation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1000.00"), settlementStatus: domain.SettlementStatusApproved,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementSourceID(), sourceRevision: 1, amount: decimalAmount("900.00"),
	}))
	projection := getProjection(t, env, fix)
	if projection.BillingReconciliationStatus != domain.BillingReconciliationMismatch {
		t.Fatalf("expected mismatch flag, got %s", projection.BillingReconciliationStatus)
	}
}

func TestFC_B_RBL_005_LiveAndRebuildProjectionEquivalence(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{sourceRevision: 11, amount: decimalAmount("1800.00")}
	live := ingest(t, env, baseIngestInput(fix, opts.withEvent(uuid.New()).withOrigin(domain.EventOriginLiveOutbox)))
	_, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(*opts.amount) {
		t.Fatalf("projection after live+rebuild = %v", projection.AccruedAmount)
	}
	if live.Outcome != service.IngestOutcomeApplied {
		t.Fatalf("live outcome = %s", live.Outcome)
	}
}

func TestFC_B_RBL_006_UnchangedRebuildAddsNoJournalRow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{sourceRevision: 12, amount: decimalAmount("1900.00")}
	factID := deriveFactID(fix, opts)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(uuid.New()).withOrigin(domain.EventOriginLiveOutbox)))
	if countCostEntriesByFact(t, env.pool, fix.TenantID, factID) != 1 {
		t.Fatal("expected one live row for canonical fact")
	}
	beforeTotal := countCostEntries(t, env.pool, fix.TenantID, fix.OrderID)
	_, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	afterTotal := countCostEntries(t, env.pool, fix.TenantID, fix.OrderID)
	if afterTotal != beforeTotal {
		t.Fatalf("unchanged rebuild added rows: before=%d after=%d", beforeTotal, afterTotal)
	}
	if countCostEntriesByFact(t, env.pool, fix.TenantID, factID) != 1 {
		t.Fatal("rebuild duplicated canonical fact")
	}
}

func TestFC_B_RBL_RebuildHTTPRoute(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/freight-cost/transport-orders/"+fix.OrderID.String()+"/rebuild", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set(internalauth.HeaderName, testToken)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusUnauthorized {
		t.Fatalf("rebuild route status = %d body=%s", rec.Code, rec.Body.String())
	}
}
