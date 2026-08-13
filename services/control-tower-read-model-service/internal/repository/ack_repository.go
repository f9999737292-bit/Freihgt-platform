package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type AckRepository struct {
	pool *pgxpool.Pool
}

func NewAckRepository(pool *pgxpool.Pool) *AckRepository {
	return &AckRepository{pool: pool}
}

func (r *AckRepository) UpsertAcknowledgement(
	ctx context.Context,
	input domain.AcknowledgeCriticalEventInput,
) (domain.CriticalEventAcknowledgement, error) {
	source := input.Source
	if source == "" {
		source = "control-tower"
	}

	const insertSQL = `
INSERT INTO control_tower.critical_event_acknowledgement (
    tenant_id,
    event_id,
    shipment_id,
    event_type,
    source,
    occurred_at,
    acknowledged_at,
    acknowledged_by_user_id
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW(), $7
)
ON CONFLICT (tenant_id, event_id) DO NOTHING`

	if _, err := r.pool.Exec(ctx, insertSQL,
		input.TenantID,
		input.EventID,
		input.ShipmentID,
		input.EventType,
		source,
		input.OccurredAt.UTC(),
		input.UserID,
	); err != nil {
		return domain.CriticalEventAcknowledgement{}, fmt.Errorf("insert acknowledgement: %w", err)
	}

	row, err := r.loadAcknowledgement(ctx, input.TenantID, input.EventID)
	if err != nil {
		return domain.CriticalEventAcknowledgement{}, err
	}
	if row == nil {
		return domain.CriticalEventAcknowledgement{}, fmt.Errorf("acknowledgement missing after upsert")
	}
	return *row, nil
}

func (r *AckRepository) LookupAcknowledgements(
	ctx context.Context,
	tenantID uuid.UUID,
	eventIDs []string,
) ([]domain.CriticalEventAcknowledgement, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}

	const lookupSQL = `
SELECT
    tenant_id,
    event_id,
    shipment_id,
    event_type,
    source,
    occurred_at,
    acknowledged_at,
    acknowledged_by_user_id
FROM control_tower.critical_event_acknowledgement
WHERE tenant_id = $1
  AND event_id = ANY($2::text[])`

	rows, err := r.pool.Query(ctx, lookupSQL, tenantID, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("lookup acknowledgements: %w", err)
	}
	defer rows.Close()

	items := make([]domain.CriticalEventAcknowledgement, 0)
	for rows.Next() {
		item, err := scanAcknowledgement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AckRepository) loadAcknowledgement(
	ctx context.Context,
	tenantID uuid.UUID,
	eventID string,
) (*domain.CriticalEventAcknowledgement, error) {
	const selectSQL = `
SELECT
    tenant_id,
    event_id,
    shipment_id,
    event_type,
    source,
    occurred_at,
    acknowledged_at,
    acknowledged_by_user_id
FROM control_tower.critical_event_acknowledgement
WHERE tenant_id = $1
  AND event_id = $2`

	row := r.pool.QueryRow(ctx, selectSQL, tenantID, eventID)
	item, err := scanAcknowledgement(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

type acknowledgementScanner interface {
	Scan(dest ...any) error
}

func scanAcknowledgement(row acknowledgementScanner) (domain.CriticalEventAcknowledgement, error) {
	var (
		item      domain.CriticalEventAcknowledgement
		occurred  time.Time
		ackedAt   time.Time
	)
	if err := row.Scan(
		&item.TenantID,
		&item.EventID,
		&item.ShipmentID,
		&item.EventType,
		&item.Source,
		&occurred,
		&ackedAt,
		&item.AcknowledgedByUserID,
	); err != nil {
		return domain.CriticalEventAcknowledgement{}, err
	}
	item.OccurredAt = occurred.UTC()
	item.AcknowledgedAt = ackedAt.UTC()
	return item, nil
}
