//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	pgintegration "github.com/freight-platform/control-tower-read-model-service/internal/integration/postgres"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
)

var errControlledOffsetCommitFailure = errors.New("controlled offset commit failure")

type failFirstCommitter struct {
	inner     consumer.OffsetCommitter
	failErr   error
	attempts  atomic.Int32
	failCount atomic.Int32
}

func newFailFirstCommitter(inner consumer.OffsetCommitter) *failFirstCommitter {
	return &failFirstCommitter{inner: inner, failErr: errControlledOffsetCommitFailure}
}

func (f *failFirstCommitter) CommitRecords(ctx context.Context, records ...*kgo.Record) error {
	f.attempts.Add(1)
	if f.failCount.Load() == 0 {
		f.failCount.Add(1)
		return f.failErr
	}
	return f.inner.CommitRecords(ctx, records...)
}

func (f *failFirstCommitter) Attempts() int {
	return int(f.attempts.Load())
}

func buildEventPayload(t *testing.T, tenantID, shipmentID, eventID, sourceEventID uuid.UUID, version int, eventType, toStatus string) []byte {
	t.Helper()
	env := map[string]any{
		"eventId":       eventID.String(),
		"eventType":     eventType,
		"schemaVersion": domain.SchemaVersionV1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId":      tenantID.String(),
		"aggregate": map[string]any{
			"type":    domain.AggregateTypeShipment,
			"id":      shipmentID.String(),
			"version": version,
		},
		"sourceEventId": sourceEventID.String(),
		"data": map[string]any{
			"toStatus":  toStatus,
			"actorType": "SYSTEM",
		},
	}
	if version == 1 && eventType == domain.EventTypeCreated {
		// fromStatus omitted for created
	} else if version > 1 {
		env["data"].(map[string]any)["fromStatus"] = domain.StatusCarrierAssigned
	}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return raw
}

func newConsumerClient(t *testing.T, brokers []string, group, clientID, topic string) *kgo.Client {
	t.Helper()
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	require.NoError(t, err)
	return client
}

func fetchCommittedOffset(ctx context.Context, brokers []string, group, topic string, partition int32) (int64, bool, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return 0, false, err
	}
	defer client.Close()

	adm := kadm.NewClient(client)
	fetched, err := adm.FetchOffsets(ctx, group)
	if err != nil {
		return 0, false, err
	}
	o, found := fetched.Lookup(topic, partition)
	if !found || o.Err != nil || o.At < 0 {
		return 0, false, nil
	}
	return o.At, true, nil
}

func waitForCommittedOffset(t *testing.T, brokers []string, group, topic string, partition int32, minOffset int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		at, ok, err := fetchCommittedOffset(ctx, brokers, group, topic, partition)
		cancel()
		require.NoError(t, err)
		if ok && at >= minOffset {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("committed offset for %s/%d did not reach %d within %s", topic, partition, minOffset, timeout)
}

func waitForInboxCount(t *testing.T, env *pgintegration.TestEnv, eventID uuid.UUID, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var count int64
		err := env.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox WHERE event_id = $1`, eventID).Scan(&count)
		cancel()
		require.NoError(t, err)
		if count >= want {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("inbox count for event %s did not reach %d", eventID, want)
}

func testConsumerConfig(topic, group string) config.Config {
	return config.Config{
		Kafka: config.KafkaConfig{Topic: topic, GroupID: group},
		Consumer: config.ConsumerConfig{
			PollTimeout:    time.Second,
			ProcessTimeout: 10 * time.Second,
			CommitTimeout:  5 * time.Second,
		},
	}
}

func TestConsumerRestartOffsetCommitFailureE2E(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 1)
	env := pgintegration.SetupTestEnv(t)
	ctx := context.Background()

	tenantID := uuid.New()
	shipmentID := uuid.New()
	eventID1 := uuid.New()
	sourceEventID1 := uuid.New()
	payload1 := buildEventPayload(t, tenantID, shipmentID, eventID1, sourceEventID1, 1, domain.EventTypeCreated, domain.StatusCarrierAssigned)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	record1 := &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: payload1}
	require.NoError(t, producer.ProduceSync(ctx, record1).FirstErr())

	group := "control-tower-restart-" + uuid.NewString()

	// Consumer-A: production path with controlled first offset commit failure.
	clientA := newConsumerClient(t, brokers, group, "control-tower-read-model-service-it-a", topic)
	t.Cleanup(clientA.Close)

	failCommitter := newFailFirstCommitter(clientA)
	metricsA := ctmetrics.NewConsumerMetrics()
	svcA := consumer.NewServiceWithCommitter(
		clientA, failCommitter, env.Repo, testConsumerConfig(topic, group),
		slog.New(slog.NewTextHandler(io.Discard, nil)), metricsA, consumer.NewFreshness(),
	)

	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)
	go func() {
		doneA <- svcA.Run(ctxA)
	}()

	waitForInboxCount(t, env, eventID1, 1, 30*time.Second)

	projectionAfterA, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projectionAfterA)
	assert.Equal(t, 1, projectionAfterA.ShipmentVersion)
	assert.Equal(t, domain.StatusCarrierAssigned, projectionAfterA.CurrentStatus)
	assert.Equal(t, eventID1, projectionAfterA.LastEventID)
	assert.Equal(t, sourceEventID1, projectionAfterA.LastSourceEventID)
	updatedAtAfterFirstApply := projectionAfterA.UpdatedAt

	var inboxOutcome string
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT outcome FROM control_tower.shipment_status_event_inbox WHERE event_id = $1`, eventID1).Scan(&inboxOutcome))
	assert.Equal(t, domain.OutcomeApplied, inboxOutcome)

	assert.Equal(t, 1, failCommitter.Attempts(), "consumer-A must attempt offset commit once")

	_, committedAfterA, err := fetchCommittedOffset(ctx, brokers, group, topic, 0)
	require.NoError(t, err)
	assert.False(t, committedAfterA, "group offset must not advance after failed commit")

	cancelA()
	select {
	case err := <-doneA:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("consumer-A shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer-A did not stop within timeout")
	}
	svcA.Close()

	// Consumer-B: same group, new client, real offset committer.
	clientB := newConsumerClient(t, brokers, group, "control-tower-read-model-service-it-b", topic)
	t.Cleanup(clientB.Close)

	metricsB := ctmetrics.NewConsumerMetrics()
	svcB := consumer.NewServiceWithCommitter(
		clientB, clientB, env.Repo, testConsumerConfig(topic, group),
		slog.New(slog.NewTextHandler(io.Discard, nil)), metricsB, consumer.NewFreshness(),
	)

	ctxB, cancelB := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelB()
	doneB := make(chan error, 1)
	go func() {
		doneB <- svcB.Run(ctxB)
	}()

	waitForCommittedOffset(t, brokers, group, topic, 0, 1, 45*time.Second)
	cancelB()
	select {
	case <-doneB:
	case <-time.After(5 * time.Second):
	}
	svcB.Close()

	var inboxCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox WHERE event_id = $1`, eventID1).Scan(&inboxCount))
	assert.Equal(t, int64(1), inboxCount, "redelivery must not create second applied inbox row")

	projectionAfterRedelivery, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projectionAfterRedelivery)
	assert.Equal(t, 1, projectionAfterRedelivery.ShipmentVersion)
	assert.Equal(t, domain.StatusCarrierAssigned, projectionAfterRedelivery.CurrentStatus)
	assert.Equal(t, updatedAtAfterFirstApply, projectionAfterRedelivery.UpdatedAt)

	// Second event: proves partition continues after duplicate re-commit.
	eventID2 := uuid.New()
	sourceEventID2 := uuid.New()
	payload2 := buildEventPayload(t, tenantID, shipmentID, eventID2, sourceEventID2, 2, domain.EventTypeStatusChanged, domain.StatusInTransit)
	record2 := &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: payload2}
	require.NoError(t, producer.ProduceSync(ctx, record2).FirstErr())

	clientC := newConsumerClient(t, brokers, group, "control-tower-read-model-service-it-c", topic)
	t.Cleanup(clientC.Close)
	svcC := consumer.NewServiceWithCommitter(
		clientC, clientC, env.Repo, testConsumerConfig(topic, group),
		slog.New(slog.NewTextHandler(io.Discard, nil)), ctmetrics.NewConsumerMetrics(), consumer.NewFreshness(),
	)

	ctxC, cancelC := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelC()
	go func() { _ = svcC.Run(ctxC) }()

	waitForInboxCount(t, env, eventID2, 1, 45*time.Second)
	waitForCommittedOffset(t, brokers, group, topic, 0, 2, 45*time.Second)
	cancelC()
	svcC.Close()

	projectionFinal, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projectionFinal)
	assert.Equal(t, 2, projectionFinal.ShipmentVersion)
	assert.Equal(t, domain.StatusInTransit, projectionFinal.CurrentStatus)
}

func TestDeadLetterRestartOffsetCommitFailureE2E(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 1)
	env := pgintegration.SetupTestEnv(t)
	ctx := context.Background()

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	poison := &kgo.Record{Topic: topic, Key: []byte(uuid.NewString()), Value: []byte("{invalid")}
	require.NoError(t, producer.ProduceSync(ctx, poison).FirstErr())

	group := "control-tower-dl-restart-" + uuid.NewString()

	clientA := newConsumerClient(t, brokers, group, "control-tower-read-model-service-dl-a", topic)
	t.Cleanup(clientA.Close)
	failCommitter := newFailFirstCommitter(clientA)
	svcA := consumer.NewServiceWithCommitter(
		clientA, failCommitter, env.Repo, testConsumerConfig(topic, group),
		slog.New(slog.NewTextHandler(io.Discard, nil)), ctmetrics.NewConsumerMetrics(), consumer.NewFreshness(),
	)

	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)
	go func() { doneA <- svcA.Run(ctxA) }()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var count int64
		_ = env.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter WHERE topic = $1`, topic).Scan(&count)
		if count == 1 && failCommitter.Attempts() >= 1 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	var dlCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter WHERE topic = $1`, topic).Scan(&dlCount))
	assert.Equal(t, int64(1), dlCount)

	var payloadColExists bool
	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'control_tower'
			  AND table_name = 'shipment_status_event_dead_letter'
			  AND column_name = 'payload'
		)`).Scan(&payloadColExists))
	assert.False(t, payloadColExists)

	cancelA()
	select {
	case <-doneA:
	case <-time.After(10 * time.Second):
	}
	svcA.Close()

	clientB := newConsumerClient(t, brokers, group, "control-tower-read-model-service-dl-b", topic)
	t.Cleanup(clientB.Close)
	svcB := consumer.NewService(clientB, env.Repo, testConsumerConfig(topic, group),
		slog.New(slog.NewTextHandler(io.Discard, nil)), ctmetrics.NewConsumerMetrics(), consumer.NewFreshness())

	ctxB, cancelB := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelB()
	go func() { _ = svcB.Run(ctxB) }()

	waitForCommittedOffset(t, brokers, group, topic, 0, 1, 45*time.Second)
	cancelB()
	svcB.Close()

	require.NoError(t, env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter WHERE topic = $1`, topic).Scan(&dlCount))
	assert.Equal(t, int64(1), dlCount)
}
