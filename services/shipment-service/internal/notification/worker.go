package notification

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/freight-platform/shipment-service/internal/push"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type WorkerConfig struct {
	Enabled      bool
	WorkerID     string
	PollInterval time.Duration
	BatchSize    int
	LeaseTimeout time.Duration
	MaxAttempts  int
	RetryBackoff time.Duration
}

type Worker struct {
	cfg      WorkerConfig
	devices  *repository.DriverDeviceRepository
	tasks    *repository.DriverTaskRepository
	provider push.Provider
	log      *slog.Logger
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewWorker(cfg WorkerConfig, devices *repository.DriverDeviceRepository, tasks *repository.DriverTaskRepository, provider push.Provider, log *slog.Logger) *Worker {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}
	if cfg.LeaseTimeout <= 0 {
		cfg.LeaseTimeout = 30 * time.Second
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 5 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	return &Worker{cfg: cfg, devices: devices, tasks: tasks, provider: provider, log: log, done: make(chan struct{})}
}

func (w *Worker) Start(ctx context.Context) {
	if !w.cfg.Enabled {
		close(w.done)
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer close(w.done)
		ticker := time.NewTicker(w.cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.pollOnce(ctx)
			}
		}
	}()
}

func (w *Worker) Wait() { <-w.done }

func (w *Worker) pollOnce(ctx context.Context) {
	if w.tasks != nil {
		if _, err := w.tasks.ExpireDueTasks(ctx, w.cfg.BatchSize); err != nil && w.log != nil {
			w.log.Warn("expire driver tasks failed", slog.String("error", err.Error()))
		}
	}
	now := time.Now().UTC()
	_ = w.devices.ReleaseStaleClaims(ctx, now)
	deliveries, err := w.devices.ClaimPendingDeliveries(ctx, w.cfg.WorkerID, w.cfg.BatchSize, now, w.cfg.LeaseTimeout)
	if err != nil {
		w.log.Warn("claim notification deliveries failed", slog.String("error", err.Error()))
		return
	}
	for _, delivery := range deliveries {
		w.processDelivery(ctx, delivery)
	}
}

func (w *Worker) processDelivery(ctx context.Context, delivery repository.NotificationDelivery) {
	if delivery.AttemptCount >= delivery.MaxAttempts {
		_ = w.devices.MarkDeliveryFailed(ctx, delivery.ID, "MAX_ATTEMPTS", nil, true)
		return
	}
	targets, err := w.devices.ListActivePushTargets(ctx, delivery.TenantID, delivery.DriverID)
	if err != nil {
		w.log.Warn("list push targets failed", slog.String("error", err.Error()))
		return
	}
	if len(targets) == 0 {
		_ = w.devices.MarkDeliveryNoDevice(ctx, delivery.ID)
		return
	}
	if w.provider == nil || !w.provider.Available() {
		retry := time.Now().UTC().Add(w.cfg.RetryBackoff)
		_ = w.devices.MarkDeliveryFailed(ctx, delivery.ID, "PROVIDER_UNAVAILABLE", &retry, false)
		return
	}

	var lastErr error
	sent := false
	for _, target := range targets {
		_, err := w.provider.Send(ctx, push.Message{
			TaskID:   delivery.TaskID.String(),
			TaskType: delivery.TaskType,
			Title:    delivery.TaskTitle,
			Token:    target.Token,
		})
		if err != nil {
			lastErr = err
			permanent, _ := push.ClassifyError(err)
			if permanent {
				_ = w.devices.RevokeByTokenHash(ctx, delivery.TenantID, target.TokenHash)
			}
			continue
		}
		sent = true
	}
	if sent {
		_ = w.devices.MarkDeliverySent(ctx, delivery.ID, delivery.TaskID.String())
		_ = w.devices.MarkTaskDelivered(ctx, delivery.TenantID, delivery.TaskID)
		return
	}
	retry := time.Now().UTC().Add(w.cfg.RetryBackoff)
	permanent := false
	errorCode := "TRANSIENT"
	if lastErr != nil {
		permanent, errorCode = push.ClassifyError(lastErr)
	}
	_ = w.devices.MarkDeliveryFailed(ctx, delivery.ID, errorCode, &retry, permanent && errorCode == "INVALID_TOKEN")
}

// ProcessOnce exposes single poll for tests.
func (w *Worker) ProcessOnce(ctx context.Context) { w.pollOnce(ctx) }
