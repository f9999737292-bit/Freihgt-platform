package consumer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
)

func TestClassifyPollErrorNoRecordsIsNotError(t *testing.T) {
	t.Parallel()
	parent := context.Background()
	pollCtx, cancel := context.WithTimeout(parent, time.Millisecond)
	defer cancel()
	<-pollCtx.Done()

	outcome, code := ClassifyPollError(parent, pollCtx, context.DeadlineExceeded)
	assert.Equal(t, PollOutcomeIdle, outcome)
	assert.Empty(t, code)
}

func TestClassifyPollErrorPollTimeoutIsNotError(t *testing.T) {
	t.Parallel()
	parent := context.Background()
	outcome, code := ClassifyPollError(parent, parent, context.DeadlineExceeded)
	assert.Equal(t, PollOutcomeIdle, outcome)
	assert.Empty(t, code)
}

func TestClassifyPollErrorShutdownCancellationIsNotError(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	outcome, code := ClassifyPollError(parent, parent, context.Canceled)
	assert.Equal(t, PollOutcomeShutdown, outcome)
	assert.Empty(t, code)
}

func TestClassifyPollErrorPollContextCanceledWhileParentAliveIsIdle(t *testing.T) {
	t.Parallel()
	parent := context.Background()
	pollCtx, cancel := context.WithCancel(parent)
	cancel()

	outcome, code := ClassifyPollError(parent, pollCtx, context.Canceled)
	assert.Equal(t, PollOutcomeIdle, outcome)
	assert.Empty(t, code)
}

func TestClassifyPollErrorBrokerUnavailable(t *testing.T) {
	t.Parallel()
	outcome, code := ClassifyPollError(context.Background(), context.Background(), syscall.ECONNREFUSED)
	assert.Equal(t, PollOutcomeError, outcome)
	assert.Equal(t, ErrorCodeBrokerUnavailable, code)
}

func TestClassifyPollErrorNetworkError(t *testing.T) {
	t.Parallel()
	err := &net.OpError{Op: "dial", Err: errors.New("timeout")}
	outcome, code := ClassifyPollError(context.Background(), context.Background(), err)
	assert.Equal(t, PollOutcomeError, outcome)
	assert.Equal(t, ErrorCodeFetchNetworkError, code)
}

func TestClassifyPollErrorAuthorizationFailure(t *testing.T) {
	t.Parallel()
	outcome, code := ClassifyPollError(context.Background(), context.Background(), errors.New("SASL authentication failed"))
	assert.Equal(t, PollOutcomeError, outcome)
	assert.Equal(t, ErrorCodeAuthorizationError, code)
}

func TestClassifyPollErrorUnknown(t *testing.T) {
	t.Parallel()
	outcome, code := ClassifyPollError(context.Background(), context.Background(), errors.New("unexpected fetch state"))
	assert.Equal(t, PollOutcomeError, outcome)
	assert.Equal(t, ErrorCodeUnknownPollError, code)
}

func TestRunPollTransientErrorUsesBoundedBackoff(t *testing.T) {
	var polls atomic.Int32
	svc := &Service{
		repo:      &mockProjectionRepo{},
		committer: &mockCommitter{},
		cfg: config.Config{
			Kafka: config.KafkaConfig{Topic: "shipment.status.v1"},
			Consumer: config.ConsumerConfig{
				PollTimeout:    50 * time.Millisecond,
				ProcessTimeout: time.Second,
				CommitTimeout:  time.Second,
			},
		},
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics:   ctmetrics.NewConsumerMetrics(),
		freshness: NewFreshness(),
		topic:     "shipment.status.v1",
		pollFetches: func(context.Context) kgo.Fetches {
			polls.Add(1)
			return fetchWithError(syscall.ECONNREFUSED)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()

	err := svc.Run(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.GreaterOrEqual(t, polls.Load(), int32(1))
	assert.LessOrEqual(t, polls.Load(), int32(4))
}

func TestRunPollRecoveryProcessesNextRecord(t *testing.T) {
	var polls atomic.Int32
	repo := &mockProjectionRepo{}
	record, _ := validCreatedRecord(t)
	svc := &Service{
		repo:      repo,
		committer: &mockCommitter{},
		cfg: config.Config{
			Kafka: config.KafkaConfig{Topic: "shipment.status.v1"},
			Consumer: config.ConsumerConfig{
				PollTimeout:    10 * time.Millisecond,
				ProcessTimeout: time.Second,
				CommitTimeout:  time.Second,
			},
		},
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics:   ctmetrics.NewConsumerMetrics(),
		freshness: NewFreshness(),
		topic:     "shipment.status.v1",
		pollFetches: func(context.Context) kgo.Fetches {
			if polls.Add(1) == 1 {
				return fetchWithError(syscall.ECONNREFUSED)
			}
			return kgo.Fetches{{
				Topics: []kgo.FetchTopic{{
					Topic: "shipment.status.v1",
					Partitions: []kgo.FetchPartition{
						{Partition: 0, Records: []*kgo.Record{record}},
					},
				}},
			}}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- svc.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return repo.processCalls >= 1
	}, time.Second, 10*time.Millisecond)

	cancel()
	<-done
	assert.GreaterOrEqual(t, repo.processCalls, 1)
}

func fetchWithError(err error) kgo.Fetches {
	return kgo.Fetches{{
		Topics: []kgo.FetchTopic{{
			Topic: "shipment.status.v1",
			Partitions: []kgo.FetchPartition{{
				Partition: 0,
				Err:       err,
			}},
		}},
	}}
}
