package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/config"
	"github.com/freight-platform/billing-register-service/internal/http/handlers"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
)

func TestInternalSyncPaidAuthMatrix(t *testing.T) {
	token := "billing-internal-token"
	registerID := uuid.New()
	cfg := config.Config{InternalServiceToken: token, Environment: "test", ServiceName: "billing-register-service"}
	registerSvc := &service.BillingRegisterService{}
	settlementRepo := repository.NewFreightSettlementRepository(nil)
	registerRepo := repository.NewBillingRegisterRepository(nil)
	router := NewRouter(slog.Default(), nil, cfg, registerSvc, nil, nil, settlementRepo, registerRepo, handlers.NewSettlementActorResolver(nil))
	body, _ := json.Marshal(map[string]string{"tenant_id": uuid.New().String()})
	path := "/internal/v1/billing-registers/" + registerID.String() + "/sync-paid"

	t.Run("INTERNAL_SYNC_PAID_NO_TOKEN=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("INTERNAL_SYNC_PAID_BAD_TOKEN=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("X-Internal-Service-Token", "wrong")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("INTERNAL_SYNC_PAID_VALID_TOKEN=PASS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("X-Internal-Service-Token", token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Fatalf("valid token must pass auth gate, got %d", rec.Code)
		}
	})
}
