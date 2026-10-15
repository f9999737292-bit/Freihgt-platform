package chainorch

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

// SnapshotFromPolicy builds an immutable KNOWN duration snapshot from a policy row.
func SnapshotFromPolicy(policy repository.ServiceDurationPolicy) repository.ServiceDurationSnapshot {
	pickup := policy.PickupSeconds
	delivery := policy.DeliverySeconds
	id := policy.ID
	version := policy.Version
	return repository.ServiceDurationSnapshot{
		State:                 repository.ServiceDurationKnown,
		PolicyID:              &id,
		PolicyVersion:         &version,
		PickupKnown:           true,
		PickupSeconds:         &pickup,
		DeliveryKnown:         true,
		DeliverySeconds:       &delivery,
		FingerprintProvenance: "ACTIVE_POLICY",
	}
}

// UnknownDurationSnapshot is used when no active policy is available.
func UnknownDurationSnapshot() repository.ServiceDurationSnapshot {
	return repository.ServiceDurationSnapshot{
		State:                 repository.ServiceDurationUnknown,
		PickupKnown:           false,
		DeliveryKnown:         false,
		FingerprintProvenance: "NO_ACTIVE_POLICY",
	}
}

// KnownZeroDurationSnapshot is an explicit known-zero pair (distinct from UNKNOWN).
func KnownZeroDurationSnapshot(policyID uuid.UUID, policyVersion int) repository.ServiceDurationSnapshot {
	zero := 0
	id := policyID
	ver := policyVersion
	return repository.ServiceDurationSnapshot{
		State:                 repository.ServiceDurationKnown,
		PolicyID:              &id,
		PolicyVersion:         &ver,
		PickupKnown:           true,
		PickupSeconds:         &zero,
		DeliveryKnown:         true,
		DeliverySeconds:       &zero,
		FingerprintProvenance: "KNOWN_ZERO",
	}
}

// ResolveActiveDurationSnapshot loads the tenant active policy, or UNKNOWN when absent.
func ResolveActiveDurationSnapshot(ctx context.Context, store *repository.Postgres, tenant uuid.UUID) (repository.ServiceDurationSnapshot, error) {
	var snap repository.ServiceDurationSnapshot
	err := store.Within(ctx, func(tx repository.Tx) error {
		policy, err := tx.ActiveServiceDurationPolicy(ctx, tenant)
		if errors.Is(err, repository.ErrNotFound) {
			snap = UnknownDurationSnapshot()
			return nil
		}
		if err != nil {
			return err
		}
		snap = SnapshotFromPolicy(policy)
		return nil
	})
	return snap, err
}

// ActionServiceSecondsFromSnapshot maps stored pickup/delivery onto route-plan action map.
// Asymmetric pickup/delivery is supported. UNKNOWN yields nil map (unknown duration).
func ActionServiceSecondsFromSnapshot(snap repository.ServiceDurationSnapshot) (map[string]int, error) {
	if snap.State == repository.ServiceDurationUnknown {
		if snap.PickupKnown || snap.DeliveryKnown {
			return nil, ErrDurationSnapshot
		}
		return nil, nil
	}
	if snap.State != repository.ServiceDurationKnown {
		return nil, ErrDurationSnapshot
	}
	if !snap.PickupKnown || !snap.DeliveryKnown || snap.PickupSeconds == nil || snap.DeliverySeconds == nil {
		return nil, ErrDurationSnapshot
	}
	return map[string]int{
		routeplan.ActionPickup:   *snap.PickupSeconds,
		routeplan.ActionDelivery: *snap.DeliverySeconds,
	}, nil
}

// ApplyStoredDurationToChainRequest wires the claim-time snapshot into chain execution.
// Callers must not re-read an active policy mid-run.
func ApplyStoredDurationToChainRequest(req *chain.Request, snap repository.ServiceDurationSnapshot) error {
	if req == nil {
		return ErrDurationSnapshot
	}
	actions, err := ActionServiceSecondsFromSnapshot(snap)
	if err != nil {
		return err
	}
	req.ActionServiceSeconds = actions
	req.ServiceDurationPolicyID = snap.PolicyID
	req.ServiceDurationPolicyVersion = snap.PolicyVersion
	// Legacy scalar remains unset when action map is present (including asymmetric).
	req.ServiceDurationSeconds = nil
	if len(actions) == 0 {
		// UNKNOWN: both scalar and action map empty — planner treats as unknown.
		return nil
	}
	return nil
}

// SnapshotsEquivalent reports whether two snapshots encode the same duration provenance.
func SnapshotsEquivalent(a, b repository.ServiceDurationSnapshot) bool {
	return ComputeRequestFingerprint(FingerprintInput{Duration: a}) ==
		ComputeRequestFingerprint(FingerprintInput{Duration: b})
}
