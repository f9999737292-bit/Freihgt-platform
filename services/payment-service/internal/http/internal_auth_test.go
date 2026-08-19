package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/config"
	"github.com/freight-platform/payment-service/internal/http/handlers"
	"github.com/freight-platform/payment-service/internal/service"
)

func TestInternalEnsureAuthMatrix(t *testing.T) {
	token := "test-internal-token"
	cfg := config.Config{InternalServiceToken: token, Environment: "test", ServiceName: "payment-service"}
	svc := &service.PaymentService{}
	router := NewRouter(slog.Default(), nil, cfg, svc, handlers.NewPaymentActorResolver(nil))
	body, _ := json.Marshal(map[string]string{
		"tenant_id":   uuid.New().String(),
		"register_id": uuid.New().String(),
	})

	t.Run("INTERNAL_ENSURE_NO_TOKEN=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/payment-obligations/ensure", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("INTERNAL_ENSURE_BAD_TOKEN=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/payment-obligations/ensure", bytes.NewReader(body))
		req.Header.Set("X-Internal-Service-Token", "bad-token")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("INTERNAL_ENSURE_VALID_TOKEN=PASS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/payment-obligations/ensure", bytes.NewReader(body))
		req.Header.Set("X-Internal-Service-Token", token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Fatalf("valid token must pass auth gate, got %d", rec.Code)
		}
	})
}
