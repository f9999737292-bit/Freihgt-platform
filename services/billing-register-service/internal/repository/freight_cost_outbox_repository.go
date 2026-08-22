package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type FreightCostOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewFreightCostOutboxRepository(pool *pgxpool.Pool) *FreightCostOutboxRepository {
	return &FreightCostOutboxRepository{pool: pool}
}

func isFreightCostOutboxDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return strings.Contains(pgErr.ConstraintName, "freight_cost_outbox")
}

func (r *FreightCostOutboxRepository) InsertTx(
	ctx context.Context,
	tx pgx.Tx,
	eventID, tenantID uuid.UUID,
	aggregateType string,
	aggregateID uuid.UUID,
	sourceRevision int64,
	eventType string,
	schemaVersion int,
	payload json.RawMessage,
	availableAt time.Time,
) error {
	_, err := tx.Exec(ctx, insertFreightCostOutboxEventQuery,
		eventID, tenantID, aggregateType, aggregateID, sourceRevision,
		eventType, schemaVersion, payload,
		domain.FreightCostOutboxStatusPending, 0, availableAt,
	)
	if err != nil {
		if isFreightCostOutboxDuplicate(err) {
			return nil
		}
		return mapDBError(err)
	}
	return nil
}

func scanFreightCostOutboxEvents(rows pgx.Rows) ([]domain.FreightCostOutboxEvent, error) {
	defer rows.Close()
	items := make([]domain.FreightCostOutboxEvent, 0)
	for rows.Next() {
		var event domain.FreightCostOutboxEvent
		err := rows.Scan(
			&event.ID, &event.TenantID, &event.AggregateType, &event.AggregateID, &event.SourceRevision,
			&event.EventType, &event.SchemaVersion, &event.Payload,
			&event.Status, &event.Attempts, &event.AvailableAt, &event.LockedAt, &event.LockedBy,
			&event.PublishedAt, &event.LastErrorCode, &event.CreatedAt,
		)
		if err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

func (r *FreightCostOutboxRepository) ClaimPendingForPublisher(
	ctx context.Context,
	workerID string,
	batchSize int,
	now time.Time,
	leaseTimeout time.Duration,
) ([]domain.FreightCostOutboxEvent, error) {
	var result []domain.FreightCostOutboxEvent
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	staleLockCutoff := now.Add(-leaseTimeout)
	rows, err := tx.Query(ctx, claimPendingFreightCostOutboxQuery, now, staleLockCutoff, batchSize)
	if err != nil {
		return nil, mapDBError(err)
	}
	events, err := scanFreightCostOutboxEvents(rows)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		tag, err := tx.Exec(ctx, lockClaimedFreightCostOutboxQuery, now, workerID, event.ID)
		if err != nil {
			return nil, mapDBError(err)
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
		return nil, mapDBError(err)
	}
	return result, nil
}

func (r *FreightCostOutboxRepository) MarkPublished(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	publishedAt time.Time,
) error {
	tag, err := r.pool.Exec(ctx, markFreightCostOutboxPublishedQuery, publishedAt, eventID, workerID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFreightCostOutboxPublishStateConflict
	}
	return nil
}

func (r *FreightCostOutboxRepository) ReleaseWithRetry(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	availableAt time.Time,
	errorCode string,
) error {
	tag, err := r.pool.Exec(ctx, releaseFreightCostOutboxWithRetryQuery, availableAt, errorCode, eventID, workerID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFreightCostOutboxPublishStateConflict
	}
	return nil
}

func (r *FreightCostOutboxRepository) MarkFailed(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	errorCode string,
) error {
	tag, err := r.pool.Exec(ctx, markFreightCostOutboxFailedQuery, errorCode, eventID, workerID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFreightCostOutboxPublishStateConflict
	}
	return nil
}

func (r *FreightCostOutboxRepository) OutboxGaugeSnapshot(ctx context.Context, now time.Time) (pending int64, failed int64, oldestPendingAgeSeconds float64, err error) {
	if scanErr := r.pool.QueryRow(ctx, countFreightCostOutboxPendingQuery).Scan(&pending); scanErr != nil {
		return 0, 0, 0, mapDBError(scanErr)
	}
	if scanErr := r.pool.QueryRow(ctx, countFreightCostOutboxFailedQuery).Scan(&failed); scanErr != nil {
		return 0, 0, 0, mapDBError(scanErr)
	}
	if scanErr := r.pool.QueryRow(ctx, oldestPendingFreightCostOutboxAgeQuery, now).Scan(&oldestPendingAgeSeconds); scanErr != nil {
		return 0, 0, 0, mapDBError(scanErr)
	}
	return pending, failed, oldestPendingAgeSeconds, nil
}
