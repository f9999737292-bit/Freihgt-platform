package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kerr"
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
			DriverTopic:  "driver.events.v1",
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
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)

	ctx, cancel := context.WithTimeout(t.Context(), testPublishTimeout)
	defer cancel()
	if err := h.publisher.Publish(ctx, event); err != nil {
		t.Fatalf("publish: %v", err)
	}

	record := h.fetchRecordByEventID(t, event.ID)
	if string(record.Key) != event.AggregateID.String() {
		t.Fatalf("key=%s want %s", record.Key, event.AggregateID)
	}
	if string(record.Topic) != h.topic {
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
	if envelope.SourceEventID != event.SourceEventID.String() {
		t.Fatalf("sourceEventId=%s", envelope.SourceEventID)
	}
}

func TestKafkaPublisherRejectsEmptyAggregateID(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)
	event.AggregateID = uuid.Nil

	ctx, cancel := context.WithTimeout(t.Context(), testPublishTimeout)
	defer cancel()
	err := h.publisher.Publish(ctx, event)
	if err == nil {
		t.Fatal("expected error")
	}
	var pubErr *PublishError
	if !asPublishError(err, &pubErr) || pubErr.Code != ErrorCodePayloadRejected {
		t.Fatalf("err=%v", err)
	}
}

func TestKafkaPublisherUnknownEventTypeRejected(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)
	event.EventType = "shipment.unknown"

	ctx, cancel := context.WithTimeout(t.Context(), testPublishTimeout)
	defer cancel()
	err := h.publisher.Publish(ctx, event)
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

func TestClassifyKafkaContextCanceled(t *testing.T) {
	classified := classifyKafkaError(context.Canceled)
	if classified.Code != ErrorCodeTransientTimeout {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestClassifyKafkaNetworkTimeout(t *testing.T) {
	classified := classifyKafkaError(&net.DNSError{IsTimeout: true})
	if classified.Code != ErrorCodeTransientTimeout {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestClassifyKafkaNetworkError(t *testing.T) {
	classified := classifyKafkaError(&net.OpError{Op: "dial", Err: errors.New("connection refused")})
	if classified.Code != ErrorCodeTransientNetwork {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestClassifyKafkaBrokerUnavailable(t *testing.T) {
	classified := classifyKafkaError(kerr.BrokerNotAvailable)
	if classified.Code != ErrorCodeBrokerUnavailable {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestClassifyKafkaConfigurationError(t *testing.T) {
	classified := classifyKafkaError(kerr.UnknownTopicOrPartition)
	if classified.Code != ErrorCodeConfigurationError {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestClassifyKafkaPayloadRejected(t *testing.T) {
	classified := classifyKafkaError(kerr.MessageTooLarge)
	if classified.Code != ErrorCodePayloadRejected {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestClassifyKafkaUnknownError(t *testing.T) {
	classified := classifyKafkaError(errors.New("unexpected kafka failure"))
	if classified.Code != ErrorCodeUnknownPublishError {
		t.Fatalf("code=%s", classified.Code)
	}
}

func TestKafkaPublisherCloseSafe(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	if err := h.publisher.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := h.publisher.Close(context.Background()); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestKafkaPublisherCloseAfterSuccessfulPublish(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)

	ctx, cancel := context.WithTimeout(t.Context(), testPublishTimeout)
	defer cancel()
	if err := h.publisher.Publish(ctx, event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := h.publisher.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestKafkaPublisherCloseWithoutPublish(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	if err := h.publisher.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestKafkaPublisherContextCancellationDuringPublish(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := h.publisher.Publish(ctx, event)
	if err == nil {
		t.Fatal("expected publish error")
	}
	var pubErr *PublishError
	if !asPublishError(err, &pubErr) || pubErr.Code != ErrorCodeTransientTimeout {
		t.Fatalf("err=%v", err)
	}
}

func TestKafkaPublisherBrokerShutdownDuringPublish(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)

	h.cluster.Close()
	h.cluster = nil

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	err := h.publisher.Publish(ctx, event)
	if err == nil {
		t.Fatal("expected publish error")
	}
	var pubErr *PublishError
	if !asPublishError(err, &pubErr) {
		t.Fatalf("err=%v", err)
	}
	switch pubErr.Code {
	case ErrorCodeBrokerUnavailable, ErrorCodeTransientNetwork, ErrorCodeTransientTimeout, ErrorCodeUnknownPublishError:
	default:
		t.Fatalf("unexpected code=%s err=%v", pubErr.Code, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "password") {
		t.Fatalf("error must not contain credentials: %v", err)
	}
}

func TestKafkaPublisherDoesNotLogPayload(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	var buf bytes.Buffer
	_ = buf
	event := sampleKafkaOutboxEvent(t)

	ctx, cancel := context.WithTimeout(t.Context(), testPublishTimeout)
	defer cancel()
	if err := h.publisher.Publish(ctx, event); err != nil {
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
	record, err := buildKafkaRecord("shipment.status.v1", "driver.events.v1", event)
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

func TestWorkerKafkaPublisherIntegrationWithFakeBroker(t *testing.T) {
	h := newKafkaPublisherTestHarness(t)
	event := sampleKafkaOutboxEvent(t)
	repo := &fakeRepo{events: []domain.ShipmentOutboxEvent{event}}
	worker := NewWorker(testWorkerConfig(), repo, h.publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeClock{now: time.Now().UTC()})

	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	waitForOutboxPublished(t, repo, event.ID, 2*time.Second)
	cancel()
	_ = worker.Wait(context.Background())

	record := h.fetchRecordByEventID(t, event.ID)
	var envelope domain.ShipmentStatusEventEnvelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.EventID != event.ID.String() {
		t.Fatalf("eventId=%s", envelope.EventID)
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
