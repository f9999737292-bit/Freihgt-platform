package outbox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/freight-platform/shipment-service/internal/domain"
)

const testPublishTimeout = 5 * time.Second

type publisherTestHarness struct {
	t         *testing.T
	cluster   *kfake.Cluster
	brokers   []string
	topic     string
	producer  *kgo.Client
	publisher *KafkaPublisher
}

func newKafkaPublisherTestHarness(t *testing.T) *publisherTestHarness {
	t.Helper()

	topic := "shipment.status.v1.test." + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, topic),
	)
	if err != nil {
		t.Fatalf("kfake cluster: %v", err)
	}

	brokers := cluster.ListenAddrs()
	waitKafkaBrokerReady(t, brokers, topic)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		cluster.Close()
		t.Fatalf("producer: %v", err)
	}

	publisher := NewKafkaPublisherWithClient(producer, topic, NewRealClock())

	h := &publisherTestHarness{
		t:         t,
		cluster:   cluster,
		brokers:   brokers,
		topic:     topic,
		producer:  producer,
		publisher: publisher,
	}

	t.Cleanup(func() {
		if h.publisher != nil {
			_ = h.publisher.Close(context.Background())
		}
		if h.producer != nil {
			h.producer.Close()
		}
		if h.cluster != nil {
			h.cluster.Close()
		}
	})

	return h
}

func waitKafkaBrokerReady(t *testing.T, brokers []string, topic string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testPublishTimeout)
	defer cancel()

	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("readiness client: %v", err)
	}
	defer client.Close()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		reqCtx, reqCancel := context.WithTimeout(ctx, time.Second)
		_, metaErr := client.Request(reqCtx, &kmsg.MetadataRequest{
			Topics: []kmsg.MetadataRequestTopic{{Topic: &topic}},
		})
		reqCancel()
		if metaErr == nil {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("broker not ready for topic %s: %v", topic, metaErr)
		case <-ticker.C:
		}
	}
}

func (h *publisherTestHarness) fetchRecordByEventID(t *testing.T, eventID uuid.UUID) *kgo.Record {
	t.Helper()

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(h.brokers...),
		kgo.ConsumeTopics(h.topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	defer consumer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), testPublishTimeout)
	defer cancel()

	for {
		if err := ctx.Err(); err != nil {
			t.Fatalf("timed out waiting for event %s", eventID)
		}

		fetches := consumer.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("fetch errors: %v", errs)
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			var envelope domain.ShipmentStatusEventEnvelope
			if err := json.Unmarshal(record.Value, &envelope); err != nil {
				t.Fatalf("unmarshal envelope: %v", err)
			}
			if envelope.EventID == eventID.String() {
				return record
			}
		}
	}
}

func waitForOutboxPublished(t *testing.T, repo *fakeRepo, eventID uuid.UUID, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if status, ok := repo.claimedStatus(eventID); ok && status == domain.OutboxStatusPublished {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	status := domain.OutboxStatusPending
	if current, ok := repo.claimedStatus(eventID); ok {
		status = current
	}
	t.Fatalf("timed out waiting for published status, got %s", status)
}

func waitForCondition(t *testing.T, timeout time.Duration, desc string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}
