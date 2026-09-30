package outbox

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/tracking-service/internal/repository"
)

type KafkaPublisher struct {
	client *kgo.Client
	topic  string
	pool   *pgxpool.Pool
	worker string
}

func NewKafkaPublisher(brokers []string, topic, clientID string, pool *pgxpool.Pool) (*KafkaPublisher, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
	)
	if err != nil {
		return nil, err
	}
	return &KafkaPublisher{client: client, topic: topic, pool: pool, worker: clientID}, nil
}

func (p *KafkaPublisher) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}

func (p *KafkaPublisher) PublishRecord(ctx context.Context, record BusRecord) error {
	headers := make([]kgo.RecordHeader, 0, len(record.Headers))
	for _, header := range record.Headers {
		headers = append(headers, kgo.RecordHeader{Key: header.Key, Value: []byte(header.Value)})
	}
	return p.client.ProduceSync(ctx, &kgo.Record{
		Topic:   record.Topic,
		Key:     []byte(record.Key),
		Value:   record.Value,
		Headers: headers,
	}).FirstErr()
}

func (p *KafkaPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.publishBatch(ctx)
		}
	}
}

func (p *KafkaPublisher) publishBatch(ctx context.Context) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
UPDATE tracking.event_outbox
SET locked_at = now(), locked_by = $1, attempts = attempts + 1
WHERE id IN (
    SELECT id FROM tracking.event_outbox
    WHERE status = 'PENDING' AND available_at <= now()
      AND (locked_at IS NULL OR locked_at < now() - interval '30 seconds')
    ORDER BY created_at
    LIMIT 20
    FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, aggregate_type, aggregate_id, aggregate_version, event_type, schema_version,
          source_event_id, payload, headers`, p.worker)
	if err != nil {
		return err
	}
	var events []repository.TrackingOutboxEvent
	for rows.Next() {
		var event repository.TrackingOutboxEvent
		if err := rows.Scan(&event.ID, &event.TenantID, &event.AggregateType, &event.AggregateID, &event.AggregateVersion,
			&event.EventType, &event.SchemaVersion, &event.SourceEventID, &event.Payload, &event.Headers); err != nil {
			rows.Close()
			return err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, event := range events {
		record, err := BuildBusRecord(p.topic, event)
		if err != nil {
			_, _ = tx.Exec(ctx, `UPDATE tracking.event_outbox SET status='FAILED', last_error_code=$2, locked_at=NULL WHERE id=$1`, event.ID, "payload_rejected")
			continue
		}
		if err := p.PublishRecord(ctx, record); err != nil {
			_, _ = tx.Exec(ctx, `UPDATE tracking.event_outbox SET locked_at=NULL, available_at=now() + interval '15 seconds', last_error_code=$2 WHERE id=$1`, event.ID, "publish_failed")
			continue
		}
		_, _ = tx.Exec(ctx, `UPDATE tracking.event_outbox SET status='PUBLISHED', published_at=now(), locked_at=NULL WHERE id=$1`, event.ID)
	}
	return tx.Commit(ctx)
}
