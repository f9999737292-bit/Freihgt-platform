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
	const (
		shipperA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		carrierA = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		carrierB = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		spoofCo  = "99999999-9999-9999-9999-999999999999"
	)
	var seen struct {
		calls         int
		tenant        string
		token         string
		caller        string
		user          string
		auth          string
		query         string
		scope         string
		company       string
		clientCompany string
	}
	analytics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.tenant = r.Header.Get("X-Tenant-ID")
		seen.token = r.Header.Get("X-Internal-Service-Token")
		seen.caller = r.Header.Get("X-Internal-Service-Name")
		seen.user = r.Header.Get("X-User-ID")
		seen.auth = r.Header.Get("Authorization")
		seen.query = r.URL.RawQuery
		seen.scope = r.Header.Get("X-Analytics-Scope")
		seen.company = r.Header.Get("X-Analytics-Company-ID")
		seen.clientCompany = r.Header.Get("X-Company-ID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kpiId":"OPS_SHIPMENTS_TOTAL","definitionVersion":1,"measure":{"type":"COUNT","value":4},"generatedAt":"2026-10-07T18:00:00Z","dataFreshness":{"status":"UNKNOWN"},"completeness":"COMPLETE"}`))
	}))
	defer analytics.Close()

	var roleMu sync.Mutex
	fixture := identityFixture{tenantID: tenantA}
	setFixture := func(next identityFixture) {
		roleMu.Lock()
		fixture.tenantRoles = next.tenantRoles
		fixture.companies = next.companies
		fixture.companyCalls = 0
		roleMu.Unlock()
	}
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleMu.Lock()
		current := fixture
		if strings.Contains(r.URL.Path, "/companies") {
			fixture.companyCalls++
			current.companyCalls = fixture.companyCalls
		}
		roleMu.Unlock()
		if !strings.Contains(r.URL.Path, "/users/user-1/") || !strings.Contains(r.URL.RawQuery, "tenant_id="+tenantA) {
			http.NotFound(w, r)
			return
		}
		current.ServeHTTP(w, r)
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
		setFixture(identityFixture{companies: []map[string]any{companyItem(carrierA, "CARRIER", "DRIVER")}})
		before := seen.calls
		rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "evil")
		if rec.Code != http.StatusForbidden || seen.calls != before {
			t.Fatalf("status=%d calls=%d body=%s", rec.Code, seen.calls, rec.Body.String())
		}
	})

	t.Run("platform admin is tenant wide", func(t *testing.T) {
		setFixture(identityFixture{tenantRoles: []map[string]any{{"code": "PLATFORM_ADMIN"}}})
		rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "shipment-service")
		if rec.Code != http.StatusOK || seen.scope != "TENANT" || seen.company != "" || seen.clientCompany != "" {
			t.Fatalf("status=%d scope=%s company=%s client=%s body=%s", rec.Code, seen.scope, seen.company, seen.clientCompany, rec.Body.String())
		}
		roleMu.Lock()
		calls := fixture.companyCalls
		roleMu.Unlock()
		if calls != 0 || seen.tenant != tenantA || seen.token != gatewayToken || seen.caller != "api-gateway" || seen.user != "" || seen.auth != "" {
			t.Fatalf("companyCalls=%d tenant=%s token=%s caller=%s user=%s auth=%s", calls, seen.tenant, seen.token, seen.caller, seen.user, seen.auth)
		}
	})

	for _, role := range []string{"SHIPPER_ADMIN", "SHIPPER_LOGIST"} {
		t.Run(role, func(t *testing.T) {
			setFixture(identityFixture{companies: []map[string]any{companyItem(shipperA, "SHIPPER", role)}})
			rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "shipment-service")
			if rec.Code != http.StatusOK || seen.scope != "SHIPPER" || seen.company != shipperA || seen.clientCompany != "" || seen.tenant != tenantA {
				t.Fatalf("status=%d scope=%s company=%s client=%s tenant=%s body=%s", rec.Code, seen.scope, seen.company, seen.clientCompany, seen.tenant, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), spoofCo) || strings.Contains(rec.Body.String(), carrierA) {
				t.Fatalf("response disclosed another company: %s", rec.Body.String())
			}
		})
	}
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT"} {
		t.Run(role, func(t *testing.T) {
			setFixture(identityFixture{companies: []map[string]any{companyItem(carrierA, "CARRIER", role)}})
			rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "shipment-service")
			if rec.Code != http.StatusOK || seen.scope != "CARRIER" || seen.company != carrierA || seen.clientCompany != "" || seen.tenant != tenantA {
				t.Fatalf("status=%d scope=%s company=%s client=%s tenant=%s body=%s", rec.Code, seen.scope, seen.company, seen.clientCompany, seen.tenant, rec.Body.String())
			}
		})
	}
	for _, role := range []string{"PROCUREMENT_MANAGER", "FORWARDER_MANAGER", "FINANCE_MANAGER"} {
		t.Run(role+" denied", func(t *testing.T) {
			setFixture(identityFixture{tenantRoles: []map[string]any{{"code": role}}})
			before := seen.calls
			rec := serveAnalytics(t, router, token, tenantB, "spoof-token", "shipment-service")
			if rec.Code != http.StatusForbidden || seen.calls != before {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, seen.calls, rec.Body.String())
			}
		})
	}
	t.Run("ambiguous membership denied", func(t *testing.T) {
		setFixture(identityFixture{companies: []map[string]any{
			companyItem(carrierA, "CARRIER", "CARRIER_ADMIN"),
			companyItem(carrierB, "CARRIER", "CARRIER_DISPATCHER"),
		}})
		before := seen.calls
		rec := serveAnalytics(t, router, token, "", "", "")
		if rec.Code != http.StatusForbidden || seen.calls != before || strings.Contains(rec.Body.String(), carrierA) || strings.Contains(rec.Body.String(), carrierB) {
			t.Fatalf("status=%d calls=%d body=%s", rec.Code, seen.calls, rec.Body.String())
		}
	})
	t.Run("shipper and carrier denied", func(t *testing.T) {
		setFixture(identityFixture{companies: []map[string]any{
			companyItem(shipperA, "SHIPPER", "SHIPPER_ADMIN"),
			companyItem(carrierA, "CARRIER", "CARRIER_ADMIN"),
		}})
		before := seen.calls
		rec := serveAnalytics(t, router, token, "", "", "")
		if rec.Code != http.StatusForbidden || seen.calls != before {
			t.Fatalf("status=%d calls=%d body=%s", rec.Code, seen.calls, rec.Body.String())
		}
	})
}

func TestAnalyticsGatewayRejectsRedirect(t *testing.T) {
	const (
		secret       = "analytics-gateway-secret"
		gatewayToken = "gateway-internal-token"
		tenantA      = "11111111-1111-1111-1111-111111111111"
	)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identityFixture{tenantID: tenantA, companies: []map[string]any{companyItem("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "SHIPPER", "SHIPPER_ADMIN")}}.ServeHTTP(w, r)
	}))
	defer identity.Close()
	token := signAnalyticsToken(t, secret, "user-1", tenantA)

	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var targetHits int
			var tokenSeen, tenantSeen, callerSeen string
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetHits++
				tokenSeen = r.Header.Get("X-Internal-Service-Token")
				tenantSeen = r.Header.Get("X-Tenant-ID")
				callerSeen = r.Header.Get("X-Internal-Service-Name")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"kpiId":"OPS_SHIPMENTS_TOTAL","measure":{"type":"COUNT","value":99}}`))
			}))
			defer target.Close()
			analytics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/stolen", status)
			}))
			defer analytics.Close()
			router := analyticsRouter(t, secret, gatewayToken, identity.URL, analytics.URL)
			rec := serveAnalytics(t, router, token, "", "", "")
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if targetHits != 0 || tokenSeen != "" || tenantSeen != "" || callerSeen != "" {
				t.Fatalf("redirect target hits=%d token=%q tenant=%q caller=%q", targetHits, tokenSeen, tenantSeen, callerSeen)
			}
			if strings.Contains(rec.Body.String(), `"value"`) || strings.Contains(rec.Body.String(), "99") {
				t.Fatalf("returned a kpi: %s", rec.Body.String())
			}
		})
	}

	t.Run("no redirect", func(t *testing.T) {
		analytics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kpiId":"OPS_SHIPMENTS_TOTAL","definitionVersion":1,"measure":{"type":"COUNT","value":4},"generatedAt":"2026-10-07T18:00:00Z","dataFreshness":{"status":"UNKNOWN"},"completeness":"COMPLETE"}`))
		}))
		defer analytics.Close()
		router := analyticsRouter(t, secret, gatewayToken, identity.URL, analytics.URL)
		rec := serveAnalytics(t, router, token, "", "", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"value":4`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestAnalyticsGatewayDependencyDown(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identityFixture{tenantID: "11111111-1111-1111-1111-111111111111", companies: []map[string]any{companyItem("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "SHIPPER", "SHIPPER_ADMIN")}}.ServeHTTP(w, r)
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

func TestAnalyticsAuthDisabledFailsClosed(t *testing.T) {
	var calls int
	analytics := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer analytics.Close()
	cfg := config.Config{
		AuthEnabled:                 false,
		DevTenantID:                 "11111111-1111-1111-1111-111111111111",
		ProxyTimeoutSeconds:         2,
		MaxRequestBodyBytes:         1 << 20,
		AnalyticsGatewayCallerToken: "gateway-internal-token",
		Services:                    config.ServiceURLs{Analytics: analytics.URL, Identity: "http://127.0.0.1:1"},
	}
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	router := gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil))
	if rec.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body.String())
	}
}

func analyticsRouter(t *testing.T, secret, token, identityURL, analyticsURL string) http.Handler {
	t.Helper()
	cfg := config.Config{
		AuthEnabled:                 true,
		JWTSecret:                   secret,
		ProxyTimeoutSeconds:         2,
		MaxRequestBodyBytes:         1 << 20,
		InternalServiceToken:        "shared-platform-token",
		AnalyticsGatewayCallerToken: token,
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
	req.Header.Set("X-Company-ID", "99999999-9999-9999-9999-999999999999")
	req.Header.Set("X-Analytics-Scope", "TENANT")
	req.Header.Set("X-Analytics-Company-ID", "99999999-9999-9999-9999-999999999999")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type identityFixture struct {
	tenantID     string
	tenantRoles  []map[string]any
	companies    []map[string]any
	companyCalls int
}

func (f identityFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.Contains(r.URL.Path, "/roles"):
		_ = json.NewEncoder(w).Encode(map[string]any{"items": f.tenantRoles})
	case strings.Contains(r.URL.Path, "/companies"):
		if f.tenantID != "" && !strings.Contains(r.URL.RawQuery, "tenant_id="+f.tenantID) {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": f.companies})
	default:
		http.NotFound(w, r)
	}
}

func companyItem(id, companyType, role string) map[string]any {
	return map[string]any{
		"company_id":   id,
		"company_type": companyType,
		"roles":        []map[string]string{{"code": role}},
	}
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
