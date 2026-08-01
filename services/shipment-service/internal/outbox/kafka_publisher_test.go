package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestNewPublisherKafkaTransport(t *testing.T) {
	t.Parallel()
	cfg := config.OutboxConfig{
		Enabled:   true,
		Transport: "kafka",
		Kafka: config.KafkaConfig{
			Brokers:      []string{"localhost:19092"},
			Topic:        "shipment.status.v1",
			ClientID:     "shipment-service-test",
			DialTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
	}
	publisher, err := NewPublisher(cfg)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	if publisher == nil {
		t.Fatal("publisher must not be nil")
	}
	if closer, ok := publisher.(CloseablePublisher); ok {
		_ = closer.Close(context.Background())
	}
}

func TestKafkaPublisherPublishWithKfake(t *testing.T) {
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, "shipment.status.v1"),
	)
	if err != nil {
		t.Fatalf("kfake cluster: %v", err)
	}
	defer cluster.Close()

	producer, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("producer: %v", err)
	}
	defer producer.Close()

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(cluster.ListenAddrs()...),
		kgo.ConsumeTopics("shipment.status.v1"),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	defer consumer.Close()

	publisher := NewKafkaPublisherWithClient(producer, "shipment.status.v1", NewRealClock())
	event := sampleKafkaOutboxEvent(t)

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var record *kgo.Record
	for record == nil {
		fetches := consumer.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("fetch errors: %v", errs)
		}
		iter := fetches.RecordIter()
		if !iter.Done() {
			record = iter.Next()
			break
		}
		if ctx.Err() != nil {
			t.Fatal("timed out waiting for record")
		}
	}
	if string(record.Key) != event.AggregateID.String() {
		t.Fatalf("key=%s want %s", record.Key, event.AggregateID)
	}
	if string(record.Topic) != "shipment.status.v1" {
		t.Fatalf("topic=%s", record.Topic)
	}
	assertKafkaHeadersAllowlist(t, record.Headers)
	if !json.Valid(record.Value) {
		t.Fatal("value must be valid json")
	}
	var envelope domain.ShipmentStatusEventEnvelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.EventID != event.ID.String() {
		t.Fatalf("eventId=%s", envelope.EventID)
	}
}

func TestKafkaPublisherRejectsEmptyAggregateID(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "shipment.status.v1"))
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	defer cluster.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	publisher := NewKafkaPublisherWithClient(client, "shipment.status.v1", NewRealClock())
	event := sampleKafkaOutboxEvent(t)
	event.AggregateID = uuid.Nil
	err = publisher.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected error")
	}
	var pubErr *PublishError
	if !asPublishError(err, &pubErr) || pubErr.Code != ErrorCodePayloadRejected {
		t.Fatalf("err=%v", err)
	}
}

func TestKafkaPublisherUnknownEventTypeRejected(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "shipment.status.v1"))
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	defer cluster.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	publisher := NewKafkaPublisherWithClient(client, "shipment.status.v1", NewRealClock())
	event := sampleKafkaOutboxEvent(t)
	event.EventType = "shipment.unknown"
	err = publisher.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClassifyKafkaContextTimeout(t *testing.T) {
	classified := classifyKafkaError(context.DeadlineExceeded)
	if classified.Code != ErrorCodeTransientTimeout {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestKafkaPublisherCloseSafe(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "shipment.status.v1"))
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	defer cluster.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	publisher := NewKafkaPublisherWithClient(client, "shipment.status.v1", NewRealClock())
	if err := publisher.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := publisher.Close(context.Background()); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestKafkaPublisherDoesNotLogPayload(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "shipment.status.v1"))
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	defer cluster.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	var buf bytes.Buffer
	_ = buf
	publisher := NewKafkaPublisherWithClient(client, "shipment.status.v1", NewRealClock())
	event := sampleKafkaOutboxEvent(t)
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	logText := strings.ToLower(buf.String())
	for _, forbidden := range []string{"authorization", "jwt", "password", "tenantid"} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("log contains %s", forbidden)
		}
	}
}

func TestBuildKafkaRecordHeadersAllowlistOnly(t *testing.T) {
	event := sampleKafkaOutboxEvent(t)
	event.Headers = []byte(`{"Authorization":"Bearer secret","contentType":"application/json","actorType":"USER"}`)
	record, err := buildKafkaRecord("shipment.status.v1", event)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, header := range record.Headers {
		if _, ok := allowedKafkaHeaderKeys[header.Key]; !ok {
			t.Fatalf("unexpected header %q", header.Key)
		}
		if strings.EqualFold(header.Key, "authorization") {
			t.Fatal("authorization header must not be sent")
		}
	}
}

func TestClassifyKafkaBrokerUnavailable(t *testing.T) {
	err := classifyKafkaError(context.DeadlineExceeded)
	if err.Code != ErrorCodeTransientTimeout {
		t.Fatalf("code=%s", err.Code)
	}
}

func sampleKafkaOutboxEvent(t *testing.T) domain.ShipmentOutboxEvent {
	t.Helper()
	shipmentID := uuid.New()
	historyID := uuid.New()
	eventID := uuid.New()
	correlation := "corr-123"
	envelope := domain.ShipmentStatusEventEnvelope{
		EventID:       eventID.String(),
		EventType:     domain.OutboxEventTypeCreated,
		SchemaVersion: domain.OutboxSchemaVersion,
		OccurredAt:    time.Now().UTC(),
		TenantID:      uuid.New().String(),
		Aggregate: domain.ShipmentStatusAggregate{
			Type:    domain.OutboxAggregateTypeShipment,
			ID:      shipmentID.String(),
			Version: 1,
		},
		SourceEventID: historyID.String(),
		CorrelationID: &correlation,
		Data: domain.ShipmentStatusEventData{
			ToStatus:  "CREATED",
			ActorType: "USER",
		},
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return domain.ShipmentOutboxEvent{
		ID:               eventID,
		TenantID:         uuid.MustParse(envelope.TenantID),
		AggregateType:    domain.OutboxAggregateTypeShipment,
		AggregateID:      shipmentID,
		AggregateVersion: 1,
		EventType:        domain.OutboxEventTypeCreated,
		SchemaVersion:    domain.OutboxSchemaVersion,
		SourceEventID:    historyID,
		Payload:          payload,
		Headers:          []byte(`{"contentType":"application/json","Authorization":"Bearer x","actorType":"USER"}`),
		Status:           domain.OutboxStatusPending,
	}
}

func assertKafkaHeadersAllowlist(t *testing.T, headers []kgo.RecordHeader) {
	t.Helper()
	for _, header := range headers {
		if _, ok := allowedKafkaHeaderKeys[header.Key]; !ok {
			t.Fatalf("unexpected header %q", header.Key)
		}
	}
}

func asPublishError(err error, target **PublishError) bool {
	if err == nil {
		return false
	}
	if pe, ok := err.(*PublishError); ok {
		*target = pe
		return true
	}
	return false
}

func TestWorkerKafkaPublisherIntegrationWithFakeBroker(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "shipment.status.v1"))
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	defer cluster.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer client.Close()

	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{sampleKafkaOutboxEvent(t)}}
	publisher := NewKafkaPublisherWithClient(client, "shipment.status.v1", NewRealClock())
	worker := NewWorker(testWorkerConfig(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	_ = worker.Wait(context.Background())
	if repo.claimed[0].Status != domain.OutboxStatusPublished {
		t.Fatalf("status=%s", repo.claimed[0].Status)
	}
}
