package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSigningSessionIsOverdue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	open := &SigningSession{ID: uuid.New(), Status: SigningSessionStatusCreated, ExpiresAt: &past}
	if !SigningSessionIsOverdue(open, now) {
		t.Fatal("created session past expires_at is overdue")
	}
	atDeadline := &SigningSession{Status: SigningSessionStatusInProgress, ExpiresAt: &now}
	if !SigningSessionIsOverdue(atDeadline, now) {
		t.Fatal("session at expires_at is overdue")
	}
	stillOpen := &SigningSession{Status: SigningSessionStatusCreated, ExpiresAt: &future}
	if SigningSessionIsOverdue(stillOpen, now) {
		t.Fatal("future expiry is not overdue")
	}
	noDeadline := &SigningSession{Status: SigningSessionStatusCreated}
	if SigningSessionIsOverdue(noDeadline, now) {
		t.Fatal("nil expires_at is not overdue")
	}
	completed := &SigningSession{Status: SigningSessionStatusCompleted, ExpiresAt: &past}
	if SigningSessionIsOverdue(completed, now) {
		t.Fatal("completed session is not rewritten as overdue")
	}
	if err := ValidateSigningSessionExpiry(&past, now); err == nil {
		t.Fatal("past expires_at must be rejected")
	}
	if err := ValidateSigningSessionExpiry(&future, now); err != nil {
		t.Fatalf("future expires_at: %v", err)
	}
	if err := ValidateSigningSessionExpiry(nil, now); err != nil {
		t.Fatalf("nil expires_at: %v", err)
	}
}
