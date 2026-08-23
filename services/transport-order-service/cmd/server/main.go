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

	"github.com/freight-platform/transport-order-service/internal/client/contractrate"
	"github.com/freight-platform/transport-order-service/internal/config"
	httpserver "github.com/freight-platform/transport-order-service/internal/http"
	"github.com/freight-platform/transport-order-service/internal/platform/database"
	"github.com/freight-platform/transport-order-service/internal/platform/logger"
	"github.com/freight-platform/transport-order-service/internal/repository"
	"github.com/freight-platform/transport-order-service/internal/service"
	toobs "github.com/freight-platform/transport-order-service/internal/observability"
	"github.com/freight-platform/shared-go/metrics"
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

	locationRepo := repository.NewLocationRepository(db.Pool)
	cargoRepo := repository.NewCargoRepository(db.Pool)
	orderRepo := repository.NewTransportOrderRepository(db.Pool)
	pricedOrderRepo := repository.NewPricedOrderRepository(db.Pool)
	rateClient := contractrate.New(contractrate.Config{
		BaseURL:              cfg.ContractRateServiceURL,
		InternalServiceToken: cfg.InternalServiceToken,
	})
	svc := service.NewTransportOrderService(locationRepo, cargoRepo, orderRepo, locationRepo)
	pricingMetrics := toobs.NewPricingMetrics(cfg.ServiceName)
	pricedSvc := service.NewPricedTransportOrderService(orderRepo, cargoRepo, locationRepo, pricedOrderRepo, rateClient, pricingMetrics)
	snapshotReadSvc := service.NewRateSnapshotReadService(pricedOrderRepo)
	analyticsDimensionRepo := repository.NewTransportOrderAnalyticsDimensionRepository(db.Pool)
	analyticsDimensionSvc := service.NewTransportOrderAnalyticsDimensionService(analyticsDimensionRepo)
	router := httpserver.NewRouter(log, db.Pool, cfg, svc, pricedSvc, snapshotReadSvc, analyticsDimensionSvc)

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
