package http_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestPublicAcceptRouteBeatsCarrierPrefix(t *testing.T) {
	proxy, err := gatewayhttp.NewProxyHandler(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	routes := proxy.Routes()
	accept := gatewayhttp.MatchRoute(gatewayhttp.PublicAcceptRoute, routes)
	if accept == nil || accept.Service != "company-service" || accept.Prefix != gatewayhttp.PublicAcceptRoute {
		t.Fatalf("accept route = %#v", accept)
	}
	rewritten, ok := gatewayhttp.RewritePath(gatewayhttp.PublicAcceptRoute)
	if !ok || rewritten != "/v1/carrier-operating-relationship-acceptances" {
		t.Fatalf("rewrite = %q ok=%v", rewritten, ok)
	}
	carrier := gatewayhttp.MatchRoute("/api/v1/carrier/rfx-events/"+uuid.NewString(), routes)
	if carrier == nil || carrier.Service != "rfx-service" || carrier.Prefix != "/api/v1/carrier" {
		t.Fatalf("carrier route = %#v", carrier)
	}
	orders := gatewayhttp.MatchRoute("/api/v1/carrier/transport-orders", routes)
	if orders == nil || orders.Service != "shipment-service" || orders.Prefix != "/api/v1/carrier/transport-orders" {
		t.Fatalf("transport-order route = %#v", orders)
	}
}

func TestPublicAcceptGatewayForwardsVerifiedIdentity(t *testing.T) {
	userID := uuid.NewString()
	tenantID := uuid.NewString()
	companyID := uuid.NewString()
	var companyCalls int
	var rfxCalls int
	company := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		companyCalls++
		if r.URL.Path != "/v1/carrier-operating-relationship-acceptances" {
			t.Errorf("upstream path = %s", r.URL.Path)
		}
		if r.Header.Get("X-User-ID") != userID || r.Header.Get("X-Tenant-ID") != tenantID {
			t.Errorf("identity user=%s tenant=%s", r.Header.Get("X-User-ID"), r.Header.Get("X-Tenant-ID"))
		}
		if r.Header.Get("X-Company-ID") != companyID {
			t.Errorf("company selector = %q", r.Header.Get("X-Company-ID"))
		}
		if r.Header.Get("X-User-Roles") != "" || r.Header.Get("X-Platform-Admin") != "" {
			t.Errorf("spoofed role headers survived user-roles=%q platform=%q", r.Header.Get("X-User-Roles"), r.Header.Get("X-Platform-Admin"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("bearer token was not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"code":"RELATIONSHIP_ACCEPTANCE_FORBIDDEN","message":"relationship command refused","details":{}}}`)
	}))
	defer company.Close()
	rfx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rfxCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer rfx.Close()

	cfg := testConfig()
	cfg.AuthEnabled = true
	cfg.MaxRequestBodyBytes = 1 << 20
	cfg.Services.Company = company.URL
	cfg.Services.RFX = rfx.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, nil)

	missing := httptest.NewRequest(http.MethodPost, gatewayhttp.PublicAcceptRoute, strings.NewReader(`{"idempotency_key":"k","acceptance_token":"token"}`))
	missing.Header.Set("X-Company-ID", companyID)
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusUnauthorized || companyCalls != 0 {
		t.Fatalf("missing bearer status=%d calls=%d body=%s", missingRec.Code, companyCalls, missingRec.Body.String())
	}

	token := signGatewayToken(t, cfg.JWTSecret, userID, tenantID)
	req := httptest.NewRequest(http.MethodPost, gatewayhttp.PublicAcceptRoute, strings.NewReader(`{"idempotency_key":"k","acceptance_token":"token"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Company-ID", companyID)
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-Roles", "PLATFORM_ADMIN")
	req.Header.Set("X-Platform-Admin", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || companyCalls != 1 || rfxCalls != 0 {
		t.Fatalf("accept status=%d company=%d rfx=%d body=%s", rec.Code, companyCalls, rfxCalls, rec.Body.String())
	}

	carrier := httptest.NewRequest(http.MethodGet, "/api/v1/carrier/unregistered-rfx-tail", nil)
	carrier.Header.Set("Authorization", "Bearer "+token)
	carrierRec := httptest.NewRecorder()
	handler.ServeHTTP(carrierRec, carrier)
	if rfxCalls != 1 || companyCalls != 1 {
		t.Fatalf("carrier tail company=%d rfx=%d status=%d", companyCalls, rfxCalls, carrierRec.Code)
	}
}

func signGatewayToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":       userID,
		"tenant_id": tenantID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
