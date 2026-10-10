package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

type trackingCapture struct {
	mu       sync.Mutex
	hits     int
	path     string
	query    string
	rawQuery string
	tenant   string
	user     string
	company  string
	actor    string
	legacy   bool
}

func (c *trackingCapture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++
	c.path = r.URL.Path
	c.query = r.URL.Query().Get("shipper_company_id")
	c.rawQuery = r.URL.RawQuery
	c.tenant = r.Header.Get("X-Tenant-ID")
	c.user = r.Header.Get("X-User-ID")
	c.company = r.Header.Get("X-Company-ID")
	c.actor = r.Header.Get("X-Actor-Kind")
	c.legacy = strings.Contains(r.URL.Path, "/v1/shipments/") && !strings.Contains(r.URL.Path, "/v1/shipper/")
}

type shipperTrackingHarness struct {
	handler    http.Handler
	tracking   *trackingCapture
	identity   *trackingCapture
	tenantRole []string
}

func newShipperTrackingHarness(t *testing.T, authMe []string, memberships []shipperMembership) *shipperTrackingHarness {
	t.Helper()
	h := &shipperTrackingHarness{tracking: &trackingCapture{}, identity: &trackingCapture{}}
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "shipment", http.StatusTeapot)
	}))
	t.Cleanup(shipment.Close)
	trackingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.tracking.record(r)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"shipmentId":     shipperReadShipment,
			"trackingStatus": "in_transit",
		})
	}))
	t.Cleanup(trackingSrv.Close)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.identity.record(r)
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": authMe})
		case strings.Contains(r.URL.Path, "/roles"):
			items := make([]map[string]any, 0)
			for _, code := range h.tenantRole {
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
			if r.URL.Query().Get("tenant_id") != shipperReadTenantID {
				t.Errorf("lookup tenant=%s", r.URL.Query().Get("tenant_id"))
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

func (h *shipperTrackingHarness) do(t *testing.T, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+signShipperReadToken(t, shipperReadTenantID))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *shipperTrackingHarness) hits() int {
	h.tracking.mu.Lock()
	defer h.tracking.mu.Unlock()
	return h.tracking.hits
}

func shipperTrackingPaths() []string {
	base := "/api/v1/shipper/shipments/" + shipperReadShipment
	return []string{
		base + "/tracking",
		base + "/tracking/locations",
		base + "/eta",
		base + "/eta/history",
		base + "/slots",
		base + "/slots/history",
	}
}

func legacyTrackingPaths() []string {
	base := "/api/v1/shipments/" + shipperReadShipment
	return []string{
		base + "/tracking",
		base + "/tracking/locations",
		base + "/eta",
		base + "/eta/history",
		base + "/slots",
		base + "/slots/history",
	}
}

func TestShipperTrackingMembershipGate(t *testing.T) {
	spoof := map[string]string{
		"X-Company-ID": shipperCompanyB,
		"X-Actor-Kind": "CARRIER",
		"X-Tenant-ID":  shipperOtherTenantID,
		"X-User-ID":    "spoof-user",
	}
	for _, path := range shipperTrackingPaths() {
		h := newShipperTrackingHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
		rec := h.do(t, path+"?shipper_company_id="+shipperCompanyA+"&from=2026-01-01T00:00:00Z&limit=10", spoof)
		body := readBody(t, rec)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, body)
		}
		if strings.Contains(body, "providerDeviceId") || strings.Contains(body, "providerEventId") || strings.Contains(body, "providerSlotId") {
			t.Fatalf("%s exposed provider field: %s", path, body)
		}
		h.tracking.mu.Lock()
		gotPath, gotQuery, gotTenant, gotUser := h.tracking.path, h.tracking.query, h.tracking.tenant, h.tracking.user
		gotCompany, gotActor, legacy := h.tracking.company, h.tracking.actor, h.tracking.legacy
		h.tracking.mu.Unlock()
		wantPath := "/v1/shipper" + strings.TrimPrefix(path, "/api/v1/shipper")
		if gotPath != wantPath || legacy {
			t.Fatalf("%s downstream path=%s legacy=%v", path, gotPath, legacy)
		}
		if gotQuery != shipperCompanyA || gotCompany != shipperCompanyA || gotActor != "BUYER" {
			t.Fatalf("%s company query=%s header=%s actor=%s", path, gotQuery, gotCompany, gotActor)
		}
		if gotTenant != shipperReadTenantID || gotUser == "spoof-user" {
			t.Fatalf("%s tenant=%s user=%s", path, gotTenant, gotUser)
		}
	}

	for _, path := range shipperTrackingPaths() {
		h := newShipperTrackingHarness(t, []string{"CARRIER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_LOGIST"))
		rec := h.do(t, path+"?shipper_company_id="+strings.ToUpper(shipperCompanyA), nil)
		if rec.Code != http.StatusOK || h.hits() != 1 {
			t.Fatalf("logist %s status=%d hits=%d body=%s", path, rec.Code, h.hits(), readBody(t, rec))
		}
		h.tracking.mu.Lock()
		if h.tracking.query != shipperCompanyA {
			t.Fatalf("logist canonical query=%s", h.tracking.query)
		}
		h.tracking.mu.Unlock()
	}
}

func TestShipperTrackingDeniesWrongMembership(t *testing.T) {
	cases := []struct {
		name        string
		authMe      []string
		memberships []shipperMembership
		tenantAdmin bool
	}{
		{
			name:   "cross company",
			authMe: []string{"SHIPPER_ADMIN"},
			memberships: []shipperMembership{
				{companyID: shipperCompanyA, companyType: "SHIPPER", roles: []string{"PROCUREMENT_MANAGER"}},
				{companyID: shipperCompanyB, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
			},
		},
		{name: "forwarder", authMe: []string{"FORWARDER_MANAGER"}, memberships: oneShipperMembership(shipperCompanyA, "FORWARDER", "FORWARDER_MANAGER")},
		{name: "lsp", authMe: []string{"SHIPPER_ADMIN"}, memberships: oneShipperMembership(shipperCompanyA, "LSP", "SHIPPER_ADMIN")},
		{name: "carrier", authMe: []string{"CARRIER_ADMIN"}, memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN")},
		{name: "no membership", authMe: []string{"SHIPPER_ADMIN"}, memberships: nil},
		{name: "platform admin", authMe: []string{"PLATFORM_ADMIN"}, memberships: nil, tenantAdmin: true},
	}
	for _, tt := range cases {
		for _, path := range shipperTrackingPaths() {
			h := newShipperTrackingHarness(t, tt.authMe, tt.memberships)
			if tt.tenantAdmin {
				h.tenantRole = []string{"PLATFORM_ADMIN"}
			}
			rec := h.do(t, path+"?shipper_company_id="+shipperCompanyA, nil)
			if rec.Code != http.StatusForbidden || h.hits() != 0 {
				t.Fatalf("%s %s status=%d hits=%d body=%s", tt.name, path, rec.Code, h.hits(), readBody(t, rec))
			}
		}
	}
}

func TestShipperTrackingQueryFailClosed(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN")
	for _, path := range shipperTrackingPaths() {
		h := newShipperTrackingHarness(t, []string{"SHIPPER_ADMIN"}, membership)
		for _, suffix := range []string{"", "?shipper_company_id=not-a-uuid"} {
			rec := h.do(t, path+suffix, nil)
			if rec.Code != http.StatusBadRequest || h.hits() != 0 {
				t.Fatalf("%s%s status=%d hits=%d", path, suffix, rec.Code, h.hits())
			}
		}
		h = newShipperTrackingHarness(t, []string{"SHIPPER_ADMIN"}, membership)
		rec := h.do(t, path+"?shipper_company_id="+shipperCompanyA+"&shipper_company_id="+shipperCompanyB, nil)
		if rec.Code != http.StatusForbidden || h.hits() != 0 {
			t.Fatalf("conflict %s status=%d hits=%d", path, rec.Code, h.hits())
		}
	}
}

func TestShipperTrackingFactTamper(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN")
	base := "/api/v1/shipper/shipments/" + shipperReadShipment
	company := "shipper_company_id=" + shipperCompanyA
	cases := []string{
		base + "/eta?" + company + "&plannedPickupAt=2026-01-01T00:00:00Z",
		base + "/eta/history?" + company + "&shipmentStatus=delivered",
		base + "/slots?" + company + "&pickupEtaStatus=late",
		base + "/slots/history?" + company + "&actualDeliveryAt=2026-01-01T00:00:00Z",
	}
	for _, path := range cases {
		h := newShipperTrackingHarness(t, []string{"SHIPPER_ADMIN"}, membership)
		rec := h.do(t, path, nil)
		if rec.Code != http.StatusBadRequest || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d body=%s", path, rec.Code, h.hits(), readBody(t, rec))
		}
	}
	h := newShipperTrackingHarness(t, []string{"SHIPPER_ADMIN"}, membership)
	rec := h.do(t, base+"/eta/history?"+company+"&targetType=delivery&from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&limit=5&offset=1", nil)
	if rec.Code != http.StatusOK || h.hits() != 1 {
		t.Fatalf("history filter status=%d hits=%d body=%s", rec.Code, h.hits(), readBody(t, rec))
	}
	h.tracking.mu.Lock()
	raw := h.tracking.rawQuery
	h.tracking.mu.Unlock()
	for _, key := range []string{"targetType=delivery", "limit=5", "offset=1", "shipper_company_id=" + shipperCompanyA} {
		if !strings.Contains(raw, key) {
			t.Fatalf("history query %q missing %s", raw, key)
		}
	}
}

func TestShipperLegacyTrackingBypass(t *testing.T) {
	for _, role := range []string{"SHIPPER_ADMIN", "SHIPPER_LOGIST"} {
		h := newShipperTrackingHarness(t, []string{role}, oneShipperMembership(shipperCompanyA, "SHIPPER", role))
		for _, path := range legacyTrackingPaths() {
			rec := h.do(t, path, nil)
			if rec.Code != http.StatusForbidden || h.hits() != 0 {
				t.Fatalf("%s %s status=%d hits=%d body=%s", role, path, rec.Code, h.hits(), readBody(t, rec))
			}
		}
	}

	h := newShipperTrackingHarness(t, []string{"PLATFORM_ADMIN"}, nil)
	h.tenantRole = []string{"PLATFORM_ADMIN"}
	for _, path := range legacyTrackingPaths() {
		before := h.hits()
		rec := h.do(t, path, map[string]string{"X-Company-ID": shipperCompanyB, "X-Actor-Kind": "CARRIER"})
		if rec.Code != http.StatusOK || h.hits() != before+1 {
			t.Fatalf("operator %s status=%d hits=%d body=%s", path, rec.Code, h.hits(), readBody(t, rec))
		}
		h.tracking.mu.Lock()
		legacy, got := h.tracking.legacy, h.tracking.path
		h.tracking.mu.Unlock()
		if !legacy || strings.Contains(got, "/v1/shipper/") {
			t.Fatalf("operator path=%s", got)
		}
	}
}

func TestShipperShipmentRouteStaysOnShipmentService(t *testing.T) {
	h := newShipperTrackingHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
	rec := h.do(t, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
	if rec.Code == http.StatusTeapot && h.hits() == 0 {
		return
	}
	if h.hits() != 0 {
		t.Fatal("shipment inbox reached tracking-service")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("shipment inbox status=%d body=%s", rec.Code, readBody(t, rec))
	}
}
