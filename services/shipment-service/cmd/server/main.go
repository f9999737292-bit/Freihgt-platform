package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shipment-service/internal/config"
	httpserver "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/outbox"
	"github.com/freight-platform/shipment-service/internal/platform/database"
	"github.com/freight-platform/shipment-service/internal/platform/logger"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := logger.New(cfg.ServiceName, cfg.LogLevel, cfg.Environment)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()
	metrics.RegisterPgxPoolMetrics(cfg.ServiceName, db.Pool)

	shipmentRepo := repository.NewShipmentRepository(db.Pool)
	driverRepo := repository.NewDriverRepository(db.Pool)
	vehicleRepo := repository.NewVehicleRepository(db.Pool)

	shipmentSvc := service.NewShipmentService(shipmentRepo, driverRepo, vehicleRepo)
	statusHistorySvc := service.NewStatusHistoryService(shipmentRepo)
	statusSummaryRepo := repository.NewShipmentStatusSummaryRepository(db.Pool)
	statusSummarySvc := service.NewStatusSummaryService(statusSummaryRepo)
	driverSvc := service.NewDriverService(driverRepo)
	vehicleSvc := service.NewVehicleService(vehicleRepo)

	var outboxWorker *outbox.Worker
	var outboxPublisher outbox.EventPublisher
	if cfg.Outbox.Enabled {
		publisher, err := outbox.NewPublisher(cfg.Outbox)
		if err != nil {
			log.Error("failed to configure outbox publisher", slog.String("error", err.Error()))
			os.Exit(1)
		}
		outboxPublisher = publisher
		outboxWorker = outbox.NewWorker(cfg.Outbox, shipmentRepo, publisher, log, outbox.NewRealClock())
		outboxWorker.Start(ctx)
		log.Info("outbox worker started",
			slog.String("worker_id", cfg.Outbox.WorkerID),
			slog.String("transport", cfg.Outbox.Transport),
		)
	}

	router := httpserver.NewRouter(log, db.Pool, shipmentSvc, statusHistorySvc, statusSummarySvc, driverSvc, vehicleSvc)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("starting http server", slog.Int("port", cfg.HTTPPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if outboxWorker != nil {
		workerWaitCtx, workerCancel := context.WithTimeout(context.Background(), cfg.Outbox.PublishTimeout+cfg.Outbox.PollInterval)
		defer workerCancel()
		if err := outboxWorker.Wait(workerWaitCtx); err != nil {
			log.Warn("outbox worker shutdown timed out", slog.String("error", err.Error()))
		}
	}

	if closer, ok := outboxPublisher.(outbox.CloseablePublisher); ok {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		if err := closer.Close(closeCtx); err != nil {
			log.Warn("outbox publisher close timed out", slog.String("error", err.Error()))
		}
	}

	log.Info("shutdown complete")
}
