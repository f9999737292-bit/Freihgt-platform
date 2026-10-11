package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

type carrierTrackingCapture struct {
	mu            sync.Mutex
	hits          int
	path          string
	rawQuery      string
	tenant        string
	company       string
	actor         string
	requestID     string
	authorization string
	user          string
}

func (c *carrierTrackingCapture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++
	c.path = r.URL.Path
	c.rawQuery = r.URL.RawQuery
	c.tenant = r.Header.Get("X-Tenant-ID")
	c.company = r.Header.Get("X-Company-ID")
	c.actor = r.Header.Get("X-Actor-Kind")
	c.requestID = r.Header.Get("X-Request-ID")
	c.authorization = r.Header.Get("Authorization")
	c.user = r.Header.Get("X-User-ID")
}

type carrierTrackingHarness struct {
	handler      http.Handler
	tracking     *carrierTrackingCapture
	trackingSrv  *httptest.Server
	shipmentHits int
	identityCode int
	trackingCode int
	trackingBody string
	tenantRole   []string
}

func newCarrierTrackingHarness(t *testing.T, authMe []string, memberships []shipperMembership) *carrierTrackingHarness {
	t.Helper()
	h := &carrierTrackingHarness{
		tracking:     &carrierTrackingCapture{},
		trackingCode: http.StatusOK,
		trackingBody: `{"shipmentId":"` + shipperReadShipment + `","trackingStatus":"in_transit"}`,
	}
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.shipmentHits++
		http.Error(w, "shipment", http.StatusTeapot)
	}))
	t.Cleanup(shipment.Close)
	h.trackingSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.tracking.record(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.trackingCode)
		_, _ = io.WriteString(w, h.trackingBody)
	}))
	t.Cleanup(h.trackingSrv.Close)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.identityCode != 0 && strings.Contains(r.URL.Path, "/companies") {
			w.WriteHeader(h.identityCode)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/auth/me"):
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": authMe})
		case strings.Contains(r.URL.Path, "/roles"):
			items := make([]map[string]any, 0)
			for _, code := range h.tenantRole {
				items = append(items, map[string]any{"code": code})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case strings.Contains(r.URL.Path, "/companies"):
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
	cfg.Services.Tracking = h.trackingSrv.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.handler = gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, tracking.NewHandler(testLogger(), cfg), nil)
	return h
}

func (h *carrierTrackingHarness) do(t *testing.T, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return h.doAuth(t, path, headers, true)
}

func (h *carrierTrackingHarness) doAuth(t *testing.T, path string, headers map[string]string, withJWT bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if withJWT {
		req.Header.Set("Authorization", "Bearer "+signShipperReadToken(t, shipperReadTenantID))
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *carrierTrackingHarness) hits() int {
	h.tracking.mu.Lock()
	defer h.tracking.mu.Unlock()
	return h.tracking.hits
}

func carrierTrackingPaths() []string {
	base := "/api/v1/carrier/shipments/" + shipperReadShipment
	return []string{
		base + "/tracking",
		base + "/tracking/locations",
		base + "/eta",
		base + "/eta/history",
		base + "/slots",
		base + "/slots/history",
	}
}

func TestCarrierTrackingMembershipGate(t *testing.T) {
	spoof := map[string]string{
		"X-Company-ID": shipperCompanyB,
		"X-Actor-Kind": "BUYER",
		"X-Tenant-ID":  shipperOtherTenantID,
		"X-User-ID":    "spoof-user",
		"X-Request-ID": "req-carrier-1",
	}
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER"} {
		for _, path := range carrierTrackingPaths() {
			h := newCarrierTrackingHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "CARRIER", role))
			rec := h.do(t, path+"?carrier_company_id="+strings.ToUpper(shipperCompanyA)+"&shipper_company_id="+shipperCompanyB+"&tenant_id=1&company_id="+shipperCompanyA+"&actor=BUYER&user_id=u&role=DRIVER", spoof)
			if rec.Code != http.StatusOK || h.hits() != 1 || h.shipmentHits != 0 {
				t.Fatalf("%s %s status=%d hits=%d shipment=%d body=%s", role, path, rec.Code, h.hits(), h.shipmentHits, readBody(t, rec))
			}
			h.tracking.mu.Lock()
			gotPath, raw, tenant, company := h.tracking.path, h.tracking.rawQuery, h.tracking.tenant, h.tracking.company
			actor, requestID, auth, user := h.tracking.actor, h.tracking.requestID, h.tracking.authorization, h.tracking.user
			h.tracking.mu.Unlock()
			wantPath := "/v1/carrier" + strings.TrimPrefix(path, "/api/v1/carrier")
			if gotPath != wantPath || strings.HasPrefix(gotPath, "/v1/shipments/") {
				t.Fatalf("%s downstream path=%s", path, gotPath)
			}
			if !strings.Contains(raw, "carrier_company_id="+shipperCompanyA) || company != shipperCompanyA || actor != "CARRIER" {
				t.Fatalf("%s query=%s company=%s actor=%s", path, raw, company, actor)
			}
			parsed, err := url.ParseQuery(raw)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"shipper_company_id", "tenant_id", "company_id", "actor", "user_id", "role"} {
				if _, present := parsed[forbidden]; present {
					t.Fatalf("%s forwarded %s in %s", path, forbidden, raw)
				}
			}
			if tenant != shipperReadTenantID || user == "spoof-user" || auth != "" || requestID != "req-carrier-1" {
				t.Fatalf("%s tenant=%s user=%s auth=%s request=%s", path, tenant, user, auth, requestID)
			}
		}
	}
}

func TestCarrierTrackingDeniesWrongMembership(t *testing.T) {
	cases := []struct {
		name        string
		memberships []shipperMembership
		tenantAdmin bool
	}{
		{
			name: "cross company role",
			memberships: []shipperMembership{
				{companyID: shipperCompanyA, companyType: "CARRIER", roles: []string{"DRIVER"}},
				{companyID: shipperCompanyB, companyType: "CARRIER", roles: []string{"CARRIER_ADMIN"}},
			},
		},
		{name: "accountant", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ACCOUNTANT")},
		{name: "driver", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "DRIVER")},
		{name: "shipper role on carrier", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "SHIPPER_ADMIN")},
		{name: "forwarder role on carrier", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "FORWARDER_MANAGER")},
		{name: "consignee", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "CONSIGNEE_OPERATOR")},
		{name: "procurement", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "PROCUREMENT_MANAGER")},
		{name: "shipper company", memberships: oneShipperMembership(shipperCompanyA, "SHIPPER", "CARRIER_ADMIN")},
		{name: "forwarder company", memberships: oneShipperMembership(shipperCompanyA, "FORWARDER", "CARRIER_DISPATCHER")},
		{name: "no membership"},
		{name: "platform admin", tenantAdmin: true},
	}
	for _, tt := range cases {
		h := newCarrierTrackingHarness(t, []string{"PLATFORM_ADMIN"}, tt.memberships)
		if tt.tenantAdmin {
			h.tenantRole = []string{"PLATFORM_ADMIN"}
		}
		rec := h.do(t, carrierTrackingPaths()[0]+"?carrier_company_id="+shipperCompanyA, nil)
		if rec.Code != http.StatusForbidden || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d body=%s", tt.name, rec.Code, h.hits(), readBody(t, rec))
		}
	}

	h := newCarrierTrackingHarness(t, nil, []shipperMembership{
		{companyID: shipperCompanyA, companyType: "CARRIER", roles: []string{"CARRIER_ADMIN"}},
		{companyID: shipperCompanyB, companyType: "CARRIER", roles: []string{"DRIVER"}},
	})
	rec := h.do(t, carrierTrackingPaths()[0]+"?carrier_company_id="+shipperCompanyB, nil)
	if rec.Code != http.StatusForbidden || h.hits() != 0 {
		t.Fatalf("company B status=%d hits=%d", rec.Code, h.hits())
	}
}

func TestCarrierTrackingQueryFailClosed(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN")
	path := carrierTrackingPaths()[0]
	h := newCarrierTrackingHarness(t, nil, membership)
	for _, suffix := range []string{"", "?carrier_company_id=not-a-uuid"} {
		rec := h.do(t, path+suffix, nil)
		if rec.Code != http.StatusBadRequest || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d", suffix, rec.Code, h.hits())
		}
	}
	rec := h.do(t, path+"?carrier_company_id="+shipperCompanyA+"&carrier_company_id="+shipperCompanyB, nil)
	if rec.Code != http.StatusForbidden || h.hits() != 0 {
		t.Fatalf("conflict status=%d hits=%d", rec.Code, h.hits())
	}
	rec = h.do(t, path+"?carrier_company_id="+shipperCompanyA+"&company_id="+shipperCompanyB, nil)
	if rec.Code != http.StatusForbidden || h.hits() != 0 {
		t.Fatalf("foreign company_id status=%d hits=%d", rec.Code, h.hits())
	}
}

func TestCarrierTrackingFactTamperAndFilters(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_DISPATCHER")
	base := "/api/v1/carrier/shipments/" + shipperReadShipment
	company := "carrier_company_id=" + shipperCompanyA
	denied := []string{
		base + "/eta?" + company + "&plannedPickupAt=2026-01-01T00:00:00Z",
		base + "/eta/history?" + company + "&shipmentStatus=delivered",
		base + "/slots?" + company + "&pickupEtaStatus=late",
		base + "/slots/history?" + company + "&actualDeliveryAt=2026-01-01T00:00:00Z",
	}
	for _, path := range denied {
		h := newCarrierTrackingHarness(t, nil, membership)
		rec := h.do(t, path, nil)
		if rec.Code != http.StatusBadRequest || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d body=%s", path, rec.Code, h.hits(), readBody(t, rec))
		}
	}

	h := newCarrierTrackingHarness(t, nil, membership)
	rec := h.do(t, base+"/tracking/locations?"+company+"&from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&limit=5&offset=1&shipper_company_id="+shipperCompanyB, nil)
	if rec.Code != http.StatusOK || h.hits() != 1 {
		t.Fatalf("locations status=%d hits=%d", rec.Code, h.hits())
	}
	h.tracking.mu.Lock()
	raw := h.tracking.rawQuery
	h.tracking.mu.Unlock()
	for _, key := range []string{"carrier_company_id=" + shipperCompanyA, "from=", "to=", "limit=5", "offset=1"} {
		if !strings.Contains(raw, key) {
			t.Fatalf("locations query %q missing %s", raw, key)
		}
	}
	if strings.Contains(raw, "shipper_company_id") {
		t.Fatalf("locations forwarded shipper company %s", raw)
	}

	h = newCarrierTrackingHarness(t, nil, membership)
	rec = h.do(t, base+"/eta/history?"+company+"&targetType=delivery&from=2026-01-01T00:00:00Z&limit=2", nil)
	if rec.Code != http.StatusOK || !strings.Contains(h.tracking.rawQuery, "targetType=delivery") {
		t.Fatalf("eta history query=%s status=%d", h.tracking.rawQuery, rec.Code)
	}
	h = newCarrierTrackingHarness(t, nil, membership)
	rec = h.do(t, base+"/slots/history?"+company+"&slotType=pickup&offset=3", nil)
	if rec.Code != http.StatusOK || !strings.Contains(h.tracking.rawQuery, "slotType=pickup") {
		t.Fatalf("slot history query=%s status=%d", h.tracking.rawQuery, rec.Code)
	}
}

func TestCarrierTrackingStatusPropagation(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN")
	path := carrierTrackingPaths()[0] + "?carrier_company_id=" + shipperCompanyA
	for _, code := range []int{http.StatusOK, http.StatusBadRequest, http.StatusNotFound, http.StatusServiceUnavailable} {
		h := newCarrierTrackingHarness(t, nil, membership)
		h.trackingCode = code
		h.trackingBody = `{"error":"source"}`
		rec := h.do(t, path, nil)
		body := readBody(t, rec)
		if rec.Code != code || !strings.Contains(body, `"error":"source"`) || strings.Contains(body, shipperCompanyB) {
			t.Fatalf("source %d status=%d body=%s", code, rec.Code, body)
		}
	}

	h := newCarrierTrackingHarness(t, nil, membership)
	h.trackingSrv.Close()
	rec := h.do(t, path, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unreachable status=%d body=%s", rec.Code, readBody(t, rec))
	}
}

func TestCarrierTrackingIdentityFailures(t *testing.T) {
	path := carrierTrackingPaths()[0] + "?carrier_company_id=" + shipperCompanyA
	h := newCarrierTrackingHarness(t, nil, oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN"))
	rec := h.doAuth(t, path, nil, false)
	if rec.Code != http.StatusUnauthorized || h.hits() != 0 {
		t.Fatalf("missing jwt status=%d hits=%d", rec.Code, h.hits())
	}
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusBadGateway} {
		h = newCarrierTrackingHarness(t, nil, oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN"))
		h.identityCode = code
		rec = h.do(t, path, nil)
		want := http.StatusServiceUnavailable
		if code == http.StatusUnauthorized {
			want = http.StatusUnauthorized
		}
		if code == http.StatusForbidden {
			want = http.StatusForbidden
		}
		if rec.Code != want || h.hits() != 0 {
			t.Fatalf("identity %d status=%d hits=%d body=%s", code, rec.Code, h.hits(), readBody(t, rec))
		}
	}
}

func TestCarrierLegacyTrackingBypass(t *testing.T) {
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER", "DRIVER"} {
		h := newCarrierTrackingHarness(t, []string{role}, oneShipperMembership(shipperCompanyA, "CARRIER", role))
		for _, path := range legacyTrackingPaths() {
			rec := h.do(t, path, nil)
			if rec.Code != http.StatusForbidden || h.hits() != 0 {
				t.Fatalf("%s %s status=%d hits=%d", role, path, rec.Code, h.hits())
			}
		}
	}
	h := newCarrierTrackingHarness(t, []string{"PLATFORM_ADMIN"}, nil)
	h.tenantRole = []string{"PLATFORM_ADMIN"}
	rec := h.do(t, legacyTrackingPaths()[0], nil)
	if rec.Code != http.StatusOK || h.hits() != 1 {
		t.Fatalf("operator status=%d hits=%d body=%s", rec.Code, h.hits(), readBody(t, rec))
	}
	h.tracking.mu.Lock()
	got := h.tracking.path
	h.tracking.mu.Unlock()
	if strings.Contains(got, "/v1/carrier/") || !strings.HasPrefix(got, "/v1/shipments/") {
		t.Fatalf("operator path=%s", got)
	}
}
