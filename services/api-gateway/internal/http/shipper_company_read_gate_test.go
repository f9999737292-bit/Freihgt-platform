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

	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

const (
	shipperReadTenantID  = "11111111-1111-1111-1111-111111111111"
	shipperOtherTenantID = "99999999-9999-9999-9999-999999999999"
	shipperReadUserID    = "22222222-2222-2222-2222-222222222222"
	shipperCompanyA      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	shipperCompanyB      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	shipperReadShipment  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	shipperReadJWTSecret = "shipper-company-context-test-secret"
)

type shipperMembership struct {
	companyID   string
	companyType string
	roles       []string
}

type shipperReadHarness struct {
	handler      http.Handler
	mu           sync.Mutex
	shipmentHits int
	trackingHits int
	companyHits  int
	roleHits     int
	authMeRoles  []string
	tenantRoles  []string
	lookupTenant string
	gotPath      string
	gotQuery     string
	gotHeaders   map[string]string
	identityCode int
}

func newShipperReadHarness(t *testing.T, authMeRoles []string, memberships []shipperMembership) *shipperReadHarness {
	t.Helper()
	h := &shipperReadHarness{authMeRoles: authMeRoles, identityCode: http.StatusOK}
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.shipmentHits++
		h.gotPath = r.URL.Path
		h.gotQuery = r.URL.Query().Get("shipper_company_id")
		h.gotHeaders = map[string]string{
			"X-Company-ID": r.Header.Get("X-Company-ID"),
			"X-Actor-Kind": r.Header.Get("X-Actor-Kind"),
		}
		company := h.gotQuery
		h.mu.Unlock()
		items := []map[string]string{
			{"id": "row-a", "shipper_company_id": shipperCompanyA},
			{"id": "row-b", "shipper_company_id": shipperCompanyB},
		}
		if company != "" {
			filtered := make([]map[string]string, 0, 1)
			for _, row := range items {
				if row["shipper_company_id"] == company {
					filtered = append(filtered, row)
				}
			}
			items = filtered
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	}))
	t.Cleanup(shipment.Close)

	trackingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.trackingHits++
		h.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(trackingSrv.Close)

	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		code := h.identityCode
		h.mu.Unlock()
		if code != http.StatusOK {
			http.Error(w, "identity down", code)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": h.authMeRoles})
		case strings.Contains(r.URL.Path, "/roles"):
			h.mu.Lock()
			h.roleHits++
			h.mu.Unlock()
			items := make([]map[string]any, 0)
			for _, code := range h.tenantRoles {
				items = append(items, map[string]any{"code": code})
			}
			for _, membership := range memberships {
				companyID := membership.companyID
				for _, role := range membership.roles {
					items = append(items, map[string]any{"code": role, "company_id": companyID})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case strings.Contains(r.URL.Path, "/companies"):
			h.mu.Lock()
			h.companyHits++
			h.lookupTenant = r.URL.Query().Get("tenant_id")
			h.mu.Unlock()
			if r.URL.Query().Get("status") != "ACTIVE" {
				t.Errorf("membership lookup status=%q", r.URL.Query().Get("status"))
			}
			items := make([]map[string]any, 0, len(memberships))
			for _, membership := range memberships {
				roles := make([]map[string]string, 0, len(membership.roles))
				for _, code := range membership.roles {
					roles = append(roles, map[string]string{"code": code})
				}
				items = append(items, map[string]any{
					"company_id":   membership.companyID,
					"company_type": membership.companyType,
					"roles":        roles,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(identity.Close)

	cfg := testConfig()
	cfg.AuthEnabled = true
	cfg.JWTSecret = shipperReadJWTSecret
	cfg.Services.Identity = identity.URL
	cfg.Services.Shipment = shipment.URL
	cfg.Services.Tracking = trackingSrv.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.handler = gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, tracking.NewHandler(testLogger(), cfg), nil)
	return h
}

func (h *shipperReadHarness) do(t *testing.T, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+signShipperReadToken(t, shipperReadTenantID))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func signShipperReadToken(t *testing.T, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     "shipper-a@example.com",
		"sub":       shipperReadUserID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(shipperReadJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func oneShipperMembership(companyID, companyType string, roles ...string) []shipperMembership {
	return []shipperMembership{{
		companyID:   companyID,
		companyType: companyType,
		roles:       roles,
	}}
}

func TestShipperCompanyReadGate(t *testing.T) {
	t.Run("shipper admin list stays on company A", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, map[string]string{
			"X-Company-ID": shipperCompanyB,
			"X-Actor-Kind": "CARRIER",
			"X-Tenant-ID":  shipperOtherTenantID,
			"X-User-ID":    "spoof",
		})
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, shipperCompanyB) || !strings.Contains(body, shipperCompanyA) {
			t.Fatalf("list isolation body=%s", body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotPath != "/v1/shipper/shipments" || h.gotQuery != shipperCompanyA {
			t.Fatalf("downstream path=%s query=%s", h.gotPath, h.gotQuery)
		}
		if h.gotHeaders["X-Company-ID"] != shipperCompanyA || h.gotHeaders["X-Actor-Kind"] != "BUYER" {
			t.Fatalf("downstream headers=%v", h.gotHeaders)
		}
		if h.roleHits != 0 {
			t.Fatalf("shipper read consulted tenant roles")
		}
		if h.lookupTenant != shipperReadTenantID {
			t.Fatalf("lookup tenant=%s", h.lookupTenant)
		}
	})

	t.Run("shipper logist detail stays on company A", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"CARRIER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_LOGIST"))
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments/"+shipperReadShipment+"?shipper_company_id="+strings.ToUpper(shipperCompanyA), nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotPath != "/v1/shipper/shipments/"+shipperReadShipment || h.gotQuery != shipperCompanyA {
			t.Fatalf("downstream path=%s query=%s", h.gotPath, h.gotQuery)
		}
		if h.gotHeaders["X-Actor-Kind"] != "BUYER" {
			t.Fatalf("actor=%s", h.gotHeaders["X-Actor-Kind"])
		}
	})

	t.Run("cross company shipper admin does not authorize company A", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, []shipperMembership{
			{companyID: shipperCompanyA, companyType: "SHIPPER", roles: []string{"PROCUREMENT_MANAGER"}},
			{companyID: shipperCompanyB, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
		})
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})

	t.Run("forwarder is not a shipper", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"FORWARDER_MANAGER"}, oneShipperMembership(shipperCompanyA, "FORWARDER", "FORWARDER_MANAGER"))
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})

	t.Run("carrier admin is not a shipper", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"CARRIER_ADMIN"}, oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN"))
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments/"+shipperReadShipment+"?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})

	t.Run("same tenant without membership is denied", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, oneShipperMembership(shipperCompanyB, "SHIPPER", "SHIPPER_ADMIN"))
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})

	t.Run("other tenant company is denied", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, nil)
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.lookupTenant != shipperReadTenantID {
			t.Fatalf("lookup tenant=%s", h.lookupTenant)
		}
	})

	t.Run("tenant platform admin without shipper membership is denied", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"PLATFORM_ADMIN"}, nil)
		h.tenantRoles = []string{"PLATFORM_ADMIN"}
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})

	t.Run("missing and malformed company fail closed", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
		for _, path := range []string{
			"/api/v1/shipper/shipments",
			"/api/v1/shipper/shipments?shipper_company_id=not-a-uuid",
			"/api/v1/shipper/shipments/" + shipperReadShipment,
		} {
			rec := h.do(t, http.MethodGet, path, nil)
			if rec.Code != http.StatusBadRequest || h.shipmentCalled() {
				t.Fatalf("%s status=%d called=%v body=%s", path, rec.Code, h.shipmentCalled(), readBody(t, rec))
			}
		}
	})

	t.Run("conflicting company query fails closed", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
		path := "/api/v1/shipper/shipments?shipper_company_id=" + shipperCompanyA + "&shipper_company_id=" + shipperCompanyB
		rec := h.do(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusForbidden || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})

	t.Run("identity failure does not proxy", func(t *testing.T) {
		h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
		h.identityCode = http.StatusInternalServerError
		rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusServiceUnavailable || h.shipmentCalled() {
			t.Fatalf("status=%d called=%v body=%s", rec.Code, h.shipmentCalled(), readBody(t, rec))
		}
	})
}

func TestShipperLegacyShipmentReadBypass(t *testing.T) {
	for _, role := range []string{"SHIPPER_ADMIN", "SHIPPER_LOGIST"} {
		h := newShipperReadHarness(t, []string{role}, oneShipperMembership(shipperCompanyA, "SHIPPER", role))
		for _, path := range []string{
			"/api/v1/shipments",
			"/api/v1/shipments/" + shipperReadShipment,
		} {
			rec := h.do(t, http.MethodGet, path, map[string]string{"X-Company-ID": shipperCompanyB})
			if rec.Code != http.StatusForbidden || h.shipmentCalled() {
				t.Fatalf("%s %s status=%d called=%v body=%s", role, path, rec.Code, h.shipmentCalled(), readBody(t, rec))
			}
		}
	}

	h := newShipperReadHarness(t, []string{"PLATFORM_ADMIN"}, nil)
	h.tenantRoles = []string{"PLATFORM_ADMIN"}
	rec := h.do(t, http.MethodGet, "/api/v1/shipments", map[string]string{
		"X-Company-ID": shipperCompanyB,
		"X-Actor-Kind": "CARRIER",
	})
	body := readBody(t, rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("operator list status=%d body=%s", rec.Code, body)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.gotPath != "/v1/shipments" {
		t.Fatalf("operator path=%s", h.gotPath)
	}
	if h.gotHeaders["X-Company-ID"] != "" || h.gotHeaders["X-Actor-Kind"] != "" {
		t.Fatalf("operator forwarded spoofed headers=%v", h.gotHeaders)
	}
}

func TestShipperProxyDoesNotCaptureTracking(t *testing.T) {
	h := newShipperReadHarness(t, []string{"SHIPPER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
	rec := h.do(t, http.MethodGet, "/api/v1/shipments/"+shipperReadShipment+"/tracking", nil)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.shipmentHits != 0 {
		t.Fatal("legacy tracking request reached shipment-service")
	}
	if h.trackingHits != 0 || rec.Code != http.StatusForbidden {
		t.Fatalf("tracking hits=%d status=%d body=%s", h.trackingHits, rec.Code, readBody(t, rec))
	}
}

func TestShipperPublicRoutesTargetShipmentService(t *testing.T) {
	cfg := testConfig()
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		in   string
		want string
	}{
		{"/api/v1/shipper/shipments", "/v1/shipper/shipments"},
		{"/api/v1/shipper/shipments/" + shipperReadShipment, "/v1/shipper/shipments/" + shipperReadShipment},
	}
	for _, tt := range cases {
		route := gatewayhttp.MatchRoute(tt.in, proxy.Routes())
		if route == nil || route.Service != "shipment-service" || route.Prefix != "/api/v1/shipper" {
			t.Fatalf("%s route=%v", tt.in, route)
		}
		rewritten, ok := gatewayhttp.RewritePath(tt.in)
		if !ok || rewritten != tt.want {
			t.Fatalf("%s rewritten=%s ok=%v", tt.in, rewritten, ok)
		}
	}
	legacy := gatewayhttp.MatchRoute("/api/v1/shipments", proxy.Routes())
	if legacy == nil || legacy.Prefix != "/api/v1/shipments" || legacy.Service != "shipment-service" {
		t.Fatalf("legacy route=%v", legacy)
	}
	carrier := gatewayhttp.MatchRoute("/api/v1/carrier/rfx-events", proxy.Routes())
	if carrier == nil || carrier.Service != "rfx-service" {
		t.Fatalf("carrier route=%v", carrier)
	}
}

func (h *shipperReadHarness) shipmentCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.shipmentHits > 0
}
