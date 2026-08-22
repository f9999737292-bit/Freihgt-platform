//go:build integration

package ledger

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC_B_OUT_001_VersionedFinancialSnapshotDistinctFacts(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlementID := settlementSourceID()
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceID: settlementID, sourceRevision: 1,
		amount: decimalAmount("1000.00"), eventType: "freight_settlement.accrual_snapshot.v1",
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceID: settlementID, sourceRevision: 1,
		amount: decimalAmount("1000.00"), eventType: "freight_settlement.current_actual_snapshot.v1",
	}))
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 2 {
		t.Fatal("expected distinct facts for versioned snapshot event types")
	}
}

func TestFC_B_OUT_002_IngestAtomicityPerEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	result := ingest(t, env, baseIngestInput(fix, ingestOpts{
		sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	if result.CostEntryID == nil {
		t.Fatal("expected cost entry id")
	}
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil {
		t.Fatal("projection must update atomically with journal insert")
	}
}

func TestFC_B_OUT_003_OneEventPerFact(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	opts := ingestOpts{eventID: eventID, sourceRevision: 1, amount: decimalAmount("1000.00")}
	ingest(t, env, baseIngestInput(fix, opts))
	second := ingest(t, env, baseIngestInput(fix, opts))
	if second.Outcome != service.IngestOutcomeNoOpEvent {
		t.Fatalf("duplicate delivery outcome = %s", second.Outcome)
	}
}

func TestFC_B_OUT_004_MultiEntryKindSameRevisionDistinctEvents(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	settlementID := settlementSourceID()
	revision := int64(20)
	kinds := []string{
		domain.EntryKindAccrualCostSnapshot,
		domain.EntryKindCurrentActualCostSnapshot,
		domain.EntryKindFinalActualCostSnapshot,
	}
	for _, kind := range kinds {
		ingest(t, env, baseIngestInput(fix, ingestOpts{
			entryKind: kind, sourceID: settlementID, sourceRevision: revision, amount: decimalAmount("1000.00"),
		}))
	}
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != len(kinds) {
		t.Fatalf("expected %d rows", len(kinds))
	}
}

func TestFC_B_OUT_005_LiveAndRebuildSameFactNoDuplicate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{sourceRevision: 21, amount: decimalAmount("2100.00")}
	factID := deriveFactID(fix, opts)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(uuid.New()).withOrigin(domain.EventOriginLiveOutbox)))
	ingest(t, env, baseIngestInput(fix, opts.withEvent(rebuildDeliveryID(fix.TenantID, factID)).withOrigin(domain.EventOriginCanonicalRebuild)))
	if countCostEntriesByFact(t, env.pool, fix.TenantID, factID) != 1 {
		t.Fatal("LIVE + REBUILD must not duplicate canonical fact")
	}
}
