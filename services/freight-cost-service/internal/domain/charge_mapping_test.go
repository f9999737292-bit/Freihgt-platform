package domain

import (
	"strings"
	"testing"
)

func TestFC_C_CHG_002_TenantOverrideBeatsPlatform(t *testing.T) {
	t.Parallel()
	platform := []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "FUEL",
		NormalizedCategory:         "OTHER",
	}}
	tenant := []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "FUEL",
		NormalizedCategory:         "FUEL",
	}}
	if got := ResolveChargeCategory("fuel", platform, tenant); got != "FUEL" {
		t.Fatalf("tenant override = %q", got)
	}
}

func TestFC_C_CHG_004_NormalizeChargeCodeUpperTrim(t *testing.T) {
	t.Parallel()
	got, err := NormalizeChargeCode("  detention  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "DETENTION" {
		t.Fatalf("normalized = %q", got)
	}
}

func TestFC_C_CHG_005_InvalidEmptyChargeCodeRejected(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeChargeCode("   "); err != ErrInvalidChargeCode {
		t.Fatalf("expected invalid charge code, got %v", err)
	}
}

func TestFC_C_CHG_006_ChargeCodeMaxLengthRejected(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("A", MaxChargeCodeLength+1)
	if _, err := NormalizeChargeCode(long); err != ErrInvalidChargeCode {
		t.Fatalf("expected length rejection, got %v", err)
	}
}

func TestFC_C_CHG_007_UnmappedCodeReturnsOther(t *testing.T) {
	t.Parallel()
	if got := ResolveChargeCategory("UNKNOWN_CODE", nil, nil); got != CategoryOther {
		t.Fatalf("unmapped = %q", got)
	}
}

func TestFC_C_CHG_008_ResolveCategoryTenantPrecedence(t *testing.T) {
	t.Parallel()
	platform := []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "LUMPER",
		NormalizedCategory:         "ACCESSORIAL",
	}}
	tenant := []ChargeCodeMapping{{
		SourceChargeCodeNormalized: "LUMPER",
		NormalizedCategory:         "DETENTION",
	}}
	if got := ResolveChargeCategory("LUMPER", platform, tenant); got != "DETENTION" {
		t.Fatalf("tenant precedence = %q", got)
	}
}
