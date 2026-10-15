package repository

import (
	"testing"
	"time"
)

func TestDeadlineAndClaimExpiryFrozenWindow(t *testing.T) {
	created := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	deadline, claim := DeadlineAndClaimExpiry(created)
	if !deadline.Equal(created.Add(20 * time.Second)) {
		t.Fatalf("deadline=%v", deadline)
	}
	if !claim.Equal(created.Add(25 * time.Second)) {
		t.Fatalf("claim=%v", claim)
	}
}

func TestServiceDurationSnapshotUnknownVsKnownZero(t *testing.T) {
	zero := 0
	known, err := marshalServiceDurationSnapshot(ServiceDurationSnapshot{
		State:           ServiceDurationKnown,
		PickupKnown:     true,
		PickupSeconds:   &zero,
		DeliveryKnown:   true,
		DeliverySeconds: &zero,
	})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := marshalServiceDurationSnapshot(ServiceDurationSnapshot{
		State:         ServiceDurationUnknown,
		PickupKnown:   false,
		DeliveryKnown: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	gotKnown, err := unmarshalServiceDurationSnapshot(known)
	if err != nil {
		t.Fatal(err)
	}
	gotUnknown, err := unmarshalServiceDurationSnapshot(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if gotKnown.State != ServiceDurationKnown || gotKnown.PickupSeconds == nil || *gotKnown.PickupSeconds != 0 {
		t.Fatalf("known zero: %+v", gotKnown)
	}
	if gotUnknown.State != ServiceDurationUnknown || gotUnknown.PickupSeconds != nil {
		t.Fatalf("unknown: %+v", gotUnknown)
	}
	if string(known) == string(unknown) {
		t.Fatal("known zero must not equal unknown encoding")
	}
}
