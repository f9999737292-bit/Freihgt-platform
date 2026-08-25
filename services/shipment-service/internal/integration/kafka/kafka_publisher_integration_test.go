//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
	shipmentoutbox "github.com/freight-platform/shipment-service/internal/outbox"
)

func newKafkaPublisher(t *testing.T, brokers []string, topic string) *shipmentoutbox.KafkaPublisher {
	t.Helper()
	publisher, err := shipmentoutbox.NewKafkaPublisher(config.KafkaConfig{
		Brokers:      brokers,
		Topic:        topic,
		ClientID:     "shipment-service-it",
		DialTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}, shipmentoutbox.NewRealClock())
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	t.Cleanup(func() {
		_ = publisher.Close(context.Background())
	})
	return publisher
}

func newKafkaConsumer(t *testing.T, brokers []string, topic string, group string) *kgo.Client {
	t.Helper()
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("shipment-service-it-consumer"),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(group),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	t.Cleanup(client.Close)
	time.Sleep(300 * time.Millisecond)
	return client
}

func buildIntegrationOutboxEvent(shipmentID uuid.UUID, version int, eventType string) domain.ShipmentOutboxEvent {
	eventID := uuid.New()
	historyID := uuid.New()
	correlation := "corr-" + uuid.NewString()
	envelope := domain.ShipmentStatusEventEnvelope{
		EventID:       eventID.String(),
		EventType:     eventType,
		SchemaVersion: domain.OutboxSchemaVersion,
		OccurredAt:    time.Now().UTC(),
		TenantID:      uuid.New().String(),
		Aggregate: domain.ShipmentStatusAggregate{
			Type:    domain.OutboxAggregateTypeShipment,
			ID:      shipmentID.String(),
			Version: version,
		},
		SourceEventID: historyID.String(),
		CorrelationID: &correlation,
		Data: domain.ShipmentStatusEventData{
			ToStatus:  "IN_TRANSIT",
			ActorType: "USER",
		},
	}
	payload, _ := json.Marshal(envelope)
	return domain.ShipmentOutboxEvent{
		ID:               eventID,
		AggregateType:    domain.OutboxAggregateTypeShipment,
		AggregateID:      shipmentID,
		AggregateVersion: version,
		EventType:        eventType,
		SchemaVersion:    domain.OutboxSchemaVersion,
		SourceEventID:    historyID,
		Payload:          payload,
		Headers:          []byte(`{"contentType":"application/json","Authorization":"Bearer secret"}`),
		Status:           domain.OutboxStatusPending,
	}
}

func consumeRecordByEventID(t *testing.T, client *kgo.Client, event domain.ShipmentOutboxEvent, timeout time.Duration) *kgo.Record {
	t.Helper()
	record := consumeRecordByEventIDString(t, client, event.ID.String(), timeout)
	assertRecordMatchesEvent(t, record, event, record.Topic)
	return record
}

func consumeRecordByEventIDString(t *testing.T, client *kgo.Client, eventID string, timeout time.Duration) *kgo.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		fetches := client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("fetch errors: %v", errs)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			var envelope domain.ShipmentStatusEventEnvelope
			if err := json.Unmarshal(record.Value, &envelope); err != nil {
				t.Fatalf("unmarshal candidate record: %v", err)
			}
			if envelope.EventID == eventID {
				return record
			}
		}
		if ctx.Err() != nil {
			t.Fatalf("timed out waiting for eventId %s", eventID)
		}
	}
}

func consumeRecordsByEventIDs(t *testing.T, client *kgo.Client, events []domain.ShipmentOutboxEvent, timeout time.Duration) []*kgo.Record {
	t.Helper()
	want := make(map[string]domain.ShipmentOutboxEvent, len(events))
	order := make([]string, 0, len(events))
	for _, event := range events {
		want[event.ID.String()] = event
		order = append(order, event.ID.String())
	}

	found := make(map[string]*kgo.Record, len(events))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for len(found) < len(events) {
		fetches := client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("fetch errors: %v", errs)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			var envelope domain.ShipmentStatusEventEnvelope
			if err := json.Unmarshal(record.Value, &envelope); err != nil {
				t.Fatalf("unmarshal candidate record: %v", err)
			}
			if event, ok := want[envelope.EventID]; ok {
				if _, seen := found[envelope.EventID]; !seen {
					assertRecordMatchesEvent(t, record, event, record.Topic)
					found[envelope.EventID] = record
				}
			}
		}
		if ctx.Err() != nil {
			t.Fatalf("timed out waiting for %d events, got %d", len(events), len(found))
		}
	}

	out := make([]*kgo.Record, 0, len(events))
	for _, eventID := range order {
		record, ok := found[eventID]
		if !ok {
			t.Fatalf("missing record for eventId %s", eventID)
		}
		out = append(out, record)
	}
	return out
}

func assertRecordMatchesEvent(t *testing.T, record *kgo.Record, event domain.ShipmentOutboxEvent, topic string) {
	t.Helper()
	if record.Topic != topic {
		t.Fatalf("topic=%s want %s", record.Topic, topic)
	}
	if string(record.Key) != event.AggregateID.String() {
		t.Fatalf("key=%s want %s", record.Key, event.AggregateID)
	}
	var envelope domain.ShipmentStatusEventEnvelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if envelope.EventID != event.ID.String() {
		t.Fatalf("eventId=%s want %s", envelope.EventID, event.ID)
	}
	if envelope.SourceEventID != event.SourceEventID.String() {
		t.Fatalf("sourceEventId mismatch")
	}
	if envelope.EventType != event.EventType {
		t.Fatalf("eventType=%s want %s", envelope.EventType, event.EventType)
	}
	if envelope.SchemaVersion != event.SchemaVersion {
		t.Fatalf("schemaVersion=%d want %d", envelope.SchemaVersion, event.SchemaVersion)
	}
	if envelope.Aggregate.ID != event.AggregateID.String() {
		t.Fatalf("aggregate.id mismatch")
	}
	if envelope.Aggregate.Version != event.AggregateVersion {
		t.Fatalf("aggregate.version=%d want %d", envelope.Aggregate.Version, event.AggregateVersion)
	}
	for _, header := range record.Headers {
		switch header.Key {
		case "event_type", "schema_version", "source_event_id", "correlation_id", "content_type":
		default:
			t.Fatalf("unexpected header %q", header.Key)
		}
		if strings.EqualFold(header.Key, "authorization") {
			t.Fatal("authorization header must not be present")
		}
	}
	validateEventSchemaFields(t, envelope)
}

func validateEventSchemaFields(t *testing.T, envelope domain.ShipmentStatusEventEnvelope) {
	t.Helper()
	if envelope.EventID == "" || envelope.EventType == "" || envelope.TenantID == "" || envelope.SourceEventID == "" {
		t.Fatal("required envelope fields missing")
	}
	if envelope.SchemaVersion < 1 || envelope.Aggregate.Type != domain.OutboxAggregateTypeShipment {
		t.Fatal("schema/aggregate validation failed")
	}
	if envelope.Data.ToStatus == "" || envelope.Data.ActorType == "" {
		t.Fatal("data fields missing")
	}
}

func consumeDistinctRecordsByEventID(t *testing.T, client *kgo.Client, eventID string, count int, timeout time.Duration) []*kgo.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	seenOffsets := make(map[int64]struct{})
	records := make([]*kgo.Record, 0, count)
	for len(records) < count {
		fetches := client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("fetch errors: %v", errs)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			var envelope domain.ShipmentStatusEventEnvelope
			if err := json.Unmarshal(record.Value, &envelope); err != nil {
				t.Fatalf("unmarshal candidate record: %v", err)
			}
			if envelope.EventID != eventID {
				continue
			}
			if _, seen := seenOffsets[record.Offset]; seen {
				continue
			}
			seenOffsets[record.Offset] = struct{}{}
			records = append(records, record)
		}
		if ctx.Err() != nil {
			t.Fatalf("timed out waiting for %d records with eventId %s, got %d", count, eventID, len(records))
		}
	}
	return records
}

func TestKafkaPublishConsumeIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 3)
	group := "shipment-it-" + uuid.NewString()
	consumer := newKafkaConsumer(t, brokers, topic, group)
	publisher := newKafkaPublisher(t, brokers, topic)

	event := buildIntegrationOutboxEvent(uuid.New(), 1, domain.OutboxEventTypeCreated)
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	record := consumeRecordByEventID(t, consumer, event, 20*time.Second)
	t.Logf("topic=%s partition=%d offset=%d event_id=%s source_event_id=%s version=%d",
		topic, record.Partition, record.Offset, event.ID, event.SourceEventID, event.AggregateVersion)
}

func TestKafkaOrderingSameShipmentKeyIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 3)
	group := "shipment-it-order-" + uuid.NewString()
	consumer := newKafkaConsumer(t, brokers, topic, group)
	publisher := newKafkaPublisher(t, brokers, topic)

	shipmentID := uuid.New()
	events := []domain.ShipmentOutboxEvent{
		buildIntegrationOutboxEvent(shipmentID, 1, domain.OutboxEventTypeCreated),
		buildIntegrationOutboxEvent(shipmentID, 2, domain.OutboxEventTypeStatusChanged),
		buildIntegrationOutboxEvent(shipmentID, 3, domain.OutboxEventTypeStatusChanged),
	}
	for _, event := range events {
		if err := publisher.Publish(context.Background(), event); err != nil {
			t.Fatalf("publish v%d: %v", event.AggregateVersion, err)
		}
	}

	records := consumeRecordsByEventIDs(t, consumer, events, 20*time.Second)
	var partition int32 = -1
	for i, record := range records {
		if string(record.Key) != shipmentID.String() {
			t.Fatalf("record %d key mismatch", i)
		}
		if partition == -1 {
			partition = record.Partition
		} else if record.Partition != partition {
			t.Fatalf("record %d partition=%d want %d", i, record.Partition, partition)
		}
		if i > 0 && records[i-1].Offset >= record.Offset {
			t.Fatalf("offsets not increasing: prev=%d current=%d", records[i-1].Offset, record.Offset)
		}
	}
}

func TestKafkaMultipleShipmentKeysIntegration(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	topic := CreateUniqueTestTopic(t, brokers, 3)
	group := "shipment-it-multi-" + uuid.NewString()
	consumer := newKafkaConsumer(t, brokers, topic, group)
	publisher := newKafkaPublisher(t, brokers, topic)

	shipmentA := uuid.New()
	shipmentB := uuid.New()
	shipmentC := uuid.New()
	events := []domain.ShipmentOutboxEvent{
		buildIntegrationOutboxEvent(shipmentA, 1, domain.OutboxEventTypeCreated),
		buildIntegrationOutboxEvent(shipmentB, 1, domain.OutboxEventTypeCreated),
		buildIntegrationOutboxEvent(shipmentC, 1, domain.OutboxEventTypeCreated),
		buildIntegrationOutboxEvent(shipmentA, 2, domain.OutboxEventTypeStatusChanged),
		buildIntegrationOutboxEvent(shipmentB, 2, domain.OutboxEventTypeStatusChanged),
		buildIntegrationOutboxEvent(shipmentC, 2, domain.OutboxEventTypeStatusChanged),
	}
	eventIDs := make(map[string]struct{}, len(events))
	for _, event := range events {
		if _, exists := eventIDs[event.ID.String()]; exists {
			t.Fatalf("duplicate event ID generated")
		}
		eventIDs[event.ID.String()] = struct{}{}
		if err := publisher.Publish(context.Background(), event); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	records := consumeRecordsByEventIDs(t, consumer, events, 30*time.Second)
	partitionByKey := make(map[string]int32)
	versionByKey := make(map[string]int)
	for _, record := range records {
		key := string(record.Key)
		if prev, ok := partitionByKey[key]; ok && prev != record.Partition {
			t.Fatalf("shipment key %s changed partition from %d to %d", key, prev, record.Partition)
		}
		partitionByKey[key] = record.Partition

		var envelope domain.ShipmentStatusEventEnvelope
		if err := json.Unmarshal(record.Value, &envelope); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if prev, ok := versionByKey[key]; ok && envelope.Aggregate.Version <= prev {
			t.Fatalf("shipment %s version order broken: prev=%d current=%d", key, prev, envelope.Aggregate.Version)
		}
		versionByKey[key] = envelope.Aggregate.Version
	}
	if len(partitionByKey) < 3 {
		t.Fatalf("expected 3 shipment keys, got %d", len(partitionByKey))
	}
}

func TestKafkaBrokerUnavailableIntegration(t *testing.T) {
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

	event := buildIntegrationOutboxEvent(uuid.New(), 1, domain.OutboxEventTypeCreated)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	err = publisher.Publish(ctx, event)
	if err == nil {
		t.Fatal("expected publish failure")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("publish took too long: %s", time.Since(start))
	}
	classified := shipmentoutbox.ClassifyPublishError(err)
	if classified.Code != shipmentoutbox.ErrorCodeBrokerUnavailable &&
		classified.Code != shipmentoutbox.ErrorCodeTransientNetwork &&
		classified.Code != shipmentoutbox.ErrorCodeTransientTimeout &&
		classified.Code != shipmentoutbox.ErrorCodeUnknownPublishError {
		t.Fatalf("error_code=%s", classified.Code)
	}
}

func TestKafkaStartupPublisherCreationWithValidConfig(t *testing.T) {
	brokers := requireKafkaBrokers(t)
	publisher, err := shipmentoutbox.NewPublisher(config.OutboxConfig{
		Enabled:   true,
		Transport: "kafka",
		Kafka: config.KafkaConfig{
			Brokers:      brokers,
			Topic:        "shipment.status.v1",
			DriverTopic:  "driver.events.v1",
			ClientID:     "shipment-service-it",
			DialTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("startup publisher: %v", err)
	}
	if closer, ok := publisher.(shipmentoutbox.CloseablePublisher); ok {
		_ = closer.Close(context.Background())
	}
}
