//go:build integration

package ledger

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC_B_LED_001_AppendOnlyDeniesUpdateAndDelete(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	amount := decimal.RequireFromString("1100.00")
	result := ingest(t, env, baseIngestInput(fix, ingestOpts{
		sourceRevision: 1,
		amount:         &amount,
	}))
	if result.Outcome != service.IngestOutcomeApplied {
		t.Fatalf("outcome = %s", result.Outcome)
	}
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `UPDATE freight_cost.cost_entry SET amount = 999 WHERE tenant_id = $1`, fix.TenantID); err == nil {
		t.Fatal("expected append-only UPDATE to fail")
	}
	if _, err := env.pool.Exec(ctx, `DELETE FROM freight_cost.cost_entry WHERE tenant_id = $1`, fix.TenantID); err == nil {
		t.Fatal("expected append-only DELETE to fail")
	}
}

func TestFC_B_LED_002_SourceEventIdUniqueness(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	amount := decimal.RequireFromString("1100.00")
	first := ingest(t, env, baseIngestInput(fix, ingestOpts{eventID: eventID, sourceRevision: 1, amount: &amount}))
	if first.Outcome != service.IngestOutcomeApplied {
		t.Fatalf("first outcome = %s", first.Outcome)
	}
	second := ingest(t, env, baseIngestInput(fix, ingestOpts{eventID: eventID, sourceRevision: 1, amount: &amount}))
	if second.Outcome != service.IngestOutcomeNoOpEvent {
		t.Fatalf("replay outcome = %s", second.Outcome)
	}
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 1 {
		t.Fatal("expected exactly one cost_entry row")
	}
}

func TestFC_B_LED_003_DualIdentityRevisionTrackedInCursor(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	amount := decimal.RequireFromString("1100.00")
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: &amount,
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 2, amount: decimalAmount("1200.00"),
	}))
	var revision int64
	err := env.pool.QueryRow(context.Background(), `
		SELECT last_source_revision FROM freight_cost.source_cursor
		WHERE tenant_id = $1 AND transport_order_id = $2 AND entry_kind = $3`,
		fix.TenantID, fix.OrderID, domain.EntryKindAccrualCostSnapshot).Scan(&revision)
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if revision != 2 {
		t.Fatalf("cursor revision = %d", revision)
	}
}

func TestFC_B_LED_004_OutOfOrderDoesNotRegressProjection(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 5, amount: decimalAmount("1500.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 3, amount: decimalAmount("900.00"),
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(decimal.RequireFromString("1500.00")) {
		t.Fatalf("projection regressed: %v", projection.AccruedAmount)
	}
}

func TestFC_B_LED_005_HigherRevisionUpdatesProjection(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 2, amount: decimalAmount("1100.00"),
	}))
	projection := getProjection(t, env, fix)
	if projection.AccruedAmount == nil || !projection.AccruedAmount.Equal(decimal.RequireFromString("1100.00")) {
		t.Fatalf("projection = %v", projection.AccruedAmount)
	}
}

func TestFC_B_LED_006_DuplicateDeliverySamePayloadNoOp(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	opts := ingestOpts{eventID: eventID, sourceRevision: 1, amount: decimalAmount("1000.00")}
	first := ingest(t, env, baseIngestInput(fix, opts))
	second := ingest(t, env, baseIngestInput(fix, opts))
	if first.Outcome != service.IngestOutcomeApplied || second.Outcome != service.IngestOutcomeNoOpEvent {
		t.Fatalf("outcomes = %s / %s", first.Outcome, second.Outcome)
	}
}

func TestFC_B_LED_007_ConflictingReplayReturnsIntegrityError(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	ingest(t, env, baseIngestInput(fix, ingestOpts{eventID: eventID, sourceRevision: 1, amount: decimalAmount("1000.00")}))
	_, err := env.ingest.Ingest(context.Background(), baseIngestInput(fix, ingestOpts{
		eventID: eventID, sourceRevision: 1, amount: decimalAmount("1000.01"),
	}))
	if err == nil {
		t.Fatal("expected integrity error for conflicting replay")
	}
}

func TestFC_B_LED_008_NullAvailabilityRepresented(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 1,
		amountAvailability: domain.AmountAvailabilityUnavailable, amount: nil,
		settlementStatus: domain.SettlementStatusDraft,
	}))
	projection := getProjection(t, env, fix)
	if projection.CurrentActualAmount != nil {
		t.Fatalf("expected NULL projection, got %v", projection.CurrentActualAmount)
	}
}

func TestFC_B_LED_009_LiveThenRebuildSameFactOneRow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{sourceRevision: 7, amount: decimalAmount("1300.00"), eventOrigin: domain.EventOriginLiveOutbox}
	factID := deriveFactID(fix, opts)
	liveEvent := uuid.New()
	ingest(t, env, baseIngestInput(fix, opts.withEvent(liveEvent)))
	rebuildEvent := rebuildDeliveryID(fix.TenantID, factID)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(rebuildEvent).withOrigin(domain.EventOriginCanonicalRebuild)))
	if countCostEntriesByFact(t, env.pool, fix.TenantID, factID) != 1 {
		t.Fatal("expected one row for same canonical fact")
	}
}

func TestFC_B_LED_010_RebuildThenLiveSameFactOneRow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{sourceRevision: 8, amount: decimalAmount("1400.00")}
	factID := deriveFactID(fix, opts)
	rebuildEvent := rebuildDeliveryID(fix.TenantID, factID)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(rebuildEvent).withOrigin(domain.EventOriginCanonicalRebuild)))
	liveEvent := uuid.New()
	ingest(t, env, baseIngestInput(fix, opts.withEvent(liveEvent).withOrigin(domain.EventOriginLiveOutbox)))
	if countCostEntriesByFact(t, env.pool, fix.TenantID, factID) != 1 {
		t.Fatal("expected one row for same canonical fact")
	}
}

func TestFC_B_LED_011_DifferentEventSameFactNoDuplicate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{sourceRevision: 9, amount: decimalAmount("1500.00")}
	factID := deriveFactID(fix, opts)
	first := ingest(t, env, baseIngestInput(fix, opts.withEvent(uuid.New())))
	second := ingest(t, env, baseIngestInput(fix, opts.withEvent(uuid.New())))
	if first.Outcome != service.IngestOutcomeApplied || second.Outcome != service.IngestOutcomeNoOpFact {
		t.Fatalf("outcomes = %s / %s", first.Outcome, second.Outcome)
	}
	if countCostEntriesByFact(t, env.pool, fix.TenantID, factID) != 1 {
		t.Fatal("expected one row")
	}
}

func TestFC_B_LED_012_EqualRevisionSameEntryKindSemanticDuplicate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	first := ingest(t, env, baseIngestInput(fix, ingestOpts{sourceRevision: 4, amount: decimalAmount("1600.00")}))
	second := ingest(t, env, baseIngestInput(fix, ingestOpts{sourceRevision: 4, amount: decimalAmount("1600.00")}))
	if first.Outcome != service.IngestOutcomeApplied {
		t.Fatalf("first = %s", first.Outcome)
	}
	if second.Outcome != service.IngestOutcomeNoOpFact {
		t.Fatalf("second = %s", second.Outcome)
	}
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 1 {
		t.Fatal("expected one ledger row for semantic duplicate")
	}
}

func TestFC_B_LED_013_EqualRevisionDifferentEntryKindAllowed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 6, amount: decimalAmount("1700.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindCurrentActualCostSnapshot, sourceRevision: 6, amount: decimalAmount("1700.00"),
	}))
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 2 {
		t.Fatal("expected distinct facts for different entry kinds")
	}
}

func (o ingestOpts) withEvent(id uuid.UUID) ingestOpts {
	o.eventID = id
	return o
}

func (o ingestOpts) withOrigin(origin string) ingestOpts {
	o.eventOrigin = origin
	return o
}
