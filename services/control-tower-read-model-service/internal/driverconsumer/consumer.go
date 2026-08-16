package driverconsumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

type OffsetCommitter interface {
	CommitRecords(ctx context.Context, records ...*kgo.Record) error
}

type eventHandler interface {
	Handle(ctx context.Context, meta domain.KafkaRecordMeta, payload []byte, receivedAt time.Time) (service.DriverEventHandleResult, error)
}

type Service struct {
	client    *kgo.Client
	committer OffsetCommitter
	handler   eventHandler
	cfg       config.Config
	log       *slog.Logger
	metrics   *Metrics
	topic     string
}

func NewService(
	client *kgo.Client,
	handler eventHandler,
	cfg config.Config,
	log *slog.Logger,
	metrics *Metrics,
) *Service {
	return &Service{
		client: client, committer: client, handler: handler, cfg: cfg, log: log, metrics: metrics,
		topic: cfg.DriverConsumer.Kafka.Topic,
	}
}

func (s *Service) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		pollCtx, cancel := context.WithTimeout(ctx, s.cfg.DriverConsumer.PollTimeout)
		fetches := s.client.PollFetches(pollCtx)
		pollErr := fetches.Err()
		cancel()
		if pollErr != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if pollErr != nil {
			s.metrics.IncFailed("")
			continue
		}
		fetches.EachRecord(func(record *kgo.Record) {
			if ctx.Err() != nil {
				return
			}
			s.processRecord(ctx, record)
		})
	}
}

func (s *Service) processRecord(ctx context.Context, record *kgo.Record) {
	start := time.Now().UTC()
	meta := domain.KafkaRecordMeta{
		Topic: record.Topic, Partition: record.Partition, Offset: record.Offset, Key: string(record.Key),
	}
	processCtx, cancel := context.WithTimeout(ctx, s.cfg.DriverConsumer.ProcessTimeout)
	defer cancel()

	result, err := s.handler.Handle(processCtx, meta, append([]byte(nil), record.Value...), start)
	if permErr, ok := err.(*domain.PermanentError); ok && permErr != nil {
		s.metrics.IncFailed(permErr.Code)
		_ = s.commitOffset(ctx, record)
		return
	}
	if err != nil {
		s.metrics.IncFailed("PROCESS_ERROR")
		return
	}
	switch {
	case result.Duplicate:
		s.metrics.IncDuplicate(result.Outcome)
	case result.Outcome == "PROCESSED":
		s.metrics.IncConsumed(recordTopicEventType(record))
	default:
		s.metrics.IncConsumed(result.Outcome)
	}
	_ = s.commitOffset(ctx, record)
}

func (s *Service) commitOffset(ctx context.Context, record *kgo.Record) error {
	commitCtx, cancel := context.WithTimeout(ctx, s.cfg.DriverConsumer.CommitTimeout)
	defer cancel()
	return s.committer.CommitRecords(commitCtx, record)
}

func recordTopicEventType(record *kgo.Record) string {
	for _, h := range record.Headers {
		if h.Key == "event_type" {
			return string(h.Value)
		}
	}
	return "unknown"
}

func NewKafkaClient(cfg config.DriverKafkaConfig) (*kgo.Client, error) {
	if err := cfg.ValidateRequired(); err != nil {
		return nil, err
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.DialTimeout(cfg.DialTimeout),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.DisableAutoCommit(),
	}
	return kgo.NewClient(opts...)
}
