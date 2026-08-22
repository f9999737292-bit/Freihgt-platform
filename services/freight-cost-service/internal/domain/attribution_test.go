package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func testProjection(t *testing.T) *CostSummaryProjection {
	t.Helper()
	tenantID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	orderID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	p := projectionWithAmounts("1000.00", "1100.00", "1100.00")
	p.TenantID = tenantID
	p.TransportOrderID = orderID
	p.ProjectionRevision = 3
	fp := ComputeDerivedStateFingerprint(p, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown})
	p.DerivedStateFingerprint = &fp
	if _, err := RecomputeDerivedProjection(p, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	return p
}

func TestFC_C_REA_001_DriverRequiresNonNullVariance(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	if len(drivers) != 0 {
		t.Fatalf("expected no drivers when variance NULL, got %d", len(drivers))
	}
}

func TestFC_C_REA_002_AvailabilityReasonWhenVarianceNull(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	reasons := BuildAvailabilityReasons(projection, VarianceKindCurrent)
	if len(reasons) == 0 {
		t.Fatal("expected availability reason when variance NULL")
	}
	if reasons[0].SemanticClass != SemanticClassVarianceAvailabilityReason {
		t.Fatalf("semantic class = %q", reasons[0].SemanticClass)
	}
}

func TestFC_C_REA_003_DriverAndAvailabilityMutuallyExclusive(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	reasons := BuildAvailabilityReasons(projection, VarianceKindCurrent)
	if len(drivers) == 0 {
		t.Fatal("expected drivers for non-null variance")
	}
	if len(reasons) != 0 {
		t.Fatal("availability reasons must not appear when variance is non-null")
	}
}

func TestFC_C_REA_004_MissingPlannedAvailabilityReason(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("", "1100.00", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	reasons := BuildAvailabilityReasons(projection, VarianceKindCurrent)
	found := false
	for _, r := range reasons {
		if r.ReasonCode == ReasonMissingPlanned {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MISSING_PLANNED reason, got %+v", reasons)
	}
}

func TestFC_C_REA_005_MissingActualAvailabilityReason(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	reasons := BuildAvailabilityReasons(projection, VarianceKindCurrent)
	found := false
	for _, r := range reasons {
		if r.ReasonCode == ReasonMissingActual {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MISSING_ACTUAL reason, got %+v", reasons)
	}
}

func TestFC_C_REA_006_ApprovedAccessorialCreatesDriver(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.New(),
			ChargeCode:    "LUMPER",
			Amount:        decimal.RequireFromString("100.00"),
		}},
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "LUMPER",
			NormalizedCategory:         "OTHER",
		}},
	})
	if len(drivers) == 0 || drivers[0].ReasonCode != ReasonAccessorial {
		t.Fatalf("expected ACCESSORIAL driver, got %+v", drivers)
	}
}

func TestFC_C_REA_007_DetentionAccessorialMappedToDriver(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.New(),
			ChargeCode:    "DETENTION",
			Amount:        decimal.RequireFromString("50.00"),
		}},
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "DETENTION",
			NormalizedCategory:         "DETENTION",
		}},
	})
	if len(drivers) == 0 || drivers[0].ReasonCode != ReasonDetention {
		t.Fatalf("expected DETENTION driver, got %+v", drivers)
	}
}

func TestFC_C_REA_008_WaitingAccessorialMappedToDriver(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.New(),
			ChargeCode:    "WAITING",
			Amount:        decimal.RequireFromString("25.00"),
		}},
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "WAITING",
			NormalizedCategory:         "WAITING",
		}},
	})
	if len(drivers) == 0 || drivers[0].ReasonCode != ReasonWaiting {
		t.Fatalf("expected WAITING driver, got %+v", drivers)
	}
}

func TestFC_C_REA_009_SnapshotFuelAloneNotFuelDriver(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	for _, d := range drivers {
		if d.ReasonCode == ReasonFuel {
			t.Fatal("snapshot FUEL component alone must not create FUEL driver")
		}
	}
}

func TestFC_C_REA_010_ApprovedFuelAccessorialCreatesFuelDriver(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.New(),
			ChargeCode:    "FUEL_SURCHARGE",
			Amount:        decimal.RequireFromString("80.00"),
		}},
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "FUEL_SURCHARGE",
			NormalizedCategory:         "FUEL",
		}},
	})
	found := false
	for _, d := range drivers {
		if d.ReasonCode == ReasonFuel {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected FUEL driver from approved accessorial, got %+v", drivers)
	}
}

func TestFC_C_REA_011_OpenDisputeAvailabilityNotDriver(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "", "")
	projection.TenantID = uuid.New()
	projection.TransportOrderID = uuid.New()
	projection.OpenDisputeCount = 2
	reasons := BuildAvailabilityReasons(projection, VarianceKindCurrent)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	found := false
	for _, r := range reasons {
		if r.ReasonCode == ReasonOpenDispute {
			found = true
		}
	}
	if !found {
		t.Fatal("OPEN_DISPUTE must be availability reason")
	}
	if len(drivers) != 0 {
		t.Fatal("OPEN_DISPUTE must not produce variance driver")
	}
}

func TestFC_C_REA_012_BillingLinkMismatchFindingNotDriver(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	projection.BillingReconciliationStatus = BillingReconciliationMismatch
	findings := DetectReconciliationFindings(projection)
	foundFinding := false
	for _, f := range findings {
		if f.FindingKind == FindingBillingLinkMismatch {
			foundFinding = true
		}
	}
	if !foundFinding {
		t.Fatal("expected BILLING_LINK_MISMATCH finding")
	}
	for _, d := range BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{}) {
		if d.ReasonCode == FindingBillingLinkMismatch {
			t.Fatal("billing link mismatch must not be variance driver")
		}
	}
}

func TestFC_C_REA_013_DuplicateRecomputeSameAttributionFactID(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	ctx := DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			ChargeCode:    "DETENTION",
			Amount:        decimal.RequireFromString("50.00"),
		}},
		MappingVersion: 1,
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "DETENTION",
			NormalizedCategory:         "DETENTION",
		}},
	}
	first := BuildVarianceDrivers(projection, VarianceKindCurrent, ctx)
	projection.ProjectionRevision = 99
	second := BuildVarianceDrivers(projection, VarianceKindCurrent, ctx)
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("expected driver rows")
	}
	if first[0].AttributionFactID != second[0].AttributionFactID {
		t.Fatalf("attribution fact id must be stable across projection revision: %s vs %s", first[0].AttributionFactID, second[0].AttributionFactID)
	}
}

func TestFC_C_REA_014_LegacyPricingDriverWhenPrincipalDiffers(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	base := decimal.RequireFromString("1200.00")
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		BaseFreightAmount: &base,
	})
	found := false
	for _, d := range drivers {
		if d.ReasonCode == ReasonLegacyPricing {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected LEGACY_PRICING driver, got %+v", drivers)
	}
}

func TestFC_C_REA_015_UnattributedFallbackWhenNoMatch(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	if len(drivers) != 1 || drivers[0].ReasonCode != ReasonUnattributed {
		t.Fatalf("expected UNATTRIBUTED fallback, got %+v", drivers)
	}
}

func TestFC_C_REA_016_AttributionFactIDUsesStateFingerprint(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	orderID := uuid.New()
	fp := "state-fp-abc"
	input := AttributionInput{
		TenantID:            tenantID,
		TransportOrderID:    orderID,
		VarianceKind:        VarianceKindCurrent,
		SemanticClass:       SemanticClassVarianceDriver,
		ReasonCode:          ReasonUnattributed,
		EvidenceFingerprint: EvidenceFingerprint("unattributed"),
		MappingVersion:      1,
		StateFingerprint:    fp,
	}
	id1 := DeriveAttributionFactID(input)
	input.StateFingerprint = "different-fp"
	id2 := DeriveAttributionFactID(input)
	if id1 == id2 {
		t.Fatal("state fingerprint must affect attribution fact id")
	}
}

func TestFC_C_DUP_001_ClassificationDoesNotAlterAccrual(t *testing.T) {
	t.Parallel()
	planned := mustNewMoney(t, "1000.00", "RUB")
	before, err := CalculateAccrual(planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = ResolveChargeCategory("FUEL", []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "FUEL",
		NormalizedCategory:         "FUEL",
	}}, nil)
	after, err := CalculateAccrual(planned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !before.Amount.Equal(after.Amount) {
		t.Fatal("classification must not alter accrual")
	}
}

func TestFC_C_DUP_002_DriverDoesNotDoubleCountSnapshot(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.New(),
			ChargeCode:    "DETENTION",
			Amount:        decimal.RequireFromString("100.00"),
		}},
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "DETENTION",
			NormalizedCategory:         "DETENTION",
		}},
	})
	for _, d := range drivers {
		if amount, ok := d.EvidenceJSON["planned_amount"]; ok && amount == d.EvidenceJSON["amount"] {
			t.Fatal("driver must not add snapshot component on top of total")
		}
	}
}

func TestFC_C_DUP_003_FuelDoubleCountDenied(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	drivers := BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{})
	for _, d := range drivers {
		if d.ReasonCode == ReasonFuel {
			t.Fatal("FUEL driver requires approved accessorial evidence, not planned snapshot")
		}
	}
}

func TestFC_C_DUP_004_AttributionDoesNotChangeVariance(t *testing.T) {
	t.Parallel()
	projection := testProjection(t)
	before := projection.CurrentVarianceAmount
	_ = BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		ApprovedAccessorials: []ApprovedAccessorialEvidence{{
			AccessorialID: uuid.New(),
			ChargeCode:    "DETENTION",
			Amount:        decimal.RequireFromString("100.00"),
		}},
	})
	if projection.CurrentVarianceAmount == nil || before == nil || !projection.CurrentVarianceAmount.Equal(*before) {
		t.Fatal("attribution must not change variance amounts")
	}
}
