package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/freight-platform/rfx-service/internal/config"
)

const maxScanCycles = 100

type ExpiredResponseProcessor interface {
	ProcessExpiredResponseDeadlines(ctx context.Context, now time.Time, batchSize int) (examined int, closed int, failures int, err error)
}

type DeadlineWorker struct {
	cfg       config.DeadlineWorkerConfig
	processor ExpiredResponseProcessor
	clock     Clock
	log       *slog.Logger
	metrics   *Metrics
	done      chan struct{}
}

func NewDeadlineWorker(
	cfg config.DeadlineWorkerConfig,
	processor ExpiredResponseProcessor,
	clock Clock,
	log *slog.Logger,
	metrics *Metrics,
) *DeadlineWorker {
	if clock == nil {
		clock = RealClock()
	}
	return &DeadlineWorker{
		cfg:       cfg,
		processor: processor,
		clock:     clock,
		log:       log,
		metrics:   metrics,
		done:      make(chan struct{}),
	}
}

func (w *DeadlineWorker) Enabled() bool {
	return w.cfg.Enabled
}

func (w *DeadlineWorker) Start(ctx context.Context) {
	defer close(w.done)
	if !w.cfg.Enabled {
		w.log.Info("deadline worker disabled")
		return
	}
	w.log.Info("deadline worker started",
		slog.Duration("interval", w.cfg.Interval),
		slog.Int("batch_size", w.cfg.BatchSize),
	)
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.log.Info("deadline worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *DeadlineWorker) runOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	w.metrics.IncRuns()
	now := w.clock.Now().UTC()
	w.log.Debug("deadline worker scan started", slog.Time("now", now))

	totalClosed := 0
	totalFailures := 0
	queryFailed := false
	for cycle := 0; cycle < maxScanCycles; cycle++ {
		if ctx.Err() != nil {
			return
		}
		examined, closed, failures, err := w.processor.ProcessExpiredResponseDeadlines(ctx, now, w.cfg.BatchSize)
		if err != nil {
			w.log.Warn("deadline worker scan query failed", slog.String("error", err.Error()))
			w.metrics.IncErrors()
			queryFailed = true
			break
		}
		totalClosed += closed
		totalFailures += failures
		for i := 0; i < failures; i++ {
			w.log.Warn("deadline worker event close failed")
			w.metrics.IncErrors()
		}
		for i := 0; i < closed; i++ {
			w.log.Info("deadline worker event closed")
		}
		if examined == 0 || examined < w.cfg.BatchSize {
			break
		}
	}
	if !queryFailed {
		w.metrics.MarkSuccess(float64(now.Unix()))
	}
	w.metrics.AddClosed(totalClosed)
	w.log.Debug("deadline worker scan completed",
		slog.Int("closed", totalClosed),
		slog.Int("failures", totalFailures),
	)
}

func (w *DeadlineWorker) Wait(ctx context.Context) error {
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunOnce executes a single scan cycle. Useful for integration tests.
func (w *DeadlineWorker) RunOnce(ctx context.Context) {
	w.runOnce(ctx)
}
