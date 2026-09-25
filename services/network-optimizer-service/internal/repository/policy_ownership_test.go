package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestBNO201CapacityPolicyTenantTakeoverRejected(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	owner := uuid.New()
	other := uuid.New()
	capacityID := uuid.New()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	if err := store.Within(ctx, func(tx Tx) error {
		return tx.InsertCapacity(ctx, domain.Capacity{
			ID: capacityID, OwnerTenantID: owner, LocationLabel: "yard",
			AvailableFrom: now, AvailableUntil: now.Add(time.Hour),
			Source: domain.SourceManual, VisibilityScope: "PRIVATE",
			Status: domain.CapacityAvailable, Version: 1, CreatedAt: now, UpdatedAt: now,
		})
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCapacityPolicy(ctx, other, capacityID, domain.NextLoadSearchPolicy{}); err != ErrNotFound {
		t.Fatalf("BNO201 takeover %v", err)
	}
	if err := store.UpsertCapacityPolicy(ctx, owner, capacityID, domain.NextLoadSearchPolicy{}); err != nil {
		t.Fatal(err)
	}
	km := 40.0
	if err := store.UpsertCapacityPolicy(ctx, other, capacityID, domain.NextLoadSearchPolicy{MaxDeadheadKm: &km}); err != ErrNotFound {
		t.Fatalf("BNO201 rewrite %v", err)
	}
	got, err := store.GetCapacityPolicy(ctx, owner, capacityID)
	if err != nil || got.MaxDeadheadKm != nil {
		t.Fatalf("BNO201 owner policy changed %+v %v", got, err)
	}
	if _, err := store.GetCapacityPolicy(ctx, other, capacityID); err != ErrNotFound {
		t.Fatalf("BNO201 foreign read %v", err)
	}
	if err := store.UpsertCapacityPolicy(ctx, owner, uuid.New(), domain.NextLoadSearchPolicy{}); err != ErrNotFound {
		t.Fatalf("BNO201 missing capacity %v", err)
	}
}
