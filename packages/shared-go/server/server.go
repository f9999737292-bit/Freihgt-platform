package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/freight-platform/shared-go/health"
)

// HTTPServer wraps the standard library HTTP server with graceful shutdown support.
type HTTPServer struct {
	server *http.Server
	log    *slog.Logger
}

// New creates an HTTP server with health endpoints registered.
func New(serviceName string, port int, log *slog.Logger) *HTTPServer {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.Handler(serviceName))
	mux.HandleFunc("GET /healthz", health.Handler(serviceName))
	mux.HandleFunc("GET /ready", health.Handler(serviceName))

	return &HTTPServer{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		log: log,
	}
}

// Start begins accepting HTTP connections.
func (s *HTTPServer) Start() error {
	s.log.Info("starting http server", slog.String("addr", s.server.Addr))
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down http server")
	return s.server.Shutdown(ctx)
}
