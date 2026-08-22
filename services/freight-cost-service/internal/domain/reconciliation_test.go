package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestFC_A_DOM_008_BillingReconciliationMatch(t *testing.T) {
	t.Parallel()

	status := DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked:      true,
		SettlementTotalExVAT:  decimalPtr("100.00"),
		SettlementCurrency:    "RUB",
		SettlementStatus:      SettlementStatusApproved,
		OpenDisputeCount:      0,
		BilledLineAmountExVAT: decimalPtr("100.00"),
		BilledLineCurrency:    "RUB",
	})
	if status != BillingReconciliationMatch {
		t.Fatalf("status = %s", status)
	}
}

func TestFC_A_DOM_009_BillingReconciliationMismatch(t *testing.T) {
	t.Parallel()

	status := DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked:      true,
		SettlementTotalExVAT:  decimalPtr("100.00"),
		SettlementCurrency:    "RUB",
		SettlementStatus:      SettlementStatusDisputed,
		OpenDisputeCount:      1,
		BilledLineAmountExVAT: decimalPtr("100.00"),
		BilledLineCurrency:    "RUB",
	})
	if status != BillingReconciliationMismatch {
		t.Fatalf("status = %s", status)
	}
}

func TestFC_A_DOM_010_BillingReconciliationUnlinked(t *testing.T) {
	t.Parallel()

	status := DetermineBillingReconciliation(BillingReconciliationInput{
		SettlementLinked: false,
	})
	if status != BillingReconciliationUnlinked {
		t.Fatalf("status = %s", status)
	}
}

func TestFC_C_REC_001_FindingIDExcludesRevisions(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	id1 := DeriveFindingID(tenantID, orderID, FindingMissingPlannedFact, "planned")
	id2 := DeriveFindingID(tenantID, orderID, FindingMissingPlannedFact, "planned")
	if id1 != id2 {
		t.Fatalf("finding id must be stable: %s vs %s", id1, id2)
	}
	id3 := DeriveFindingID(tenantID, orderID, FindingBillingLinkMismatch, "billing_link")
	if id1 == id3 {
		t.Fatal("different kind/reference must yield different finding id")
	}
}

func TestFC_C_REC_002_MissingPlannedFactDetected(t *testing.T) {
	t.Parallel()
	projection := &CostSummaryProjection{
		TenantID:         uuid.New(),
		TransportOrderID: uuid.New(),
	}
	findings := DetectReconciliationFindings(projection)
	found := false
	for _, f := range findings {
		if f.FindingKind == FindingMissingPlannedFact {
			found = true
		}
	}
	if !found {
		t.Fatal("expected MISSING_PLANNED_FACT finding")
	}
}

func TestFC_C_REC_003_MissingAccrualFactWhenLinked(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	projection.SettlementLinked = true
	findings := DetectReconciliationFindings(projection)
	found := false
	for _, f := range findings {
		if f.FindingKind == FindingMissingAccrualFact {
			found = true
		}
	}
	if !found {
		t.Fatal("expected MISSING_ACCRUAL_FACT when linked without accrual")
	}
}

func TestFC_C_REC_004_MissingFinalActualWhenReadyForPayment(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	projection.SettlementStatus = SettlementStatusReadyForPayment
	findings := DetectReconciliationFindings(projection)
	found := false
	for _, f := range findings {
		if f.FindingKind == FindingMissingFinalActual {
			found = true
		}
	}
	if !found {
		t.Fatal("expected MISSING_FINAL_ACTUAL finding")
	}
}

func TestFC_C_REC_005_BillingLinkMismatchFinding(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	projection.BillingReconciliationStatus = BillingReconciliationMismatch
	findings := DetectReconciliationFindings(projection)
	found := false
	for _, f := range findings {
		if f.FindingKind == FindingBillingLinkMismatch {
			found = true
			if f.Status != FindingStatusOpen {
				t.Fatalf("status = %q", f.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected billing link mismatch finding")
	}
}

func TestFC_C_REC_006_OrphanBillingLinkFinding(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	projection.SettlementLinked = true
	projection.BillingReconciliationStatus = BillingReconciliationUnlinked
	findings := DetectReconciliationFindings(projection)
	found := false
	for _, f := range findings {
		if f.FindingKind == FindingOrphanBillingLink {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ORPHAN_BILLING_LINK finding")
	}
}
