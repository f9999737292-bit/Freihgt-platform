//go:build integration

package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestMarkPublishedOwnership(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-PUB-1"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_ = shipment
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-A", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: err=%v count=%d", err, len(events))
	}
	eventID := events[0].ID

	if err := env.repo.MarkPublished(ctx, eventID, "worker-B", now); !errors.Is(err, domain.ErrOutboxPublishStateConflict) {
		t.Fatalf("foreign worker mark published err=%v", err)
	}
	if err := env.repo.MarkPublished(ctx, eventID, "worker-A", now); err != nil {
		t.Fatalf("owner mark published: %v", err)
	}

	var status string
	var lockedBy *string
	var lockedAt *time.Time
	var lastError *string
	var publishedAt *time.Time
	if err := env.pool.QueryRow(ctx, `
		SELECT status, locked_by, locked_at, last_error_code, published_at
		FROM transport.shipment_event_outbox WHERE id = $1
	`, eventID).Scan(&status, &lockedBy, &lockedAt, &lastError, &publishedAt); err != nil {
		t.Fatalf("load row: %v", err)
	}
	if status != string(domain.OutboxStatusPublished) || lockedBy != nil || lockedAt != nil || lastError != nil || publishedAt == nil {
		t.Fatalf("published row invalid: status=%s lockedBy=%v publishedAt=%v", status, lockedBy, publishedAt)
	}
}

func TestReleaseWithRetryOwnership(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()
	future := claimNow().Add(30 * time.Second)

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-RETRY-1"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-A", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: %v", err)
	}
	eventID := events[0].ID

	if err := env.repo.ReleaseWithRetry(ctx, eventID, "worker-B", future, "TRANSIENT_NETWORK"); !errors.Is(err, domain.ErrOutboxPublishStateConflict) {
		t.Fatalf("foreign release err=%v", err)
	}
	if err := env.repo.ReleaseWithRetry(ctx, eventID, "worker-A", future, "TRANSIENT_NETWORK"); err != nil {
		t.Fatalf("owner release: %v", err)
	}

	var status string
	var availableAt time.Time
	var lockedBy *string
	var errorCode *string
	if err := env.pool.QueryRow(ctx, `
		SELECT status, available_at, locked_by, last_error_code
		FROM transport.shipment_event_outbox WHERE id = $1
	`, eventID).Scan(&status, &availableAt, &lockedBy, &errorCode); err != nil {
		t.Fatalf("load row: %v", err)
	}
	if status != string(domain.OutboxStatusPending) || !availableAt.After(now) || lockedBy != nil || errorCode == nil || *errorCode != "TRANSIENT_NETWORK" {
		t.Fatalf("retry row invalid: status=%s availableAt=%s error=%v", status, availableAt, errorCode)
	}
}

func TestMarkFailedOwnership(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-FAIL-1"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-A", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: %v", err)
	}
	eventID := events[0].ID

	if err := env.repo.MarkFailed(ctx, eventID, "worker-B", "PAYLOAD_REJECTED"); !errors.Is(err, domain.ErrOutboxPublishStateConflict) {
		t.Fatalf("foreign mark failed err=%v", err)
	}
	if err := env.repo.MarkFailed(ctx, eventID, "worker-A", "PAYLOAD_REJECTED"); err != nil {
		t.Fatalf("owner mark failed: %v", err)
	}

	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM transport.shipment_event_outbox WHERE id = $1`, eventID).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != string(domain.OutboxStatusFailed) {
		t.Fatalf("status=%s", status)
	}

	reclaimed, err := env.repo.ClaimPendingForPublisher(ctx, "worker-C", 5, now, time.Minute)
	if err != nil {
		t.Fatalf("claim after failed: %v", err)
	}
	for _, event := range reclaimed {
		if event.ID == eventID {
			t.Fatal("FAILED event must not be claimed")
		}
	}
}
