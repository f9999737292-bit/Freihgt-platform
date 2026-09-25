package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/config"
	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/locationclient"
	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing/twogis"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/network-optimizer-service/internal/sourceclient"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed")
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed")
		os.Exit(1)
	}
	defer pool.Close()
	store := repository.NewPostgres(pool)
	verifier := sourceverify.NewHTTP(cfg.TransportOrderURL, cfg.ShipmentURL, cfg.InternalServiceToken)
	svc := service.New(store, verifier)
	svc.UseDirectory(locationclient.NewHTTP(cfg.TransportOrderURL, cfg.ShipmentURL, cfg.InternalServiceToken))
	svc.UsePolicies(store)
	svc.UseScoreProfiles(store)
	if cfg.RoutingProvider == "2GIS" {
		svc.UseRouting(twogis.New(cfg.TwoGISRoutingBaseURL, cfg.TwoGISAPIKey, log))
	}
	svc.ConfigurePrediction(sourceclient.New(cfg.ShipmentURL, cfg.TrackingURL, cfg.InternalServiceToken), predict.Policy{
		Unload: cfg.Prediction.Unload, Uncertainty: cfg.Prediction.Uncertainty, MaxETAAge: cfg.Prediction.MaxETAAge,
		ConfidenceFloor: cfg.Prediction.ConfidenceFloor, AutoActivate: cfg.Prediction.AutoActivate,
	})
	router := httpserver.NewRouterWithCatalog(log, svc, ready(store), store)
	server := &http.Server{
		Addr:              ":" + itoa(cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("network optimizer listening", slog.Int("port", cfg.HTTPPort))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped")
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func ready(store *repository.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
