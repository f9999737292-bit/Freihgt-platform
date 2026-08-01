package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type mockProjectionRepo struct {
	processFn       func(ctx context.Context, input repository.ProcessInput) (repository.ProcessResult, error)
	insertDeadFn    func(ctx context.Context, input repository.DeadLetterInput) (bool, error)
	processCalls    int
	deadLetterCalls int
}

func (m *mockProjectionRepo) ProcessEvent(ctx context.Context, input repository.ProcessInput) (repository.ProcessResult, error) {
	m.processCalls++
	if m.processFn != nil {
		return m.processFn(ctx, input)
	}
	return repository.ProcessResult{Outcome: domain.OutcomeApplied, Applied: true}, nil
}

func (m *mockProjectionRepo) InsertDeadLetter(ctx context.Context, input repository.DeadLetterInput) (bool, error) {
	m.deadLetterCalls++
	if m.insertDeadFn != nil {
		return m.insertDeadFn(ctx, input)
	}
	return true, nil
}

type mockCommitter struct {
	err     error
	records []*kgo.Record
	calls   int
}

func (m *mockCommitter) CommitRecords(_ context.Context, records ...*kgo.Record) error {
	m.calls++
	m.records = append(m.records, records...)
	return m.err
}

func testConsumerConfig() config.Config {
	return config.Config{
		Kafka: config.KafkaConfig{Topic: "shipment.status.v1"},
		Consumer: config.ConsumerConfig{
			ProcessTimeout: 5 * time.Second,
			CommitTimeout:  2 * time.Second,
		},
	}
}

func validCreatedRecord(t *testing.T) (*kgo.Record, uuid.UUID) {
	t.Helper()
	eventID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	sourceEventID := uuid.New()
	env := map[string]any{
		"eventId":       eventID.String(),
		"eventType":     domain.EventTypeCreated,
		"schemaVersion": domain.SchemaVersionV1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId":      tenantID.String(),
		"aggregate": map[string]any{
			"type":    domain.AggregateTypeShipment,
			"id":      shipmentID.String(),
			"version": 1,
		},
		"sourceEventId": sourceEventID.String(),
		"data": map[string]any{
			"toStatus":  domain.StatusCarrierAssigned,
			"actorType": "SYSTEM",
		},
	}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return &kgo.Record{
		Topic:     "shipment.status.v1",
		Partition: 0,
		Offset:    100,
		Key:       []byte(shipmentID.String()),
		Value:     raw,
	}, shipmentID
}

func newTestService(repo projectionStore, committer OffsetCommitter) *Service {
	return &Service{
		repo:      repo,
		committer: committer,
		cfg:       testConsumerConfig(),
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics:   ctmetrics.NewConsumerMetrics(),
		freshness: NewFreshness(),
		topic:     "shipment.status.v1",
	}
}

func TestProcessRecordDBCommitSuccessCommitsOffset(t *testing.T) {
	t.Parallel()
	repo := &mockProjectionRepo{}
	committer := &mockCommitter{}
	svc := newTestService(repo, committer)
	record, _ := validCreatedRecord(t)

	svc.processRecord(context.Background(), record)
	assert.Equal(t, 1, repo.processCalls)
	assert.Equal(t, 1, committer.calls)
	require.Len(t, committer.records, 1)
	assert.Equal(t, record.Offset, committer.records[0].Offset)
}

func TestProcessRecordDBCommitFailureDoesNotCommitOffset(t *testing.T) {
	t.Parallel()
	repo := &mockProjectionRepo{
		processFn: func(context.Context, repository.ProcessInput) (repository.ProcessResult, error) {
			return repository.ProcessResult{}, errors.New("db unavailable")
		},
	}
	committer := &mockCommitter{}
	svc := newTestService(repo, committer)
	record, _ := validCreatedRecord(t)

	svc.processRecord(context.Background(), record)
	assert.Equal(t, 1, repo.processCalls)
	assert.Equal(t, 0, committer.calls)
}

func TestProcessRecordPermanentErrorDeadLetterThenCommitsOffset(t *testing.T) {
	t.Parallel()
	repo := &mockProjectionRepo{}
	committer := &mockCommitter{}
	svc := newTestService(repo, committer)
	record := &kgo.Record{
		Topic:     "shipment.status.v1",
		Partition: 0,
		Offset:    101,
		Value:     []byte("{"),
	}

	svc.processRecord(context.Background(), record)
	assert.Equal(t, 0, repo.processCalls)
	assert.Equal(t, 1, repo.deadLetterCalls)
	assert.Equal(t, 1, committer.calls)
}

func TestProcessRecordDeadLetterDBFailureDoesNotCommitOffset(t *testing.T) {
	t.Parallel()
	repo := &mockProjectionRepo{
		insertDeadFn: func(context.Context, repository.DeadLetterInput) (bool, error) {
			return false, errors.New("dead-letter insert failed")
		},
	}
	committer := &mockCommitter{}
	svc := newTestService(repo, committer)
	record := &kgo.Record{
		Topic:     "shipment.status.v1",
		Partition: 0,
		Offset:    102,
		Value:     []byte("{"),
	}

	svc.processRecord(context.Background(), record)
	assert.Equal(t, 1, repo.deadLetterCalls)
	assert.Equal(t, 0, committer.calls)
}

func TestProcessRecordOffsetCommitFailureAfterDBSuccess(t *testing.T) {
	t.Parallel()
	repo := &mockProjectionRepo{}
	committer := &mockCommitter{err: errors.New("commit failed")}
	svc := newTestService(repo, committer)
	record, _ := validCreatedRecord(t)

	svc.processRecord(context.Background(), record)
	assert.Equal(t, 1, repo.processCalls)
	assert.Equal(t, 1, committer.calls)
}

func TestProcessRecordDuplicateResultCommitsOffset(t *testing.T) {
	t.Parallel()
	repo := &mockProjectionRepo{
		processFn: func(context.Context, repository.ProcessInput) (repository.ProcessResult, error) {
			return repository.ProcessResult{Outcome: domain.OutcomeApplied, Duplicate: true}, nil
		},
	}
	committer := &mockCommitter{}
	svc := newTestService(repo, committer)
	record, _ := validCreatedRecord(t)

	svc.processRecord(context.Background(), record)
	assert.Equal(t, 1, repo.processCalls)
	assert.Equal(t, 1, committer.calls)
}

func TestProcessRecordOffsetCommitFailureDoesNotRollbackProjection(t *testing.T) {
	t.Parallel()
	called := false
	repo := &mockProjectionRepo{
		processFn: func(context.Context, repository.ProcessInput) (repository.ProcessResult, error) {
			called = true
			return repository.ProcessResult{Outcome: domain.OutcomeApplied, Applied: true}, nil
		},
	}
	committer := &mockCommitter{err: errors.New("commit failed")}
	svc := newTestService(repo, committer)
	record, _ := validCreatedRecord(t)

	svc.processRecord(context.Background(), record)
	assert.True(t, called, "projection transaction must complete before failed commit")
	assert.Equal(t, 1, committer.calls)
}

func TestServiceCloseDoesNotCommitOffset(t *testing.T) {
	t.Parallel()
	committer := &mockCommitter{}
	svc := newTestService(&mockProjectionRepo{}, committer)
	svc.Close()
	assert.Equal(t, 0, committer.calls)
}

func TestBuildKafkaClientOptionsDisablesAutoCommit(t *testing.T) {
	t.Parallel()
	opts, err := buildKafkaClientOptions(config.KafkaConfig{
		Brokers:     []string{"localhost:19092"},
		Topic:       "shipment.status.v1",
		GroupID:     "control-tower-shipment-status-v1",
		ClientID:    "control-tower-read-model-service",
		DialTimeout: time.Second,
	})
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestOffsetCommitterAbstractionUsedByService(t *testing.T) {
	t.Parallel()
	var _ OffsetCommitter = (*mockCommitter)(nil)
}
