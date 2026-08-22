//go:build integration

package variance

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC_C_OUT_002_DuplicateEventNoNewAttribution(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	opts := ingestOpts{
		entryKind:      domain.EntryKindCurrentActualCostSnapshot,
		sourceRevision: 1,
		amount:         decimalAmount("1100.00"),
	}
	first := ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))
	second := ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))
	if first.Outcome != service.IngestOutcomeApplied {
		t.Fatalf("first outcome = %s", first.Outcome)
	}
	if second.Outcome != service.IngestOutcomeNoOpEvent && second.Outcome != service.IngestOutcomeNoOpFact {
		t.Fatalf("duplicate event outcome = %s", second.Outcome)
	}
	count := countAttributions(t, env, fix)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))
	if after := countAttributions(t, env, fix); after != count {
		t.Fatalf("duplicate event must not add attribution rows: before=%d after=%d", count, after)
	}
}

func TestFC_C_OUT_003_DuplicateIngestNoProjectionBump(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	revision := projection.ProjectionRevision
	fingerprint := projection.DerivedStateFingerprint

	eventID := uuid.New()
	opts := ingestOpts{
		entryKind:      domain.EntryKindCurrentActualCostSnapshot,
		sourceRevision: 1,
		amount:         decimalAmount("1100.00"),
	}
	ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))
	ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))

	after := getProjection(t, env, fix)
	if after.ProjectionRevision != revision {
		t.Fatalf("duplicate ingest bumped revision: %d -> %d", revision, after.ProjectionRevision)
	}
	if fingerprint != nil && after.DerivedStateFingerprint != nil && *fingerprint != *after.DerivedStateFingerprint {
		t.Fatal("duplicate ingest must not change derived fingerprint")
	}
}

func TestFC_C_OUT_004_RetryAfterCrashNoDuplicateRows(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	opts := ingestOpts{
		entryKind:        domain.EntryKindPlannedCostSnapshot,
		sourceType:       domain.SourceTypeTORateSnapshot,
		sourceService:    domain.SourceServiceTransportOrder,
		sourceID:         fix.SnapshotID,
		sourceRevision:   1,
		revisionSemantic: domain.RevisionSemanticImmutable,
		amount:           decimalAmount(fix.PlannedAmount.StringFixed(domain.MoneyScale)),
	}
	eventID := uuid.New()
	ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))
	before := countCostEntries(t, env, fix)
	ingest(t, env, baseIngestInput(fix, opts.withEvent(eventID)))
	if after := countCostEntries(t, env, fix); after != before {
		t.Fatalf("retry must not duplicate cost_entry rows: before=%d after=%d", before, after)
	}
	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if final := countCostEntries(t, env, fix); final != before {
		t.Fatalf("rebuild retry must remain idempotent: before=%d final=%d", before, final)
	}
}
