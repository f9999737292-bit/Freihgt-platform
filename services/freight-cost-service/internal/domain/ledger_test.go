package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeriveSourceFactID_ExcludesEventOrigin(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sourceID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	live := DeriveSourceFactID(
		tenantID,
		SourceServiceBillingRegister,
		SourceTypeFreightSettlement,
		sourceID,
		"7",
		EntryKindCurrentActualCostSnapshot,
	)
	rebuild := DeriveSourceFactID(
		tenantID,
		SourceServiceBillingRegister,
		SourceTypeFreightSettlement,
		sourceID,
		"7",
		EntryKindCurrentActualCostSnapshot,
	)
	if live != rebuild {
		t.Fatalf("expected identical source_fact_id for same canonical fact, got live=%s rebuild=%s", live, rebuild)
	}

	deliveryLive := DeriveRebuildDeliveryID(tenantID, live)
	deliveryRebuild := DeriveRebuildDeliveryID(tenantID, rebuild)
	if deliveryLive != deliveryRebuild {
		t.Fatalf("expected identical rebuild delivery id for same fact")
	}
	if deliveryLive == live {
		t.Fatalf("delivery id must differ from fact id")
	}
}

func TestDeriveSourceFactID_PlannedImmutable(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	snapshotID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	factID := DeriveSourceFactID(
		tenantID,
		SourceServiceTransportOrder,
		SourceTypeTORateSnapshot,
		snapshotID,
		RevisionSemanticImmutable,
		EntryKindPlannedCostSnapshot,
	)
	if factID == uuid.Nil {
		t.Fatal("expected non-nil fact id")
	}
}

func TestSourceRevisionSemantic(t *testing.T) {
	if got := SourceRevisionSemantic(SourceTypeTORateSnapshot, 99); got != RevisionSemanticImmutable {
		t.Fatalf("expected IMMUTABLE, got %q", got)
	}
	if got := SourceRevisionSemantic(SourceTypeFreightSettlement, 12); got != "12" {
		t.Fatalf("expected 12, got %q", got)
	}
}
