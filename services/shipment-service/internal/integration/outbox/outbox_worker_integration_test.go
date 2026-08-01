//go:build integration

package outbox

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
	shipmentoutbox "github.com/freight-platform/shipment-service/internal/outbox"
)

func TestWorkerIntegrationWithPostgreSQL(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-WORKER-1"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-int", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: %v len=%d", err, len(events))
	}

	cfg := testOutboxConfig("worker-int")
	worker := shipmentoutbox.NewWorker(cfg, env.repo, successPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM transport.shipment_event_outbox WHERE id = $1`, events[0].ID).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "PUBLISHED" {
		t.Fatalf("status=%s want PUBLISHED", status)
	}
}

func TestWorkerTransientErrorKeepsPending(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-WORKER-2"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-int", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: %v", err)
	}

	cfg := testOutboxConfig("worker-int")
	worker := shipmentoutbox.NewWorker(cfg, env.repo, errPublisher{err: &shipmentoutbox.PublishError{Code: shipmentoutbox.ErrorCodeTransientNetwork, Retryable: true}}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	var status string
	var availableAt time.Time
	if err := env.pool.QueryRow(ctx, `SELECT status, available_at FROM transport.shipment_event_outbox WHERE id = $1`, events[0].ID).Scan(&status, &availableAt); err != nil {
		t.Fatalf("load: %v", err)
	}
	if status != "PENDING" || !availableAt.After(now) {
		t.Fatalf("status=%s availableAt=%s", status, availableAt)
	}
}

func TestWorkerPermanentErrorMarksFailed(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-WORKER-3"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-int", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: %v", err)
	}

	cfg := testOutboxConfig("worker-int")
	worker := shipmentoutbox.NewWorker(cfg, env.repo, errPublisher{err: &shipmentoutbox.PublishError{Code: shipmentoutbox.ErrorCodePayloadRejected, Retryable: false}}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM transport.shipment_event_outbox WHERE id = $1`, events[0].ID).Scan(&status); err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "FAILED" {
		t.Fatalf("status=%s", status)
	}
}

func TestWorkerLogsDoNotContainPayload(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	_, err := env.repo.CreateShipment(ctx, repositoryCreateParams(fix, "SHP-WORKER-LOG"), userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	now := claimNow()
	events, err := env.repo.ClaimPendingForPublisher(ctx, "worker-int", 1, now, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("claim: %v", err)
	}

	var buf bytes.Buffer
	cfg := testOutboxConfig("worker-int")
	worker := shipmentoutbox.NewWorker(cfg, env.repo, successPublisher{}, slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})), &fakeClock{now: now})
	worker.ProcessEventForIntegration(ctx, events[0])

	logText := strings.ToLower(buf.String())
	for _, forbidden := range []string{"payload", "authorization", "jwt", "email", "phone", "password"} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("log contains forbidden %s", forbidden)
		}
	}
}

func TestStartupDisabledAndEnabledWithoutTransport(t *testing.T) {
	t.Parallel()
	if _, err := shipmentoutbox.NewPublisher(config.OutboxConfig{Enabled: false}); err != nil {
		t.Fatalf("disabled publisher: %v", err)
	}
	if _, err := shipmentoutbox.NewPublisher(config.OutboxConfig{Enabled: true, Transport: ""}); err == nil {
		t.Fatal("enabled without transport must fail")
	}
	if _, err := shipmentoutbox.NewPublisher(config.OutboxConfig{
		Enabled:   true,
		Transport: "kafka",
		Kafka: config.KafkaConfig{
			Brokers:      []string{"localhost:19092"},
			Topic:        "shipment.status.v1",
			ClientID:     "shipment-service-it",
			DialTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
	}); err != nil {
		t.Fatalf("kafka publisher should start with valid config: %v", err)
	}
	if _, err := shipmentoutbox.NewPublisher(config.OutboxConfig{Enabled: true, Transport: "nats"}); err == nil {
		t.Fatal("unsupported transport must fail")
	}
}

func testOutboxConfig(workerID string) config.OutboxConfig {
	return config.OutboxConfig{
		Enabled:        true,
		PollInterval:   time.Second,
		BatchSize:      10,
		PublishTimeout: 5 * time.Second,
		LeaseTimeout:   10 * time.Second,
		MaxAttempts:    5,
		WorkerID:       workerID,
	}
}

type successPublisher struct{}

func (successPublisher) Publish(ctx context.Context, event domain.ShipmentOutboxEvent) error {
	return nil
}

type errPublisher struct{ err error }

func (p errPublisher) Publish(context.Context, domain.ShipmentOutboxEvent) error {
	return p.err
}

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }
