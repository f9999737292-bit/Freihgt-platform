package outbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertOutboxEventQuery = `
INSERT INTO transport.shipment_event_outbox (
	id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
	event_type, schema_version, source_event_id, payload, headers, status, attempts, available_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

type DriverEventParams struct {
	EventID         uuid.UUID
	EventType       string
	TenantID        uuid.UUID
	ShipmentID      uuid.UUID
	ShipmentVersion int
	DriverID        uuid.UUID
	SourceEventID   uuid.UUID
	OccurredAt      time.Time
	Payload         []byte
}

type Publisher struct {
	pool *pgxpool.Pool
}

func NewPublisher(pool *pgxpool.Pool) *Publisher {
	return &Publisher{pool: pool}
}

func (p *Publisher) InsertPending(ctx context.Context, params DriverEventParams) error {
	headers, _ := json.Marshal(map[string]string{
		"contentType": "application/json",
		"source":      "tracking-service",
		"eventType":   params.EventType,
	})
	_, err := p.pool.Exec(ctx, insertOutboxEventQuery,
		params.EventID,
		params.TenantID,
		"SHIPMENT",
		params.ShipmentID,
		params.ShipmentVersion,
		params.EventType,
		1,
		params.SourceEventID,
		params.Payload,
		headers,
		"PENDING",
		0,
		time.Now().UTC(),
	)
	return err
}
