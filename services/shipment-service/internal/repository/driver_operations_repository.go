package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type DriverOperationsRepository struct {
	pool *pgxpool.Pool
}

func NewDriverOperationsRepository(pool *pgxpool.Pool) *DriverOperationsRepository {
	return &DriverOperationsRepository{pool: pool}
}

func (r *DriverOperationsRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *DriverOperationsRepository) GetIdempotencyRecord(
	ctx context.Context,
	tenantID, driverID uuid.UUID,
	operationType, idempotencyKey string,
) (*domain.DriverOperationIdempotencyRecord, error) {
	const q = `
SELECT id, tenant_id, driver_id, operation_type, idempotency_key,
       resource_type, resource_id, response_status_code, response_body
FROM transport.driver_operation_idempotency
WHERE tenant_id = $1 AND driver_id = $2 AND operation_type = $3 AND idempotency_key = $4`
	var rec domain.DriverOperationIdempotencyRecord
	err := r.pool.QueryRow(ctx, q, tenantID, driverID, operationType, idempotencyKey).Scan(
		&rec.ID, &rec.TenantID, &rec.DriverID, &rec.OperationType, &rec.IdempotencyKey,
		&rec.ResourceType, &rec.ResourceID, &rec.ResponseStatusCode, &rec.ResponseBody,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &rec, nil
}

func (r *DriverOperationsRepository) SaveIdempotencyRecord(ctx context.Context, tx pgx.Tx, rec domain.DriverOperationIdempotencyRecord) error {
	const q = `
INSERT INTO transport.driver_operation_idempotency (
	tenant_id, driver_id, operation_type, idempotency_key,
	resource_type, resource_id, response_status_code, response_body
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant_id, driver_id, operation_type, idempotency_key) DO NOTHING`
	_, err := tx.Exec(ctx, q,
		rec.TenantID, rec.DriverID, rec.OperationType, rec.IdempotencyKey,
		rec.ResourceType, rec.ResourceID, rec.ResponseStatusCode, rec.ResponseBody,
	)
	return mapDBError(err)
}

type ReportDriverExceptionParams struct {
	Exception      domain.DriverReportedException
	ShipmentVersion int
	CorrelationID  *string
}

func (r *DriverOperationsRepository) ReportException(ctx context.Context, params ReportDriverExceptionParams) (*domain.DriverReportedException, uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	const insertException = `
INSERT INTO transport.driver_reported_exception (
	tenant_id, shipment_id, driver_id, category, comment,
	occurred_at, received_at, source, idempotency_key
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (tenant_id, driver_id, idempotency_key) DO NOTHING
RETURNING id, tenant_id, shipment_id, driver_id, category, comment,
	occurred_at, received_at, source, idempotency_key, created_at`

	var exc domain.DriverReportedException
	err = tx.QueryRow(ctx, insertException,
		params.Exception.TenantID,
		params.Exception.ShipmentID,
		params.Exception.DriverID,
		params.Exception.Category,
		optionalString(params.Exception.Comment),
		params.Exception.OccurredAt,
		params.Exception.ReceivedAt,
		params.Exception.Source,
		params.Exception.IdempotencyKey,
	).Scan(
		&exc.ID, &exc.TenantID, &exc.ShipmentID, &exc.DriverID, &exc.Category, &exc.Comment,
		&exc.OccurredAt, &exc.ReceivedAt, &exc.Source, &exc.IdempotencyKey, &exc.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := r.getExceptionByIdempotency(ctx, tx, params.Exception.TenantID, params.Exception.DriverID, params.Exception.IdempotencyKey)
		if lookupErr != nil {
			return nil, uuid.Nil, lookupErr
		}
		return existing, uuid.Nil, nil
	}
	if err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}

	payload, err := domain.BuildDriverExceptionOutboxPayload(exc, params.ShipmentVersion, params.CorrelationID)
	if err != nil {
		return nil, uuid.Nil, err
	}

	outboxID := uuid.New()
	headers, _ := json.Marshal(map[string]any{"source": domain.DriverExceptionSource})
	outbox := domain.ShipmentOutboxEvent{
		ID:               outboxID,
		TenantID:         exc.TenantID,
		AggregateType:    domain.OutboxAggregateTypeShipment,
		AggregateID:      exc.ShipmentID,
		AggregateVersion: params.ShipmentVersion,
		EventType:        domain.OutboxEventTypeDriverExceptionReported,
		SchemaVersion:    domain.OutboxSchemaVersion,
		SourceEventID:    exc.ID,
		Payload:          payload,
		Headers:          headers,
		Status:           domain.OutboxStatusPending,
		Attempts:         0,
		AvailableAt:      time.Now().UTC(),
	}
	if err := insertOutboxRow(ctx, tx, outbox); err != nil {
		return nil, uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	return &exc, outboxID, nil
}

func (r *DriverOperationsRepository) getExceptionByIdempotency(ctx context.Context, tx pgx.Tx, tenantID, driverID uuid.UUID, key string) (*domain.DriverReportedException, error) {
	const q = `
SELECT id, tenant_id, shipment_id, driver_id, category, comment,
	occurred_at, received_at, source, idempotency_key, created_at
FROM transport.driver_reported_exception
WHERE tenant_id = $1 AND driver_id = $2 AND idempotency_key = $3`
	var exc domain.DriverReportedException
	err := tx.QueryRow(ctx, q, tenantID, driverID, key).Scan(
		&exc.ID, &exc.TenantID, &exc.ShipmentID, &exc.DriverID, &exc.Category, &exc.Comment,
		&exc.OccurredAt, &exc.ReceivedAt, &exc.Source, &exc.IdempotencyKey, &exc.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.Conflict("exception idempotency conflict", nil)
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &exc, nil
}
