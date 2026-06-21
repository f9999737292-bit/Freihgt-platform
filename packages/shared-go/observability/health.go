package observability

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

var startTime = time.Now().UTC()

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status        string `json:"status"`
	Service       string `json:"service"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	Timestamp     string `json:"timestamp"`
}

// HealthHandler returns a liveness probe handler.
func HealthHandler(serviceName string) http.HandlerFunc {
	version := os.Getenv("SERVICE_VERSION")
	if version == "" {
		version = "0.1.0"
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:        "ok",
			Service:       serviceName,
			Version:       version,
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		})
	}
}
