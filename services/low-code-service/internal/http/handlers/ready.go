package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/freight-platform/low-code-service/internal/platform/database"
)

type readyResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database,omitempty"`
	Schema   string `json:"schema,omitempty"`
	Error    string `json:"error,omitempty"`
}

func Ready(checker *database.ReadinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		result := checker.Check(ctx)
		resp := readyResponse{Service: serviceName}

		if !result.Ready {
			resp.Status = "not_ready"
			if result.DatabaseOK {
				resp.Database = "ok"
			} else {
				resp.Database = "down"
			}
			if result.SchemaOK {
				resp.Schema = "lowcode"
			}
			resp.Error = result.Error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp.Status = "ready"
		resp.Database = "ok"
		resp.Schema = "lowcode"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
