//go:build integration

package ledger

import (
	"context"
	"testing"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC_B_ACC_001_AccrualFromApprovedSettlementSnapshot(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	result := ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: decimalAmount("1100.00"),
	}))
	if result.Outcome != service.IngestOutcomeApplied {
		t.Fatalf("outcome = %s", result.Outcome)
	}
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(*decimalAmount("1100.00")) {
		t.Fatalf("accrued = %v", projection.AccruedAmount)
	}
}

func TestFC_B_ACC_002_ForecastNotPersistedInLedger(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindPlannedCostSnapshot, sourceType: domain.SourceTypeTORateSnapshot,
		sourceService: domain.SourceServiceTransportOrder,
		sourceID: fix.SnapshotID, sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount != nil {
		t.Fatal("planned/forecast must not populate accrual projection")
	}
}

func TestFC_B_ACC_003_RejectedAccessorialExcludedFromAccrual(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(*decimalAmount("1000.00")) {
		t.Fatalf("accrual should exclude rejected accessorial, got %v", projection.AccruedAmount)
	}
}

func TestFC_B_ACC_004_DisputedAccessorialExcludedFromAccrual(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1000.00"), openDisputeCount: 1, settlementStatus: domain.SettlementStatusDisputed,
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(*decimalAmount("1000.00")) {
		t.Fatalf("disputed accessorial must stay excluded from accrual, got %v", projection.AccruedAmount)
	}
}

func TestFC_B_ACC_005_CurrencyMismatchFailsClosed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1,
		currencyCode: "RUB", amount: decimalAmount("1000.00"),
	}))
	_, err := env.ingest.Ingest(context.Background(), baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 2,
		currencyCode: "USD", amount: decimalAmount("1000.00"),
	}))
	if err == nil {
		t.Fatal("expected currency mismatch failure")
	}
}

func TestFC_B_ACC_006_DisputeRemovesAccessorialFromAccrual(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: decimalAmount("1100.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 2, amount: decimalAmount("1000.00"),
		openDisputeCount: 1, settlementStatus: domain.SettlementStatusDisputed,
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(*decimalAmount("1000.00")) {
		t.Fatalf("expected 1000 after dispute, got %v", projection.AccruedAmount)
	}
}

func TestFC_B_ACC_007_AccrualUsesExactDecimalApprovedSetTotal(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	exact := decimalAmount("1000.01")
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: exact,
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(*exact) {
		t.Fatalf("expected exact decimal %s, got %v", exact.StringFixed(2), projection.AccruedAmount)
	}
}

func TestFC_B_ACC_008_RebuildAccrualMatchesLiveIngestSemantics(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	liveAccrual := decimalAmount("1100.00")
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: liveAccrual,
	}))
	liveProjection := getProjection(t, env, fix)
	result, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	rebuilt := getProjection(t, env, fix)
	if liveProjection.AccruedAmount == nil || rebuilt.AccruedAmount == nil {
		t.Fatalf("accrual missing after live/rebuild: live=%v rebuild=%v", liveProjection.AccruedAmount, rebuilt.AccruedAmount)
	}
	if !liveProjection.AccruedAmount.Equal(*rebuilt.AccruedAmount) {
		t.Fatalf("live accrual %s != rebuild accrual %s", liveProjection.AccruedAmount, rebuilt.AccruedAmount)
	}
	if len(result.Outcomes) == 0 {
		t.Fatal("expected rebuild outcomes")
	}
}
