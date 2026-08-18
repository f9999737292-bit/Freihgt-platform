package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func insertStatusHistoryAndOutbox(ctx context.Context, tx pgx.Tx, write statusHistoryWrite) error {
	history, err := insertStatusHistoryRowReturning(ctx, tx, write)
	if err != nil {
		return err
	}
	outbox, err := domain.BuildOutboxEventFromStatusHistory(history, write.snapshot)
	if err != nil {
		return err
	}
	if err := domain.ValidateOutboxAgainstHistory(outbox, history); err != nil {
		return err
	}
	return insertOutboxRow(ctx, tx, outbox)
}

func insertStatusHistoryRowReturning(ctx context.Context, tx pgx.Tx, write statusHistoryWrite) (domain.ShipmentStatusHistory, error) {
	var history domain.ShipmentStatusHistory
	err := tx.QueryRow(ctx, insertStatusHistoryReturningQuery,
		write.tenantID,
		write.shipmentID,
		write.shipmentVersion,
		write.fromStatus,
		write.toStatus,
		optionalString(write.reasonCode),
		write.source,
		write.actorType,
		optionalUUID(write.actorID),
		optionalString(write.correlationID),
		write.occurredAt,
	).Scan(
		&history.ID,
		&history.TenantID,
		&history.ShipmentID,
		&history.ShipmentVersion,
		&history.FromStatus,
		&history.ToStatus,
		&history.ReasonCode,
		&history.Source,
		&history.ActorType,
		&history.ActorID,
		&history.CorrelationID,
		&history.OccurredAt,
		&history.RecordedAt,
	)
	if err != nil {
		return domain.ShipmentStatusHistory{}, mapDBError(err)
	}
	return history, nil
}

func insertOutboxRow(ctx context.Context, tx pgx.Tx, event domain.ShipmentOutboxEvent) error {
	_, err := tx.Exec(ctx, insertOutboxEventQuery,
		event.ID,
		event.TenantID,
		event.AggregateType,
		event.AggregateID,
		event.AggregateVersion,
		event.EventType,
		event.SchemaVersion,
		event.SourceEventID,
		event.Payload,
		event.Headers,
		string(event.Status),
		event.Attempts,
		event.AvailableAt,
	)
	return mapDBError(err)
}

func scanOutboxEvents(rows pgx.Rows) ([]domain.ShipmentOutboxEvent, error) {
	defer rows.Close()
	items := make([]domain.ShipmentOutboxEvent, 0)
	for rows.Next() {
		var event domain.ShipmentOutboxEvent
		var status string
		err := rows.Scan(
			&event.ID,
			&event.TenantID,
			&event.AggregateType,
			&event.AggregateID,
			&event.AggregateVersion,
			&event.EventType,
			&event.SchemaVersion,
			&event.SourceEventID,
			&event.Payload,
			&event.Headers,
			&status,
			&event.Attempts,
			&event.AvailableAt,
			&event.LockedAt,
			&event.LockedBy,
			&event.PublishedAt,
			&event.LastErrorCode,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, mapDBError(err)
		}
		event.Status = domain.OutboxStatus(status)
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return items, nil
}

func (r *ShipmentRepository) ClaimPendingForPublisher(
	ctx context.Context,
	workerID string,
	batchSize int,
	now time.Time,
	leaseTimeout time.Duration,
) ([]domain.ShipmentOutboxEvent, error) {
	var result []domain.ShipmentOutboxEvent
	err := measureDB("shipment_repository", "claim_outbox_pending", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		staleLockCutoff := now.Add(-leaseTimeout)
		rows, err := tx.Query(ctx, claimPendingOutboxQuery, now, staleLockCutoff, batchSize)
		if err != nil {
			return mapDBError(err)
		}
		events, err := scanOutboxEvents(rows)
		if err != nil {
			return err
		}
		for _, event := range events {
			tag, err := tx.Exec(ctx, lockClaimedOutboxQuery, now, workerID, event.ID)
			if err != nil {
				return mapDBError(err)
			}
			if tag.RowsAffected() == 0 {
				continue
			}
			lockedAt := now
			lockedBy := workerID
			event.LockedAt = &lockedAt
			event.LockedBy = &lockedBy
			event.Attempts++
			result = append(result, event)
		}
		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return result, err
}

func (r *ShipmentRepository) MarkPublished(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	publishedAt time.Time,
) error {
	return measureDB("shipment_repository", "mark_outbox_published", func() error {
		tag, err := r.pool.Exec(ctx, markOutboxPublishedQuery, publishedAt, eventID, workerID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrOutboxPublishStateConflict
		}
		return nil
	})
}

func (r *ShipmentRepository) ReleaseWithRetry(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	availableAt time.Time,
	errorCode string,
) error {
	return measureDB("shipment_repository", "release_outbox_retry", func() error {
		tag, err := r.pool.Exec(ctx, releaseOutboxWithRetryQuery, availableAt, errorCode, eventID, workerID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrOutboxPublishStateConflict
		}
		return nil
	})
}

func (r *ShipmentRepository) MarkFailed(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	errorCode string,
) error {
	return measureDB("shipment_repository", "mark_outbox_failed", func() error {
		tag, err := r.pool.Exec(ctx, markOutboxFailedQuery, errorCode, eventID, workerID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrOutboxPublishStateConflict
		}
		return nil
	})
}

func (r *ShipmentRepository) OutboxGaugeSnapshot(ctx context.Context, now time.Time) (pending int64, failed int64, oldestPendingAgeSeconds float64, err error) {
	err = measureDB("shipment_repository", "outbox_gauge_snapshot", func() error {
		if scanErr := r.pool.QueryRow(ctx, countOutboxPendingQuery).Scan(&pending); scanErr != nil {
			return mapDBError(scanErr)
		}
		if scanErr := r.pool.QueryRow(ctx, countOutboxFailedQuery).Scan(&failed); scanErr != nil {
			return mapDBError(scanErr)
		}
		if scanErr := r.pool.QueryRow(ctx, oldestPendingOutboxAgeQuery, now).Scan(&oldestPendingAgeSeconds); scanErr != nil {
			return mapDBError(scanErr)
		}
		return nil
	})
	return pending, failed, oldestPendingAgeSeconds, err
}
