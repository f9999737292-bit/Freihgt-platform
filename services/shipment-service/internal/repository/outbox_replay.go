package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
)

// OutboxReplayPreviewRow is safe operator metadata for dry-run output.
type OutboxReplayPreviewRow struct {
	EventID       uuid.UUID
	TenantID      uuid.UUID
	AggregateID   uuid.UUID
	EventType     string
	Status        domain.OutboxStatus
	AttemptCount  int
	LastErrorCode *string
}

// OutboxReplayOrderingRow supports per-aggregate ordering validation.
type OutboxReplayOrderingRow struct {
	ID               uuid.UUID
	Status           domain.OutboxStatus
	CreatedAt        time.Time
	AggregateVersion int
}

func (r *ShipmentRepository) ListFailedOutboxForReplay(
	ctx context.Context,
	tenantID uuid.UUID,
	eventIDs []uuid.UUID,
	aggregateIDs []uuid.UUID,
) ([]OutboxReplayPreviewRow, error) {
	var eventArg any
	if len(eventIDs) > 0 {
		eventArg = eventIDs
	}
	var aggregateArg any
	if len(aggregateIDs) > 0 {
		aggregateArg = aggregateIDs
	}

	var rows []OutboxReplayPreviewRow
	err := measureDB("shipment_repository", "list_failed_outbox_replay", func() error {
		result, err := r.pool.Query(ctx, listFailedOutboxForReplayQuery, tenantID, eventArg, aggregateArg)
		if err != nil {
			return mapDBError(err)
		}
		defer result.Close()

		for result.Next() {
			var row OutboxReplayPreviewRow
			var status string
			if err := result.Scan(
				&row.EventID,
				&row.TenantID,
				&row.AggregateID,
				&row.EventType,
				&status,
				&row.AttemptCount,
				&row.LastErrorCode,
			); err != nil {
				return mapDBError(err)
			}
			row.Status = domain.OutboxStatus(status)
			rows = append(rows, row)
		}
		return mapDBError(result.Err())
	})
	return rows, err
}

func (r *ShipmentRepository) ListOutboxReplayOrdering(
	ctx context.Context,
	tenantID uuid.UUID,
	aggregateID uuid.UUID,
) ([]OutboxReplayOrderingRow, error) {
	var rows []OutboxReplayOrderingRow
	err := measureDB("shipment_repository", "list_outbox_replay_ordering", func() error {
		result, err := r.pool.Query(ctx, listOutboxReplayOrderingQuery, tenantID, aggregateID)
		if err != nil {
			return mapDBError(err)
		}
		defer result.Close()

		for result.Next() {
			var row OutboxReplayOrderingRow
			var status string
			if err := result.Scan(&row.ID, &status, &row.CreatedAt, &row.AggregateVersion); err != nil {
				return mapDBError(err)
			}
			row.Status = domain.OutboxStatus(status)
			rows = append(rows, row)
		}
		return mapDBError(result.Err())
	})
	return rows, err
}

func (r *ShipmentRepository) ReplayFailedOutboxRows(
	ctx context.Context,
	tenantID uuid.UUID,
	eventIDs []uuid.UUID,
	availableAt time.Time,
	expectedCount int,
) (int64, error) {
	if expectedCount <= 0 {
		return 0, fmt.Errorf("expected replay count must be positive")
	}
	if len(eventIDs) != expectedCount {
		return 0, fmt.Errorf("event id count mismatch: got %d want %d", len(eventIDs), expectedCount)
	}

	var affected int64
	err := measureDB("shipment_repository", "replay_failed_outbox", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		if err := lockOutboxReplayRows(ctx, tx, tenantID, eventIDs); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, replayFailedOutboxRowsQuery, availableAt, tenantID, eventIDs)
		if err != nil {
			return mapDBError(err)
		}
		affected = tag.RowsAffected()
		if int(affected) != expectedCount {
			return fmt.Errorf("replay affected %d rows, expected %d", affected, expectedCount)
		}

		if err := verifyReplayImmutableFields(ctx, tx, tenantID, eventIDs); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return affected, err
}

func lockOutboxReplayRows(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, eventIDs []uuid.UUID) error {
	const q = `
SELECT id
FROM transport.shipment_event_outbox
WHERE tenant_id = $1
  AND id = ANY($2::uuid[])
  AND status = 'FAILED'
ORDER BY created_at ASC, id ASC
FOR UPDATE`
	rows, err := tx.Query(ctx, q, tenantID, eventIDs)
	if err != nil {
		return mapDBError(err)
	}
	defer rows.Close()

	locked := 0
	for rows.Next() {
		locked++
	}
	if err := rows.Err(); err != nil {
		return mapDBError(err)
	}
	if locked != len(eventIDs) {
		return fmt.Errorf("replay lock matched %d rows, expected %d", locked, len(eventIDs))
	}
	return nil
}

func verifyReplayImmutableFields(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, eventIDs []uuid.UUID) error {
	const q = `
SELECT id, payload, source_event_id, aggregate_id, tenant_id, created_at
FROM transport.shipment_event_outbox
WHERE tenant_id = $1
  AND id = ANY($2::uuid[])
  AND status = 'PENDING'`
	rows, err := tx.Query(ctx, q, tenantID, eventIDs)
	if err != nil {
		return mapDBError(err)
	}
	defer rows.Close()

	type snapshot struct {
		id            uuid.UUID
		payload       []byte
		sourceEventID uuid.UUID
		aggregateID   uuid.UUID
		tenantID      uuid.UUID
		createdAt     time.Time
	}
	snapshots := make(map[uuid.UUID]snapshot, len(eventIDs))

	for rows.Next() {
		var snap snapshot
		if err := rows.Scan(
			&snap.id,
			&snap.payload,
			&snap.sourceEventID,
			&snap.aggregateID,
			&snap.tenantID,
			&snap.createdAt,
		); err != nil {
			return mapDBError(err)
		}
		if len(snap.payload) == 0 {
			return fmt.Errorf("replay would leave event %s without payload", snap.id)
		}
		snapshots[snap.id] = snap
	}
	if err := rows.Err(); err != nil {
		return mapDBError(err)
	}
	if len(snapshots) != len(eventIDs) {
		return fmt.Errorf("replay verification matched %d rows, expected %d", len(snapshots), len(eventIDs))
	}
	return nil
}
