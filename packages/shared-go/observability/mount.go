package observability

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/shared-go/metrics"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

// MountOptions configures standard observability routes and middleware.
type MountOptions struct {
	ServiceName string
	Log         *slog.Logger
	Metrics     *metrics.Collector
	DB          DatabasePinger
}

// Mount registers request middleware, /health, /ready, and /metrics on the router.
func Mount(r chi.Router, opts MountOptions) {
	r.Use(sharedmiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(sharedmiddleware.Recover(opts.Log, opts.ServiceName))
	r.Use(sharedmiddleware.AccessLog(opts.Log, opts.ServiceName))
	if opts.Metrics != nil {
		r.Use(opts.Metrics.Middleware)
	}

	r.Get("/health", HealthHandler(opts.ServiceName))
	r.Get("/ready", ReadyHandler(opts.ServiceName, opts.DB))
	if opts.Metrics != nil {
		r.Handle("/metrics", opts.Metrics.Handler())
	}
}
