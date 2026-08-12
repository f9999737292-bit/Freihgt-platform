package controltower

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func TestSummaryUsesVerifiedTenantNotSpoofedHeader(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"

	var downstreamTenant string
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamTenant = r.URL.Query().Get("tenant_id")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", tenantB)
	req.Header.Set("X-User-ID", "spoofed-user")

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if downstreamTenant != tenantA {
		t.Fatalf("downstream tenant=%q want verified tenant %q", downstreamTenant, tenantA)
	}
}

func TestSummaryIgnoresTenantIDQueryParameter(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"

	var downstreamTenant string
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamTenant = r.URL.Query().Get("tenant_id")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary?tenant_id="+tenantB, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if downstreamTenant != tenantA {
		t.Fatalf("downstream tenant=%q want verified tenant %q", downstreamTenant, tenantA)
	}
}

func TestSummaryMissingVerifiedTenantReturns401(t *testing.T) {
	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL: "http://127.0.0.1:1",
		identityURL: "http://127.0.0.1:1",
		authEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("X-Tenant-ID", "22222222-2222-2222-2222-222222222222")

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestSummaryDownstreamReceivesTrustedHeaders(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	userA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	requestID := "req-trusted-123"

	var gotQueryTenant, gotHeaderTenant, gotUser, gotAuth, gotRequestID string
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQueryTenant = r.URL.Query().Get("tenant_id")
		gotHeaderTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotAuth = r.Header.Get("Authorization")
		gotRequestID = r.Header.Get("X-Request-ID")
		if gotHeaderTenant == "" {
			http.Error(w, "tenant context is required", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_DISPATCHER"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	})

	token := signTestToken(t, "secret", userA, tenantA, "dispatcher@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(gwmiddleware.RequestIDHeader, requestID)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotQueryTenant != tenantA {
		t.Fatalf("query tenant_id=%q want %q", gotQueryTenant, tenantA)
	}
	if gotHeaderTenant != tenantA {
		t.Fatalf("header X-Tenant-ID=%q want %q", gotHeaderTenant, tenantA)
	}
	if gotUser != userA {
		t.Fatalf("user header=%q want %q", gotUser, userA)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("authorization not forwarded: %q", gotAuth)
	}
	if gotRequestID != requestID {
		t.Fatalf("request id=%q want %q", gotRequestID, requestID)
	}
}

func TestSummaryForbiddenRoleReturns403(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("shipments should not be called for forbidden role")
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	})

	token := signTestToken(t, "secret", "finance-user", tenantA, "finance@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

func TestSummaryShipmentsUnavailableReturns503(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CONTROL_TOWER_SHIPMENTS_UNAVAILABLE") {
		t.Fatalf("expected control tower shipments error code, body=%s", rec.Body.String())
	}
}

func TestSummaryOptionalCompanyFailureReturnsPartial200(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer shipmentServer.Close()

	companyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer companyServer.Close()

	transportServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer transportServer.Close()

	documentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer documentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandler(t, testHandlerConfig{
		shipmentURL:  shipmentServer.URL,
		companyURL:   companyServer.URL,
		transportURL: transportServer.URL,
		documentURL:  documentServer.URL,
		identityURL:  identityServer.URL,
		authEnabled:  true,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `"partial":true`) {
		t.Fatalf("expected partial response, body=%s", string(body))
	}
	if !strings.Contains(string(body), "COMPANIES_UNAVAILABLE") {
		t.Fatalf("expected companies warning, body=%s", string(body))
	}
}

func TestBuildRequestContextDevModeUsesConfiguredTenant(t *testing.T) {
	devTenant := "33333333-3333-3333-3333-333333333333"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("X-Tenant-ID", "22222222-2222-2222-2222-222222222222")

	ctx, err := buildRequestContext(req, false, devTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.TenantID != devTenant {
		t.Fatalf("tenant=%q want configured dev tenant %q", ctx.TenantID, devTenant)
	}
}

type testHandlerConfig struct {
	shipmentURL  string
	companyURL   string
	transportURL string
	documentURL  string
	identityURL  string
	authEnabled  bool
	devTenantID  string
}

func newTestSummaryHandler(t *testing.T, cfg testHandlerConfig) http.Handler {
	t.Helper()
	if cfg.companyURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.companyURL = server.URL
	}
	if cfg.transportURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.transportURL = server.URL
	}
	if cfg.documentURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.documentURL = server.URL
	}

	h := NewHandler(slog.New(slog.DiscardHandler), config.Config{
		AuthEnabled:         cfg.authEnabled,
		DevTenantID:         cfg.devTenantID,
		ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{
			Identity:       cfg.identityURL,
			Company:        cfg.companyURL,
			TransportOrder: cfg.transportURL,
			Shipment:       cfg.shipmentURL,
			Document:       cfg.documentURL,
		},
		ControlTower: config.ControlTowerConfig{MaxDownstreamFetchLimit: 200},
	})
	return http.HandlerFunc(h.Summary)
}

func serveThroughAuth(t *testing.T, handler http.Handler, req *http.Request, secret string, enabled bool) *httptest.ResponseRecorder {
	t.Helper()
	chain := gwmiddleware.Auth(enabled, secret)(handler)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec
}

func signTestToken(t *testing.T, secret, userID, tenantID, email string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     email,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
