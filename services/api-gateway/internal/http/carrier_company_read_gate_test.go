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
)

const (
	carrierReadTenantID  = "11111111-1111-1111-1111-111111111111"
	carrierReadUserID    = "22222222-2222-2222-2222-222222222222"
	carrierCompanyA      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	carrierCompanyB      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	shipperCompanyID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	carrierReadOrderID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	carrierReadJWTSecret = "carrier-company-context-test-secret"
)

type carrierMembership struct {
	companyID   string
	companyType string
	roles       []string
}

type carrierReadHarness struct {
	handler     http.Handler
	mu          sync.Mutex
	called      bool
	tenantRoles []string
	gotQuery    map[string]string
	gotHeaders  map[string]string
}

func newCarrierReadHarness(t *testing.T, globalRoles []string, memberships []carrierMembership) *carrierReadHarness {
	t.Helper()
	h := &carrierReadHarness{}
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.called = true
		h.gotQuery = map[string]string{
			"carrier_company_id": r.URL.Query().Get("carrier_company_id"),
			"company_id":         r.URL.Query().Get("company_id"),
			"actor":              r.URL.Query().Get("actor"),
		}
		h.gotHeaders = map[string]string{
			"X-Company-ID": r.Header.Get("X-Company-ID"),
			"X-Actor-Kind": r.Header.Get("X-Actor-Kind"),
		}
		company := h.gotQuery["carrier_company_id"]
		if strings.Contains(r.URL.Path, "/order-execution/transport-orders/") && !strings.HasSuffix(r.URL.Path, "/transport-orders") {
			company = h.gotQuery["company_id"]
		}
		h.mu.Unlock()

		items := filteredCarrierRows(company)
		if strings.Contains(r.URL.Path, "/order-execution/transport-orders/") && !strings.Contains(r.URL.Path, "/carrier/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"company_id": company,
				"actor":      r.URL.Query().Get("actor"),
				"items":      items,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	}))
	t.Cleanup(shipment.Close)

	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": globalRoles})
		case strings.Contains(r.URL.Path, "/roles"):
			items := make([]map[string]any, 0, len(h.tenantRoles)+len(memberships))
			for _, code := range h.tenantRoles {
				items = append(items, map[string]any{"code": code})
			}
			for _, membership := range memberships {
				companyID := membership.companyID
				for _, code := range membership.roles {
					items = append(items, map[string]any{
						"code":       code,
						"company_id": companyID,
					})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case strings.Contains(r.URL.Path, "/companies"):
			if r.URL.Query().Get("status") != "ACTIVE" {
				t.Errorf("membership lookup status=%q", r.URL.Query().Get("status"))
			}
			if r.URL.Query().Get("tenant_id") != carrierReadTenantID {
				t.Errorf("membership lookup tenant=%q", r.URL.Query().Get("tenant_id"))
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
	cfg.JWTSecret = carrierReadJWTSecret
	cfg.Services.Identity = identity.URL
	cfg.Services.Shipment = shipment.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.handler = gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, nil)
	return h
}

func filteredCarrierRows(company string) []map[string]string {
	rows := []map[string]string{
		{"id": "row-a", "carrier_company_id": carrierCompanyA},
		{"id": "row-b", "carrier_company_id": carrierCompanyB},
	}
	if company == "" {
		return rows
	}
	filtered := make([]map[string]string, 0, 1)
	for _, row := range rows {
		if row["carrier_company_id"] == company {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (h *carrierReadHarness) do(t *testing.T, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+signCarrierReadToken(t))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func signCarrierReadToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": carrierReadTenantID,
		"email":     "carrier-a@example.com",
		"sub":       carrierReadUserID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(carrierReadJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func carrierAMembership() []carrierMembership {
	return []carrierMembership{{
		companyID:   carrierCompanyA,
		companyType: "CARRIER",
		roles:       []string{"CARRIER_DISPATCHER"},
	}}
}

func TestCarrierCompanyReadIsolation(t *testing.T) {
	t.Run("carrier A company passes and cannot see carrier B fleet", func(t *testing.T) {
		for _, path := range []string{
			"/api/v1/drivers?carrier_company_id=" + carrierCompanyA,
			"/api/v1/vehicles?carrier_company_id=" + carrierCompanyA,
		} {
			h := newCarrierReadHarness(t, []string{"DRIVER"}, carrierAMembership())
			rec := h.do(t, path, nil)
			body := readBody(t, rec)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status=%d body=%s", path, rec.Code, body)
			}
			if strings.Contains(body, carrierCompanyB) {
				t.Fatalf("%s returned carrier B rows: %s", path, body)
			}
			if !strings.Contains(body, carrierCompanyA) {
				t.Fatalf("%s missing carrier A rows: %s", path, body)
			}
			h.mu.Lock()
			gotCompany := h.gotQuery["carrier_company_id"]
			gotHeader := h.gotHeaders["X-Company-ID"]
			h.mu.Unlock()
			if gotCompany != carrierCompanyA || gotHeader != carrierCompanyA {
				t.Fatalf("%s downstream company query=%s header=%s", path, gotCompany, gotHeader)
			}
		}
	})

	t.Run("carrier A transport orders stay on carrier A", func(t *testing.T) {
		h := newCarrierReadHarness(t, []string{"DRIVER"}, carrierAMembership())
		rec := h.do(t, "/api/v1/carrier/transport-orders?carrier_company_id="+carrierCompanyA, nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyB) || !strings.Contains(body, carrierCompanyA) {
			t.Fatalf("list isolation failed: %s", body)
		}
	})

	t.Run("carrier A execution detail stays on carrier A", func(t *testing.T) {
		h := newCarrierReadHarness(t, []string{"DRIVER"}, carrierAMembership())
		rec := h.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+carrierCompanyA+"&actor=CARRIER", nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyB) {
			t.Fatalf("detail exposed carrier B: %s", body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotQuery["company_id"] != carrierCompanyA || h.gotQuery["actor"] != "CARRIER" {
			t.Fatalf("downstream query=%v", h.gotQuery)
		}
		if h.gotHeaders["X-Actor-Kind"] != "CARRIER" {
			t.Fatalf("downstream actor header=%s", h.gotHeaders["X-Actor-Kind"])
		}
	})

	t.Run("same tenant carrier B company is denied", func(t *testing.T) {
		paths := []string{
			"/api/v1/drivers?carrier_company_id=" + carrierCompanyB,
			"/api/v1/vehicles?carrier_company_id=" + carrierCompanyB,
			"/api/v1/carrier/transport-orders?carrier_company_id=" + carrierCompanyB,
			"/api/v1/order-execution/transport-orders/" + carrierReadOrderID + "?company_id=" + carrierCompanyB + "&actor=CARRIER",
		}
		for _, path := range paths {
			h := newCarrierReadHarness(t, []string{"CARRIER_DISPATCHER"}, carrierAMembership())
			rec := h.do(t, path, nil)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s status=%d body=%s", path, rec.Code, readBody(t, rec))
			}
			h.mu.Lock()
			called := h.called
			h.mu.Unlock()
			if called {
				t.Fatalf("%s reached downstream", path)
			}
		}
	})

	t.Run("shipper company is denied", func(t *testing.T) {
		memberships := []carrierMembership{{
			companyID:   shipperCompanyID,
			companyType: "SHIPPER",
			roles:       []string{"SHIPPER_ADMIN"},
		}}
		paths := []string{
			"/api/v1/drivers?carrier_company_id=" + shipperCompanyID,
			"/api/v1/vehicles?carrier_company_id=" + shipperCompanyID,
			"/api/v1/carrier/transport-orders?carrier_company_id=" + shipperCompanyID,
		}
		for _, path := range paths {
			h := newCarrierReadHarness(t, []string{"SHIPPER_ADMIN"}, memberships)
			rec := h.do(t, path, nil)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s status=%d body=%s", path, rec.Code, readBody(t, rec))
			}
		}
		carrier := newCarrierReadHarness(t, []string{"CARRIER_DISPATCHER"}, carrierAMembership())
		rec := carrier.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+shipperCompanyID+"&actor=BUYER", nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("carrier token shipper company status=%d body=%s", rec.Code, readBody(t, rec))
		}
		shipper := newCarrierReadHarness(t, []string{"SHIPPER_ADMIN"}, memberships)
		rec = shipper.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+shipperCompanyID+"&actor=CARRIER", nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("shipper actor spoof status=%d body=%s", rec.Code, readBody(t, rec))
		}
		shipper.mu.Lock()
		called := shipper.called
		shipper.mu.Unlock()
		if called {
			t.Fatal("downstream called for shipper actor spoof")
		}
	})

	t.Run("verified shipper execution detail stays on the shipper company", func(t *testing.T) {
		memberships := []carrierMembership{{
			companyID:   shipperCompanyID,
			companyType: "SHIPPER",
			roles:       []string{"SHIPPER_ADMIN"},
		}}
		h := newCarrierReadHarness(t, []string{"DRIVER"}, memberships)
		rec := h.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+shipperCompanyID+"&actor=BUYER", nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyA) || strings.Contains(body, carrierCompanyB) {
			t.Fatalf("shipper detail exposed carrier rows: %s", body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotQuery["company_id"] != shipperCompanyID || h.gotQuery["actor"] != "BUYER" {
			t.Fatalf("downstream query=%v", h.gotQuery)
		}
		if h.gotHeaders["X-Actor-Kind"] != "BUYER" || h.gotHeaders["X-Company-ID"] != shipperCompanyID {
			t.Fatalf("downstream headers=%v", h.gotHeaders)
		}
	})

	t.Run("driver role is denied", func(t *testing.T) {
		memberships := []carrierMembership{{
			companyID:   carrierCompanyA,
			companyType: "CARRIER",
			roles:       []string{"DRIVER"},
		}}
		h := newCarrierReadHarness(t, []string{"DRIVER"}, memberships)
		rec := h.do(t, "/api/v1/drivers?carrier_company_id="+carrierCompanyA, nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, readBody(t, rec))
		}
	})

	t.Run("shipper role is denied", func(t *testing.T) {
		memberships := []carrierMembership{{
			companyID:   shipperCompanyID,
			companyType: "SHIPPER",
			roles:       []string{"SHIPPER"},
		}}
		h := newCarrierReadHarness(t, []string{"SHIPPER"}, memberships)
		rec := h.do(t, "/api/v1/carrier/transport-orders?carrier_company_id="+shipperCompanyID, nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, readBody(t, rec))
		}
	})

	t.Run("spoofed company and actor headers are ignored", func(t *testing.T) {
		h := newCarrierReadHarness(t, []string{"DRIVER"}, carrierAMembership())
		rec := h.do(t, "/api/v1/drivers?carrier_company_id="+carrierCompanyA, map[string]string{
			"X-Company-ID": carrierCompanyB,
			"X-Actor-Kind": "BUYER",
		})
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyB) {
			t.Fatalf("spoofed company changed the fleet result: %s", body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotHeaders["X-Company-ID"] != carrierCompanyA {
			t.Fatalf("downstream company header=%s", h.gotHeaders["X-Company-ID"])
		}
		if h.gotHeaders["X-Actor-Kind"] != "CARRIER" {
			t.Fatalf("downstream actor header=%s", h.gotHeaders["X-Actor-Kind"])
		}
	})

	t.Run("spoofed execution actor query is rejected", func(t *testing.T) {
		h := newCarrierReadHarness(t, []string{"CARRIER_DISPATCHER"}, carrierAMembership())
		rec := h.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+carrierCompanyA+"&actor=BUYER", map[string]string{
			"X-Actor-Kind": "BUYER",
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, readBody(t, rec))
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.called {
			t.Fatal("downstream called for spoofed actor")
		}
	})
}

func TestCarrierFleetListRequiresCompanyConstraint(t *testing.T) {
	h := newCarrierReadHarness(t, []string{"CARRIER_DISPATCHER"}, carrierAMembership())
	rec := h.do(t, "/api/v1/drivers", nil)
	body := readBody(t, rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	if strings.Contains(body, carrierCompanyB) {
		t.Fatal("unscoped fleet response included carrier B")
	}
}

func TestPerCompanyRoleBinding(t *testing.T) {
	t.Run("accountant at A is not authorized by dispatcher at B", func(t *testing.T) {
		memberships := []carrierMembership{
			{companyID: carrierCompanyA, companyType: "CARRIER", roles: []string{"CARRIER_ACCOUNTANT"}},
			{companyID: carrierCompanyB, companyType: "CARRIER", roles: []string{"CARRIER_DISPATCHER"}},
		}
		paths := []string{
			"/api/v1/drivers?carrier_company_id=" + carrierCompanyA,
			"/api/v1/vehicles?carrier_company_id=" + carrierCompanyA,
			"/api/v1/carrier/transport-orders?carrier_company_id=" + carrierCompanyA,
			"/api/v1/order-execution/carrier/transport-orders?carrier_company_id=" + carrierCompanyA,
		}
		for _, path := range paths {
			h := newCarrierReadHarness(t, []string{"CARRIER_ACCOUNTANT", "CARRIER_DISPATCHER"}, memberships)
			rec := h.do(t, path, nil)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s status=%d body=%s", path, rec.Code, readBody(t, rec))
			}
			h.mu.Lock()
			called := h.called
			h.mu.Unlock()
			if called {
				t.Fatalf("%s reached downstream", path)
			}
		}
	})

	t.Run("carrier admin at A is authorized despite shipper admin at B", func(t *testing.T) {
		memberships := []carrierMembership{
			{companyID: carrierCompanyA, companyType: "CARRIER", roles: []string{"CARRIER_ADMIN"}},
			{companyID: carrierCompanyB, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
		}
		h := newCarrierReadHarness(t, []string{"SHIPPER_ADMIN"}, memberships)
		rec := h.do(t, "/api/v1/drivers?carrier_company_id="+carrierCompanyA, nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyB) || !strings.Contains(body, carrierCompanyA) {
			t.Fatalf("fleet isolation failed: %s", body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotQuery["carrier_company_id"] != carrierCompanyA || h.gotHeaders["X-Company-ID"] != carrierCompanyA {
			t.Fatalf("downstream company query=%s header=%s", h.gotQuery["carrier_company_id"], h.gotHeaders["X-Company-ID"])
		}
		if h.gotHeaders["X-Actor-Kind"] != "CARRIER" {
			t.Fatalf("downstream actor=%s", h.gotHeaders["X-Actor-Kind"])
		}
	})

	t.Run("shipper admin at A stays buyer and cannot become carrier", func(t *testing.T) {
		memberships := []carrierMembership{
			{companyID: carrierCompanyA, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
			{companyID: carrierCompanyB, companyType: "CARRIER", roles: []string{"CARRIER_DISPATCHER"}},
		}
		h := newCarrierReadHarness(t, []string{"CARRIER_DISPATCHER"}, memberships)
		rec := h.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+carrierCompanyA+"&actor=BUYER", nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("buyer status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyB) {
			t.Fatalf("buyer detail exposed carrier B: %s", body)
		}
		h.mu.Lock()
		if h.gotQuery["company_id"] != carrierCompanyA || h.gotQuery["actor"] != "BUYER" || h.gotHeaders["X-Actor-Kind"] != "BUYER" {
			t.Fatalf("downstream query=%v headers=%v", h.gotQuery, h.gotHeaders)
		}
		h.mu.Unlock()

		denied := newCarrierReadHarness(t, []string{"CARRIER_DISPATCHER"}, memberships)
		rec = denied.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+carrierCompanyA+"&actor=CARRIER", nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("carrier actor status=%d body=%s", rec.Code, readBody(t, rec))
		}
		denied.mu.Lock()
		called := denied.called
		denied.mu.Unlock()
		if called {
			t.Fatal("downstream called for carrier actor on shipper membership")
		}
	})

	t.Run("unlisted role at A is not authorized by another company", func(t *testing.T) {
		memberships := []carrierMembership{
			{companyID: carrierCompanyA, companyType: "SHIPPER", roles: []string{"SHIPPER"}},
			{companyID: carrierCompanyB, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
		}
		h := newCarrierReadHarness(t, []string{"SHIPPER_ADMIN", "CARRIER_DISPATCHER"}, memberships)
		rec := h.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+carrierCompanyA+"&actor=BUYER", nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, readBody(t, rec))
		}
		h.mu.Lock()
		called := h.called
		h.mu.Unlock()
		if called {
			t.Fatal("downstream called for unlisted company A role")
		}
	})

	t.Run("carrier membership only in A cannot read B", func(t *testing.T) {
		h := newCarrierReadHarness(t, []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER"}, carrierAMembership())
		for _, path := range []string{
			"/api/v1/drivers?carrier_company_id=" + carrierCompanyB,
			"/api/v1/carrier/transport-orders?carrier_company_id=" + carrierCompanyB,
			"/api/v1/order-execution/transport-orders/" + carrierReadOrderID + "?company_id=" + carrierCompanyB + "&actor=CARRIER",
		} {
			rec := h.do(t, path, nil)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s status=%d body=%s", path, rec.Code, readBody(t, rec))
			}
		}
		h.mu.Lock()
		called := h.called
		h.mu.Unlock()
		if called {
			t.Fatal("downstream called for company B")
		}
	})

	t.Run("spoofed headers cannot change verified company or actor", func(t *testing.T) {
		h := newCarrierReadHarness(t, []string{"SHIPPER_ADMIN"}, carrierAMembership())
		rec := h.do(t, "/api/v1/drivers?carrier_company_id="+carrierCompanyA, map[string]string{
			"X-Company-ID": carrierCompanyB,
			"X-Actor-Kind": "BUYER",
		})
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		if strings.Contains(body, carrierCompanyB) {
			t.Fatalf("spoofed company changed the fleet result: %s", body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotHeaders["X-Company-ID"] != carrierCompanyA || h.gotHeaders["X-Actor-Kind"] != "CARRIER" {
			t.Fatalf("downstream headers=%v", h.gotHeaders)
		}
	})

	t.Run("tenant platform admin keeps buyer execution without another company role", func(t *testing.T) {
		memberships := []carrierMembership{
			{companyID: shipperCompanyID, companyType: "SHIPPER", roles: []string{"SHIPPER"}},
		}
		h := newCarrierReadHarness(t, []string{"DRIVER"}, memberships)
		h.tenantRoles = []string{"PLATFORM_ADMIN"}
		rec := h.do(t, "/api/v1/order-execution/transport-orders/"+carrierReadOrderID+"?company_id="+shipperCompanyID+"&actor=BUYER", nil)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, body)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.gotQuery["actor"] != "BUYER" || h.gotHeaders["X-Company-ID"] != shipperCompanyID {
			t.Fatalf("downstream query=%v headers=%v", h.gotQuery, h.gotHeaders)
		}
	})
}
