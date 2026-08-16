package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DriverEventRepository struct {
	pool *pgxpool.Pool
}

func NewDriverEventRepository(pool *pgxpool.Pool) *DriverEventRepository {
	return &DriverEventRepository{pool: pool}
}

type DriverEventProcessInput struct {
	TenantID          uuid.UUID
	EventID           uuid.UUID
	EventType         string
	ShipmentID        *uuid.UUID
	SourceEventID     *uuid.UUID
	KafkaTopic        string
	KafkaPartition    int32
	KafkaOffset       int64
	PayloadSHA256     string
	ProcessingOutcome string
	ReceivedAt        time.Time
}

type DriverEventProcessResult struct {
	Duplicate bool
	Inserted  bool
}

func (r *DriverEventRepository) ProcessEvent(ctx context.Context, input DriverEventProcessInput) (DriverEventProcessResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return DriverEventProcessResult{}, err
	}
	defer tx.Rollback(ctx)

	var existingOutcome string
	err = tx.QueryRow(ctx, `
SELECT processing_outcome FROM control_tower.driver_event_inbox
WHERE tenant_id = $1 AND event_id = $2`, input.TenantID, input.EventID).Scan(&existingOutcome)
	if err == nil {
		return DriverEventProcessResult{Duplicate: true}, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return DriverEventProcessResult{}, err
	}

	if input.ShipmentID != nil {
		var shipmentTenant uuid.UUID
		err = tx.QueryRow(ctx, `SELECT tenant_id FROM transport.shipments WHERE id = $1`, *input.ShipmentID).Scan(&shipmentTenant)
		if errors.Is(err, pgx.ErrNoRows) {
			return DriverEventProcessResult{Inserted: false}, tx.Commit(ctx)
		}
		if err != nil {
			return DriverEventProcessResult{}, err
		}
		if shipmentTenant != input.TenantID {
			return DriverEventProcessResult{}, errTenantMismatch
		}
	}

	_, err = tx.Exec(ctx, `
INSERT INTO control_tower.driver_event_inbox (
	tenant_id, event_id, event_type, shipment_id, source_event_id,
	kafka_topic, kafka_partition, kafka_offset, payload_sha256, processing_outcome, processed_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		input.TenantID, input.EventID, input.EventType, input.ShipmentID, input.SourceEventID,
		input.KafkaTopic, input.KafkaPartition, input.KafkaOffset, input.PayloadSHA256,
		input.ProcessingOutcome, input.ReceivedAt.UTC(),
	)
	if err != nil {
		return DriverEventProcessResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DriverEventProcessResult{}, err
	}
	return DriverEventProcessResult{Inserted: true}, nil
}

var errTenantMismatch = errors.New("tenant mismatch for shipment")

func IsTenantMismatch(err error) bool {
	return errors.Is(err, errTenantMismatch)
}
