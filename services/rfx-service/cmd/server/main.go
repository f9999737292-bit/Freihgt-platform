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

	"github.com/freight-platform/rfx-service/internal/config"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	"github.com/freight-platform/rfx-service/internal/platform/database"
	"github.com/freight-platform/rfx-service/internal/platform/logger"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/transportorderclient"
	"github.com/freight-platform/rfx-service/internal/worker"
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

	rfxRepo := repository.NewRfxRepository(db.Pool)
	qRepo := repository.NewQuestionnaireRepository(db.Pool)
	answerRepo := repository.NewAnswerRepository(db.Pool)
	scoreRepo := repository.NewScoreRepository(db.Pool)
	frRepo := repository.NewFreightRequestRepository(db.Pool)
	bidRepo := repository.NewBidRepository(db.Pool)
	membershipRepo := repository.NewMembershipRepository(db.Pool)
	auditRepo := repository.NewAuditRepository(db.Pool)

	toClient := transportorderclient.New(transportorderclient.Config{
		BaseURL:              cfg.TransportOrderServiceURL,
		InternalServiceToken: cfg.InternalServiceToken,
	})

	rfxSvc := service.NewRfxServiceWithAtomic(db.Pool, rfxRepo, auditRepo, membershipRepo, toClient)
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	scoringSvc := service.NewScoringService(db.Pool, rfxRepo, answerRepo, qRepo, scoreRepo, auditRepo)
	crSvc := service.NewCarrierResponseServiceWithScoring(db.Pool, rfxRepo, answerRepo, qRepo, auditRepo, membershipRepo, rfxSvc, scoringSvc)
	scoreModelSvc := service.NewScoreModelService(rfxRepo, scoreRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	frSvc := service.NewFreightRequestServiceWithAuth(frRepo, membershipRepo)
	bidSvc := service.NewBidServiceWithAtomic(db.Pool, bidRepo, frRepo, membershipRepo, auditRepo)

	pricingRepo := repository.NewPricingRepository(db.Pool)
	pricingSvc := service.NewPricingService(pricingRepo)

	deadlineMetrics := worker.NewMetrics(cfg.ServiceName)
	deadlineWorker := worker.NewDeadlineWorker(cfg.DeadlineWorker, rfxSvc, worker.RealClock(), log, deadlineMetrics)

	router := httpserver.NewRouter(log, db.Pool, cfg, rfxSvc, qSvc, crSvc, scoreModelSvc, scoringSvc, frSvc, bidSvc, pricingSvc)

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

	if cfg.DeadlineWorker.Enabled {
		go deadlineWorker.Start(ctx)
	} else {
		log.Info("deadline worker disabled by configuration")
	}

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
