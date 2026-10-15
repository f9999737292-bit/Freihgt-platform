package chainorch

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestI2B_15_KnownZeroDistinctFromUnknown(t *testing.T) {
	zero := KnownZeroDurationSnapshot(uuid.New(), 1)
	unknown := UnknownDurationSnapshot()
	if SnapshotsEquivalent(zero, unknown) {
		t.Fatal("KNOWN_ZERO must not equal UNKNOWN fingerprint material")
	}
	fpZero := ComputeRequestFingerprint(FingerprintInput{Duration: zero})
	fpUnknown := ComputeRequestFingerprint(FingerprintInput{Duration: unknown})
	if fpZero == fpUnknown {
		t.Fatal("request fingerprints must differ for KNOWN_ZERO vs UNKNOWN")
	}
	sec, err := EffectiveServiceDurationSeconds(zero)
	if err != nil || sec == nil || *sec != 0 {
		t.Fatalf("known zero seconds=%v err=%v", sec, err)
	}
	sec, err = EffectiveServiceDurationSeconds(unknown)
	if err != nil || sec != nil {
		t.Fatalf("unknown must yield nil seconds got=%v err=%v", sec, err)
	}
}

func TestI2B_14_DurationPolicyChangeChangesFingerprint(t *testing.T) {
	id := uuid.New()
	a := SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: id, Version: 1, PickupSeconds: 60, DeliverySeconds: 60})
	b := SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: id, Version: 2, PickupSeconds: 60, DeliverySeconds: 60})
	c := SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: id, Version: 1, PickupSeconds: 90, DeliverySeconds: 90})
	base := FingerprintInput{
		PolicyVersion: "p", AlgorithmVersion: "a", CatalogFingerprint: "c",
		RoutingProviderVersion: "r", SeedSource: repository.ChainSeedManualAvailableCapacity,
		Duration: a,
	}
	fpA := ComputeRequestFingerprint(base)
	base.Duration = b
	fpB := ComputeRequestFingerprint(base)
	base.Duration = c
	fpC := ComputeRequestFingerprint(base)
	if fpA == fpB || fpA == fpC {
		t.Fatalf("duration provenance must change fingerprint a=%s b=%s c=%s", fpA, fpB, fpC)
	}
}

func TestApplyStoredDurationToChainRequest(t *testing.T) {
	var dest *int
	snap := SnapshotFromPolicy(repository.ServiceDurationPolicy{
		ID: uuid.New(), Version: 1, PickupSeconds: 120, DeliverySeconds: 120,
	})
	if err := ApplyStoredDurationToChainRequest(&dest, snap); err != nil || dest == nil || *dest != 120 {
		t.Fatalf("apply known dest=%v err=%v", dest, err)
	}
	asym := snap
	d := 200
	asym.DeliverySeconds = &d
	if err := ApplyStoredDurationToChainRequest(&dest, asym); err == nil {
		t.Fatal("asymmetric snapshot must fail closed")
	}
}
