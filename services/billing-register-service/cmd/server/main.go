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

	"github.com/freight-platform/billing-register-service/internal/client"
	"github.com/freight-platform/billing-register-service/internal/client/freight_cost"
	"github.com/freight-platform/billing-register-service/internal/config"
	httpserver "github.com/freight-platform/billing-register-service/internal/http"
	"github.com/freight-platform/billing-register-service/internal/http/handlers"
	"github.com/freight-platform/billing-register-service/internal/outbox"
	"github.com/freight-platform/billing-register-service/internal/platform/database"
	"github.com/freight-platform/billing-register-service/internal/platform/logger"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
	billingobs "github.com/freight-platform/billing-register-service/internal/observability"
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

	outboxRepo := repository.NewFreightCostOutboxRepository(db.Pool)
	outboxEmitter := repository.NewFreightCostOutboxEmitter(outboxRepo)

	registerRepo := repository.NewBillingRegisterRepository(db.Pool)
	registerRepo.SetOutboxEmitter(outboxEmitter)
	closingRepo := repository.NewClosingDocumentRepository(db.Pool)
	settlementMetrics := billingobs.NewSettlementMetrics(cfg.ServiceName)
	settlementRepo := repository.NewFreightSettlementRepository(db.Pool, settlementMetrics)
	settlementRepo.SetOutboxEmitter(outboxEmitter)
	membershipRepo := repository.NewMembershipRepository(db.Pool)
	obligationLookup := repository.NewPaymentObligationLookupRepository(db.Pool)
	paymentClient := client.NewPaymentServiceClient(cfg.PaymentServiceURL, cfg.InternalServiceToken)

	registerSvc := service.NewBillingRegisterServiceWithPayments(registerRepo, obligationLookup, paymentClient)
	closingSvc := service.NewClosingDocumentService(registerRepo, closingRepo)
	settlementSvc := service.NewFreightSettlementService(settlementRepo)
	actorResolver := handlers.NewSettlementActorResolver(membershipRepo)

	var outboxWorker *outbox.Worker
	if cfg.Outbox.Enabled {
		publisher := outbox.NewRouterPublisher(freight_cost.NewClient(cfg.FreightCostServiceURL, cfg.InternalServiceToken))
		outboxWorker = outbox.NewWorker(cfg.Outbox, outboxRepo, publisher, log, outbox.NewRealClock())
		outboxWorker.Start(ctx)
		log.Info("freight cost outbox worker started", slog.String("worker_id", cfg.Outbox.WorkerID))
	}

	router := httpserver.NewRouter(log, db.Pool, cfg, registerSvc, closingSvc, settlementSvc, settlementRepo, registerRepo, actorResolver)

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
			log.Warn("freight cost outbox worker shutdown wait ended", slog.String("error", err.Error()))
		}
	}

	log.Info("shutdown complete")
}
