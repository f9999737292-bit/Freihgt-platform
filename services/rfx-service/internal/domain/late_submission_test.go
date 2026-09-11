package domain

import (
	"testing"
	"time"
)

func mustTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestResponseDeadlinePassedBoundary(t *testing.T) {
	deadline := mustTime("2026-01-01T12:00:00Z")
	if ResponseDeadlinePassed(&deadline, mustTime("2026-01-01T11:59:59Z")) {
		t.Fatal("expected deadline not passed one second before")
	}
	if !ResponseDeadlinePassed(&deadline, mustTime("2026-01-01T12:00:00Z")) {
		t.Fatal("expected deadline passed exactly at boundary")
	}
	if !ResponseDeadlinePassed(&deadline, mustTime("2026-01-01T12:00:01Z")) {
		t.Fatal("expected deadline passed one second after")
	}
}

func TestValidateLateRequestEligibility(t *testing.T) {
	deadline := mustTime("2026-01-01T12:00:00Z")
	if err := ValidateLateRequestEligibility(&deadline, mustTime("2026-01-01T11:59:59Z")); err == nil {
		t.Fatal("expected rejection before deadline")
	}
	if err := ValidateLateRequestEligibility(&deadline, mustTime("2026-01-01T12:00:00Z")); err != nil {
		t.Fatalf("expected allowed at deadline: %v", err)
	}
}

func TestValidateApprovedWindowActive(t *testing.T) {
	from := mustTime("2026-01-02T10:00:00Z")
	until := mustTime("2026-01-02T12:00:00Z")
	req := &LateSubmissionRequest{
		Status:             LateSubmissionStatusApproved,
		ApprovedValidFrom:  &from,
		ApprovedValidUntil: &until,
	}
	if err := ValidateApprovedWindowActive(req, mustTime("2026-01-02T09:59:59Z")); err == nil {
		t.Fatal("expected before window rejection")
	}
	if err := ValidateApprovedWindowActive(req, mustTime("2026-01-02T10:00:00Z")); err != nil {
		t.Fatalf("expected active at valid_from: %v", err)
	}
	if err := ValidateApprovedWindowActive(req, mustTime("2026-01-02T11:59:59Z")); err != nil {
		t.Fatalf("expected active inside window: %v", err)
	}
	if err := ValidateApprovedWindowActive(req, mustTime("2026-01-02T12:00:00Z")); err == nil {
		t.Fatal("expected expired at valid_until boundary")
	}
}

func TestValidateLateSubmissionTransition(t *testing.T) {
	cases := []struct {
		from, to LateSubmissionStatus
		ok       bool
	}{
		{LateSubmissionStatusRequested, LateSubmissionStatusApproved, true},
		{LateSubmissionStatusRequested, LateSubmissionStatusRejected, true},
		{LateSubmissionStatusApproved, LateSubmissionStatusConsumed, true},
		{LateSubmissionStatusApproved, LateSubmissionStatusRejected, false},
		{LateSubmissionStatusRejected, LateSubmissionStatusApproved, false},
		{LateSubmissionStatusConsumed, LateSubmissionStatusApproved, false},
	}
	for _, tc := range cases {
		err := ValidateLateSubmissionTransition(tc.from, tc.to)
		if tc.ok && err != nil {
			t.Fatalf("%s -> %s expected ok: %v", tc.from, tc.to, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s -> %s expected conflict", tc.from, tc.to)
		}
	}
}

func TestCanCreateAfterRejected(t *testing.T) {
	from := mustTime("2026-01-02T10:00:00Z")
	until := mustTime("2026-01-02T12:00:00Z")
	rejected := &LateSubmissionRequest{Status: LateSubmissionStatusRejected}
	if err := CanCreateLateSubmissionRequest(rejected, mustTime("2026-01-02T11:00:00Z")); err != nil {
		t.Fatalf("expected allowed after reject: %v", err)
	}
	active := &LateSubmissionRequest{
		Status: LateSubmissionStatusApproved, ApprovedValidFrom: &from, ApprovedValidUntil: &until,
	}
	if err := CanCreateLateSubmissionRequest(active, mustTime("2026-01-02T11:00:00Z")); err == nil {
		t.Fatal("expected conflict for active approved request")
	}
}

func TestIsLateSubmissionApprovedExpiredBoundary(t *testing.T) {
	from := mustTime("2026-01-02T10:00:00Z")
	until := mustTime("2026-01-02T12:00:00Z")
	req := &LateSubmissionRequest{
		Status: LateSubmissionStatusApproved, ApprovedValidFrom: &from, ApprovedValidUntil: &until,
	}
	if IsLateSubmissionApprovedExpired(req, mustTime("2026-01-02T11:59:59Z")) {
		t.Fatal("before valid_until must not be expired")
	}
	if !IsLateSubmissionApprovedExpired(req, mustTime("2026-01-02T12:00:00Z")) {
		t.Fatal("at valid_until boundary must be expired")
	}
}

func TestEffectiveLateSubmissionStatusExpired(t *testing.T) {
	from := mustTime("2026-01-02T10:00:00Z")
	until := mustTime("2026-01-02T12:00:00Z")
	req := &LateSubmissionRequest{
		Status:             LateSubmissionStatusApproved,
		ApprovedValidFrom:  &from,
		ApprovedValidUntil: &until,
	}
	if got := EffectiveLateSubmissionStatus(req, mustTime("2026-01-02T11:00:00Z")); got != LateSubmissionStatusApproved {
		t.Fatalf("expected APPROVED, got %s", got)
	}
	if got := EffectiveLateSubmissionStatus(req, mustTime("2026-01-02T12:00:00Z")); got != LateSubmissionStatusExpired {
		t.Fatalf("expected EXPIRED, got %s", got)
	}
}
