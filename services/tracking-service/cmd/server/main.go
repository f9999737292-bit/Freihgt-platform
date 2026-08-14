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
	"github.com/freight-platform/tracking-service/internal/config"
	httpserver "github.com/freight-platform/tracking-service/internal/http"
	"github.com/freight-platform/tracking-service/internal/http/handlers"
	trackingmetrics "github.com/freight-platform/tracking-service/internal/metrics"
	"github.com/freight-platform/tracking-service/internal/platform/database"
	"github.com/freight-platform/tracking-service/internal/platform/logger"
	"github.com/freight-platform/tracking-service/internal/provider"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
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

	trackingRepo := repository.NewTrackingRepository(db.Pool)
	etaRepo := repository.NewETARepository(db.Pool)
	registry := provider.NewRegistry(provider.GenericAdapter{}, provider.DriverMobileAdapter{})
	etaRegistry := provider.NewETARegistry(provider.GenericETAAdapter{})
	evaluator := service.NewStateEvaluator(trackingRepo, cfg)
	etaEvaluator := service.NewETAStateEvaluator(etaRepo, cfg)
	querySvc := service.NewTrackingQueryService(trackingRepo, evaluator, cfg)
	etaQuerySvc := service.NewETAQueryService(etaRepo, etaEvaluator)
	telemetryMetrics := trackingmetrics.New(cfg.ServiceName)
	ingestSvc := service.NewIngestService(trackingRepo, registry, cfg, evaluator, log, telemetryMetrics)
	etaIngestSvc := service.NewETAIngestService(trackingRepo, etaRepo, etaRegistry, cfg, etaEvaluator, log, telemetryMetrics)

	trackingHandler := handlers.NewTrackingHandler(querySvc)
	etaHandler := handlers.NewETAHandler(etaQuerySvc)
	etaInternal := handlers.NewETAInternalHandler(etaQuerySvc, cfg.InternalServiceToken)
	ingestHandler := handlers.NewIngestHandler(ingestSvc, etaIngestSvc, cfg.ProviderSecrets)
	internalHandler := handlers.NewInternalHandler(querySvc, cfg.InternalServiceToken)
	metricsCollector := metrics.New(cfg.ServiceName)

	router := httpserver.NewRouter(log, db.Pool, trackingHandler, etaHandler, etaInternal, ingestHandler, internalHandler, metricsCollector)

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
	log.Info("shutdown complete")
}
