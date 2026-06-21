package http_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/config"
	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestUnknownRouteReturnsRouteNotFound(t *testing.T) {
	proxy, err := gatewayhttp.NewProxyHandler(testConfig())
	if err != nil {
		t.Fatal(err)
	}

	handler := gatewayhttp.NewRouter(testLogger(), testConfig(), proxy)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rec.Code)
	}
	if body := readBody(t, rec); !strings.Contains(body, "ROUTE_NOT_FOUND") {
		t.Fatalf("expected ROUTE_NOT_FOUND, body=%s", body)
	}
}

func TestProxyRewritesCompaniesPath(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/companies" {
			t.Fatalf("backend path=%q want /v1/companies", r.URL.Path)
		}
		if r.Header.Get("X-Request-ID") == "" {
			t.Fatal("expected X-Request-ID header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer backend.Close()

	cfg := testConfig()
	cfg.Services.Company = backend.URL

	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := gatewayhttp.NewRouter(testLogger(), cfg, proxy)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies?tenant_id=abc", nil)
	req.Header.Set("X-Request-ID", "gateway-req-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, readBody(t, rec))
	}
}

func testConfig() config.Config {
	return config.Config{
		ServiceName:         "api-gateway",
		Environment:         "test",
		HTTPPort:            8080,
		LogLevel:            "error",
		AuthEnabled:         false,
		JWTSecret:           "test-secret",
		CORSAllowedOrigins:  []string{"http://localhost:3000"},
		ProxyTimeoutSeconds: 5,
		ReadyCheckTimeoutMS: 1000,
		OpenAPIDir:          "../../packages/openapi",
		Services: config.ServiceURLs{
			Identity:        "http://localhost:8081",
			Company:         "http://localhost:8082",
			TransportOrder:  "http://localhost:8083",
			RFX:             "http://localhost:8084",
			Shipment:        "http://localhost:8085",
			Document:        "http://localhost:8086",
			BillingRegister: "http://localhost:8087",
		},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func readBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
