package http_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
)

type pingDB struct{}

func (pingDB) Ping(context.Context) error { return nil }

func TestDriverStopRouterValidation(t *testing.T) {
	router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), pingDB{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "token", nil)
	tenant := uuid.NewString()
	user := uuid.NewString()
	stopID := uuid.NewString()

	t.Run("missing auth", func(t *testing.T) {
		rec := serve(router, http.MethodGet, "/v1/driver/me/stops", tenant, "", "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
	})
	t.Run("bad stop id", func(t *testing.T) {
		rec := serve(router, http.MethodPost, "/v1/driver/me/stops/not-a-uuid/arrive", tenant, user, "key-1", []byte(`{"expectedVersion":1}`))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("missing idempotency key", func(t *testing.T) {
		rec := serve(router, http.MethodPost, "/v1/driver/me/stops/"+stopID+"/arrive", tenant, user, "", []byte(`{"expectedVersion":1}`))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Idempotency-Key") {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("rejects client execution identity", func(t *testing.T) {
		rec := serve(router, http.MethodPost, "/v1/driver/me/stops/"+stopID+"/start-service", tenant, user, "key-2", []byte(`{"expectedVersion":1,"executionId":"`+uuid.NewString()+`"}`))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid occurredAt", func(t *testing.T) {
		rec := serve(router, http.MethodPost, "/v1/driver/me/stops/"+stopID+"/complete", tenant, user, "key-3", []byte(`{"expectedVersion":1,"occurredAt":"yesterday"}`))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("notice tasks stay separate", func(t *testing.T) {
		rec := serve(router, http.MethodGet, "/v1/driver/me/tasks", "", "", "", nil)
		if rec.Code == http.StatusNotFound {
			t.Fatal("notice task route was removed")
		}
	})
}

func serve(router http.Handler, method, path, tenant, user, key string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	if user != "" {
		req.Header.Set("X-User-ID", user)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
