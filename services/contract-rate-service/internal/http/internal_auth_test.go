package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/config"
	"github.com/freight-platform/contract-rate-service/internal/http/handlers"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

func TestInternalAuthMatrix(t *testing.T) {
	token := "test-internal-token"
	cfg := config.Config{InternalServiceToken: token, Environment: "test", ServiceName: "contract-rate-service"}
	router := NewRouter(slog.Default(), nil, cfg,
		&service.ContractService{}, &service.RateCardService{},
		&service.RateLineService{}, &service.RateComponentService{}, &service.ResolutionService{},
		handlers.NewActorResolver(nil))
	body, _ := json.Marshal(map[string]string{"buyer_company_id": uuid.New().String()})

	t.Run("NO_S2S_AUTH=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-contracts", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("INVALID_S2S_AUTH=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-contracts", bytes.NewReader(body))
		req.Header.Set("X-Internal-Service-Token", "bad")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("VALID_S2S_AUTH=PASSES_GATE", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-contracts", bytes.NewReader(body))
		req.Header.Set("X-Internal-Service-Token", token)
		req.Header.Set("X-Tenant-ID", uuid.New().String())
		req.Header.Set("X-User-ID", uuid.New().String())
		req.Header.Set("X-Company-ID", uuid.New().String())
		req.Header.Set("X-Actor-Kind", "BUYER")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized && strings.Contains(rec.Body.String(), "internal service authentication failed") {
			t.Fatalf("valid token must pass S2S auth gate, got %d", rec.Code)
		}
	})
}
