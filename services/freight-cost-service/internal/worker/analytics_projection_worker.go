package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/config"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type AnalyticsProjectionWorker struct {
	cfg       config.AnalyticsProjectionConfig
	analytics *service.AnalyticsProjectionService
	state     *repository.AnalyticsProjectionStateRepository
	log       *slog.Logger
	done      chan struct{}
	closeDone sync.Once
	wg        sync.WaitGroup
}

func NewAnalyticsProjectionWorker(
	cfg config.AnalyticsProjectionConfig,
	analytics *service.AnalyticsProjectionService,
	state *repository.AnalyticsProjectionStateRepository,
	log *slog.Logger,
) *AnalyticsProjectionWorker {
	if log == nil {
		log = slog.Default()
	}
	return &AnalyticsProjectionWorker{
		cfg:       cfg,
		analytics: analytics,
		state:     state,
		log:       log,
		done:      make(chan struct{}),
	}
}

func (w *AnalyticsProjectionWorker) Start(ctx context.Context) {
	if !w.cfg.Enabled || w.analytics == nil {
		w.closeDone.Do(func() { close(w.done) })
		return
	}
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *AnalyticsProjectionWorker) Wait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-w.done:
	}
	w.wg.Wait()
}

func (w *AnalyticsProjectionWorker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.closeDone.Do(func() { close(w.done) })

	dirtyTicker := time.NewTicker(w.cfg.DirtyPollInterval)
	rebuildTicker := time.NewTicker(w.cfg.RebuildInterval)
	defer dirtyTicker.Stop()
	defer rebuildTicker.Stop()

	w.pollDirtyOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-dirtyTicker.C:
			w.pollDirtyOnce(ctx)
		case <-rebuildTicker.C:
			w.rebuildAllTenantsOnce(ctx)
		}
	}
}

func (w *AnalyticsProjectionWorker) pollDirtyOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	processed, err := w.analytics.ProcessDirtyBatch(ctx, w.cfg.DirtyBatchSize)
	if err != nil {
		w.log.Error("analytics dirty batch failed", slog.String("error", err.Error()))
		return
	}
	if processed > 0 {
		w.log.Info("analytics dirty batch processed", slog.Int("count", processed))
	}
}

func (w *AnalyticsProjectionWorker) rebuildAllTenantsOnce(ctx context.Context) {
	if ctx.Err() != nil || w.state == nil {
		return
	}
	tenantIDs, err := w.state.ListTenantIDs(ctx)
	if err != nil {
		w.log.Error("list analytics rebuild tenants failed", slog.String("error", err.Error()))
		return
	}
	for _, tenantID := range tenantIDs {
		if err := w.analytics.RebuildTenant(ctx, tenantID); err != nil {
			w.log.Error("scheduled analytics tenant rebuild failed",
				slog.String("tenant_id", tenantID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}
		w.log.Info("scheduled analytics tenant rebuild completed", slog.String("tenant_id", tenantID.String()))
	}
}

func (w *AnalyticsProjectionWorker) RebuildTenantNow(ctx context.Context, tenantID uuid.UUID) error {
	if w.analytics == nil {
		return nil
	}
	return w.analytics.RebuildTenant(ctx, tenantID)
}
