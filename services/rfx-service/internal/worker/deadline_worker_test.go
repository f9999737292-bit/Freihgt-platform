package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/freight-platform/rfx-service/internal/config"
)

type mockProcessor struct {
	calls      atomic.Int32
	examined   []int
	closed     []int
	failures   []int
	queryErr   error
	eventErrAt int
}

func (m *mockProcessor) ProcessExpiredResponseDeadlines(_ context.Context, _ time.Time, batchSize int) (int, int, int, error) {
	m.calls.Add(1)
	if m.queryErr != nil {
		return 0, 0, 0, m.queryErr
	}
	call := int(m.calls.Load())
	examined := batchSize
	if call > 1 {
		examined = 0
	}
	closed := 0
	failures := 0
	if call == 1 {
		closed = 2
	}
	if m.eventErrAt > 0 && call == m.eventErrAt {
		failures = 1
	}
	m.examined = append(m.examined, examined)
	m.closed = append(m.closed, closed)
	m.failures = append(m.failures, failures)
	return examined, closed, failures, nil
}

func testWorkerConfig(enabled bool, interval time.Duration, batchSize int) config.DeadlineWorkerConfig {
	return config.DeadlineWorkerConfig{
		Enabled:   enabled,
		Interval:  interval,
		BatchSize: batchSize,
	}
}

func newTestWorker(t *testing.T, cfg config.DeadlineWorkerConfig, processor ExpiredResponseProcessor) *DeadlineWorker {
	t.Helper()
	return NewDeadlineWorker(cfg, processor, RealClock(), slog.New(slog.NewTextHandler(io.Discard, nil)), NewMetrics("rfx-service-test"))
}

func TestWorkerDisabledDoesNotRun(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{}
	worker := newTestWorker(t, testWorkerConfig(false, time.Second, 10), processor)
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	_ = worker.Wait(context.Background())
	if processor.calls.Load() != 0 {
		t.Fatalf("calls=%d", processor.calls.Load())
	}
}

func TestWorkerEnabledRunsScan(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{}
	cfg := testWorkerConfig(true, 20*time.Millisecond, 10)
	worker := newTestWorker(t, cfg, processor)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()
	deadline := time.After(200 * time.Millisecond)
	for processor.calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("expected at least one scan")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	cancel()
	<-done
}

func TestWorkerProcessesMultipleBatchesUntilEmpty(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{}
	worker := newTestWorker(t, testWorkerConfig(true, time.Second, 5), processor)
	worker.RunOnce(context.Background())
	if processor.calls.Load() != 2 {
		t.Fatalf("calls=%d want 2 (full batch then empty)", processor.calls.Load())
	}
	if len(processor.closed) != 2 || processor.closed[0] != 2 || processor.closed[1] != 0 {
		t.Fatalf("closed=%v", processor.closed)
	}
}

func TestWorkerDBErrorDoesNotPanic(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{queryErr: errors.New("db down")}
	worker := newTestWorker(t, testWorkerConfig(true, time.Second, 10), processor)
	worker.RunOnce(context.Background())
}

func TestWorkerContextCancelStopsWorker(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{}
	worker := newTestWorker(t, testWorkerConfig(true, time.Hour, 10), processor)
	ctx, cancel := context.WithCancel(context.Background())
	go worker.Start(ctx)
	worker.RunOnce(ctx)
	cancel()
	if err := worker.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

func TestWorkerEventErrorIsolationContinuesBatch(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{eventErrAt: 1}
	worker := newTestWorker(t, testWorkerConfig(true, time.Second, 10), processor)
	worker.RunOnce(context.Background())
	if len(processor.failures) == 0 || processor.failures[0] != 1 {
		t.Fatalf("failures=%v", processor.failures)
	}
}

func TestWorkerBatchLimitRespected(t *testing.T) {
	t.Parallel()
	processor := &mockProcessor{}
	worker := newTestWorker(t, testWorkerConfig(true, time.Second, 7), processor)
	worker.RunOnce(context.Background())
	if processor.calls.Load() != 2 {
		t.Fatalf("calls=%d", processor.calls.Load())
	}
}
