package http_test

import (
	"bytes"
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

type carrierDocumentCapture struct {
	mu       sync.Mutex
	hits     int
	path     string
	rawQuery string
	tenant   string
	user     string
	company  string
	actor    string
	method   string
}

func (c *carrierDocumentCapture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++
	c.path = r.URL.Path
	c.rawQuery = r.URL.RawQuery
	c.tenant = r.Header.Get("X-Tenant-ID")
	c.user = r.Header.Get("X-User-ID")
	c.company = r.Header.Get("X-Company-ID")
	c.actor = r.Header.Get("X-Actor-Kind")
	c.method = r.Method
}

type carrierDocumentHarness struct {
	handler      http.Handler
	document     *carrierDocumentCapture
	documentSrv  *httptest.Server
	shipmentHits int
	identityCode int
	statusCode   int
	tenantRole   []string
}

func newCarrierDocumentHarness(t *testing.T, authMe []string, memberships []shipperMembership) *carrierDocumentHarness {
	t.Helper()
	h := &carrierDocumentHarness{document: &carrierDocumentCapture{}, statusCode: http.StatusOK}
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.shipmentHits++
		http.Error(w, "shipment", http.StatusTeapot)
	}))
	t.Cleanup(shipment.Close)
	h.documentSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.document.record(r)
		if strings.HasSuffix(r.URL.Path, "/content") {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="pod.pdf"`)
			w.Header().Set("X-Content-SHA256", "abc123")
			w.Header().Set("X-Storage-Bucket", "secret-bucket")
			w.Header().Set("X-Object-Key", "secret-key")
			w.Header().Set("X-Storage-Key", "secret-storage")
			_, _ = w.Write([]byte{0x25, 0x50, 0x44, 0x46})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.statusCode)
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	t.Cleanup(h.documentSrv.Close)
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
	cfg.Services.Document = h.documentSrv.URL
	cfg.Services.Tracking = shipment.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.handler = gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, tracking.NewHandler(testLogger(), cfg), nil)
	return h
}

func (h *carrierDocumentHarness) do(t *testing.T, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return h.doAuth(t, method, path, headers, true)
}

func (h *carrierDocumentHarness) doAuth(t *testing.T, method, path string, headers map[string]string, withJWT bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
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

func (h *carrierDocumentHarness) hits() int {
	h.document.mu.Lock()
	defer h.document.mu.Unlock()
	return h.document.hits
}

func carrierDocumentPaths() []struct {
	path   string
	list   bool
	source string
} {
	base := "/api/v1/carrier/documents"
	detail := base + "/" + shipperDocumentID
	attachment := detail + "/attachments/" + shipperAttachmentID
	return []struct {
		path   string
		list   bool
		source string
	}{
		{base, true, "/v1/carrier/documents"},
		{detail, false, "/v1/carrier/documents/" + shipperDocumentID},
		{detail + "/attachments", false, "/v1/carrier/documents/" + shipperDocumentID + "/attachments"},
		{attachment, false, "/v1/carrier/documents/" + shipperDocumentID + "/attachments/" + shipperAttachmentID},
		{attachment + "/content", false, "/v1/carrier/documents/" + shipperDocumentID + "/attachments/" + shipperAttachmentID + "/content"},
	}
}

func TestCarrierDocumentMembershipGate(t *testing.T) {
	spoof := map[string]string{
		"X-Company-ID": shipperCompanyB,
		"X-Actor-Kind": "BUYER",
		"X-Tenant-ID":  shipperOtherTenantID,
		"X-User-ID":    "spoof-user",
	}
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER"} {
		for _, route := range carrierDocumentPaths() {
			h := newCarrierDocumentHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "CARRIER", role))
			query := "carrier_company_id=" + strings.ToUpper(shipperCompanyA)
			if route.list {
				query += "&shipment_id=" + strings.ToUpper(shipperReadShipment) + "&document_type=POD&document_status=SIGNED&limit=20&offset=0&owner_company_id=" + shipperCompanyB + "&shipper_company_id=" + shipperCompanyB + "&actor=BUYER&role=DRIVER&related_entity_type=SHIPMENT"
			}
			rec := h.do(t, http.MethodGet, route.path+"?"+query, spoof)
			if rec.Code != http.StatusOK || h.hits() != 1 || h.shipmentHits != 0 {
				t.Fatalf("%s %s status=%d hits=%d shipment=%d body=%s", role, route.path, rec.Code, h.hits(), h.shipmentHits, readBody(t, rec))
			}
			h.document.mu.Lock()
			gotPath, raw, tenant, user := h.document.path, h.document.rawQuery, h.document.tenant, h.document.user
			company, actor := h.document.company, h.document.actor
			h.document.mu.Unlock()
			if gotPath != route.source || strings.HasPrefix(gotPath, "/v1/documents") || strings.Contains(gotPath, "/v1/shipper/") {
				t.Fatalf("%s path=%s", route.path, gotPath)
			}
			parsed, err := url.ParseQuery(raw)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Get("carrier_company_id") != shipperCompanyA || company != shipperCompanyA || actor != "CARRIER" {
				t.Fatalf("%s query=%s company=%s actor=%s", route.path, raw, company, actor)
			}
			if tenant != shipperReadTenantID || user != shipperReadUserID {
				t.Fatalf("%s tenant=%s user=%s", route.path, tenant, user)
			}
			for _, forbidden := range []string{"tenant_id", "company_id", "shipper_company_id", "owner_company_id", "actor", "role", "related_entity_type"} {
				if _, present := parsed[forbidden]; present {
					t.Fatalf("%s forwarded %s in %s", route.path, forbidden, raw)
				}
			}
			if route.list {
				if parsed.Get("shipment_id") != shipperReadShipment || parsed.Get("document_type") != "POD" || parsed.Get("document_status") != "SIGNED" || parsed.Get("limit") != "20" || parsed.Get("offset") != "0" {
					t.Fatalf("list query=%s", raw)
				}
			} else if _, present := parsed["shipment_id"]; present || parsed.Get("document_type") != "" {
				t.Fatalf("detail query=%s", raw)
			}
		}
	}
}

func TestCarrierDocumentDeniesWrongMembership(t *testing.T) {
	path := "/api/v1/carrier/documents?carrier_company_id=" + shipperCompanyA + "&shipment_id=" + shipperReadShipment
	cases := []struct {
		name        string
		memberships []shipperMembership
		tenantAdmin bool
	}{
		{name: "cross company", memberships: []shipperMembership{
			{companyID: shipperCompanyA, companyType: "CARRIER", roles: []string{"CARRIER_ACCOUNTANT"}},
			{companyID: shipperCompanyB, companyType: "CARRIER", roles: []string{"CARRIER_DISPATCHER"}},
		}},
		{name: "accountant", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ACCOUNTANT")},
		{name: "driver", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "DRIVER")},
		{name: "shipper admin", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "SHIPPER_ADMIN")},
		{name: "shipper logist", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "SHIPPER_LOGIST")},
		{name: "forwarder role", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "FORWARDER_MANAGER")},
		{name: "consignee", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "CONSIGNEE_OPERATOR")},
		{name: "procurement", memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "PROCUREMENT_MANAGER")},
		{name: "shipper company", memberships: oneShipperMembership(shipperCompanyA, "SHIPPER", "CARRIER_ADMIN")},
		{name: "forwarder company", memberships: oneShipperMembership(shipperCompanyA, "FORWARDER", "CARRIER_DISPATCHER")},
		{name: "no membership"},
		{name: "platform admin", tenantAdmin: true},
	}
	for _, tt := range cases {
		h := newCarrierDocumentHarness(t, []string{"PLATFORM_ADMIN"}, tt.memberships)
		if tt.tenantAdmin {
			h.tenantRole = []string{"PLATFORM_ADMIN"}
		}
		rec := h.do(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusForbidden || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d body=%s", tt.name, rec.Code, h.hits(), readBody(t, rec))
		}
	}
}

func TestCarrierDocumentQueryFailClosed(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN")
	base := "/api/v1/carrier/documents"
	h := newCarrierDocumentHarness(t, nil, membership)
	for _, path := range []string{
		base,
		base + "?carrier_company_id=not-a-uuid",
		base + "?carrier_company_id=" + shipperCompanyA,
		base + "?carrier_company_id=" + shipperCompanyA + "&shipment_id=not-a-uuid",
		base + "?carrier_company_id=" + shipperCompanyA + "&shipment_id=" + shipperReadShipment + "&tenant_id=" + shipperReadTenantID,
		base + "?carrier_company_id=" + shipperCompanyA + "&shipment_id=" + shipperReadShipment + "&related_entity_type=INVOICE",
		base + "/" + shipperDocumentID + "?carrier_company_id=" + shipperCompanyA + "&tenant_id=" + shipperReadTenantID,
	} {
		rec := h.do(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusBadRequest || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d body=%s", path, rec.Code, h.hits(), readBody(t, rec))
		}
	}
	rec := h.do(t, http.MethodGet, base+"?carrier_company_id="+shipperCompanyA+"&carrier_company_id="+shipperCompanyB+"&shipment_id="+shipperReadShipment, nil)
	if rec.Code != http.StatusForbidden || h.hits() != 0 {
		t.Fatalf("conflict status=%d hits=%d", rec.Code, h.hits())
	}
	rec = h.do(t, http.MethodGet, "/api/v1/carrier/documents/..?carrier_company_id="+shipperCompanyA, nil)
	if rec.Code != http.StatusBadRequest || h.hits() != 0 {
		t.Fatalf("traversal status=%d hits=%d body=%s", rec.Code, h.hits(), readBody(t, rec))
	}
}

func TestCarrierDocumentStatusAndContent(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN")
	path := "/api/v1/carrier/documents/" + shipperDocumentID + "?carrier_company_id=" + shipperCompanyA
	for _, code := range []int{http.StatusOK, http.StatusBadRequest, http.StatusNotFound, http.StatusServiceUnavailable} {
		h := newCarrierDocumentHarness(t, nil, membership)
		h.statusCode = code
		rec := h.do(t, http.MethodGet, path, nil)
		body := readBody(t, rec)
		if rec.Code != code || strings.Contains(body, shipperCompanyB) {
			t.Fatalf("source %d status=%d body=%s", code, rec.Code, body)
		}
	}
	h := newCarrierDocumentHarness(t, nil, membership)
	h.documentSrv.Close()
	rec := h.do(t, http.MethodGet, path, nil)
	if rec.Code != http.StatusServiceUnavailable || h.hits() != 0 {
		t.Fatalf("unreachable status=%d hits=%d", rec.Code, h.hits())
	}

	h = newCarrierDocumentHarness(t, nil, membership)
	content := "/api/v1/carrier/documents/" + shipperDocumentID + "/attachments/" + shipperAttachmentID + "/content?carrier_company_id=" + shipperCompanyA
	rec = h.do(t, http.MethodGet, content, nil)
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !bytes.Equal(body, []byte{0x25, 0x50, 0x44, 0x46}) {
		t.Fatalf("content status=%d body=%x", rec.Code, body)
	}
	if rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-SHA256") != "abc123" || !strings.Contains(rec.Header().Get("Content-Disposition"), "pod.pdf") || rec.Header().Get("Content-Length") == "" {
		t.Fatalf("headers=%v", rec.Header())
	}
	if rec.Header().Get("X-Storage-Bucket") != "" || rec.Header().Get("X-Object-Key") != "" || rec.Header().Get("X-Storage-Key") != "" {
		t.Fatal("storage metadata leaked")
	}
}

func TestCarrierDocumentIdentityFailures(t *testing.T) {
	path := "/api/v1/carrier/documents?carrier_company_id=" + shipperCompanyA + "&shipment_id=" + shipperReadShipment
	h := newCarrierDocumentHarness(t, nil, oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN"))
	rec := h.doAuth(t, http.MethodGet, path, nil, false)
	if rec.Code != http.StatusUnauthorized || h.hits() != 0 {
		t.Fatalf("missing jwt status=%d", rec.Code)
	}
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusBadGateway} {
		h = newCarrierDocumentHarness(t, nil, oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN"))
		h.identityCode = code
		rec = h.do(t, http.MethodGet, path, nil)
		want := http.StatusServiceUnavailable
		if code == http.StatusUnauthorized {
			want = http.StatusUnauthorized
		}
		if code == http.StatusForbidden {
			want = http.StatusForbidden
		}
		if rec.Code != want || h.hits() != 0 {
			t.Fatalf("identity %d status=%d hits=%d", code, rec.Code, h.hits())
		}
	}
}

func TestCarrierLegacyDocumentBypass(t *testing.T) {
	id := shipperDocumentID
	att := shipperAttachmentID
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/documents"},
		{http.MethodGet, "/api/v1/documents/" + id},
		{http.MethodPost, "/api/v1/documents"},
		{http.MethodPost, "/api/v1/documents/" + id + "/versions"},
		{http.MethodPost, "/api/v1/documents/" + id + "/files"},
		{http.MethodPost, "/api/v1/documents/" + id + "/ready-for-signing"},
		{http.MethodPost, "/api/v1/documents/" + id + "/signing-sessions"},
		{http.MethodPost, "/api/v1/documents/" + id + "/cancel"},
		{http.MethodPost, "/api/v1/documents/" + id + "/archive"},
		{http.MethodPost, "/api/v1/documents/" + id + "/attachments"},
		{http.MethodGet, "/api/v1/documents/" + id + "/attachments/" + att},
		{http.MethodGet, "/api/v1/documents/" + id + "/attachments/" + att + "/content"},
		{http.MethodPost, "/api/v1/documents/" + id + "/attachments/" + att + "/finalize"},
		{http.MethodPost, "/api/v1/documents/" + id + "/attachments/" + att + "/signatures"},
		{http.MethodGet, "/api/v1/signing-sessions/" + id},
		{http.MethodPost, "/api/v1/signing-sessions/" + id + "/signatures"},
	}
	for _, role := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT", "DRIVER"} {
		h := newCarrierDocumentHarness(t, []string{role}, oneShipperMembership(shipperCompanyA, "CARRIER", role))
		for _, route := range paths {
			rec := h.do(t, route.method, route.path, nil)
			if rec.Code != http.StatusForbidden || h.hits() != 0 {
				t.Fatalf("%s %s %s status=%d hits=%d", role, route.method, route.path, rec.Code, h.hits())
			}
		}
	}
	h := newCarrierDocumentHarness(t, []string{"PLATFORM_ADMIN"}, nil)
	h.tenantRole = []string{"PLATFORM_ADMIN"}
	rec := h.do(t, http.MethodGet, "/api/v1/documents", nil)
	if rec.Code != http.StatusOK || h.hits() != 1 {
		t.Fatalf("operator status=%d hits=%d body=%s", rec.Code, h.hits(), readBody(t, rec))
	}
	h.document.mu.Lock()
	got := h.document.path
	h.document.mu.Unlock()
	if strings.Contains(got, "/v1/carrier/") {
		t.Fatalf("operator path=%s", got)
	}
	rec = h.do(t, http.MethodGet, "/api/v1/carrier/documents?carrier_company_id="+shipperCompanyA+"&shipment_id="+shipperReadShipment, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("platform customer status=%d", rec.Code)
	}
}
