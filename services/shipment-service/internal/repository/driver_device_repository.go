package repository

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type DriverDeviceRepository struct {
	pool *pgxpool.Pool
}

func NewDriverDeviceRepository(pool *pgxpool.Pool) *DriverDeviceRepository {
	return &DriverDeviceRepository{pool: pool}
}

type RegisterDeviceInput struct {
	TenantID         uuid.UUID
	DriverID         uuid.UUID
	Platform         string
	PushToken        string
	DeviceInstanceID string
	AppVersion       *string
	Locale           *string
}

func encodePushToken(token string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(token)))
}

func decodePushToken(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (r *DriverDeviceRepository) RegisterDevice(ctx context.Context, in RegisterDeviceInput) (*domain.DriverDevice, error) {
	tokenHash := HashPushToken(in.PushToken)
	ciphertext := encodePushToken(in.PushToken)
	now := time.Now().UTC()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	_, _ = tx.Exec(ctx, `
UPDATE transport.driver_device SET revoked_at=$3, updated_at=$3
WHERE tenant_id=$1 AND push_token_hash=$2 AND revoked_at IS NULL AND driver_id<>$4`,
		in.TenantID, tokenHash, now, in.DriverID)

	const upsert = `
INSERT INTO transport.driver_device
	(tenant_id, driver_id, platform, push_provider, push_token_hash, push_token_ciphertext,
	 device_instance_id, app_version, locale, last_seen_at)
VALUES ($1,$2,$3,'FCM',$4,$5,$6,$7,$8,$9)
ON CONFLICT (tenant_id, driver_id, device_instance_id) WHERE revoked_at IS NULL
DO UPDATE SET push_token_hash=EXCLUDED.push_token_hash, push_token_ciphertext=EXCLUDED.push_token_ciphertext,
	platform=EXCLUDED.platform, app_version=EXCLUDED.app_version, locale=EXCLUDED.locale,
	last_seen_at=EXCLUDED.last_seen_at, updated_at=now()
RETURNING id, tenant_id, driver_id, platform, push_provider, push_token_hash, device_instance_id,
	app_version, locale, last_seen_at, created_at, updated_at, revoked_at`
	var dev domain.DriverDevice
	err = tx.QueryRow(ctx, upsert,
		in.TenantID, in.DriverID, strings.ToUpper(in.Platform), tokenHash, ciphertext, in.DeviceInstanceID,
		optionalString(in.AppVersion), optionalString(in.Locale), now,
	).Scan(
		&dev.ID, &dev.TenantID, &dev.DriverID, &dev.Platform, &dev.PushProvider, &dev.PushTokenHash,
		&dev.DeviceInstanceID, &dev.AppVersion, &dev.Locale, &dev.LastSeenAt, &dev.CreatedAt, &dev.UpdatedAt, &dev.RevokedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	return &dev, nil
}

func (r *DriverDeviceRepository) RevokeDevice(ctx context.Context, tenantID, driverID, deviceID uuid.UUID) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
UPDATE transport.driver_device SET revoked_at=$4, updated_at=$4
WHERE tenant_id=$1 AND driver_id=$2 AND id=$3 AND revoked_at IS NULL`,
		tenantID, driverID, deviceID, now)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("device not found")
	}
	return nil
}

func (r *DriverDeviceRepository) ListActiveDevices(ctx context.Context, tenantID, driverID uuid.UUID) ([]domain.DriverDevice, error) {
	const q = `
SELECT id, tenant_id, driver_id, platform, push_provider, push_token_hash, device_instance_id,
	app_version, locale, last_seen_at, created_at, updated_at, revoked_at
FROM transport.driver_device
WHERE tenant_id=$1 AND driver_id=$2 AND revoked_at IS NULL`
	rows, err := r.pool.Query(ctx, q, tenantID, driverID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.DriverDevice, 0)
	for rows.Next() {
		var dev domain.DriverDevice
		if err := rows.Scan(
			&dev.ID, &dev.TenantID, &dev.DriverID, &dev.Platform, &dev.PushProvider, &dev.PushTokenHash,
			&dev.DeviceInstanceID, &dev.AppVersion, &dev.Locale, &dev.LastSeenAt, &dev.CreatedAt, &dev.UpdatedAt, &dev.RevokedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, dev)
	}
	return items, mapDBError(rows.Err())
}

type DevicePushTarget struct {
	DeviceID uuid.UUID
	Token    string
	TokenHash string
}

func (r *DriverDeviceRepository) ListActivePushTargets(ctx context.Context, tenantID, driverID uuid.UUID) ([]DevicePushTarget, error) {
	const q = `
SELECT id, push_token_ciphertext, push_token_hash
FROM transport.driver_device
WHERE tenant_id=$1 AND driver_id=$2 AND revoked_at IS NULL`
	rows, err := r.pool.Query(ctx, q, tenantID, driverID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]DevicePushTarget, 0)
	for rows.Next() {
		var item DevicePushTarget
		var ciphertext string
		if err := rows.Scan(&item.DeviceID, &ciphertext, &item.TokenHash); err != nil {
			return nil, mapDBError(err)
		}
		token, err := decodePushToken(ciphertext)
		if err != nil {
			continue
		}
		item.Token = token
		items = append(items, item)
	}
	return items, mapDBError(rows.Err())
}

func (r *DriverDeviceRepository) RevokeByTokenHash(ctx context.Context, tenantID uuid.UUID, tokenHash string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE transport.driver_device SET revoked_at=$3, updated_at=$3
WHERE tenant_id=$1 AND push_token_hash=$2 AND revoked_at IS NULL`, tenantID, tokenHash, now)
	return mapDBError(err)
}

type NotificationDelivery struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	DriverID      uuid.UUID
	TaskID        uuid.UUID
	Channel       string
	Status        string
	Provider      *string
	AttemptCount  int
	MaxAttempts   int
	NextAttemptAt time.Time
	TaskType      string
	TaskTitle     string
}

func (r *DriverDeviceRepository) ClaimPendingDeliveries(ctx context.Context, workerID string, batchSize int, now time.Time, lease time.Duration) ([]NotificationDelivery, error) {
	const q = `
UPDATE transport.driver_notification_delivery d
SET status='processing', claimed_by=$1, claimed_until=$2, updated_at=now()
FROM transport.driver_task t
WHERE d.id IN (
	SELECT d2.id FROM transport.driver_notification_delivery d2
	WHERE d2.status IN ('pending','processing') AND d2.next_attempt_at <= $3
	  AND (d2.claimed_until IS NULL OR d2.claimed_until < $3)
	ORDER BY d2.next_attempt_at ASC
	LIMIT $4
	FOR UPDATE SKIP LOCKED
) AND d.task_id = t.id
RETURNING d.id, d.tenant_id, d.driver_id, d.task_id, d.channel, d.status, d.provider,
	d.attempt_count, d.max_attempts, d.next_attempt_at, t.task_type, t.title`
	until := now.Add(lease)
	rows, err := r.pool.Query(ctx, q, workerID, until, now, batchSize)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]NotificationDelivery, 0)
	for rows.Next() {
		var item NotificationDelivery
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.DriverID, &item.TaskID, &item.Channel, &item.Status,
			&item.Provider, &item.AttemptCount, &item.MaxAttempts, &item.NextAttemptAt, &item.TaskType, &item.TaskTitle,
		); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, item)
	}
	return items, mapDBError(rows.Err())
}

func (r *DriverDeviceRepository) MarkDeliverySent(ctx context.Context, deliveryID uuid.UUID, providerMessageID string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE transport.driver_notification_delivery
SET status='sent', attempt_count=attempt_count+1, provider_message_id=$2, sent_at=$3,
	claimed_by=NULL, claimed_until=NULL, updated_at=$3
WHERE id=$1`, deliveryID, providerMessageID, now)
	return mapDBError(err)
}

func (r *DriverDeviceRepository) MarkDeliveryNoDevice(ctx context.Context, deliveryID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE transport.driver_notification_delivery
SET status='no_device', attempt_count=attempt_count+1, error_code='NO_DEVICE',
	claimed_by=NULL, claimed_until=NULL, updated_at=$2
WHERE id=$1`, deliveryID, now)
	return mapDBError(err)
}

func (r *DriverDeviceRepository) MarkDeliveryFailed(ctx context.Context, deliveryID uuid.UUID, errorCode string, retryAt *time.Time, permanent bool) error {
	now := time.Now().UTC()
	status := "pending"
	if permanent {
		status = "failed"
	}
	nextAttempt := now
	if retryAt != nil && !permanent {
		nextAttempt = *retryAt
	}
	_, err := r.pool.Exec(ctx, `
UPDATE transport.driver_notification_delivery
SET status=$2, attempt_count=attempt_count+1, error_code=$3, next_attempt_at=$4,
	claimed_by=NULL, claimed_until=NULL, updated_at=$5
WHERE id=$1`, deliveryID, status, errorCode, nextAttempt, now)
	return mapDBError(err)
}

func (r *DriverDeviceRepository) MarkTaskDelivered(ctx context.Context, tenantID, taskID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE transport.driver_task
SET status=CASE WHEN status='PENDING' THEN 'DELIVERED' ELSE status END,
    delivered_at=COALESCE(delivered_at,$3), version=version+1
WHERE tenant_id=$1 AND id=$2 AND status IN ('PENDING','DELIVERED')`, tenantID, taskID, now)
	return mapDBError(err)
}

func (r *DriverDeviceRepository) ReleaseStaleClaims(ctx context.Context, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE transport.driver_notification_delivery
SET status='pending', claimed_by=NULL, claimed_until=NULL, updated_at=now()
WHERE status='processing' AND claimed_until IS NOT NULL AND claimed_until < $1`, now)
	return mapDBError(err)
}

func (r *DriverDeviceRepository) CountDeliveryAttempts(ctx context.Context, deliveryID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT attempt_count FROM transport.driver_notification_delivery WHERE id=$1`, deliveryID).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperrors.NotFound("delivery not found")
	}
	return count, mapDBError(err)
}
