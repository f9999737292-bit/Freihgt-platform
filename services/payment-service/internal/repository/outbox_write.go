package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/payment-service/internal/domain"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func isPaymentOutboxDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	name := pgErr.ConstraintName
	return name == paymentOutboxLegacyPaidConstraint ||
		name == paymentOutboxPaidSnapshotConstraint ||
		strings.Contains(name, "payment_outbox")
}

func insertPaymentObligationPaidOutboxTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	obligation *domain.PaymentObligation,
) error {
	payload, err := domain.BuildObligationPaidOutboxPayload(tenantID, obligation.ID, obligation.SourceID)
	if err != nil {
		return mapDBError(err)
	}
	_, err = tx.Exec(ctx, insertPaymentOutboxEventQuery,
		uuid.New(),
		tenantID,
		domain.AggregatePaymentObligation,
		obligation.ID,
		int64(0),
		domain.PaymentEventObligationPaid,
		domain.PaymentOutboxSchemaVersion,
		payload,
		domain.PaymentOutboxStatusPending,
		0,
		time.Now().UTC(),
	)
	if err != nil {
		if isPaymentOutboxDuplicate(err) {
			return nil
		}
		return mapDBError(err)
	}
	return nil
}

func insertPaymentObligationPaidSnapshotOutboxTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	obligation *domain.PaymentObligation,
	occurredAt time.Time,
) error {
	eventID := uuid.New()
	payload, err := domain.BuildObligationPaidSnapshotOutboxPayload(eventID, obligation, occurredAt)
	if err != nil {
		return mapDBError(err)
	}
	_, err = tx.Exec(ctx, insertPaymentOutboxEventQuery,
		eventID,
		tenantID,
		domain.AggregatePaymentObligation,
		obligation.ID,
		int64(obligation.Version),
		domain.PaymentEventObligationPaidSnapshot,
		domain.PaymentPaidSnapshotSchemaVersion,
		payload,
		domain.PaymentOutboxStatusPending,
		0,
		time.Now().UTC(),
	)
	if err != nil {
		if isPaymentOutboxDuplicate(err) {
			return nil
		}
		return mapDBError(err)
	}
	return nil
}

func scanPaymentOutboxEvents(rows pgx.Rows) ([]domain.PaymentOutboxEvent, error) {
	defer rows.Close()
	items := make([]domain.PaymentOutboxEvent, 0)
	for rows.Next() {
		var event domain.PaymentOutboxEvent
		var status string
		err := rows.Scan(
			&event.ID,
			&event.TenantID,
			&event.AggregateType,
			&event.AggregateID,
			&event.AggregateVersion,
			&event.EventType,
			&event.SchemaVersion,
			&event.Payload,
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
		event.Status = domain.PaymentOutboxStatus(status)
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return items, nil
}

func (r *OutboxRepository) ClaimPendingForPublisher(
	ctx context.Context,
	workerID string,
	batchSize int,
	now time.Time,
	leaseTimeout time.Duration,
) ([]domain.PaymentOutboxEvent, error) {
	var result []domain.PaymentOutboxEvent
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	staleLockCutoff := now.Add(-leaseTimeout)
	rows, err := tx.Query(ctx, claimPendingPaymentOutboxQuery, now, staleLockCutoff, batchSize)
	if err != nil {
		return nil, mapDBError(err)
	}
	events, err := scanPaymentOutboxEvents(rows)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		tag, err := tx.Exec(ctx, lockClaimedPaymentOutboxQuery, now, workerID, event.ID)
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

func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	publishedAt time.Time,
) error {
	tag, err := r.pool.Exec(ctx, markPaymentOutboxPublishedQuery, publishedAt, eventID, workerID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOutboxPublishStateConflict
	}
	return nil
}

func (r *OutboxRepository) MarkPublishedByAggregate(
	ctx context.Context,
	tenantID uuid.UUID,
	eventType string,
	aggregateID uuid.UUID,
	publishedAt time.Time,
) error {
	_, err := r.pool.Exec(ctx, markPaymentOutboxPublishedByAggregateQuery, publishedAt, tenantID, eventType, aggregateID)
	return mapDBError(err)
}

func (r *OutboxRepository) ReleaseWithRetry(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	availableAt time.Time,
	errorCode string,
) error {
	tag, err := r.pool.Exec(ctx, releasePaymentOutboxWithRetryQuery, availableAt, errorCode, eventID, workerID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOutboxPublishStateConflict
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(
	ctx context.Context,
	eventID uuid.UUID,
	workerID string,
	errorCode string,
) error {
	tag, err := r.pool.Exec(ctx, markPaymentOutboxFailedQuery, errorCode, eventID, workerID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOutboxPublishStateConflict
	}
	return nil
}

func (r *OutboxRepository) OutboxGaugeSnapshot(ctx context.Context, now time.Time) (pending int64, failed int64, oldestPendingAgeSeconds float64, err error) {
	if scanErr := r.pool.QueryRow(ctx, countPaymentOutboxPendingQuery).Scan(&pending); scanErr != nil {
		return 0, 0, 0, mapDBError(scanErr)
	}
	if scanErr := r.pool.QueryRow(ctx, countPaymentOutboxFailedQuery).Scan(&failed); scanErr != nil {
		return 0, 0, 0, mapDBError(scanErr)
	}
	if scanErr := r.pool.QueryRow(ctx, oldestPendingPaymentOutboxAgeQuery, now).Scan(&oldestPendingAgeSeconds); scanErr != nil {
		return 0, 0, 0, mapDBError(scanErr)
	}
	return pending, failed, oldestPendingAgeSeconds, nil
}

func (r *OutboxRepository) CountOutboxByAggregate(
	ctx context.Context,
	tenantID uuid.UUID,
	eventType string,
	aggregateID uuid.UUID,
) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM billing.payment_outbox
		WHERE tenant_id = $1 AND event_type = $2 AND aggregate_id = $3`,
		tenantID, eventType, aggregateID,
	).Scan(&count)
	return count, mapDBError(err)
}

func (r *OutboxRepository) CountOutboxByAggregateVersion(
	ctx context.Context,
	tenantID uuid.UUID,
	eventType string,
	aggregateID uuid.UUID,
	aggregateVersion int64,
) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM billing.payment_outbox
		WHERE tenant_id = $1 AND event_type = $2 AND aggregate_id = $3 AND aggregate_version = $4`,
		tenantID, eventType, aggregateID, aggregateVersion,
	).Scan(&count)
	return count, mapDBError(err)
}

func (r *OutboxRepository) GetOutboxByAggregate(
	ctx context.Context,
	tenantID uuid.UUID,
	eventType string,
	aggregateID uuid.UUID,
) (*domain.PaymentOutboxEvent, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
			event_type, schema_version, payload,
			status, attempts, available_at, locked_at, locked_by,
			published_at, last_error_code, created_at
		FROM billing.payment_outbox
		WHERE tenant_id = $1 AND event_type = $2 AND aggregate_id = $3`,
		tenantID, eventType, aggregateID,
	)
	return scanPaymentOutboxEventRow(row)
}

func scanPaymentOutboxEventRow(row pgx.Row) (*domain.PaymentOutboxEvent, error) {
	var event domain.PaymentOutboxEvent
	var status string
	if err := row.Scan(
		&event.ID, &event.TenantID, &event.AggregateType, &event.AggregateID, &event.AggregateVersion,
		&event.EventType, &event.SchemaVersion, &event.Payload,
		&status, &event.Attempts, &event.AvailableAt, &event.LockedAt, &event.LockedBy,
		&event.PublishedAt, &event.LastErrorCode, &event.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	event.Status = domain.PaymentOutboxStatus(status)
	return &event, nil
}
