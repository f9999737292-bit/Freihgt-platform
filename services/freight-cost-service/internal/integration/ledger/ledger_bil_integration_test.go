//go:build integration

package ledger

import (
	"testing"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func TestFC_B_BIL_001_BillingLinkLinkedCreatesBilledSnapshot(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlementID := settlementSourceID()
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementID, sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	projection := getProjection(t, env, fix)
	if !projection.SettlementLinked || projection.BillingRegisterAmount == nil {
		t.Fatal("expected linked billed snapshot")
	}
}

func TestFC_B_BIL_002_UnlinkedBillingLinkState(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementSourceID(), sourceRevision: 2,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
	}))
	projection := getProjection(t, env, fix)
	if projection.SettlementLinked {
		t.Fatal("expected UNLINKED billed state")
	}
}

func TestFC_B_BIL_003_BillingReconciliationMatch(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1000.00"), settlementStatus: domain.SettlementStatusApproved,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementSourceID(), sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	projection := getProjection(t, env, fix)
	if projection.BillingReconciliationStatus != domain.BillingReconciliationMatch {
		t.Fatalf("status = %s", projection.BillingReconciliationStatus)
	}
}

func TestFC_B_BIL_004_BillingReconciliationMismatchOnAmount(t *testing.T) {
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
		t.Fatalf("status = %s", projection.BillingReconciliationStatus)
	}
}

func TestFC_B_BIL_005_BillingReconciliationMismatchOnDispute(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amount: decimalAmount("1000.00"), settlementStatus: domain.SettlementStatusApproved,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementSourceID(), sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 2,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
		openDisputeCount: 1, settlementStatus: domain.SettlementStatusDisputed,
	}))
	projection := getProjection(t, env, fix)
	if projection.BillingReconciliationStatus != domain.BillingReconciliationMismatch {
		t.Fatalf("status = %s", projection.BillingReconciliationStatus)
	}
}

func TestFC_B_BIL_006_RegisterRemovalUnlinkedHistoryRetained(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlementID := settlementSourceID()
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementID, sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementID, sourceRevision: 2,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
	}))
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) < 2 {
		t.Fatal("historical billed entry must remain after UNLINKED")
	}
	projection := getProjection(t, env, fix)
	if projection.SettlementLinked {
		t.Fatal("expected UNLINKED projection")
	}
}

func TestFC_B_BIL_007_RelinkMonotonicBillingLinkRevision(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlementID := settlementSourceID()
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementID, sourceRevision: 3, amount: decimalAmount("1000.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementID, sourceRevision: 4, amount: decimalAmount("1000.00"),
	}))
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 2 {
		t.Fatal("relink should append new billed snapshot row")
	}
}

func TestFC_B_BIL_008_RebuildAfterDeletionReconstructsUnlinked(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlementID := settlementSourceID()
	opts := ingestOpts{
		entryKind: domain.EntryKindBilledCostSnapshot, sourceType: domain.SourceTypeFreightSettlementBillingLink,
		sourceID: settlementID, sourceRevision: 5,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
	}
	factID := deriveFactID(fix, opts)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(rebuildDeliveryID(fix.TenantID, factID)).withOrigin(domain.EventOriginCanonicalRebuild)))
	projection := getProjection(t, env, fix)
	if projection.SettlementLinked {
		t.Fatal("rebuild should reconstruct UNLINKED billing link")
	}
}
