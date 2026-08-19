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

	"github.com/freight-platform/payment-service/internal/config"
	httpserver "github.com/freight-platform/payment-service/internal/http"
	"github.com/freight-platform/payment-service/internal/http/handlers"
	"github.com/freight-platform/payment-service/internal/outbox"
	"github.com/freight-platform/payment-service/internal/platform/database"
	"github.com/freight-platform/payment-service/internal/platform/logger"
	"github.com/freight-platform/payment-service/internal/repository"
	"github.com/freight-platform/payment-service/internal/service"
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

	paymentRepo := repository.NewPaymentRepository(db.Pool)
	outboxRepo := repository.NewOutboxRepository(db.Pool)
	registerLookup := repository.NewBillingRegisterLookupRepository(db.Pool)
	membershipRepo := repository.NewMembershipRepository(db.Pool)
	billingClient := service.NewBillingRegisterHTTPClient(cfg.BillingRegisterURL, cfg.InternalServiceToken)

	paymentSvc := service.NewPaymentService(paymentRepo, registerLookup, membershipRepo, billingClient, outboxRepo)
	actorResolver := handlers.NewPaymentActorResolver(membershipRepo)

	var outboxWorker *outbox.Worker
	if cfg.Outbox.Enabled {
		publisher := outbox.NewPublisher(billingClient)
		outboxWorker = outbox.NewWorker(cfg.Outbox, outboxRepo, publisher, log, outbox.NewRealClock())
		outboxWorker.Start(ctx)
		log.Info("payment outbox worker started", slog.String("worker_id", cfg.Outbox.WorkerID))
	}

	router := httpserver.NewRouter(log, db.Pool, cfg, paymentSvc, actorResolver)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Outbox.PublishTimeout+cfg.Outbox.PollInterval+5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if outboxWorker != nil {
		if err := outboxWorker.Wait(shutdownCtx); err != nil {
			log.Warn("payment outbox worker shutdown wait ended", slog.String("error", err.Error()))
		}
	}

	log.Info("shutdown complete")
}
