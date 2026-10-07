package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestAnalyticsGatewayAccessAndSpoofing(t *testing.T) {
	const (
		secret       = "analytics-gateway-secret"
		gatewayToken = "gateway-internal-token"
		tenantA      = "11111111-1111-1111-1111-111111111111"
		tenantB      = "22222222-2222-2222-2222-222222222222"
	)
	var seen struct {
		calls  int
		tenant string
		token  string
		caller string
		user   string
		auth   string
		query  string
	}
	analytics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.tenant = r.Header.Get("X-Tenant-ID")
		seen.token = r.Header.Get("X-Internal-Service-Token")
		seen.caller = r.Header.Get("X-Internal-Service-Name")
		seen.user = r.Header.Get("X-User-ID")
		seen.auth = r.Header.Get("Authorization")
		seen.query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kpiId":"OPS_SHIPMENTS_TOTAL","definitionVersion":1,"measure":{"type":"COUNT","value":4},"generatedAt":"2026-10-07T18:00:00Z","dataFreshness":{"status":"UNKNOWN"},"completeness":"COMPLETE"}`))
	}))
	defer analytics.Close()

	var roleMu sync.Mutex
	role := "SHIPPER_ADMIN"
	setRole := func(next string) {
		roleMu.Lock()
		role = next
		roleMu.Unlock()
	}
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleMu.Lock()
		current := role
		roleMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{current}})
	}))
	defer identity.Close()

	router := analyticsRouter(t, secret, gatewayToken, identity.URL, analytics.URL)
	token := signAnalyticsToken(t, secret, "user-1", tenantA)

	t.Run("missing jwt", func(t *testing.T) {
		before := seen.calls
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || seen.calls != before {
			t.Fatalf("status=%d calls=%d body=%s", rec.Code, seen.calls, rec.Body.String())
		}
	})

	t.Run("driver denied", func(t *testing.T) {
		setRole("DRIVER")
		before := seen.calls
		rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "evil")
		if rec.Code != http.StatusForbidden || seen.calls != before {
			t.Fatalf("status=%d calls=%d body=%s", rec.Code, seen.calls, rec.Body.String())
		}
	})

	for _, allowed := range []string{"PLATFORM_ADMIN", "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER", "FINANCE_MANAGER", "CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT"} {
		t.Run(allowed, func(t *testing.T) {
			setRole(allowed)
			rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "shipment-service")
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"value":4`) {
				t.Fatalf("payload changed: %s", rec.Body.String())
			}
			if seen.tenant != tenantA || seen.token != gatewayToken || seen.caller != "api-gateway" || seen.user != "" || seen.auth != "" {
				t.Fatalf("downstream headers tenant=%s token=%s caller=%s user=%s auth=%s", seen.tenant, seen.token, seen.caller, seen.user, seen.auth)
			}
		})
	}
}

func TestAnalyticsGatewayDependencyDown(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"SHIPPER_ADMIN"}})
	}))
	defer identity.Close()
	secret := "analytics-gateway-secret"
	router := analyticsRouter(t, secret, "gateway-internal-token", identity.URL, "http://127.0.0.1:1")
	token := signAnalyticsToken(t, secret, "user-1", "11111111-1111-1111-1111-111111111111")
	rec := serveAnalytics(t, router, token, "", "", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"value"`) {
		t.Fatalf("returned a kpi: %s", rec.Body.String())
	}
}

func analyticsRouter(t *testing.T, secret, token, identityURL, analyticsURL string) http.Handler {
	t.Helper()
	cfg := config.Config{
		AuthEnabled:          true,
		JWTSecret:            secret,
		ProxyTimeoutSeconds:  2,
		MaxRequestBodyBytes:  1 << 20,
		InternalServiceToken: token,
		Services: config.ServiceURLs{
			Identity:  identityURL,
			Analytics: analyticsURL,
		},
	}
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, nil)
}

func serveAnalytics(t *testing.T, router http.Handler, token, spoofTenant, spoofToken, spoofCaller string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if spoofTenant != "" {
		req.Header.Set("X-Tenant-ID", spoofTenant)
	}
	if spoofToken != "" {
		req.Header.Set("X-Internal-Service-Token", spoofToken)
	}
	if spoofCaller != "" {
		req.Header.Set("X-Internal-Service-Name", spoofCaller)
	}
	req.Header.Set("X-User-ID", "spoof-user")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func signAnalyticsToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
