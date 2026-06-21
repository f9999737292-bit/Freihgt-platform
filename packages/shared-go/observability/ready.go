package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// DatabasePinger checks database connectivity.
type DatabasePinger interface {
	Ping(ctx context.Context) error
}

// ReadyResponse is returned by GET /ready.
type ReadyResponse struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// ReadyHandler returns a readiness probe handler with optional database check.
func ReadyHandler(serviceName string, db DatabasePinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{}
		allOK := true

		if db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := db.Ping(ctx); err != nil {
				checks["database"] = "down"
				allOK = false
			} else {
				checks["database"] = "ok"
			}
		}

		status := "ready"
		httpStatus := http.StatusOK
		if !allOK {
			status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(ReadyResponse{
			Status:  status,
			Service: serviceName,
			Checks:  checks,
		})
	}
}
