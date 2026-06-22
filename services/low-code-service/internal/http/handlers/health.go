package handlers

import (
	"encoding/json"
	"net/http"
)

const serviceName = "low-code-service"

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:  "ok",
			Service: serviceName,
		})
	}
}
