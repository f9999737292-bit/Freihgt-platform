package outbox

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/freight-platform/billing-register-service/internal/config"
	"github.com/freight-platform/billing-register-service/internal/domain"
)

type Worker struct {
	cfg       config.OutboxConfig
	repo      OutboxRepository
	publisher EventPublisher
	log       *slog.Logger
	clock     Clock
	done      chan struct{}
	closeDone sync.Once
	wg        sync.WaitGroup
}

func NewWorker(cfg config.OutboxConfig, repo OutboxRepository, publisher EventPublisher, log *slog.Logger, clock Clock) *Worker {
	if clock == nil {
		clock = NewRealClock()
	}
	if log == nil {
		log = slog.Default()
	}
	return &Worker{
		cfg:       cfg,
		repo:      repo,
		publisher: publisher,
		log:       log,
		clock:     clock,
		done:      make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	if !w.cfg.Enabled {
		w.closeDone.Do(func() { close(w.done) })
		return
	}
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.closeDone.Do(func() { close(w.done) })
	defer func() {
		if recovered := recover(); recovered != nil {
			w.log.Error("freight cost outbox worker panic recovered", slog.Any("panic", recovered))
		}
	}()

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	w.pollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollOnce(ctx)
		}
	}
}

func (w *Worker) pollOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	now := w.clock.Now()

	events, err := w.repo.ClaimPendingForPublisher(ctx, w.cfg.WorkerID, w.cfg.BatchSize, now, w.cfg.LeaseTimeout)
	if err != nil {
		w.log.Error("freight cost outbox claim failed", slog.String("error_code", ErrorCodeUnknownPublishError))
		return
	}
	for _, event := range events {
		w.publishOne(ctx, event)
	}
}

func (w *Worker) publishOne(parent context.Context, event domain.FreightCostOutboxEvent) {
	start := w.clock.Now()
	publishCtx, cancel := context.WithTimeout(parent, w.cfg.PublishTimeout)
	defer cancel()

	err := w.publisher.Publish(publishCtx, event)
	if err != nil {
		w.handlePublishFailure(parent, event, err)
		return
	}

	publishedAt := w.clock.Now()
	if err := w.repo.MarkPublished(parent, event.ID, w.cfg.WorkerID, publishedAt); err != nil {
		classified := ClassifyPublishError(err)
		w.log.Warn("freight cost outbox mark published failed",
			slog.String("tenant_id", event.TenantID.String()),
			slog.String("event_id", event.ID.String()),
			slog.String("aggregate_id", event.AggregateID.String()),
			slog.String("event_type", event.EventType),
			slog.Int("attempt", event.Attempts),
			slog.String("worker_id", w.cfg.WorkerID),
			slog.String("error_code", classified.Code),
		)
		return
	}

	w.log.Info("freight cost outbox event published",
		slog.String("tenant_id", event.TenantID.String()),
		slog.String("event_id", event.ID.String()),
		slog.String("aggregate_id", event.AggregateID.String()),
		slog.String("event_type", event.EventType),
		slog.Int("attempt", event.Attempts),
		slog.String("worker_id", w.cfg.WorkerID),
		slog.Int64("duration_ms", w.clock.Now().Sub(start).Milliseconds()),
	)
}

func (w *Worker) handlePublishFailure(ctx context.Context, event domain.FreightCostOutboxEvent, err error) {
	classified := ClassifyPublishError(err)
	now := w.clock.Now()

	if IsPermanentPublishError(classified.Code) || event.Attempts >= w.cfg.MaxAttempts {
		if markErr := w.repo.MarkFailed(ctx, event.ID, w.cfg.WorkerID, classified.Code); markErr != nil {
			w.log.Error("freight cost outbox mark failed error",
				slog.String("tenant_id", event.TenantID.String()),
				slog.String("event_id", event.ID.String()),
				slog.String("error_code", ErrorCodePublishStateConflict),
			)
			return
		}
		w.log.Error("freight cost outbox event permanently failed",
			slog.String("tenant_id", event.TenantID.String()),
			slog.String("event_id", event.ID.String()),
			slog.String("aggregate_id", event.AggregateID.String()),
			slog.String("event_type", event.EventType),
			slog.Int("attempt", event.Attempts),
			slog.String("worker_id", w.cfg.WorkerID),
			slog.String("error_code", classified.Code),
		)
		return
	}

	availableAt := NextRetryAvailableAt(event.Attempts, now)
	if releaseErr := w.repo.ReleaseWithRetry(ctx, event.ID, w.cfg.WorkerID, availableAt, classified.Code); releaseErr != nil {
		w.log.Error("freight cost outbox release retry failed",
			slog.String("tenant_id", event.TenantID.String()),
			slog.String("event_id", event.ID.String()),
			slog.String("error_code", ErrorCodePublishStateConflict),
		)
		return
	}
	w.log.Warn("freight cost outbox publish retry scheduled",
		slog.String("tenant_id", event.TenantID.String()),
		slog.String("event_id", event.ID.String()),
		slog.String("aggregate_id", event.AggregateID.String()),
		slog.String("event_type", event.EventType),
		slog.Int("attempt", event.Attempts),
		slog.String("worker_id", w.cfg.WorkerID),
		slog.String("error_code", classified.Code),
	)
}

func (w *Worker) Wait(ctx context.Context) error {
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
