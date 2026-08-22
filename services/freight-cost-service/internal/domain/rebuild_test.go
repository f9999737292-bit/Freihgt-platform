package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFC_C_RBL_001_AttributionIdempotentSameState(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	proposed := ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}
	changed1, err := RecomputeDerivedProjection(projection, proposed)
	if err != nil {
		t.Fatal(err)
	}
	rev := projection.ProjectionRevision
	changed2, err := RecomputeDerivedProjection(projection, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if changed1 && changed2 {
		t.Fatal("second recompute with same inputs must be idempotent")
	}
	if projection.ProjectionRevision != rev {
		t.Fatalf("revision bumped on idempotent recompute: %d -> %d", rev, projection.ProjectionRevision)
	}
}

func TestFC_C_RBL_003_MappingChangeDoesNotAlterVarianceAmount(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1150.00", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	before := projection.CurrentVarianceAmount
	_ = ResolveChargeCategory("FUEL", []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "FUEL",
		NormalizedCategory:         "FUEL",
	}}, []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "FUEL",
		NormalizedCategory:         "OTHER",
	}})
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	if before == nil || projection.CurrentVarianceAmount == nil || !before.Equal(*projection.CurrentVarianceAmount) {
		t.Fatalf("mapping change must not alter variance: before=%v after=%v", before, projection.CurrentVarianceAmount)
	}
}

func TestFC_C_RBL_004_FinancialRebuildMappingIndependentDomain(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1200.00", "")
	if _, err := RecomputeDerivedProjection(projection, ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}); err != nil {
		t.Fatal(err)
	}
	variance := projection.CurrentVarianceAmount
	if variance == nil || !variance.Equal(decimal.RequireFromString("200.00")) {
		t.Fatalf("variance = %v", variance)
	}
	_ = BuildVarianceDrivers(projection, VarianceKindCurrent, DriverAttributionContext{
		MappingVersion: 99,
		PlatformMappings: []ChargeCodeMapping{{
			SourceChargeCodeNormalized: "X",
			NormalizedCategory:         "OTHER",
			MappingVersion:             99,
		}},
	})
	if projection.CurrentVarianceAmount == nil || !projection.CurrentVarianceAmount.Equal(*variance) {
		t.Fatal("driver mapping context must not alter financial variance")
	}
}
