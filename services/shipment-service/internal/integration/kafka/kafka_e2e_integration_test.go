//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
	shipmentoutbox "github.com/freight-platform/shipment-service/internal/outbox"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestWorkerPostgreSQLRedpandaEndToEndIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 3)
	env := setupPGTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-KAFKA-E2E"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	var outboxStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM transport.shipment_event_outbox WHERE aggregate_id = $1`, shipment.ID).Scan(&outboxStatus); err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	if outboxStatus != "PENDING" {
		t.Fatalf("outbox status=%s want PENDING", outboxStatus)
	}

	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "kafka-e2e-worker", 1, now, time.Minute)
	if err != nil || len(events) != 1 {
		t.Fatalf("claim: %v len=%d", err, len(events))
	}

	group := "shipment-e2e-" + uuid.NewString()
	consumer := newKafkaConsumer(t, brokers, topic, group)
	publisher := newKafkaPublisher(t, brokers, topic)
	cfg := config.OutboxConfig{
		Enabled:        true,
		PollInterval:   time.Second,
		BatchSize:      1,
		PublishTimeout: 10 * time.Second,
		LeaseTimeout:   20 * time.Second,
		MaxAttempts:    5,
		WorkerID:       "kafka-e2e-worker",
	}
	worker := shipmentoutbox.NewWorker(cfg, env.repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fixedClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	if err := env.pool.QueryRow(ctx, `SELECT status FROM transport.shipment_event_outbox WHERE id = $1`, events[0].ID).Scan(&outboxStatus); err != nil {
		t.Fatalf("reload outbox: %v", err)
	}
	if outboxStatus != "PUBLISHED" {
		t.Fatalf("outbox status=%s want PUBLISHED", outboxStatus)
	}

	record := consumeRecordByEventID(t, consumer, events[0], 20*time.Second)
	if events[0].AggregateID != shipment.ID {
		t.Fatalf("aggregate ID mismatch")
	}
	t.Logf("topic=%s partition=%d offset=%d event_id=%s source_event_id=%s version=%d shipment_id=%s",
		topic, record.Partition, record.Offset, events[0].ID, events[0].SourceEventID, events[0].AggregateVersion, shipment.ID)
}

func TestDuplicateDeliveryAtLeastOnceIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 3)
	env := setupPGTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-DUP-E2E"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "kafka-dup-worker", 1, now, time.Minute)
	if err != nil || len(events) != 1 {
		t.Fatalf("claim: %v", err)
	}

	group := "shipment-dup-" + uuid.NewString()
	consumer := newKafkaConsumer(t, brokers, topic, group)
	publisher := newKafkaPublisher(t, brokers, topic)
	repo := &markPublishedFailOnceRepo{ShipmentRepository: env.repo}
	cfg := config.OutboxConfig{
		Enabled:        true,
		PollInterval:   time.Second,
		BatchSize:      1,
		PublishTimeout: 10 * time.Second,
		LeaseTimeout:   20 * time.Second,
		MaxAttempts:    5,
		WorkerID:       "kafka-dup-worker",
	}
	worker := shipmentoutbox.NewWorker(cfg, repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fixedClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	_, _ = env.pool.Exec(ctx, `UPDATE transport.shipment_event_outbox SET locked_at = NULL, locked_by = NULL, available_at = NOW() - INTERVAL '1 second', status = 'PENDING' WHERE id = $1`, events[0].ID)
	events, err = env.repo.ClaimPendingForPublisher(ctx, "kafka-dup-worker", 1, claimNow(), time.Minute)
	if err != nil || len(events) != 1 {
		t.Fatalf("reclaim: %v len=%d", err, len(events))
	}
	worker.ProcessEventForIntegration(ctx, events[0])

	expectedEventID := events[0].ID.String()
	expectedSourceEventID := events[0].SourceEventID.String()
	duplicates := consumeDistinctRecordsByEventID(t, consumer, expectedEventID, 2, 30*time.Second)
	first, second := duplicates[0], duplicates[1]
	if first.Offset == second.Offset {
		t.Fatal("expected two distinct kafka messages")
	}
	var env1, env2 domain.ShipmentStatusEventEnvelope
	if err := json.Unmarshal(first.Value, &env1); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if err := json.Unmarshal(second.Value, &env2); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	if env1.EventID != expectedEventID || env2.EventID != expectedEventID {
		t.Fatalf("duplicate messages must share eventId")
	}
	if env1.SourceEventID != expectedSourceEventID || env2.SourceEventID != expectedSourceEventID {
		t.Fatalf("duplicate messages must share sourceEventId")
	}
	if string(first.Key) != string(second.Key) {
		t.Fatalf("duplicate messages must share shipment key")
	}

	var finalStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM transport.shipment_event_outbox WHERE id = $1`, events[0].ID).Scan(&finalStatus); err != nil {
		t.Fatalf("final status: %v", err)
	}
	if finalStatus != "PUBLISHED" {
		t.Fatalf("final outbox status=%s want PUBLISHED", finalStatus)
	}
}

func TestWorkerBrokerUnavailableKeepsPendingIntegration(t *testing.T) {
	env := setupPGTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()
	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-BROKER-DOWN"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "kafka-down-worker", 1, now, time.Minute)
	if err != nil || len(events) != 1 {
		t.Fatalf("claim: %v", err)
	}

	publisher, err := shipmentoutbox.NewKafkaPublisher(config.KafkaConfig{
		Brokers:      []string{"127.0.0.1:59999"},
		Topic:        "shipment.status.v1.test.unavailable",
		ClientID:     "shipment-service-it",
		DialTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	}, shipmentoutbox.NewRealClock())
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer publisher.Close(context.Background())

	cfg := config.OutboxConfig{
		Enabled:        true,
		PublishTimeout: 2 * time.Second,
		LeaseTimeout:   10 * time.Second,
		MaxAttempts:    5,
		WorkerID:       "kafka-down-worker",
	}
	worker := shipmentoutbox.NewWorker(cfg, env.repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fixedClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	var status string
	var availableAt time.Time
	var lastErrorCode *string
	if err := env.pool.QueryRow(ctx, `SELECT status, available_at, last_error_code FROM transport.shipment_event_outbox WHERE id = $1`, events[0].ID).
		Scan(&status, &availableAt, &lastErrorCode); err != nil {
		t.Fatalf("load: %v", err)
	}
	if status != "PENDING" {
		t.Fatalf("status=%s want PENDING", status)
	}
	if !availableAt.After(now) {
		t.Fatalf("available_at not rescheduled: %s", availableAt)
	}
	if lastErrorCode == nil || *lastErrorCode == "" {
		t.Fatal("expected safe last_error_code")
	}
	switch *lastErrorCode {
	case shipmentoutbox.ErrorCodeTransientTimeout,
		shipmentoutbox.ErrorCodeBrokerUnavailable,
		shipmentoutbox.ErrorCodeTransientNetwork,
		shipmentoutbox.ErrorCodeUnknownPublishError:
	default:
		t.Fatalf("unexpected last_error_code %q", *lastErrorCode)
	}
}

type fixedClock struct{ now time.Time }

func (f *fixedClock) Now() time.Time { return f.now }

type markPublishedFailOnceRepo struct {
	*repository.ShipmentRepository
	failedOnce bool
}

func (r *markPublishedFailOnceRepo) ClaimPendingForPublisher(ctx context.Context, workerID string, batchSize int, now time.Time, leaseTimeout time.Duration) ([]domain.ShipmentOutboxEvent, error) {
	return r.ShipmentRepository.ClaimPendingForPublisher(ctx, workerID, batchSize, now, leaseTimeout)
}

func (r *markPublishedFailOnceRepo) MarkPublished(ctx context.Context, eventID uuid.UUID, workerID string, publishedAt time.Time) error {
	if !r.failedOnce {
		r.failedOnce = true
		return errors.New("simulated mark published failure")
	}
	return r.ShipmentRepository.MarkPublished(ctx, eventID, workerID, publishedAt)
}

func (r *markPublishedFailOnceRepo) ReleaseWithRetry(ctx context.Context, eventID uuid.UUID, workerID string, availableAt time.Time, errorCode string) error {
	return r.ShipmentRepository.ReleaseWithRetry(ctx, eventID, workerID, availableAt, errorCode)
}

func (r *markPublishedFailOnceRepo) MarkFailed(ctx context.Context, eventID uuid.UUID, workerID string, errorCode string) error {
	return r.ShipmentRepository.MarkFailed(ctx, eventID, workerID, errorCode)
}

func (r *markPublishedFailOnceRepo) OutboxGaugeSnapshot(ctx context.Context, now time.Time) (int64, int64, float64, error) {
	return r.ShipmentRepository.OutboxGaugeSnapshot(ctx, now)
}
