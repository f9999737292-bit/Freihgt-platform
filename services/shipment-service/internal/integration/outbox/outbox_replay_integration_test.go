//go:build integration

package outbox

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/outboxreplay"
)

func TestOutboxReplayDryRunDoesNotMutate(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipmentID, events := seedFailedOutboxChain(t, env, fix, "SHP-REPLAY-DRY")
	svc := outboxreplay.NewService(env.repo)

	result, err := svc.ReplayFailedOutbox(ctx, outboxreplay.Request{
		TenantID:     fix.TenantID,
		AggregateIDs: []uuid.UUID{shipmentID},
		Execute:      false,
	})
	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.Len(t, result.Preview, len(events))

	for _, eventID := range events {
		var status string
		require.NoError(t, env.pool.QueryRow(ctx, `
			SELECT status FROM transport.shipment_event_outbox WHERE id = $1
		`, eventID).Scan(&status))
		require.Equal(t, "FAILED", status)
	}
}

func TestOutboxReplayExecuteResetsFailedRowsPreservesIdentity(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipmentID, events := seedFailedOutboxChain(t, env, fix, "SHP-REPLAY-EXEC")
	before := snapshotOutboxRows(t, env, events)

	svc := outboxreplay.NewService(env.repo)
	result, err := svc.ReplayFailedOutbox(ctx, outboxreplay.Request{
		TenantID:     fix.TenantID,
		AggregateIDs: []uuid.UUID{shipmentID},
		Execute:      true,
		Now:          time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, int64(len(events)), result.AffectedCount)

	after := snapshotOutboxRows(t, env, events)
	for _, eventID := range events {
		require.Equal(t, "PENDING", after[eventID].status)
		require.Zero(t, after[eventID].attempts)
		require.Nil(t, after[eventID].lastErrorCode)
		require.Nil(t, after[eventID].lockedAt)
		require.Nil(t, after[eventID].lockedBy)
		require.Nil(t, after[eventID].publishedAt)

		require.Equal(t, before[eventID].payload, after[eventID].payload)
		require.Equal(t, before[eventID].sourceEventID, after[eventID].sourceEventID)
		require.Equal(t, before[eventID].aggregateID, after[eventID].aggregateID)
		require.Equal(t, before[eventID].tenantID, after[eventID].tenantID)
		require.Equal(t, before[eventID].createdAt, after[eventID].createdAt)
	}
}

func TestOutboxReplayMultipleEventsPreserveOrdering(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipmentID, events := seedFailedOutboxChain(t, env, fix, "SHP-REPLAY-ORDER")
	require.GreaterOrEqual(t, len(events), 2)

	svc := outboxreplay.NewService(env.repo)
	_, err := svc.ReplayFailedOutbox(ctx, outboxreplay.Request{
		TenantID:     fix.TenantID,
		AggregateIDs: []uuid.UUID{shipmentID},
		Execute:      true,
	})
	require.NoError(t, err)

	now := claimNow()
	claimed, err := env.repo.ClaimPendingForPublisher(ctx, "replay-order-worker", 10, now, time.Minute)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(claimed), 1)
	require.Equal(t, events[0], claimed[0].ID)
}

func TestOutboxReplayPartialSelectionRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, events := seedFailedOutboxChain(t, env, fix, "SHP-REPLAY-PARTIAL")
	require.GreaterOrEqual(t, len(events), 2)

	svc := outboxreplay.NewService(env.repo)
	_, err := svc.ReplayFailedOutbox(ctx, outboxreplay.Request{
		TenantID: fix.TenantID,
		EventIDs: []uuid.UUID{events[len(events)-1]},
		Execute:  false,
	})
	require.ErrorIs(t, err, outboxreplay.ErrPartialAggregateReplay)
}

func TestOutboxReplayDuplicateExecuteSafe(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipmentID, _ := seedFailedOutboxChain(t, env, fix, "SHP-REPLAY-DUP")
	svc := outboxreplay.NewService(env.repo)
	req := outboxreplay.Request{
		TenantID:     fix.TenantID,
		AggregateIDs: []uuid.UUID{shipmentID},
		Execute:      true,
	}
	_, err := svc.ReplayFailedOutbox(ctx, req)
	require.NoError(t, err)

	_, err = svc.ReplayFailedOutbox(ctx, req)
	require.ErrorIs(t, err, outboxreplay.ErrNoMatchingFailedRows)
}

func TestOutboxReplayConcurrentInvocationSafe(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipmentID, _ := seedFailedOutboxChain(t, env, fix, "SHP-REPLAY-CONC")
	svc := outboxreplay.NewService(env.repo)
	req := outboxreplay.Request{
		TenantID:     fix.TenantID,
		AggregateIDs: []uuid.UUID{shipmentID},
		Execute:      true,
	}

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, results[idx] = svc.ReplayFailedOutbox(ctx, req)
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
}

func TestOutboxReplayPublishedRowProtected(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-REPLAY-PUB"), userTransition(fix.UserID))
	require.NoError(t, err)

	var publishedID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT id FROM transport.shipment_event_outbox WHERE aggregate_id = $1 LIMIT 1
	`, shipment.ID).Scan(&publishedID))
	_, err = env.pool.Exec(ctx, `
		UPDATE transport.shipment_event_outbox SET status = 'PUBLISHED', published_at = now() WHERE id = $1
	`, publishedID)
	require.NoError(t, err)

	svc := outboxreplay.NewService(env.repo)
	_, err = svc.ReplayFailedOutbox(ctx, outboxreplay.Request{
		TenantID:     fix.TenantID,
		AggregateIDs: []uuid.UUID{shipment.ID},
		Execute:      false,
	})
	require.ErrorIs(t, err, outboxreplay.ErrNoMatchingFailedRows)
}

type outboxSnapshot struct {
	status        string
	attempts      int
	lastErrorCode *string
	lockedAt      *time.Time
	lockedBy      *string
	publishedAt   *time.Time
	payload       []byte
	sourceEventID uuid.UUID
	aggregateID   uuid.UUID
	tenantID      uuid.UUID
	createdAt     time.Time
}

func seedFailedOutboxChain(t *testing.T, env *testEnv, fix seedFixture, number string) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, number), userTransition(fix.UserID))
	require.NoError(t, err)

	_, err = env.repo.UpdateStatus(
		ctx,
		shipment.ID,
		fix.TenantID,
		domain.ShipmentStatusCarrierAssigned,
		domain.ShipmentStatusAcceptedByCarrier,
		nil,
		nil,
		shipment.Version,
		userTransition(fix.UserID),
	)
	require.NoError(t, err)

	rows, err := env.pool.Query(ctx, `
		SELECT id FROM transport.shipment_event_outbox
		WHERE aggregate_id = $1
		ORDER BY created_at ASC, id ASC
	`, shipment.ID)
	require.NoError(t, err)
	defer rows.Close()

	var eventIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		eventIDs = append(eventIDs, id)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, eventIDs)

	code := "TRANSIENT_TIMEOUT"
	for _, eventID := range eventIDs {
		_, err = env.pool.Exec(ctx, `
			UPDATE transport.shipment_event_outbox
			SET status = 'FAILED', attempts = 5, last_error_code = $2, published_at = NULL
			WHERE id = $1
		`, eventID, code)
		require.NoError(t, err)
	}
	return shipment.ID, eventIDs
}

func snapshotOutboxRows(t *testing.T, env *testEnv, eventIDs []uuid.UUID) map[uuid.UUID]outboxSnapshot {
	t.Helper()
	ctx := context.Background()
	out := make(map[uuid.UUID]outboxSnapshot, len(eventIDs))
	for _, eventID := range eventIDs {
		var snap outboxSnapshot
		err := env.pool.QueryRow(ctx, `
			SELECT status, attempts, last_error_code, locked_at, locked_by, published_at,
			       payload, source_event_id, aggregate_id, tenant_id, created_at
			FROM transport.shipment_event_outbox
			WHERE id = $1
		`, eventID).Scan(
			&snap.status,
			&snap.attempts,
			&snap.lastErrorCode,
			&snap.lockedAt,
			&snap.lockedBy,
			&snap.publishedAt,
			&snap.payload,
			&snap.sourceEventID,
			&snap.aggregateID,
			&snap.tenantID,
			&snap.createdAt,
		)
		require.NoError(t, err)
		out[eventID] = snap
	}
	return out
}
