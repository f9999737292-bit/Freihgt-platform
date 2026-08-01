package consumer

import (
	"context"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
	"github.com/freight-platform/control-tower-read-model-service/internal/projection"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type projectionStore interface {
	ProcessEvent(ctx context.Context, input repository.ProcessInput) (repository.ProcessResult, error)
	InsertDeadLetter(ctx context.Context, input repository.DeadLetterInput) (bool, error)
}

type Service struct {
	client    *kgo.Client
	committer OffsetCommitter
	repo      projectionStore
	cfg       config.Config
	log       *slog.Logger
	metrics   *ctmetrics.ConsumerMetrics
	freshness *Freshness
	topic     string
}

func NewService(
	client *kgo.Client,
	repo *repository.ProjectionRepository,
	cfg config.Config,
	log *slog.Logger,
	metrics *ctmetrics.ConsumerMetrics,
	freshness *Freshness,
) *Service {
	return NewServiceWithCommitter(client, client, repo, cfg, log, metrics, freshness)
}

// NewServiceWithCommitter wires the production consumer with an optional offset committer.
// Production passes the Kafka client as committer; integration tests may inject a wrapper.
func NewServiceWithCommitter(
	client *kgo.Client,
	committer OffsetCommitter,
	repo *repository.ProjectionRepository,
	cfg config.Config,
	log *slog.Logger,
	metrics *ctmetrics.ConsumerMetrics,
	freshness *Freshness,
) *Service {
	if committer == nil {
		committer = client
	}
	return &Service{
		client:    client,
		committer: committer,
		repo:      repo,
		cfg:       cfg,
		log:       log,
		metrics:   metrics,
		freshness: freshness,
		topic:     cfg.Kafka.Topic,
	}
}

func (s *Service) Run(ctx context.Context) error {
	s.freshness.SetRunning(true)
	defer s.freshness.SetRunning(false)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pollCtx, cancel := context.WithTimeout(ctx, s.cfg.Consumer.PollTimeout)
		fetches := s.client.PollFetches(pollCtx)
		cancel()
		if err := fetches.Err(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.metrics.ObserveError("POLL_ERROR")
			s.log.Warn("kafka poll failed", slog.String("error", s.cfg.Kafka.ErrorString(err)))
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
	receivedAt := start
	s.freshness.MarkRecordReceived(receivedAt)
	s.metrics.SetLastRecordAt(receivedAt)

	meta := domain.KafkaRecordMeta{
		Topic:     record.Topic,
		Partition: record.Partition,
		Offset:    record.Offset,
		Key:       string(record.Key),
	}

	event, permErr := projection.ParseAndValidate(record.Value, meta, s.topic)
	if permErr != nil {
		s.handlePermanentError(ctx, record, meta, permErr, receivedAt)
		return
	}

	processCtx, cancel := context.WithTimeout(ctx, s.cfg.Consumer.ProcessTimeout)
	defer cancel()

	result, err := s.repo.ProcessEvent(processCtx, repository.ProcessInput{
		Event:      event,
		Meta:       meta,
		ReceivedAt: receivedAt,
	})
	if err != nil {
		s.metrics.ObserveError("DB_PROCESS_ERROR")
		s.log.Warn("projection transaction failed",
			slog.String("event_id", event.EventID.String()),
			slog.String("shipment_id", event.Aggregate.ID.String()),
			slog.Int("aggregate_version", event.Aggregate.Version),
			slog.String("event_type", event.EventType),
			slog.String("topic", meta.Topic),
			slog.Int("partition", int(meta.Partition)),
			slog.Int64("offset", meta.Offset),
			slog.String("error", err.Error()),
		)
		return
	}

	outcome := result.Outcome
	if result.Duplicate {
		outcome = domain.OutcomeDuplicate
	}
	s.observeSuccess(event, outcome, start)
	if result.Applied || outcome == domain.OutcomeApplied || outcome == domain.OutcomeGapApplied {
		s.freshness.MarkProjectionApplied(time.Now().UTC())
		s.metrics.SetLastAppliedAt(time.Now().UTC())
	}

	if err := s.commitOffset(ctx, record); err != nil {
		s.metrics.ObserveOffsetCommitError()
		s.log.Warn("kafka offset commit failed after db commit",
			slog.String("event_id", event.EventID.String()),
			slog.String("topic", meta.Topic),
			slog.Int("partition", int(meta.Partition)),
			slog.Int64("offset", meta.Offset),
			slog.String("error", s.cfg.Kafka.ErrorString(err)),
		)
	}
}

func (s *Service) handlePermanentError(ctx context.Context, record *kgo.Record, meta domain.KafkaRecordMeta, permErr *domain.PermanentError, receivedAt time.Time) {
	payloadHash := projection.PayloadSHA256(record.Value)
	s.log.Warn("permanent invalid shipment status event",
		slog.String("safe_error_code", permErr.Code),
		slog.String("payload_sha256", payloadHash),
		slog.Int("payload_size", len(record.Value)),
		slog.String("topic", meta.Topic),
		slog.Int("partition", int(meta.Partition)),
		slog.Int64("offset", meta.Offset),
	)

	processCtx, cancel := context.WithTimeout(ctx, s.cfg.Consumer.ProcessTimeout)
	defer cancel()

	inserted, err := s.repo.InsertDeadLetter(processCtx, repository.DeadLetterInput{
		Meta:          meta,
		PayloadSHA256: payloadHash,
		ErrorCode:     permErr.Code,
		ReceivedAt:    receivedAt,
	})
	if err != nil {
		s.metrics.ObserveError("DEAD_LETTER_DB_ERROR")
		s.log.Warn("dead-letter insert failed",
			slog.String("safe_error_code", permErr.Code),
			slog.String("topic", meta.Topic),
			slog.Int("partition", int(meta.Partition)),
			slog.Int64("offset", meta.Offset),
			slog.String("error", err.Error()),
		)
		return
	}
	if inserted {
		s.metrics.ObserveDeadLetter(permErr.Code)
	}
	s.metrics.ObserveError(permErr.Code)

	if err := s.commitOffset(ctx, record); err != nil {
		s.metrics.ObserveOffsetCommitError()
		s.log.Warn("kafka offset commit failed after dead-letter commit",
			slog.String("safe_error_code", permErr.Code),
			slog.String("topic", meta.Topic),
			slog.Int("partition", int(meta.Partition)),
			slog.Int64("offset", meta.Offset),
			slog.String("error", s.cfg.Kafka.ErrorString(err)),
		)
	}
}

func (s *Service) commitOffset(ctx context.Context, record *kgo.Record) error {
	committer := s.committer
	if committer == nil {
		committer = s.client
	}
	commitCtx, cancel := context.WithTimeout(ctx, s.cfg.Consumer.CommitTimeout)
	defer cancel()
	return committer.CommitRecords(commitCtx, record)
}

func (s *Service) observeSuccess(event domain.ShipmentStatusEvent, outcome string, start time.Time) {
	s.metrics.ObserveRecord(event.EventType, outcome, time.Since(start))
	s.metrics.ObserveOutcome(outcome, event.EventType)
	s.log.Info("shipment status event processed",
		slog.String("event_id", event.EventID.String()),
		slog.String("source_event_id", event.SourceEventID.String()),
		slog.String("shipment_id", event.Aggregate.ID.String()),
		slog.Int("aggregate_version", event.Aggregate.Version),
		slog.String("event_type", event.EventType),
		slog.String("outcome", outcome),
		slog.Duration("duration", time.Since(start)),
	)
}

func (s *Service) ProcessRecordForIntegration(ctx context.Context, record *kgo.Record) {
	s.processRecord(ctx, record)
}

func (s *Service) Close() {
	if s.client != nil {
		s.client.Close()
	}
}
