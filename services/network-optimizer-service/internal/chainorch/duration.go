package chainorch

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
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

// EffectiveServiceDurationSeconds maps a stored snapshot onto the single
// chain/route-plan ServiceDurationSeconds input.
//
// UNKNOWN -> nil (no invented duration).
// KNOWN with both legs present and equal -> that value (includes known-zero).
// Asymmetric or incomplete KNOWN snapshots fail closed — route-plan cannot
// safely consume mismatched pickup/delivery through one scalar field.
func EffectiveServiceDurationSeconds(snap repository.ServiceDurationSnapshot) (*int, error) {
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
	if *snap.PickupSeconds != *snap.DeliverySeconds {
		return nil, ErrDurationSnapshot
	}
	v := *snap.PickupSeconds
	return &v, nil
}

// ApplyStoredDurationToChainRequest sets chain.Request.ServiceDurationSeconds from
// the claim-time snapshot. Callers must not re-read an active policy mid-run.
func ApplyStoredDurationToChainRequest(serviceDurationSeconds **int, snap repository.ServiceDurationSnapshot) error {
	v, err := EffectiveServiceDurationSeconds(snap)
	if err != nil {
		return err
	}
	*serviceDurationSeconds = v
	return nil
}

// SnapshotsEquivalent reports whether two snapshots encode the same duration provenance.
func SnapshotsEquivalent(a, b repository.ServiceDurationSnapshot) bool {
	return ComputeRequestFingerprint(FingerprintInput{Duration: a}) ==
		ComputeRequestFingerprint(FingerprintInput{Duration: b})
}
