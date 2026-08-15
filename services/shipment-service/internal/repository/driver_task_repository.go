package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type DriverTaskRepository struct {
	pool *pgxpool.Pool
}

func NewDriverTaskRepository(pool *pgxpool.Pool) *DriverTaskRepository {
	return &DriverTaskRepository{pool: pool}
}

func HashPushToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

type CreateDriverTaskParams struct {
	Task            domain.DriverTask
	ShipmentVersion int
}

func (r *DriverTaskRepository) CreateTask(ctx context.Context, params CreateDriverTaskParams) (*domain.DriverTask, uuid.UUID, error) {
	return r.createTaskOnce(ctx, params)
}

func (r *DriverTaskRepository) createTaskOnce(ctx context.Context, params CreateDriverTaskParams) (*domain.DriverTask, uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	const insert = `
INSERT INTO transport.driver_task (
	tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	available_at, expires_at, created_by_type, created_by_id, source,
	correlation_id, source_event_id, idempotency_key
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
ON CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL
DO UPDATE SET id = transport.driver_task.id
RETURNING id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version,
	(xmax = 0) AS was_inserted`

	task, inserted, err := scanTaskInsertWithFlag(tx, ctx, insert, params.Task)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if !inserted {
		return task, uuid.Nil, nil
	}

	outboxID, err := insertTaskOutbox(ctx, tx, *task, domain.OutboxEventTypeDriverTaskCreated, params.ShipmentVersion, nil, task.ID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	const notif = `
INSERT INTO transport.driver_notification_delivery
	(tenant_id, driver_id, task_id, channel, status, provider, max_attempts, next_attempt_at)
VALUES ($1,$2,$3,'PUSH','pending','FCM',3,now())
ON CONFLICT (tenant_id, task_id, channel) DO NOTHING`
	if _, err := tx.Exec(ctx, notif, task.TenantID, task.DriverID, task.ID); err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	committed = true
	return task, outboxID, nil
}

func (r *DriverTaskRepository) findExistingTask(ctx context.Context, q pgxQuerier, in domain.DriverTask) (*domain.DriverTask, error) {
	if in.IdempotencyKey != nil && strings.TrimSpace(*in.IdempotencyKey) != "" {
		const qry = `
SELECT id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version
FROM transport.driver_task WHERE tenant_id=$1 AND idempotency_key=$2`
		task, err := scanTaskRow(q.QueryRow(ctx, qry, in.TenantID, *in.IdempotencyKey))
		if err == nil {
			return task, nil
		}
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	if in.SourceEventID != nil && in.ShipmentID != nil {
		const qry = `
SELECT id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version
FROM transport.driver_task
WHERE tenant_id=$1 AND source=$2 AND source_event_id=$3 AND task_type=$4 AND driver_id=$5 AND shipment_id=$6`
		task, err := scanTaskRow(q.QueryRow(ctx, qry, in.TenantID, in.Source, *in.SourceEventID, in.TaskType, in.DriverID, *in.ShipmentID))
		if err == nil {
			return task, nil
		}
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return nil, apperrors.Conflict("task already exists but could not be resolved", nil)
}

func isNotFoundError(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

func (r *DriverTaskRepository) GetResponseByIdempotency(ctx context.Context, tenantID, taskID uuid.UUID, key string) (*domain.DriverTaskResponse, error) {
	const q = `
SELECT id, tenant_id, task_id, driver_id, response_type, response_body, occurred_at, received_at, idempotency_key, created_at
FROM transport.driver_task_response WHERE tenant_id=$1 AND task_id=$2 AND idempotency_key=$3`
	var resp domain.DriverTaskResponse
	err := r.pool.QueryRow(ctx, q, tenantID, taskID, key).Scan(
		&resp.ID, &resp.TenantID, &resp.TaskID, &resp.DriverID, &resp.ResponseType, &resp.ResponseBody,
		&resp.OccurredAt, &resp.ReceivedAt, &resp.IdempotencyKey, &resp.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &resp, nil
}

func (r *DriverTaskRepository) GetTaskByID(ctx context.Context, tenantID, taskID uuid.UUID) (*domain.DriverTask, error) {
	const q = `
SELECT id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version
FROM transport.driver_task WHERE tenant_id=$1 AND id=$2`
	return scanTaskRow(r.pool.QueryRow(ctx, q, tenantID, taskID))
}

func (r *DriverTaskRepository) GetTaskByIDAndDriver(ctx context.Context, tenantID, driverID, taskID uuid.UUID) (*domain.DriverTask, error) {
	task, err := r.GetTaskByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, err
	}
	if task.DriverID != driverID {
		return nil, apperrors.NotFound("task not found")
	}
	return task, nil
}

func (r *DriverTaskRepository) GetTaskByIdempotency(ctx context.Context, tenantID uuid.UUID, key string) (*domain.DriverTask, error) {
	const q = `
SELECT id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version
FROM transport.driver_task WHERE tenant_id=$1 AND idempotency_key=$2`
	return scanTaskRow(r.pool.QueryRow(ctx, q, tenantID, key))
}

func (r *DriverTaskRepository) ListTasks(ctx context.Context, filter domain.ListDriverTasksFilter) ([]domain.DriverTask, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	args := []any{filter.TenantID, filter.DriverID}
	where := "tenant_id=$1 AND driver_id=$2"
	if filter.Unread {
		where += " AND status IN ('PENDING','DELIVERED')"
	} else if filter.Status != nil && strings.TrimSpace(*filter.Status) != "" {
		args = append(args, strings.TrimSpace(*filter.Status))
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	var total int
	countQ := "SELECT COUNT(*) FROM transport.driver_task WHERE " + where
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, mapDBError(err)
	}
	args = append(args, filter.Limit, filter.Offset)
	listQ := `
SELECT id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version
FROM transport.driver_task WHERE ` + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.DriverTask, 0)
	for rows.Next() {
		task, err := scanTaskRows(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *task)
	}
	return items, total, mapDBError(rows.Err())
}

type TransitionTaskParams struct {
	Task            domain.DriverTask
	NewStatus       string
	ShipmentVersion int
	SetReadAt       bool
	SetAckAt        bool
	SetDeliveredAt  bool
}

func (r *DriverTaskRepository) TransitionTask(ctx context.Context, params TransitionTaskParams) (*domain.DriverTask, error) {
	now := time.Now().UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	setClauses := "status=$3, version=version+1"
	args := []any{params.Task.TenantID, params.Task.ID, params.NewStatus}
	idx := 4
	if params.SetDeliveredAt {
		setClauses += fmt.Sprintf(", delivered_at=COALESCE(delivered_at,$%d)", idx)
		args = append(args, now)
		idx++
	}
	if params.SetReadAt {
		setClauses += fmt.Sprintf(", read_at=COALESCE(read_at,$%d)", idx)
		args = append(args, now)
		idx++
	}
	if params.SetAckAt {
		setClauses += fmt.Sprintf(", acknowledged_at=COALESCE(acknowledged_at,$%d)", idx)
		args = append(args, now)
		idx++
	}
	args = append(args, params.Task.Status, params.Task.Version)
	q := fmt.Sprintf(`
UPDATE transport.driver_task SET %s
WHERE tenant_id=$1 AND id=$2 AND status=$%d AND version=$%d
RETURNING id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version`, setClauses, idx, idx+1)
	task, err := scanTaskRow(tx.QueryRow(ctx, q, args...))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	return task, nil
}

type CompleteTaskParams struct {
	Task            domain.DriverTask
	Response        domain.DriverTaskResponse
	ShipmentVersion int
}

func (r *DriverTaskRepository) CompleteTask(ctx context.Context, params CompleteTaskParams) (*domain.DriverTask, *domain.DriverTaskResponse, uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	const insertResp = `
INSERT INTO transport.driver_task_response
	(tenant_id, task_id, driver_id, response_type, response_body, occurred_at, received_at, idempotency_key)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant_id, task_id, idempotency_key) DO NOTHING
RETURNING id, tenant_id, task_id, driver_id, response_type, response_body, occurred_at, received_at, idempotency_key, created_at`
	var resp domain.DriverTaskResponse
	err = tx.QueryRow(ctx, insertResp,
		params.Response.TenantID, params.Response.TaskID, params.Response.DriverID,
		params.Response.ResponseType, params.Response.ResponseBody,
		params.Response.OccurredAt, params.Response.ReceivedAt, params.Response.IdempotencyKey,
	).Scan(&resp.ID, &resp.TenantID, &resp.TaskID, &resp.DriverID, &resp.ResponseType, &resp.ResponseBody,
		&resp.OccurredAt, &resp.ReceivedAt, &resp.IdempotencyKey, &resp.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		const existing = `
SELECT id, tenant_id, task_id, driver_id, response_type, response_body, occurred_at, received_at, idempotency_key, created_at
FROM transport.driver_task_response WHERE tenant_id=$1 AND task_id=$2 AND idempotency_key=$3`
		err = tx.QueryRow(ctx, existing, params.Response.TenantID, params.Response.TaskID, params.Response.IdempotencyKey).Scan(
			&resp.ID, &resp.TenantID, &resp.TaskID, &resp.DriverID, &resp.ResponseType, &resp.ResponseBody,
			&resp.OccurredAt, &resp.ReceivedAt, &resp.IdempotencyKey, &resp.CreatedAt,
		)
		if err != nil {
			return nil, nil, uuid.Nil, mapDBError(err)
		}
		task, err := scanTaskRow(tx.QueryRow(ctx, `
SELECT id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version
FROM transport.driver_task WHERE tenant_id=$1 AND id=$2`, params.Task.TenantID, params.Task.ID))
		return task, &resp, uuid.Nil, err
	}
	if err != nil {
		return nil, nil, uuid.Nil, mapDBError(err)
	}

	now := time.Now().UTC()
	const updateTask = `
UPDATE transport.driver_task
SET status='COMPLETED', completed_at=$3, version=version+1
WHERE tenant_id=$1 AND id=$2 AND status NOT IN ('COMPLETED','EXPIRED','CANCELLED') AND version=$4
RETURNING id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version`
	task, err := scanTaskRow(tx.QueryRow(ctx, updateTask, params.Task.TenantID, params.Task.ID, now, params.Task.Version))
	if err != nil {
		return nil, nil, uuid.Nil, err
	}

	outboxID, err := insertTaskOutbox(ctx, tx, *task, domain.OutboxEventTypeDriverTaskCompleted, params.ShipmentVersion, &resp, resp.ID)
	if err != nil {
		return nil, nil, uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, uuid.Nil, mapDBError(err)
	}
	return task, &resp, outboxID, nil
}

func (r *DriverTaskRepository) CancelTask(ctx context.Context, task domain.DriverTask, shipmentVersion int) (*domain.DriverTask, uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	const q = `
UPDATE transport.driver_task SET status='CANCELLED', cancelled_at=$3, version=version+1
WHERE tenant_id=$1 AND id=$2 AND status NOT IN ('COMPLETED','EXPIRED','CANCELLED') AND version=$4
RETURNING id, tenant_id, driver_id, shipment_id, task_type, status, priority, title, payload,
	created_at, available_at, expires_at, delivered_at, read_at, acknowledged_at, completed_at, cancelled_at,
	created_by_type, created_by_id, source, correlation_id, source_event_id, idempotency_key, version`
	updated, err := scanTaskRow(tx.QueryRow(ctx, q, task.TenantID, task.ID, now, task.Version))
	if err != nil {
		return nil, uuid.Nil, err
	}
	outboxID, err := insertTaskOutbox(ctx, tx, *updated, domain.OutboxEventTypeDriverTaskCancelled, shipmentVersion, nil, updated.ID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, uuid.Nil, mapDBError(err)
	}
	return updated, outboxID, nil
}

func (r *DriverTaskRepository) ExpireDueTasks(ctx context.Context, limit int) (int, error) {
	now := time.Now().UTC()
	const q = `
UPDATE transport.driver_task SET status='EXPIRED', version=version+1
WHERE id IN (
	SELECT id FROM transport.driver_task
	WHERE status IN ('PENDING','DELIVERED','READ','ACKNOWLEDGED')
	  AND expires_at IS NOT NULL AND expires_at <= $1
	ORDER BY expires_at ASC
	LIMIT $2
	FOR UPDATE SKIP LOCKED
) RETURNING id`
	rows, err := r.pool.Query(ctx, q, now, limit)
	if err != nil {
		return 0, mapDBError(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	return count, mapDBError(rows.Err())
}

func insertTaskOutbox(ctx context.Context, tx pgx.Tx, task domain.DriverTask, eventType string, shipmentVersion int, response *domain.DriverTaskResponse, sourceEventID uuid.UUID) (uuid.UUID, error) {
	payload, err := domain.BuildDriverTaskOutboxPayload(task, eventType, shipmentVersion, response)
	if err != nil {
		return uuid.Nil, err
	}
	outboxID := uuid.New()
	aggregateID := task.ID
	if task.ShipmentID != nil {
		aggregateID = *task.ShipmentID
	}
	headers, _ := json.Marshal(map[string]any{"source": task.Source})
	outbox := domain.ShipmentOutboxEvent{
		ID: outboxID, TenantID: task.TenantID,
		AggregateType: domain.OutboxAggregateTypeShipment, AggregateID: aggregateID,
		AggregateVersion: shipmentVersion, EventType: eventType,
		SchemaVersion: domain.OutboxSchemaVersion, SourceEventID: sourceEventID,
		Payload: payload, Headers: headers, Status: domain.OutboxStatusPending,
		Attempts: 0, AvailableAt: time.Now().UTC(),
	}
	return outboxID, insertOutboxRow(ctx, tx, outbox)
}

type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func scanTaskInsertWithFlag(q pgxQuerier, ctx context.Context, sql string, in domain.DriverTask) (*domain.DriverTask, bool, error) {
	var task domain.DriverTask
	var payload []byte
	var inserted bool
	err := q.QueryRow(ctx, sql,
		in.TenantID, in.DriverID, in.ShipmentID, in.TaskType, in.Status, in.Priority, in.Title, in.Payload,
		in.AvailableAt, in.ExpiresAt, in.CreatedByType, in.CreatedByID, in.Source,
		in.CorrelationID, in.SourceEventID, in.IdempotencyKey,
	).Scan(
		&task.ID, &task.TenantID, &task.DriverID, &task.ShipmentID, &task.TaskType, &task.Status, &task.Priority, &task.Title, &payload,
		&task.CreatedAt, &task.AvailableAt, &task.ExpiresAt, &task.DeliveredAt, &task.ReadAt, &task.AcknowledgedAt, &task.CompletedAt, &task.CancelledAt,
		&task.CreatedByType, &task.CreatedByID, &task.Source, &task.CorrelationID, &task.SourceEventID, &task.IdempotencyKey, &task.Version,
		&inserted,
	)
	if err != nil {
		return nil, false, mapDBError(err)
	}
	task.Payload = json.RawMessage(payload)
	return &task, inserted, nil
}

func scanTaskInsert(q pgxQuerier, ctx context.Context, sql string, in domain.DriverTask) (*domain.DriverTask, bool, error) {
	task, err := scanTaskRow(q.QueryRow(ctx, sql,
		in.TenantID, in.DriverID, in.ShipmentID, in.TaskType, in.Status, in.Priority, in.Title, in.Payload,
		in.AvailableAt, in.ExpiresAt, in.CreatedByType, in.CreatedByID, in.Source,
		in.CorrelationID, in.SourceEventID, in.IdempotencyKey,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return task, true, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTaskRow(row pgx.Row) (*domain.DriverTask, error) {
	var task domain.DriverTask
	var payload []byte
	err := row.Scan(
		&task.ID, &task.TenantID, &task.DriverID, &task.ShipmentID, &task.TaskType, &task.Status, &task.Priority, &task.Title, &payload,
		&task.CreatedAt, &task.AvailableAt, &task.ExpiresAt, &task.DeliveredAt, &task.ReadAt, &task.AcknowledgedAt, &task.CompletedAt, &task.CancelledAt,
		&task.CreatedByType, &task.CreatedByID, &task.Source, &task.CorrelationID, &task.SourceEventID, &task.IdempotencyKey, &task.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("task not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	task.Payload = json.RawMessage(payload)
	return &task, nil
}

func scanTaskRows(row taskScanner) (*domain.DriverTask, error) {
	var task domain.DriverTask
	var payload []byte
	err := row.Scan(
		&task.ID, &task.TenantID, &task.DriverID, &task.ShipmentID, &task.TaskType, &task.Status, &task.Priority, &task.Title, &payload,
		&task.CreatedAt, &task.AvailableAt, &task.ExpiresAt, &task.DeliveredAt, &task.ReadAt, &task.AcknowledgedAt, &task.CompletedAt, &task.CancelledAt,
		&task.CreatedByType, &task.CreatedByID, &task.Source, &task.CorrelationID, &task.SourceEventID, &task.IdempotencyKey, &task.Version,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	task.Payload = json.RawMessage(payload)
	return &task, nil
}
