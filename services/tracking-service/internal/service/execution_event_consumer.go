package service

import (
	"context"
	"encoding/json"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/tracking-service/internal/config"
)

type ExecutionEventConsumer struct {
	client *kgo.Client
	apply  func(context.Context, []byte) error
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
	return &ExecutionEventConsumer{client: client, apply: apply}, nil
}

func (c *ExecutionEventConsumer) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func (c *ExecutionEventConsumer) Run(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		failed := false
		fetches.EachRecord(func(record *kgo.Record) {
			if failed || !isExecutionStopEvent(record.Value) {
				return
			}
			if err := c.apply(ctx, record.Value); err != nil {
				failed = true
			}
		})
		if failed {
			continue
		}
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil && ctx.Err() == nil {
			return
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
	case "shipment.route_stop.current", "shipment.route_stop.arrived", "shipment.route_stop.service_started",
		"shipment.route_stop.completed", "shipment.route_stop.sequence_overridden":
		return true
	default:
		return false
	}
}
