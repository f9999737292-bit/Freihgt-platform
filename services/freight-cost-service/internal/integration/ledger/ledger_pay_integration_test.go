//go:build integration

package ledger

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func TestFC_B_PAY_001_SameObligationRevisionOnePaidSnapshotEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	obligationID := uuid.New()
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindPaidAmountSnapshot, sourceType: domain.SourceTypePaymentObligation,
		sourceService: domain.SourceServicePayment, sourceID: obligationID, sourceRevision: 2,
		taxBasis: domain.TaxBasisWithVAT, amount: decimalAmount("500.00"),
	}))
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 1 {
		t.Fatal("expected one paid snapshot row at revision 2")
	}
}

func TestFC_B_PAY_002_ReplaySameRevisionNoDuplicate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	obligationID := uuid.New()
	eventID := uuid.New()
	opts := ingestOpts{
		entryKind: domain.EntryKindPaidAmountSnapshot, sourceType: domain.SourceTypePaymentObligation,
		sourceService: domain.SourceServicePayment, sourceID: obligationID, sourceRevision: 2,
		taxBasis: domain.TaxBasisWithVAT, amount: decimalAmount("500.00"), eventID: eventID,
	}
	first := ingest(t, env, baseIngestInput(fix, opts))
	second := ingest(t, env, baseIngestInput(fix, opts))
	if first.Outcome != service.IngestOutcomeApplied || second.Outcome != service.IngestOutcomeNoOpEvent {
		t.Fatalf("outcomes = %s / %s", first.Outcome, second.Outcome)
	}
}

func TestFC_B_PAY_003_RevisionThreeCreatesSecondPaidSnapshotRow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	obligationID := uuid.New()
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindPaidAmountSnapshot, sourceType: domain.SourceTypePaymentObligation,
		sourceService: domain.SourceServicePayment, sourceID: obligationID, sourceRevision: 2,
		taxBasis: domain.TaxBasisWithVAT, amount: decimalAmount("500.00"),
	}))
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindPaidAmountSnapshot, sourceType: domain.SourceTypePaymentObligation,
		sourceService: domain.SourceServicePayment, sourceID: obligationID, sourceRevision: 3,
		taxBasis: domain.TaxBasisWithVAT, amount: decimalAmount("1000.00"),
	}))
	if countCostEntries(t, env.pool, fix.TenantID, fix.OrderID) != 2 {
		t.Fatal("expected distinct paid snapshot rows per obligation revision")
	}
	projection := getProjection(t, env, fix)
	if projection.PaidAmount == nil || !projection.PaidAmount.Equal(*decimalAmount("1000.00")) {
		t.Fatalf("paid projection = %v", projection.PaidAmount)
	}
}
