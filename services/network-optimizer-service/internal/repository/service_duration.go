package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	DurationPolicyDraft    = "DRAFT"
	DurationPolicyActive   = "ACTIVE"
	DurationPolicyRetired  = "RETIRED"
	DurationActionPickup   = "PICKUP"
	DurationActionDelivery = "DELIVERY"
)

// ServiceDurationPolicy is the operating tenant's server-owned dwell policy.
// Network-optimizer-service owns the row. A client request cannot supply it.
type ServiceDurationPolicy struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Version         int
	Status          string
	PickupSeconds   int
	DeliverySeconds int
	CreatedAt       time.Time
	PublishedAt     *time.Time
	RetiredAt       *time.Time
}

func DurationPolicyFingerprint(pickupSeconds, deliverySeconds int) string {
	return "PICKUP=" + itoa(pickupSeconds) + "|DELIVERY=" + itoa(deliverySeconds)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [12]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func cloneDurationPolicies(in map[uuid.UUID]ServiceDurationPolicy) map[uuid.UUID]ServiceDurationPolicy {
	out := make(map[uuid.UUID]ServiceDurationPolicy, len(in))
	for id, row := range in {
		out[id] = row
	}
	return out
}

func (t *memTx) ActiveServiceDurationPolicy(_ context.Context, tenant uuid.UUID) (ServiceDurationPolicy, error) {
	for _, row := range t.durations {
		if row.TenantID == tenant && row.Status == DurationPolicyActive {
			return row, nil
		}
	}
	return ServiceDurationPolicy{}, ErrNotFound
}

func (t *memTx) InsertServiceDurationDraft(_ context.Context, row ServiceDurationPolicy) error {
	if row.PickupSeconds < 0 || row.DeliverySeconds < 0 || row.Version < 1 || row.Status != DurationPolicyDraft {
		return ErrConflict
	}
	if t.durations == nil {
		t.durations = map[uuid.UUID]ServiceDurationPolicy{}
	}
	for _, existing := range t.durations {
		if existing.TenantID == row.TenantID && existing.Version == row.Version {
			return ErrConflict
		}
	}
	t.durations[row.ID] = row
	return nil
}

func (t *memTx) UpdateServiceDurationDraft(_ context.Context, tenant, id uuid.UUID, pickup, delivery int) error {
	row, ok := t.durations[id]
	if !ok || row.TenantID != tenant {
		return ErrNotFound
	}
	if row.Status != DurationPolicyDraft || pickup < 0 || delivery < 0 {
		return ErrConflict
	}
	row.PickupSeconds = pickup
	row.DeliverySeconds = delivery
	t.durations[id] = row
	return nil
}

func (t *memTx) PublishServiceDurationPolicy(_ context.Context, tenant, id uuid.UUID, at time.Time) (ServiceDurationPolicy, error) {
	row, ok := t.durations[id]
	if !ok || row.TenantID != tenant {
		return ServiceDurationPolicy{}, ErrNotFound
	}
	if row.Status != DurationPolicyDraft {
		return ServiceDurationPolicy{}, ErrConflict
	}
	stamp := at.UTC()
	for otherID, existing := range t.durations {
		if existing.TenantID != tenant || existing.Status != DurationPolicyActive {
			continue
		}
		existing.Status = DurationPolicyRetired
		existing.RetiredAt = &stamp
		t.durations[otherID] = existing
	}
	row.Status = DurationPolicyActive
	row.PublishedAt = &stamp
	t.durations[id] = row
	return row, nil
}

func (t *memTx) NextServiceDurationVersion(_ context.Context, tenant uuid.UUID) (int, error) {
	next := 1
	for _, row := range t.durations {
		if row.TenantID == tenant && row.Version >= next {
			next = row.Version + 1
		}
	}
	return next, nil
}
