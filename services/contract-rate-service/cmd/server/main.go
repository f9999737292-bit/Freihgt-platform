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

	"github.com/freight-platform/contract-rate-service/internal/config"
	httpserver "github.com/freight-platform/contract-rate-service/internal/http"
	"github.com/freight-platform/contract-rate-service/internal/http/handlers"
	"github.com/freight-platform/contract-rate-service/internal/observability"
	"github.com/freight-platform/contract-rate-service/internal/platform/database"
	"github.com/freight-platform/contract-rate-service/internal/platform/logger"
	"github.com/freight-platform/contract-rate-service/internal/repository"
	"github.com/freight-platform/contract-rate-service/internal/rfxclient"
	"github.com/freight-platform/contract-rate-service/internal/service"
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

	auditRepo := repository.NewAuditRepository()
	contractRepo := repository.NewContractRepository(db.Pool, auditRepo)
	rateCardRepo := repository.NewRateCardRepository(db.Pool, contractRepo, auditRepo)
	locationRepo := repository.NewLocationRepository(db.Pool)
	rateLineRepo := repository.NewRateLineRepository(db.Pool, rateCardRepo, locationRepo, auditRepo)
	rateComponentRepo := repository.NewRateComponentRepository(db.Pool, rateLineRepo, rateCardRepo, auditRepo)
	resolutionRepo := repository.NewResolutionRepository(db.Pool, auditRepo)
	membershipRepo := repository.NewMembershipRepository(db.Pool)
	rateMetrics := observability.NewMetrics(cfg.ServiceName)

	contractSvc := service.NewContractService(contractRepo, membershipRepo)
	rateCardSvc := service.NewRateCardService(rateCardRepo, contractRepo)
	rateLineSvc := service.NewRateLineService(rateLineRepo, rateCardRepo, contractRepo)
	rateComponentSvc := service.NewRateComponentService(rateComponentRepo, rateLineRepo, rateCardRepo, contractRepo)
	rfxClient := rfxclient.New(rfxclient.Config{
		BaseURL:              cfg.RFXServiceURL,
		InternalServiceToken: cfg.InternalServiceToken,
	})
	resolutionSvc := service.NewResolutionService(resolutionRepo, membershipRepo, rfxClient, rateMetrics)
	actorResolver := handlers.NewActorResolver(membershipRepo)

	router := httpserver.NewRouter(
		log, db.Pool, cfg,
		contractSvc, rateCardSvc, rateLineSvc, rateComponentSvc, resolutionSvc, actorResolver,
	)

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
