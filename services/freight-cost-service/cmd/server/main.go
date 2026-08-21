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

	"github.com/freight-platform/freight-cost-service/internal/client/transport_order"
	"github.com/freight-platform/freight-cost-service/internal/config"
	httpserver "github.com/freight-platform/freight-cost-service/internal/http"
	"github.com/freight-platform/freight-cost-service/internal/platform/logger"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := logger.New(cfg.ServiceName, cfg.LogLevel, cfg.Environment)
	domainMetrics := fcmetrics.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	transportClient := transport_order.NewClient(cfg.TransportOrderURL, cfg.InternalServiceToken, domainMetrics)
	costSvc := service.NewCostService(transportClient)
	router := httpserver.NewRouter(log, cfg, costSvc, domainMetrics)

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
