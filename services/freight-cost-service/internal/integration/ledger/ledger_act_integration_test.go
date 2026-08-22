//go:build integration

package ledger

import (
	"testing"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func TestFC_B_ACT_001_CurrentActualAvailableWhenApprovedNoDisputes(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1500.00"), settlementStatus: domain.SettlementStatusApproved,
	}))
	projection := getProjection(t, env, fix)
	if projection.CurrentActualAmount == nil {
		t.Fatal("expected current actual amount")
	}
}

func TestFC_B_ACT_002_CurrentActualNullWhenDraft(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
		settlementStatus: domain.SettlementStatusDraft,
	}))
	projection := getProjection(t, env, fix)
	if projection.CurrentActualAmount != nil {
		t.Fatalf("expected null current actual, got %v", projection.CurrentActualAmount)
	}
}

func TestFC_B_ACT_003_CurrentActualNullWhenDisputeOpen(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
		openDisputeCount: 1, settlementStatus: domain.SettlementStatusDisputed,
	}))
	projection := getProjection(t, env, fix)
	if projection.CurrentActualAmount != nil {
		t.Fatal("dispute should nullify current actual")
	}
}

func TestFC_B_ACT_004_FinalActualOnlyWhenReadyForPayment(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindFinalActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1500.00"), settlementStatus: domain.SettlementStatusReadyForPayment,
	}))
	projection := getProjection(t, env, fix)
	if projection.FinalActualAmount == nil {
		t.Fatal("expected final actual")
	}
}

func TestFC_B_ACT_005_DisputeNullifiesActualProjection(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1500.00"), settlementStatus: domain.SettlementStatusApproved,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 2,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
		openDisputeCount: 2, settlementStatus: domain.SettlementStatusDisputed,
	}))
	projection := getProjection(t, env, fix)
	if projection.CurrentActualAmount != nil {
		t.Fatal("expected null after dispute nullification")
	}
}

func TestFC_B_ACT_006_ActualCannotReenableWithStalePreDisputeTotals(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1500.00"), settlementStatus: domain.SettlementStatusApproved,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 2,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
		openDisputeCount: 1, settlementStatus: domain.SettlementStatusDisputed,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 3,
		amount: decimalAmount("1500.00"), settlementStatus: domain.SettlementStatusApproved,
		openDisputeCount: 1,
	}))
	projection := getProjection(t, env, fix)
	if projection.CurrentActualAmount != nil {
		t.Fatal("stale pre-dispute total must not re-enable actual while dispute open")
	}
}
