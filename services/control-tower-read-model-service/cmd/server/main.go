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

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/driverconsumer"
	httpserver "github.com/freight-platform/control-tower-read-model-service/internal/http"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/database"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/logger"
	ctmetrics "github.com/freight-platform/control-tower-read-model-service/internal/platform/metrics"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
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

	repo := repository.NewProjectionRepository(db.Pool)
	ackRepo := repository.NewAckRepository(db.Pool)
	workflowRepo := repository.NewWorkflowRepository(db.Pool)
	riskRepo := repository.NewRiskRepository(db.Pool)
	workItemRepo := repository.NewWorkItemRepository(db.Pool, workflowRepo, riskRepo)
	viewRepo := repository.NewViewRepository(db.Pool)
	handoffRepo := repository.NewHandoffRepository(db.Pool, workItemRepo, workflowRepo, riskRepo)
	caseRepo := repository.NewCaseRepository(db.Pool)
	automationRepo := repository.NewAutomationRepository(db.Pool)
	driverEventRepo := repository.NewDriverEventRepository(db.Pool)
	automationSvc := service.NewAutomationService(automationRepo)
	automationMetrics := ctmetrics.NewAutomationMetrics()
	automationIngress := service.NewAutomationTriggerIngress(automationSvc, automationMetrics, log)
	driverEventSvc := service.NewDriverEventService(driverEventRepo, workflowRepo, automationSvc, automationIngress, log)
	executionRepo := repository.NewExecutionProjectionRepository(db.Pool)
	driverEventSvc.SetExecutionLinker(executionRepo)
	driverEventMetrics := driverconsumer.NewMetrics()
	freshness := consumer.NewFreshness()
	consumerMetrics := ctmetrics.NewConsumerMetrics()
	shipmentOwner := handlers.HTTPShipmentOwner{BaseURL: cfg.ShipmentBaseURL, Token: cfg.InternalToken}

	router := httpserver.NewRouter(log, db.Pool, repo, ackRepo, workflowRepo, riskRepo, workItemRepo, viewRepo, handoffRepo, caseRepo, automationRepo, automationSvc, automationIngress, freshness, executionRepo, shipmentOwner)

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

	var consumerSvc *consumer.Service
	if cfg.Consumer.Enabled {
		kafkaClient, err := consumer.NewKafkaClient(cfg.Kafka)
		if err != nil {
			log.Error("failed to create kafka consumer", slog.String("error", cfg.Kafka.ErrorString(err)))
			os.Exit(1)
		}
		consumerSvc = consumer.NewService(kafkaClient, repo, cfg, log, consumerMetrics, freshness)
		consumerSvc.SetExecutionApplier(executionRepo)
		go func() {
			log.Info("starting shipment status consumer",
				slog.String("topic", cfg.Kafka.Topic),
				slog.String("group_id", cfg.Kafka.GroupID),
			)
			if err := consumerSvc.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("consumer stopped with error", slog.String("error", err.Error()))
			}
		}()
	} else {
		log.Info("shipment status consumer disabled")
	}

	var driverConsumerSvc *driverconsumer.Service
	if cfg.DriverConsumer.Enabled {
		kafkaClient, err := driverconsumer.NewKafkaClient(cfg.DriverConsumer.Kafka)
		if err != nil {
			log.Error("failed to create driver events kafka consumer", slog.String("error", err.Error()))
			os.Exit(1)
		}
		driverConsumerSvc = driverconsumer.NewService(kafkaClient, driverEventSvc, cfg, log, driverEventMetrics)
		go func() {
			log.Info("starting driver events consumer",
				slog.String("topic", cfg.DriverConsumer.Kafka.Topic),
				slog.String("group_id", cfg.DriverConsumer.Kafka.GroupID),
			)
			if err := driverConsumerSvc.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("driver events consumer stopped with error", slog.String("error", err.Error()))
			}
		}()
	} else {
		log.Info("driver events consumer disabled")
	}

	var trackingConsumerSvc *consumer.Service
	if cfg.TrackingConsumer.Enabled {
		trackCfg := cfg
		trackCfg.Kafka.Topic = cfg.TrackingConsumer.Kafka.Topic
		trackCfg.Kafka.GroupID = cfg.TrackingConsumer.Kafka.GroupID
		trackCfg.Kafka.ClientID = cfg.TrackingConsumer.Kafka.ClientID
		kafkaClient, err := consumer.NewKafkaClient(trackCfg.Kafka)
		if err != nil {
			log.Error("failed to create tracking consumer", slog.String("error", trackCfg.Kafka.ErrorString(err)))
			os.Exit(1)
		}
		trackingConsumerSvc = consumer.NewService(kafkaClient, repo, trackCfg, log, consumerMetrics, freshness)
		trackingConsumerSvc.SetExecutionApplier(executionRepo)
		go func() {
			log.Info("starting tracking approach consumer",
				slog.String("topic", trackCfg.Kafka.Topic),
				slog.String("group_id", trackCfg.Kafka.GroupID),
			)
			if err := trackingConsumerSvc.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("tracking consumer stopped with error", slog.String("error", err.Error()))
			}
		}()
	}

	<-ctx.Done()
	log.Info("shutdown signal received")

	if consumerSvc != nil {
		consumerSvc.Close()
	}
	if trackingConsumerSvc != nil {
		trackingConsumerSvc.Close()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("shutdown complete")
}
