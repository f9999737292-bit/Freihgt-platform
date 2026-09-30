package service

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/domain"
)

type ExecutionEventConsumer struct {
	client *kgo.Client
	apply  func(context.Context, []byte) error
	sleep  func(context.Context) error
}

func NewExecutionEventConsumer(cfg config.KafkaConfig, apply func(context.Context, []byte) error) (*ExecutionEventConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.ExecutionTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}
	return &ExecutionEventConsumer{client: client, apply: apply, sleep: defaultRetrySleep}, nil
}

func (c *ExecutionEventConsumer) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func (c *ExecutionEventConsumer) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		var records []*kgo.Record
		fetches.EachRecord(func(record *kgo.Record) {
			records = append(records, record)
		})
		if err := consumeBatch(ctx, records, c.apply, c.commitRecord, c.sleep); err != nil {
			return
		}
	}
}

func (c *ExecutionEventConsumer) commitRecord(ctx context.Context, record *kgo.Record) error {
	return c.client.CommitRecords(ctx, record)
}

func defaultRetrySleep(ctx context.Context) error {
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type partitionKey struct {
	topic     string
	partition int32
}

func consumeBatch(ctx context.Context, records []*kgo.Record, apply func(context.Context, []byte) error, commit func(context.Context, *kgo.Record) error, sleep func(context.Context) error) error {
	grouped := map[partitionKey][]*kgo.Record{}
	var keys []partitionKey
	for _, record := range records {
		if record == nil {
			continue
		}
		key := partitionKey{topic: record.Topic, partition: record.Partition}
		if _, ok := grouped[key]; !ok {
			keys = append(keys, key)
		}
		grouped[key] = append(grouped[key], record)
	}
	for _, key := range keys {
		part := grouped[key]
		sort.Slice(part, func(i, j int) bool { return part[i].Offset < part[j].Offset })
		for _, record := range part {
			if isExecutionStopEvent(record.Value) {
				if err := applyUntil(ctx, apply, sleep, record.Value); err != nil {
					return err
				}
			}
			if err := commit(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyUntil(ctx context.Context, apply func(context.Context, []byte) error, sleep func(context.Context) error, payload []byte) error {
	if sleep == nil {
		sleep = defaultRetrySleep
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := apply(ctx, payload)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := sleep(ctx); err != nil {
			return err
		}
	}
}

func isExecutionStopEvent(raw []byte) bool {
	var payload struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	switch payload.EventType {
	case domain.EventRouteStopCurrent, domain.EventRouteStopArrived, domain.EventRouteStopServiceStarted,
		domain.EventRouteStopCompleted, domain.EventRouteStopSequenceOverride:
		return true
	default:
		return false
	}
}
