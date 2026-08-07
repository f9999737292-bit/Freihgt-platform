//go:build integration

package outbox

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestConcurrentTwoWorkerClaimNoOverlap(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()
	now := claimNow()
	lease := 60 * time.Second

	for i := 0; i < 4; i++ {
		_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, fmt.Sprintf("SHP-CLAIM-%d", i)), userTransition(fix.UserID))
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	type claimResult struct {
		worker string
		events []domain.ShipmentOutboxEvent
		err    error
	}
	results := make([]claimResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for idx, workerID := range []string{"worker-A", "worker-B"} {
		go func(i int, worker string) {
			defer wg.Done()
			events, err := env.repo.ClaimPendingForPublisher(ctx, worker, 2, now, lease)
			results[i] = claimResult{worker: worker, events: events, err: err}
		}(idx, workerID)
	}
	wg.Wait()

	seen := map[uuid.UUID]string{}
	total := 0
	for _, res := range results {
		if res.err != nil {
			t.Fatalf("worker %s claim failed: %v", res.worker, res.err)
		}
		if len(res.events) > 2 {
			t.Fatalf("worker %s batch too large: %d", res.worker, len(res.events))
		}
		for _, event := range res.events {
			if prev, ok := seen[event.ID]; ok {
				t.Fatalf("event %s claimed by both %s and %s", event.ID, prev, res.worker)
			}
			seen[event.ID] = res.worker
			if event.LockedBy == nil || *event.LockedBy != res.worker {
				t.Fatalf("locked_by mismatch for %s", event.ID)
			}
			if event.Attempts != 1 {
				t.Fatalf("attempts=%d want 1", event.Attempts)
			}
			total++
		}
	}
	if total == 0 {
		t.Fatal("expected at least one claimed event")
	}

	var lockedCount int64
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transport.shipment_event_outbox
		WHERE locked_by IS NOT NULL AND status = 'PENDING'
	`).Scan(&lockedCount); err != nil {
		t.Fatalf("count locks: %v", err)
	}
	if int(lockedCount) != total {
		t.Fatalf("visible locks=%d claimed=%d", lockedCount, total)
	}
}

func TestActiveLeaseBlocksOtherWorker(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()
	lease := 60 * time.Second

	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-LEASE-ACTIVE"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	var eventID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT id FROM transport.shipment_event_outbox WHERE aggregate_id = $1 LIMIT 1
	`, shipment.ID).Scan(&eventID); err != nil {
		t.Fatalf("event id: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE transport.shipment_event_outbox
		SET locked_by = 'worker-A', locked_at = $1, attempts = 1
		WHERE id = $2
	`, now, eventID); err != nil {
		t.Fatalf("seed active lock: %v", err)
	}

	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-B", 10, now, lease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	for _, event := range events {
		if event.ID == eventID {
			t.Fatal("worker-B must not claim actively locked event")
		}
	}
}

func TestStaleLeaseRecovery(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()
	lease := 30 * time.Second

	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-LEASE-STALE"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	var eventID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT id FROM transport.shipment_event_outbox WHERE aggregate_id = $1 LIMIT 1
	`, shipment.ID).Scan(&eventID); err != nil {
		t.Fatalf("event id: %v", err)
	}
	staleLockedAt := now.Add(-2 * lease)
	if _, err := env.pool.Exec(ctx, `
		UPDATE transport.shipment_event_outbox
		SET locked_by = 'worker-A', locked_at = $1, attempts = 2, available_at = $1
		WHERE id = $2
	`, staleLockedAt, eventID); err != nil {
		t.Fatalf("seed stale lock: %v", err)
	}

	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-B", 10, now, lease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	var claimed *domain.ShipmentOutboxEvent
	for i := range events {
		if events[i].ID == eventID {
			claimed = &events[i]
			break
		}
	}
	if claimed == nil {
		t.Fatal("worker-B must reclaim stale lock")
	}
	if claimed.LockedBy == nil || *claimed.LockedBy != "worker-B" {
		t.Fatal("locked_by must switch to worker-B")
	}
	if claimed.Attempts != 3 {
		t.Fatalf("attempts=%d want 3", claimed.Attempts)
	}
}

func TestPublishedAndFailedExcludedFromClaim(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()
	now := claimNow()

	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-STATE-1"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	publishedID := uuid.New()
	failedID := uuid.New()
	historyPublished := uuid.New()
	historyFailed := uuid.New()
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.shipment_status_history (
			id, tenant_id, shipment_id, shipment_version, from_status, to_status, source, actor_type, occurred_at
		) VALUES ($1,$2,$3,99,'A','B','SHIPMENT_SERVICE','USER',now()),
		         ($4,$2,$3,100,'B','C','SHIPMENT_SERVICE','USER',now())
	`, historyPublished, fix.TenantID, shipment.ID, historyFailed)
	if err != nil {
		t.Fatalf("seed history: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.shipment_event_outbox (
			id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			event_type, schema_version, source_event_id, payload, headers, status, attempts, available_at, published_at
		) VALUES
		($1,$2,'SHIPMENT',$3,99,'shipment.status.changed',1,$4,'{}','{}','PUBLISHED',1,now(),now()),
		($5,$2,'SHIPMENT',$3,100,'shipment.status.changed',1,$6,'{}','{}','FAILED',5,now(),NULL)
	`, publishedID, fix.TenantID, shipment.ID, historyPublished, failedID, historyFailed)
	if err != nil {
		t.Fatalf("seed published/failed: %v", err)
	}

	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-Z", 20, now, time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	for _, event := range events {
		if event.ID == publishedID || event.ID == failedID {
			t.Fatalf("must not claim status=%s id=%s", event.Status, event.ID)
		}
	}
}

func repositoryCreateParams(fix seedFixture, number string) repository.CreateShipmentParams {
	return repository.CreateShipmentParams{
		TenantID:              fix.TenantID,
		ShipmentNumber:        number,
		TransportOrderID:      fix.TransportOrderID,
		ShipperCompanyID:      fix.ShipperID,
		ConsigneeCompanyID:    fix.ConsigneeID,
		CarrierCompanyID:      fix.CarrierID,
		OriginLocationID:      fix.OriginID,
		DestinationLocationID: fix.DestinationID,
		TransportMode:         "ROAD",
	}
}
