package chainorch

import (
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
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
	actions, err := ActionServiceSecondsFromSnapshot(zero)
	if err != nil || actions == nil || actions[routeplan.ActionPickup] != 0 || actions[routeplan.ActionDelivery] != 0 {
		t.Fatalf("known zero actions=%v err=%v", actions, err)
	}
	actions, err = ActionServiceSecondsFromSnapshot(unknown)
	if err != nil || actions != nil {
		t.Fatalf("unknown must yield nil actions got=%v err=%v", actions, err)
	}
}

func TestI2B_14_DurationPolicyChangeChangesFingerprint(t *testing.T) {
	id := uuid.New()
	a := SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: id, Version: 1, PickupSeconds: 60, DeliverySeconds: 60})
	b := SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: id, Version: 2, PickupSeconds: 60, DeliverySeconds: 60})
	c := SnapshotFromPolicy(repository.ServiceDurationPolicy{ID: id, Version: 1, PickupSeconds: 90, DeliverySeconds: 90})
	base := FingerprintInput{
		TenantID: uuid.New(), PolicyVersion: "p", AlgorithmVersion: "a", CatalogFingerprint: "c",
		RoutingProviderVersion: "r", SeedSource: repository.ChainSeedManualAvailableCapacity,
		HorizonAnchorPolicy: HorizonAnchorSearchClock, Duration: a,
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

func TestI2B_DUR_AsymmetricPickupDelivery(t *testing.T) {
	snap := SnapshotFromPolicy(repository.ServiceDurationPolicy{
		ID: uuid.New(), Version: 1, PickupSeconds: 900, DeliverySeconds: 1200,
	})
	actions, err := ActionServiceSecondsFromSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if actions[routeplan.ActionPickup] != 900 || actions[routeplan.ActionDelivery] != 1200 {
		t.Fatalf("asymmetric actions=%v", actions)
	}
	var req chain.Request
	if err := ApplyStoredDurationToChainRequest(&req, snap); err != nil {
		t.Fatal(err)
	}
	if req.ServiceDurationSeconds != nil {
		t.Fatal("scalar must stay nil when action map is used")
	}
	if req.ActionServiceSeconds[routeplan.ActionPickup] != 900 {
		t.Fatalf("pickup=%v", req.ActionServiceSeconds)
	}
}
