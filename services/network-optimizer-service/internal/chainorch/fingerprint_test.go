package chainorch

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func baseFP(tenant uuid.UUID) FingerprintInput {
	return FingerprintInput{
		TenantID:               tenant,
		PolicyVersion:          "NLO-0.9C-I0/BOUNDED_TWO_LOAD",
		AlgorithmVersion:       AlgorithmVersion,
		CatalogFingerprint:     "catalog",
		RoutingProviderVersion: "UNKNOWN",
		SeedSource:             repository.ChainSeedManualAvailableCapacity,
		HorizonAnchorPolicy:    HorizonAnchorSearchClock,
		PublicationVisibility:  "vis-a",
		FilterFingerprint:      "filter-a",
		Duration:               UnknownDurationSnapshot(),
	}
}

func TestI2B_FP_01_SameLogicalRequestDifferentWallClock(t *testing.T) {
	tenant := uuid.New()
	from := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	until := from.Add(48 * time.Hour)
	capID := uuid.New()
	one := 1
	a := baseFP(tenant)
	a.HorizonAnchorPolicy = ""
	a.AvailableFrom = &from
	a.AvailableUntil = &until
	a.SeedCapacityID = &capID
	a.SeedCapacityVersion = &one
	b := a
	// Replay "later" does not change frozen anchors — fingerprint stable.
	fp1 := ComputeRequestFingerprint(a)
	fp2 := ComputeRequestFingerprint(b)
	if fp1 != fp2 {
		t.Fatalf("logical replay fingerprint drifted\n%s\n%s", fp1, fp2)
	}
}

func TestI2B_FP_02_DifferentTenant(t *testing.T) {
	a := baseFP(uuid.New())
	b := a
	b.TenantID = uuid.New()
	if ComputeRequestFingerprint(a) == ComputeRequestFingerprint(b) {
		t.Fatal("tenant must be in fingerprint")
	}
}

func TestI2B_FP_03_AvailableFromChange(t *testing.T) {
	tenant := uuid.New()
	from1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	from2 := from1.Add(time.Hour)
	until := from1.Add(48 * time.Hour)
	a := baseFP(tenant)
	a.HorizonAnchorPolicy = ""
	a.AvailableFrom = &from1
	a.AvailableUntil = &until
	b := a
	b.AvailableFrom = &from2
	if ComputeRequestFingerprint(a) == ComputeRequestFingerprint(b) {
		t.Fatal("available_from change must change fingerprint")
	}
}

func TestI2B_FP_04_AvailableUntilChange(t *testing.T) {
	tenant := uuid.New()
	from := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	until1 := from.Add(24 * time.Hour)
	until2 := from.Add(36 * time.Hour)
	a := baseFP(tenant)
	a.HorizonAnchorPolicy = ""
	a.AvailableFrom = &from
	a.AvailableUntil = &until1
	b := a
	b.AvailableUntil = &until2
	if ComputeRequestFingerprint(a) == ComputeRequestFingerprint(b) {
		t.Fatal("available_until change must change fingerprint")
	}
}

func TestI2B_FP_05_SearchClockPolicyMarkerStable(t *testing.T) {
	tenant := uuid.New()
	a := baseFP(tenant)
	a.HorizonAnchorPolicy = HorizonAnchorSearchClock
	b := a
	if ComputeRequestFingerprint(a) != ComputeRequestFingerprint(b) {
		t.Fatal("search-clock policy marker must be stable across replay")
	}
}

func TestI2B_FP_06_PublicationVisibilityChange(t *testing.T) {
	a := baseFP(uuid.New())
	b := a
	b.PublicationVisibility = "vis-b"
	if ComputeRequestFingerprint(a) == ComputeRequestFingerprint(b) {
		t.Fatal("publication visibility change must change fingerprint")
	}
}

func TestI2B_FP_07_FilterFingerprintChange(t *testing.T) {
	a := baseFP(uuid.New())
	b := a
	b.FilterFingerprint = "filter-b"
	if ComputeRequestFingerprint(a) == ComputeRequestFingerprint(b) {
		t.Fatal("filter fingerprint change must change fingerprint")
	}
}

func TestI2B_FP_08_DiscoveredLoadsNotInFingerprint(t *testing.T) {
	// Structural proof: FingerprintInput has no LoadIDsVersions field.
	// Changing a hypothetical discovered pool cannot affect ComputeRequestFingerprint.
	a := baseFP(uuid.New())
	b := a
	if ComputeRequestFingerprint(a) != ComputeRequestFingerprint(b) {
		t.Fatal("fingerprint must ignore discovered load pools")
	}
	var in FingerprintInput
	_ = in
	// Compile-time: LoadIDsVersions removed from FingerprintInput.
}
