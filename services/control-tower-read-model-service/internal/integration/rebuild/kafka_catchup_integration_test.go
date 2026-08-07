//go:build integration

package rebuild

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	kafkatest "github.com/freight-platform/control-tower-read-model-service/internal/integration/kafka"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/statussnapshot"
)

const rebuildCatchUpGroup = "control-tower-shipment-status-v1"

func buildKafkaEventPayload(t *testing.T, tenantID, shipmentID, eventID, sourceEventID uuid.UUID, version int, eventType, toStatus string) []byte {
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
	if version > 1 {
		env["data"].(map[string]any)["fromStatus"] = domain.StatusCarrierAssigned
	}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return raw
}

func fetchGroupOffset(t *testing.T, brokers []string, group, topic string, partition int32) (offset int64, committed bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	defer client.Close()
	adm := kadm.NewClient(client)
	fetched, err := adm.FetchOffsets(ctx, group)
	require.NoError(t, err)
	o, found := fetched.Lookup(topic, partition)
	if !found || o.Err != nil || o.At < 0 {
		return 0, false
	}
	return o.At, true
}

func logGroupOffsets(t *testing.T, label string, brokers []string, group, topic string) {
	t.Helper()
	off, ok := fetchGroupOffset(t, brokers, group, topic, 0)
	t.Logf("%s group=%s topic=%s partition=0 offset=%d committed=%v", label, group, topic, off, ok)
}

func newRebuildKafkaConsumer(t *testing.T, brokers []string, group, topic string, repo *repository.ProjectionRepository) *consumer.Service {
	t.Helper()
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("control-tower-read-model-service-rebuild-it"),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	cfg := config.Config{
		Kafka: config.KafkaConfig{Topic: topic, GroupID: group},
		Consumer: config.ConsumerConfig{
			PollTimeout:    time.Second,
			ProcessTimeout: 10 * time.Second,
			CommitTimeout:  5 * time.Second,
		},
	}
	return consumer.NewService(client, repo, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), ctmetrics.NewConsumerMetrics(), consumer.NewFreshness())
}

func setupRebuildKafkaEnv(t *testing.T) (brokers []string, topic string, pool *pgxpool.Pool, repo *repository.ProjectionRepository, actRepo apprebuild.ActivationRepository) {
	t.Helper()
	brokers = kafkatest.RequireKafkaBrokers(t)
	topic = kafkatest.CreateUniqueTestTopic(t, brokers, 1)
	pool = setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo = repository.NewProjectionRepository(pool)
	actRepo = apprebuild.NewActivationRepository(pool)
	return brokers, topic, pool, repo, actRepo
}

func firstShipmentIDFromStream(t *testing.T, stream []byte) uuid.UUID {
	t.Helper()
	dec := statussnapshot.NewDecoder(bytes.NewReader(stream), statussnapshot.DecoderOptions{})
	for {
		rec, err := dec.Next()
		require.NoError(t, err)
		if ship, ok := rec.(statussnapshot.ShipmentRecord); ok {
			return ship.ShipmentID
		}
	}
}

func waitForInboxByEvent(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, want int64, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int64
		err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox WHERE event_id=$1`, eventID).Scan(&count)
		require.NoError(t, err)
		if count >= want {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("inbox count for %s did not reach %d", eventID, want)
}

func TestKafkaCatchUpIntegration(t *testing.T) {
	brokers, topic, pool, repo, actRepo := setupRebuildKafkaEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	stream := buildTenantScopedStreamAtVersion(t, tenantID, 1, 1)
	shipmentID := firstShipmentIDFromStream(t, stream)
	snapshotID := importSnapshot(t, pool, stream)

	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)
	logGroupOffsets(t, "group_before", brokers, rebuildCatchUpGroup, topic)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	svc := newRebuildKafkaConsumer(t, brokers, rebuildCatchUpGroup, topic, repo)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = svc.Run(runCtx) }()
	t.Cleanup(svc.Close)

	postEventID := uuid.New()
	postSourceID := uuid.New()
	postPayload := buildKafkaEventPayload(t, tenantID, shipmentID, postEventID, postSourceID, 2, domain.EventTypeStatusChanged, domain.StatusInTransit)
	require.NoError(t, producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: postPayload}).FirstErr())

	waitForInboxByEvent(t, pool, postEventID, 1, 30*time.Second)
	logGroupOffsets(t, "group_after_activation", brokers, rebuildCatchUpGroup, topic)
	logGroupOffsets(t, "group_after_resume", brokers, rebuildCatchUpGroup, topic)

	snap, ok := snapshotProjectionRow(t, pool, tenantID, shipmentID)
	require.True(t, ok)
	require.Equal(t, 2, snap.ShipmentVersion)
	require.Equal(t, apprebuild.ProjectionSourceLiveEvent, snap.ProjectionSource)
}

func TestOffsetPreservationIntegration(t *testing.T) {
	brokers, topic, pool, repo, actRepo := setupRebuildKafkaEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	eventID := uuid.New()
	sourceID := uuid.New()
	payload := buildKafkaEventPayload(t, tenantID, shipmentID, eventID, sourceID, 1, domain.EventTypeCreated, domain.StatusCarrierAssigned)
	require.NoError(t, producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: payload}).FirstErr())

	svc := newRebuildKafkaConsumer(t, brokers, rebuildCatchUpGroup, topic, repo)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = svc.Run(runCtx) }()
	t.Cleanup(svc.Close)

	waitForInboxByEvent(t, pool, eventID, 1, 30*time.Second)
	beforeOff, beforeOK := fetchGroupOffset(t, brokers, rebuildCatchUpGroup, topic, 0)
	logGroupOffsets(t, "offsets_before_pause", brokers, rebuildCatchUpGroup, topic)

	snapshotID := importSnapshot(t, pool, buildTenantScopedStream(t, tenantID, 1))
	_, err = actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	afterOff, afterOK := fetchGroupOffset(t, brokers, rebuildCatchUpGroup, topic, 0)
	logGroupOffsets(t, "offsets_after_activation", brokers, rebuildCatchUpGroup, topic)
	require.Equal(t, beforeOK, afterOK)
	if beforeOK {
		require.Equal(t, beforeOff, afterOff)
	}
}

func TestEventsDuringPauseIntegration(t *testing.T) {
	brokers, topic, pool, repo, actRepo := setupRebuildKafkaEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	stream := buildTenantScopedStreamAtVersion(t, tenantID, 1, 1)
	shipmentID := firstShipmentIDFromStream(t, stream)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	logGroupOffsets(t, "offsets_before_pause", brokers, rebuildCatchUpGroup, topic)

	snapshotID := importSnapshot(t, pool, stream)
	release := make(chan struct{})
	pauseEntered := make(chan struct{})
	apprebuild.SetActivationPauseHookForTest(apprebuild.FailPointAfterDelete, release, pauseEntered)
	t.Cleanup(func() { apprebuild.SetActivationPauseHookForTest("", nil, nil) })

	var wg sync.WaitGroup
	var activateErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, activateErr = actRepo.Activate(ctx, snapshotID)
	}()
	select {
	case <-pauseEntered:
	case <-time.After(30 * time.Second):
		t.Fatal("activation did not reach pause hook")
	}

	duringEventID := uuid.New()
	duringSourceID := uuid.New()
	duringPayload := buildKafkaEventPayload(t, tenantID, shipmentID, duringEventID, duringSourceID, 2, domain.EventTypeStatusChanged, domain.StatusInTransit)
	require.NoError(t, producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: duringPayload}).FirstErr())

	close(release)
	wg.Wait()
	require.NoError(t, activateErr)
	logGroupOffsets(t, "offsets_after_activation", brokers, rebuildCatchUpGroup, topic)

	svc := newRebuildKafkaConsumer(t, brokers, rebuildCatchUpGroup, topic, repo)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = svc.Run(runCtx) }()
	t.Cleanup(svc.Close)

	waitForInboxByEvent(t, pool, duringEventID, 1, 30*time.Second)
	logGroupOffsets(t, "offsets_after_resume", brokers, rebuildCatchUpGroup, topic)

	snap, ok := snapshotProjectionRow(t, pool, tenantID, shipmentID)
	require.True(t, ok)
	require.Equal(t, 2, snap.ShipmentVersion)
}

func TestGapAfterActivationIntegration(t *testing.T) {
	brokers, topic, pool, repo, actRepo := setupRebuildKafkaEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	stream := buildTenantScopedStreamAtVersion(t, tenantID, 1, 5)
	shipmentID := firstShipmentIDFromStream(t, stream)
	snapshotID := importSnapshot(t, pool, stream)
	_, err := actRepo.Activate(ctx, snapshotID)
	require.NoError(t, err)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)

	gapEventID := uuid.New()
	gapSourceID := uuid.New()
	gapPayload := buildKafkaEventPayload(t, tenantID, shipmentID, gapEventID, gapSourceID, 7, domain.EventTypeStatusChanged, domain.StatusDelivered)
	require.NoError(t, producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(shipmentID.String()), Value: gapPayload}).FirstErr())

	svc := newRebuildKafkaConsumer(t, brokers, rebuildCatchUpGroup, topic, repo)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() { _ = svc.Run(runCtx) }()
	t.Cleanup(svc.Close)

	waitForInboxByEvent(t, pool, gapEventID, 1, 30*time.Second)

	var outcome string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT outcome FROM control_tower.shipment_status_event_inbox WHERE event_id=$1`, gapEventID).Scan(&outcome))
	require.Equal(t, domain.OutcomeGapApplied, outcome)

	snap, ok := snapshotProjectionRow(t, pool, tenantID, shipmentID)
	require.True(t, ok)
	require.Equal(t, 7, snap.ShipmentVersion)
	require.True(t, snap.GapDetected)
}
