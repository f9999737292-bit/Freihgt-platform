package domain

import "testing"

func TestFC_C_OUT_001_IdempotentRecomputeSameFingerprint(t *testing.T) {
	t.Parallel()
	projection := projectionWithAmounts("1000.00", "1100.00", "")
	proposed := ProposedAccessorialInput{SourceStatus: ProposedSourceUnknown}
	changed1, err := RecomputeDerivedProjection(projection, proposed)
	if err != nil {
		t.Fatal(err)
	}
	fp := projection.DerivedStateFingerprint
	changed2, err := RecomputeDerivedProjection(projection, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if !changed1 || changed2 {
		t.Fatalf("idempotent recompute: first=%v second=%v", changed1, changed2)
	}
	if fp == nil || projection.DerivedStateFingerprint == nil || *fp != *projection.DerivedStateFingerprint {
		t.Fatal("fingerprint must remain stable on idempotent recompute")
	}
}
