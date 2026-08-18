package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/projection"
	"github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
)

type ProjectionRepository struct {
	pool *pgxpool.Pool
}

func NewProjectionRepository(pool *pgxpool.Pool) *ProjectionRepository {
	return &ProjectionRepository{pool: pool}
}

type ProcessInput struct {
	Event      domain.ShipmentStatusEvent
	Meta       domain.KafkaRecordMeta
	ReceivedAt time.Time
}

type ProcessResult struct {
	Outcome   string
	Duplicate bool
	Applied   bool
}

type DeadLetterInput struct {
	Meta          domain.KafkaRecordMeta
	PayloadSHA256 string
	ErrorCode     string
	EventID       *uuid.UUID
	SourceEventID *uuid.UUID
	TenantID      *uuid.UUID
	ShipmentID    *uuid.UUID
	SchemaVersion *int
	ReceivedAt    time.Time
}

func (r *ProjectionRepository) ProcessEvent(ctx context.Context, input ProcessInput) (ProcessResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ProcessResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := rebuild.AcquireProjectionSharedLock(ctx, tx); err != nil {
		return ProcessResult{}, err
	}

	dupOutcome, dup, err := findDuplicateInbox(ctx, tx, input)
	if err != nil {
		return ProcessResult{}, err
	}
	if dup {
		return ProcessResult{Outcome: dupOutcome, Duplicate: true}, tx.Commit(ctx)
	}

	existing, err := lockProjection(ctx, tx, input.Event.TenantID, input.Event.Aggregate.ID)
	if err != nil {
		return ProcessResult{}, err
	}

	now := input.ReceivedAt.UTC()
	applyResult := projection.ApplyEvent(projection.ApplyInput{
		Event:    input.Event,
		Existing: existing,
		Now:      now,
	})

	if err := insertInbox(ctx, tx, input, applyResult.Outcome, now); err != nil {
		return ProcessResult{}, err
	}

	if applyResult.Updated {
		if err := upsertProjection(ctx, tx, applyResult.Projection); err != nil {
			return ProcessResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{
		Outcome: applyResult.Outcome,
		Applied: applyResult.Updated,
	}, nil
}

func findDuplicateInbox(ctx context.Context, tx pgx.Tx, input ProcessInput) (string, bool, error) {
	const q = `
SELECT outcome
FROM control_tower.shipment_status_event_inbox
WHERE event_id = $1
   OR source_event_id = $2
   OR (topic = $3 AND partition_id = $4 AND message_offset = $5)
LIMIT 1`
	var outcome string
	err := tx.QueryRow(ctx, q,
		input.Event.EventID,
		input.Event.SourceEventID,
		input.Meta.Topic,
		input.Meta.Partition,
		input.Meta.Offset,
	).Scan(&outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return outcome, true, nil
}

func lockProjection(ctx context.Context, tx pgx.Tx, tenantID, shipmentID uuid.UUID) (*domain.ShipmentStatusProjection, error) {
	const q = `
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at,
       planned_pickup_at, planned_delivery_at, actual_pickup_at, actual_delivery_at,
       complete, gap_detected,
       gap_from_version, gap_to_version, created_at, updated_at
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1 AND shipment_id = $2
FOR UPDATE`
	row := tx.QueryRow(ctx, q, tenantID, shipmentID)
	p, err := scanProjection(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func insertInbox(ctx context.Context, tx pgx.Tx, input ProcessInput, outcome string, processedAt time.Time) error {
	const q = `
INSERT INTO control_tower.shipment_status_event_inbox (
    event_id, source_event_id, tenant_id, shipment_id, aggregate_version,
    event_type, schema_version, topic, partition_id, message_offset, outcome,
    occurred_at, received_at, processed_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11,
    $12, $13, $14
)`
	_, err := tx.Exec(ctx, q,
		input.Event.EventID,
		input.Event.SourceEventID,
		input.Event.TenantID,
		input.Event.Aggregate.ID,
		input.Event.Aggregate.Version,
		input.Event.EventType,
		input.Event.SchemaVersion,
		input.Meta.Topic,
		input.Meta.Partition,
		input.Meta.Offset,
		outcome,
		input.Event.OccurredAt.UTC(),
		input.ReceivedAt.UTC(),
		processedAt,
	)
	return err
}

func upsertProjection(ctx context.Context, tx pgx.Tx, p domain.ShipmentStatusProjection) error {
	const q = `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at,
    planned_pickup_at, planned_delivery_at, actual_pickup_at, actual_delivery_at,
    complete, gap_detected,
    gap_from_version, gap_to_version, projection_source, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10,
    $11, $12, $13, $14,
    $15, $16,
    $17, $18, $19, $20, $21
)
ON CONFLICT (tenant_id, shipment_id) DO UPDATE SET
    shipment_version = EXCLUDED.shipment_version,
    current_status = EXCLUDED.current_status,
    previous_status = EXCLUDED.previous_status,
    last_event_id = EXCLUDED.last_event_id,
    last_source_event_id = EXCLUDED.last_source_event_id,
    last_event_type = EXCLUDED.last_event_type,
    last_occurred_at = EXCLUDED.last_occurred_at,
    last_consumed_at = EXCLUDED.last_consumed_at,
    planned_pickup_at = COALESCE(EXCLUDED.planned_pickup_at, control_tower.shipment_status_projection.planned_pickup_at),
    planned_delivery_at = COALESCE(EXCLUDED.planned_delivery_at, control_tower.shipment_status_projection.planned_delivery_at),
    actual_pickup_at = COALESCE(control_tower.shipment_status_projection.actual_pickup_at, EXCLUDED.actual_pickup_at),
    actual_delivery_at = COALESCE(control_tower.shipment_status_projection.actual_delivery_at, EXCLUDED.actual_delivery_at),
    complete = EXCLUDED.complete,
    gap_detected = EXCLUDED.gap_detected,
    gap_from_version = EXCLUDED.gap_from_version,
    gap_to_version = EXCLUDED.gap_to_version,
    projection_source = $19,
    snapshot_id = NULL,
    authoritative_as_of = NULL,
    updated_at = EXCLUDED.updated_at`
	_, err := tx.Exec(ctx, q,
		p.TenantID, p.ShipmentID, p.ShipmentVersion, p.CurrentStatus, p.PreviousStatus,
		p.LastEventID, p.LastSourceEventID, p.LastEventType,
		p.LastOccurredAt, p.LastConsumedAt,
		p.PlannedPickupAt, p.PlannedDeliveryAt, p.ActualPickupAt, p.ActualDeliveryAt,
		p.Complete, p.GapDetected,
		p.GapFromVersion, p.GapToVersion, rebuild.ProjectionSourceLiveEvent, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *ProjectionRepository) InsertDeadLetter(ctx context.Context, input DeadLetterInput) (bool, error) {
	const q = `
INSERT INTO control_tower.shipment_status_event_dead_letter (
    id, topic, partition_id, message_offset,
    event_id, source_event_id, tenant_id, shipment_id, schema_version,
    payload_sha256, error_code, received_at
) VALUES (
    gen_random_uuid(), $1, $2, $3,
    $4, $5, $6, $7, $8,
    $9, $10, $11
)
ON CONFLICT (topic, partition_id, message_offset) DO NOTHING
RETURNING id`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q,
		input.Meta.Topic,
		input.Meta.Partition,
		input.Meta.Offset,
		input.EventID,
		input.SourceEventID,
		input.TenantID,
		input.ShipmentID,
		input.SchemaVersion,
		input.PayloadSHA256,
		input.ErrorCode,
		input.ReceivedAt.UTC(),
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *ProjectionRepository) GetProjection(ctx context.Context, tenantID, shipmentID uuid.UUID) (*domain.ShipmentStatusProjection, error) {
	const q = `
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at,
       planned_pickup_at, planned_delivery_at, actual_pickup_at, actual_delivery_at,
       complete, gap_detected,
       gap_from_version, gap_to_version, created_at, updated_at
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1 AND shipment_id = $2`
	row := r.pool.QueryRow(ctx, q, tenantID, shipmentID)
	p, err := scanProjection(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type StatusSummary struct {
	TotalShipments            int64
	ByStatus                  map[string]int64
	IncompleteProjections     int64
	OldestProjectionUpdatedAt *time.Time
	LatestProjectionUpdatedAt *time.Time
}

func (r *ProjectionRepository) GetStatusSummary(ctx context.Context, tenantID uuid.UUID) (StatusSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return StatusSummary{}, err
	}
	defer tx.Rollback(ctx)

	summary := StatusSummary{ByStatus: map[string]int64{}}

	const totalQ = `SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id = $1`
	if err := tx.QueryRow(ctx, totalQ, tenantID).Scan(&summary.TotalShipments); err != nil {
		return StatusSummary{}, err
	}

	const byStatusQ = `
SELECT current_status, COUNT(*)
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1
GROUP BY current_status`
	rows, err := tx.Query(ctx, byStatusQ, tenantID)
	if err != nil {
		return StatusSummary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return StatusSummary{}, err
		}
		summary.ByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return StatusSummary{}, err
	}

	const incompleteQ = `
SELECT COUNT(*)
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1 AND complete = FALSE`
	if err := tx.QueryRow(ctx, incompleteQ, tenantID).Scan(&summary.IncompleteProjections); err != nil {
		return StatusSummary{}, err
	}

	const boundsQ = `
SELECT MIN(updated_at), MAX(updated_at)
FROM control_tower.shipment_status_projection
WHERE tenant_id = $1`
	var oldest, latest *time.Time
	if err := tx.QueryRow(ctx, boundsQ, tenantID).Scan(&oldest, &latest); err != nil {
		return StatusSummary{}, err
	}
	summary.OldestProjectionUpdatedAt = oldest
	summary.LatestProjectionUpdatedAt = latest

	if err := tx.Commit(ctx); err != nil {
		return StatusSummary{}, err
	}
	return summary, nil
}

type ListFilter struct {
	TenantID uuid.UUID
	Status   string
	Limit    int
	Cursor   *ListCursor
}

type ListCursor struct {
	UpdatedAt  time.Time
	ShipmentID uuid.UUID
}

type ListItem struct {
	Projection domain.ShipmentStatusProjection
}

func (r *ProjectionRepository) ListProjections(ctx context.Context, filter ListFilter) ([]ListItem, *ListCursor, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	args := []any{filter.TenantID}
	where := "tenant_id = $1"
	argN := 2
	if filter.Status != "" {
		where += fmt.Sprintf(" AND current_status = $%d", argN)
		args = append(args, filter.Status)
		argN++
	}
	if filter.Cursor != nil {
		where += fmt.Sprintf(" AND (updated_at, shipment_id) < ($%d, $%d)", argN, argN+1)
		args = append(args, filter.Cursor.UpdatedAt.UTC(), filter.Cursor.ShipmentID)
		argN += 2
	}
	query := fmt.Sprintf(`
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at,
       planned_pickup_at, planned_delivery_at, actual_pickup_at, actual_delivery_at,
       complete, gap_detected,
       gap_from_version, gap_to_version, created_at, updated_at
FROM control_tower.shipment_status_projection
WHERE %s
ORDER BY updated_at DESC, shipment_id DESC
LIMIT %d`, where, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		p, err := scanProjection(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, ListItem{Projection: p})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var next *ListCursor
	if len(items) > limit {
		last := items[limit-1].Projection
		next = &ListCursor{UpdatedAt: last.UpdatedAt, ShipmentID: last.ShipmentID}
		items = items[:limit]
	}
	return items, next, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanProjection(row scannable) (domain.ShipmentStatusProjection, error) {
	var p domain.ShipmentStatusProjection
	err := row.Scan(
		&p.TenantID, &p.ShipmentID, &p.ShipmentVersion, &p.CurrentStatus, &p.PreviousStatus,
		&p.LastEventID, &p.LastSourceEventID, &p.LastEventType,
		&p.LastOccurredAt, &p.LastConsumedAt,
		&p.PlannedPickupAt, &p.PlannedDeliveryAt, &p.ActualPickupAt, &p.ActualDeliveryAt,
		&p.Complete, &p.GapDetected,
		&p.GapFromVersion, &p.GapToVersion, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

// ProcessEventTx exposes transaction-level processing for concurrent tests.
func (r *ProjectionRepository) ProcessEventTx(ctx context.Context, tx pgx.Tx, input ProcessInput) (ProcessResult, error) {
	if err := rebuild.AcquireProjectionSharedLock(ctx, tx); err != nil {
		return ProcessResult{}, err
	}
	dupOutcome, dup, err := findDuplicateInbox(ctx, tx, input)
	if err != nil {
		return ProcessResult{}, err
	}
	if dup {
		return ProcessResult{Outcome: dupOutcome, Duplicate: true}, nil
	}
	existing, err := lockProjection(ctx, tx, input.Event.TenantID, input.Event.Aggregate.ID)
	if err != nil {
		return ProcessResult{}, err
	}
	now := input.ReceivedAt.UTC()
	applyResult := projection.ApplyEvent(projection.ApplyInput{
		Event:    input.Event,
		Existing: existing,
		Now:      now,
	})
	if err := insertInbox(ctx, tx, input, applyResult.Outcome, now); err != nil {
		return ProcessResult{}, err
	}
	if applyResult.Updated {
		if err := upsertProjection(ctx, tx, applyResult.Projection); err != nil {
			return ProcessResult{}, err
		}
	}
	return ProcessResult{Outcome: applyResult.Outcome, Applied: applyResult.Updated}, nil
}

func (r *ProjectionRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *ProjectionRepository) Pool() *pgxpool.Pool {
	return r.pool
}
