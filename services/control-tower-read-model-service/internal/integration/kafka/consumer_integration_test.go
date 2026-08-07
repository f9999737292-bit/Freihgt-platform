//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	pgintegration "github.com/freight-platform/control-tower-read-model-service/internal/integration/postgres"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
)

func buildCreatedEventPayload(t *testing.T, tenantID, shipmentID, eventID, sourceEventID uuid.UUID) []byte {
	t.Helper()
	env := map[string]any{
		"eventId":       eventID.String(),
		"eventType":     domain.EventTypeCreated,
		"schemaVersion": domain.SchemaVersionV1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId":      tenantID.String(),
		"aggregate": map[string]any{
			"type":    domain.AggregateTypeShipment,
			"id":      shipmentID.String(),
			"version": 1,
		},
		"sourceEventId": sourceEventID.String(),
		"data": map[string]any{
			"toStatus":  domain.StatusCarrierAssigned,
			"actorType": "SYSTEM",
		},
	}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return raw
}

func TestConsumerReadsValidEventAndCommitsOffsetIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 1)
	env := pgintegration.SetupTestEnv(t)
	ctx := context.Background()

	tenantID := uuid.New()
	shipmentID := uuid.New()
	eventID := uuid.New()
	sourceEventID := uuid.New()
	payload := buildCreatedEventPayload(t, tenantID, shipmentID, eventID, sourceEventID)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	record := &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: payload}
	require.NoError(t, producer.ProduceSync(ctx, record).FirstErr())

	group := "control-tower-it-" + uuid.NewString()
	consClient, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("control-tower-read-model-service-it"),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	require.NoError(t, err)
	t.Cleanup(consClient.Close)

	cfg := config.Config{
		Kafka: config.KafkaConfig{Topic: topic, GroupID: group},
		Consumer: config.ConsumerConfig{
			ProcessTimeout: 10 * time.Second,
			CommitTimeout:  5 * time.Second,
		},
	}
	svc := consumer.NewService(consClient, env.Repo, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), ctmetrics.NewConsumerMetrics(), consumer.NewFreshness())

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		fetches := consClient.PollFetches(ctx)
		if fetches.Err() != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if fetches.NumRecords() == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		fetches.EachRecord(func(rec *kgo.Record) {
			svc.ProcessRecordForIntegration(ctx, rec)
		})
		break
	}

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.Equal(t, domain.StatusCarrierAssigned, projection.CurrentStatus)

	var inboxCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox WHERE event_id = $1`, eventID).Scan(&inboxCount))
	assert.Equal(t, int64(1), inboxCount)
}

func TestConsumerInvalidPayloadGoesToDeadLetterIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 1)
	env := pgintegration.SetupTestEnv(t)
	ctx := context.Background()

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	record := &kgo.Record{Topic: topic, Key: []byte(uuid.NewString()), Value: []byte("{")}
	require.NoError(t, producer.ProduceSync(ctx, record).FirstErr())

	group := "control-tower-dl-it-" + uuid.NewString()
	consClient, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("control-tower-read-model-service-it"),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	require.NoError(t, err)
	t.Cleanup(consClient.Close)

	cfg := config.Config{
		Kafka: config.KafkaConfig{Topic: topic, GroupID: group},
		Consumer: config.ConsumerConfig{
			ProcessTimeout: 10 * time.Second,
			CommitTimeout:  5 * time.Second,
		},
	}
	svc := consumer.NewService(consClient, env.Repo, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), ctmetrics.NewConsumerMetrics(), consumer.NewFreshness())

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		fetches := consClient.PollFetches(ctx)
		if fetches.Err() != nil || fetches.NumRecords() == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		fetches.EachRecord(func(rec *kgo.Record) {
			svc.ProcessRecordForIntegration(ctx, rec)
		})
		break
	}

	var deadLetterCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter WHERE topic = $1`, topic).Scan(&deadLetterCount))
	assert.Equal(t, int64(1), deadLetterCount)
}
