//go:build integration

package kafka

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

const defaultTestTopicPrefix = "shipment.status.v1.test"

func testTopicPrefix() string {
	if prefix := strings.TrimSpace(os.Getenv("TEST_KAFKA_TOPIC_PREFIX")); prefix != "" {
		return prefix
	}
	return defaultTestTopicPrefix
}

func requireKafkaBrokers(t *testing.T) []string {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("TEST_KAFKA_BROKERS"))
	if raw == "" {
		t.Skip("TEST_KAFKA_BROKERS is not set; skipping Kafka integration tests")
	}
	var brokers []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			brokers = append(brokers, part)
		}
	}
	if len(brokers) == 0 {
		t.Skip("TEST_KAFKA_BROKERS is empty; skipping Kafka integration tests")
	}
	return brokers
}

// RequireKafkaBrokers returns broker addresses or skips the test when unset.
func RequireKafkaBrokers(t *testing.T) []string {
	return requireKafkaBrokers(t)
}

func CreateUniqueTestTopic(t *testing.T, brokers []string, partitions int32) string {
	t.Helper()
	if partitions <= 0 {
		partitions = 3
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	topic := fmt.Sprintf("%s.%s", testTopicPrefix(), suffix)
	if len(topic) > 249 {
		topic = topic[:249]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("admin client: %v", err)
	}
	t.Cleanup(client.Close)

	adm := kadm.NewClient(client)
	cleanupPolicy := "delete"
	resp, err := adm.CreateTopics(ctx, partitions, 1, map[string]*string{
		"cleanup.policy": &cleanupPolicy,
	}, topic)
	if err != nil {
		t.Fatalf("create topic %s: %v", topic, err)
	}
	for _, result := range resp {
		if result.Err != nil {
			t.Fatalf("create topic %s: %v", topic, result.Err)
		}
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		topics, listErr := adm.ListTopics(ctx, topic)
		if listErr == nil {
			if detail, ok := topics[topic]; ok && detail.Topic == topic {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	topics, err := adm.ListTopics(ctx, topic)
	if err != nil {
		t.Fatalf("list topic %s metadata: %v", topic, err)
	}
	if detail, ok := topics[topic]; !ok || detail.Topic != topic {
		t.Fatalf("topic %s not visible in metadata", topic)
	}

	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ccancel()
		delResp, delErr := adm.DeleteTopics(cctx, topic)
		if delErr != nil {
			t.Errorf("cleanup delete topic %s: %v", topic, delErr)
			return
		}
		for _, result := range delResp {
			if result.Err != nil {
				t.Errorf("cleanup delete topic %s: %v", topic, result.Err)
			}
		}
	})

	return topic
}
