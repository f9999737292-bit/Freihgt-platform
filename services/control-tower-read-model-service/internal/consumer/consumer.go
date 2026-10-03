package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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

type executionApplier interface {
	ApplyRecord(ctx context.Context, payload []byte, meta domain.KafkaRecordMeta, receivedAt time.Time) error
}

type Service struct {
	client      *kgo.Client
	committer   OffsetCommitter
	repo        projectionStore
	execution   executionApplier
	cfg         config.Config
	log         *slog.Logger
	metrics     *ctmetrics.ConsumerMetrics
	freshness   *Freshness
	topic       string
	pollFetches func(context.Context) kgo.Fetches // test hook only
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

func (s *Service) SetExecutionApplier(applier executionApplier) {
	s.execution = applier
}

func (s *Service) Run(ctx context.Context) error {
	s.freshness.SetRunning(true)
	defer s.freshness.SetRunning(false)

	var pollBackoff time.Duration

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pollCtx, cancel := context.WithTimeout(ctx, s.cfg.Consumer.PollTimeout)
		fetches := s.pollOnce(pollCtx)
		pollErr := fetches.Err()
		cancel()

		outcome, errorCode := ClassifyPollError(ctx, pollCtx, pollErr)
		switch outcome {
		case PollOutcomeShutdown:
			return ctx.Err()
		case PollOutcomeIdle:
			pollBackoff = 0
			continue
		case PollOutcomeError:
			s.metrics.ObserveError(errorCode)
			s.log.Warn("kafka poll failed",
				slog.String("error_code", errorCode),
				slog.String("error", s.cfg.Kafka.ErrorString(pollErr)),
			)
			pollBackoff = nextPollBackoff(pollBackoff, s.cfg.Consumer.PollTimeout)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollBackoff):
			}
			continue
		case PollOutcomeOK:
			pollBackoff = 0
		}

		var records []*kgo.Record
		fetches.EachRecord(func(record *kgo.Record) {
			records = append(records, record)
		})
		s.processOrdered(ctx, records)
	}
}

func (s *Service) processOrdered(ctx context.Context, records []*kgo.Record) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Topic != records[j].Topic {
			return records[i].Topic < records[j].Topic
		}
		if records[i].Partition != records[j].Partition {
			return records[i].Partition < records[j].Partition
		}
		return records[i].Offset < records[j].Offset
	})
	blocked := map[string]bool{}
	for _, record := range records {
		if ctx.Err() != nil {
			return
		}
		key := fmt.Sprintf("%s:%d", record.Topic, record.Partition)
		if blocked[key] {
			continue
		}
		if !s.processRecord(ctx, record) {
			blocked[key] = true
		}
	}
}

func (s *Service) pollOnce(ctx context.Context) kgo.Fetches {
	if s.pollFetches != nil {
		return s.pollFetches(ctx)
	}
	return s.client.PollFetches(ctx)
}

func (s *Service) processRecord(ctx context.Context, record *kgo.Record) bool {
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

	kind, _ := domain.ClassifyEventPayload(record.Value)
	switch kind {
	case domain.RouteExecution, domain.RouteApproach, domain.RouteDisposition:
		if s.execution == nil {
			s.metrics.ObserveError("EXECUTION_APPLIER_MISSING")
			return false
		}
		if err := s.execution.ApplyRecord(ctx, record.Value, meta, receivedAt); err != nil {
			s.metrics.ObserveError("EXECUTION_DB_PROCESS_ERROR")
			s.log.Warn("execution projection failed",
				slog.String("topic", meta.Topic),
				slog.Int("partition", int(meta.Partition)),
				slog.Int64("offset", meta.Offset),
				slog.String("error", err.Error()),
			)
			s.rewindRecord(record)
			return false
		}
		return s.commitRecord(ctx, record, meta, "")
	case domain.RouteUnknown:
		return s.handlePermanentError(ctx, record, meta, &domain.PermanentError{Code: "UNKNOWN_EVENT_TYPE"}, receivedAt)
	}

	event, permErr := projection.ParseAndValidate(record.Value, meta, s.topic)
	if permErr != nil {
		return s.handlePermanentError(ctx, record, meta, permErr, receivedAt)
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
		return false
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

	return s.commitRecord(ctx, record, meta, event.EventID.String())
}

func (s *Service) rewindRecord(record *kgo.Record) {
	if s.client == nil || record == nil {
		return
	}
	s.client.SetOffsets(map[string]map[int32]kgo.EpochOffset{
		record.Topic: {
			record.Partition: {Offset: record.Offset},
		},
	})
}

func (s *Service) commitRecord(ctx context.Context, record *kgo.Record, meta domain.KafkaRecordMeta, eventID string) bool {
	if err := s.commitOffset(ctx, record); err != nil {
		s.metrics.ObserveOffsetCommitError()
		s.log.Warn("kafka offset commit failed",
			slog.String("event_id", eventID),
			slog.String("topic", meta.Topic),
			slog.Int("partition", int(meta.Partition)),
			slog.Int64("offset", meta.Offset),
			slog.String("error", s.cfg.Kafka.ErrorString(err)),
		)
		return false
	}
	return true
}

func (s *Service) handlePermanentError(ctx context.Context, record *kgo.Record, meta domain.KafkaRecordMeta, permErr *domain.PermanentError, receivedAt time.Time) bool {
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
		return false
	}
	if inserted {
		s.metrics.ObserveDeadLetter(permErr.Code)
	}
	s.metrics.ObserveError(permErr.Code)
	return s.commitRecord(ctx, record, meta, "")
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
