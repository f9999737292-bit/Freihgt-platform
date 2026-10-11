package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/freight-platform/api-gateway/internal/driver"
	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

const (
	shipperDocumentID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	shipperAttachmentID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
)

type documentCapture struct {
	mu       sync.Mutex
	hits     int
	path     string
	rawQuery string
	tenant   string
	user     string
	company  string
	actor    string
}

func (c *documentCapture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++
	c.path = r.URL.Path
	c.rawQuery = r.URL.RawQuery
	c.tenant = r.Header.Get("X-Tenant-ID")
	c.user = r.Header.Get("X-User-ID")
	c.company = r.Header.Get("X-Company-ID")
	c.actor = r.Header.Get("X-Actor-Kind")
}

type shipperDocumentHarness struct {
	handler    http.Handler
	document   *documentCapture
	tenantRole []string
}

func newShipperDocumentHarness(t *testing.T, authMe []string, memberships []shipperMembership) *shipperDocumentHarness {
	t.Helper()
	h := &shipperDocumentHarness{document: &documentCapture{}}
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "shipment", http.StatusTeapot)
	}))
	t.Cleanup(shipment.Close)
	documents := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.document.record(r)
		if strings.HasSuffix(r.URL.Path, "/content") {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="pod.pdf"`)
			w.Header().Set("X-Content-SHA256", "abc123")
			w.Header().Set("X-Storage-Bucket", "secret-bucket")
			w.Header().Set("X-Object-Key", "secret-key")
			_, _ = w.Write([]byte{0x25, 0x50, 0x44, 0x46})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	t.Cleanup(documents.Close)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			items := make([]map[string]any, 0, len(memberships))
			for _, membership := range memberships {
				roles := make([]map[string]string, 0, len(membership.roles))
				for _, code := range membership.roles {
					roles = append(roles, map[string]string{"code": code})
				}
				items = append(items, map[string]any{
					"company_id": membership.companyID, "company_type": membership.companyType, "roles": roles,
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
	cfg.Services.Document = documents.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.handler = gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, nil)
	return h
}

func (h *shipperDocumentHarness) do(t *testing.T, method, path string, headers map[string]string) *httptest.ResponseRecorder {
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

func (h *shipperDocumentHarness) hits() int {
	h.document.mu.Lock()
	defer h.document.mu.Unlock()
	return h.document.hits
}

func shipperDocumentRoutes() []struct {
	path string
	list bool
	want string
} {
	base := "/api/v1/shipper/documents"
	detail := base + "/" + shipperDocumentID
	attachment := detail + "/attachments/" + shipperAttachmentID
	return []struct {
		path string
		list bool
		want string
	}{
		{base, true, "/v1/shipper/documents"},
		{detail, false, "/v1/shipper/documents/" + shipperDocumentID},
		{detail + "/attachments", false, "/v1/shipper/documents/" + shipperDocumentID + "/attachments"},
		{attachment, false, "/v1/shipper/documents/" + shipperDocumentID + "/attachments/" + shipperAttachmentID},
		{attachment + "/content", false, "/v1/shipper/documents/" + shipperDocumentID + "/attachments/" + shipperAttachmentID + "/content"},
	}
}

func documentQuery(list bool) string {
	query := "shipper_company_id=" + shipperCompanyA
	if list {
		query += "&shipment_id=" + shipperReadShipment + "&document_type=POD&document_status=SIGNED&limit=20&offset=0"
	}
	return query
}

func TestShipperDocumentMembershipGate(t *testing.T) {
	spoof := map[string]string{
		"X-Company-ID": shipperCompanyB,
		"X-Actor-Kind": "CARRIER",
		"X-Tenant-ID":  shipperOtherTenantID,
		"X-User-ID":    "spoof-user",
	}
	for _, route := range shipperDocumentRoutes() {
		h := newShipperDocumentHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
		rec := h.do(t, http.MethodGet, route.path+"?"+documentQuery(route.list), spoof)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", route.path, rec.Code, readBody(t, rec))
		}
		h.document.mu.Lock()
		gotPath, gotQuery := h.document.path, h.document.rawQuery
		gotTenant, gotUser := h.document.tenant, h.document.user
		gotCompany, gotActor := h.document.company, h.document.actor
		h.document.mu.Unlock()
		if gotPath != route.want || strings.Contains(gotPath, "/v1/documents") {
			t.Fatalf("%s path=%s", route.path, gotPath)
		}
		if !strings.Contains(gotQuery, "shipper_company_id="+shipperCompanyA) {
			t.Fatalf("%s query=%s", route.path, gotQuery)
		}
		if gotTenant != shipperReadTenantID || gotUser != shipperReadUserID || gotUser == "spoof-user" {
			t.Fatalf("%s tenant=%s user=%s", route.path, gotTenant, gotUser)
		}
		if gotCompany != shipperCompanyA || gotActor != "BUYER" {
			t.Fatalf("%s company=%s actor=%s", route.path, gotCompany, gotActor)
		}
		if route.list {
			for _, key := range []string{"shipment_id=" + shipperReadShipment, "document_type=POD", "document_status=SIGNED", "limit=20", "offset=0"} {
				if !strings.Contains(gotQuery, key) {
					t.Fatalf("%s missing %s in %s", route.path, key, gotQuery)
				}
			}
		}
	}

	for _, route := range shipperDocumentRoutes() {
		h := newShipperDocumentHarness(t, []string{"CARRIER_ADMIN"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_LOGIST"))
		rec := h.do(t, http.MethodGet, route.path+"?"+documentQuery(route.list), nil)
		if rec.Code != http.StatusOK || h.hits() != 1 {
			t.Fatalf("logist %s status=%d hits=%d body=%s", route.path, rec.Code, h.hits(), readBody(t, rec))
		}
	}
}

func TestShipperDocumentDeniesWrongMembership(t *testing.T) {
	cases := []struct {
		name        string
		authMe      []string
		memberships []shipperMembership
		tenantAdmin bool
	}{
		{
			name: "cross company", authMe: []string{"SHIPPER_ADMIN"},
			memberships: []shipperMembership{
				{companyID: shipperCompanyA, companyType: "SHIPPER", roles: []string{"PROCUREMENT_MANAGER"}},
				{companyID: shipperCompanyB, companyType: "SHIPPER", roles: []string{"SHIPPER_ADMIN"}},
			},
		},
		{name: "forwarder", authMe: []string{"FORWARDER_MANAGER"}, memberships: oneShipperMembership(shipperCompanyA, "FORWARDER", "FORWARDER_MANAGER")},
		{name: "lsp", authMe: []string{"SHIPPER_ADMIN"}, memberships: oneShipperMembership(shipperCompanyA, "LSP", "SHIPPER_ADMIN")},
		{name: "carrier", authMe: []string{"CARRIER_ADMIN"}, memberships: oneShipperMembership(shipperCompanyA, "CARRIER", "CARRIER_ADMIN")},
		{name: "no membership", authMe: []string{"SHIPPER_ADMIN"}},
		{name: "platform admin", authMe: []string{"PLATFORM_ADMIN"}, tenantAdmin: true},
	}
	for _, tt := range cases {
		for _, route := range shipperDocumentRoutes() {
			h := newShipperDocumentHarness(t, tt.authMe, tt.memberships)
			if tt.tenantAdmin {
				h.tenantRole = []string{"PLATFORM_ADMIN"}
			}
			rec := h.do(t, http.MethodGet, route.path+"?"+documentQuery(route.list), nil)
			if rec.Code != http.StatusForbidden || h.hits() != 0 {
				t.Fatalf("%s %s status=%d hits=%d body=%s", tt.name, route.path, rec.Code, h.hits(), readBody(t, rec))
			}
		}
	}
}

func TestShipperDocumentQueryFailClosed(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN")
	for _, route := range shipperDocumentRoutes() {
		h := newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, membership)
		for _, path := range []string{route.path, route.path + "?shipper_company_id=not-a-uuid"} {
			rec := h.do(t, http.MethodGet, path, nil)
			if rec.Code != http.StatusBadRequest || h.hits() != 0 {
				t.Fatalf("%s status=%d hits=%d", path, rec.Code, h.hits())
			}
		}
		h = newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, membership)
		rec := h.do(t, http.MethodGet, route.path+"?shipper_company_id="+shipperCompanyA+"&shipper_company_id="+shipperCompanyB, nil)
		if rec.Code != http.StatusForbidden || h.hits() != 0 {
			t.Fatalf("conflict %s status=%d hits=%d", route.path, rec.Code, h.hits())
		}
	}
}

func TestShipperDocumentListFilters(t *testing.T) {
	membership := oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN")
	base := "/api/v1/shipper/documents?shipper_company_id=" + shipperCompanyA
	h := newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, membership)
	for _, path := range []string{base, base + "&shipment_id=not-a-uuid"} {
		rec := h.do(t, http.MethodGet, path, nil)
		if rec.Code != http.StatusBadRequest || h.hits() != 0 {
			t.Fatalf("%s status=%d hits=%d", path, rec.Code, h.hits())
		}
	}
	h = newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, membership)
	rec := h.do(t, http.MethodGet, base+"&shipment_id="+shipperReadShipment+"&tenant_id="+shipperOtherTenantID, nil)
	if rec.Code != http.StatusBadRequest || h.hits() != 0 {
		t.Fatalf("tenant query status=%d hits=%d body=%s", rec.Code, h.hits(), readBody(t, rec))
	}
	h = newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, membership)
	rec = h.do(t, http.MethodGet, base+"&shipment_id="+shipperReadShipment+"&related_entity_type=INVOICE", nil)
	if rec.Code != http.StatusBadRequest || h.hits() != 0 {
		t.Fatalf("entity override status=%d hits=%d", rec.Code, h.hits())
	}
	h = newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, membership)
	rec = h.do(t, http.MethodGet, base+"&shipment_id="+strings.ToUpper(shipperReadShipment)+"&related_entity_type=SHIPMENT", nil)
	if rec.Code != http.StatusOK || h.hits() != 1 {
		t.Fatalf("shipment filter status=%d hits=%d body=%s", rec.Code, h.hits(), readBody(t, rec))
	}
	h.document.mu.Lock()
	raw := h.document.rawQuery
	h.document.mu.Unlock()
	if strings.Contains(raw, "related_entity_type") || strings.Contains(raw, "tenant_id") {
		t.Fatalf("forwarded customer authority query=%s", raw)
	}
	if !strings.Contains(raw, "shipment_id="+shipperReadShipment) {
		t.Fatalf("shipment query=%s", raw)
	}
}

func TestShipperAttachmentContentProxy(t *testing.T) {
	h := newShipperDocumentHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
	path := "/api/v1/shipper/documents/" + shipperDocumentID + "/attachments/" + shipperAttachmentID + "/content?shipper_company_id=" + shipperCompanyA
	rec := h.do(t, http.MethodGet, path, nil)
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !bytes.Equal(body, []byte{0x25, 0x50, 0x44, 0x46}) {
		t.Fatalf("status=%d body=%x", rec.Code, body)
	}
	if rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-SHA256") != "abc123" {
		t.Fatalf("headers=%v", rec.Header())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "pod.pdf") {
		t.Fatalf("disposition=%s", rec.Header().Get("Content-Disposition"))
	}
	if rec.Header().Get("Content-Length") == "" {
		t.Fatal("Content-Length not preserved")
	}
	if rec.Header().Get("X-Storage-Bucket") != "" || rec.Header().Get("X-Object-Key") != "" {
		t.Fatal("storage metadata leaked")
	}
	if strings.Contains(string(body), "secret-bucket") || strings.HasPrefix(string(body), "{") {
		t.Fatalf("content rewritten: %s", body)
	}

	denied := newShipperDocumentHarness(t, []string{"SHIPPER_ADMIN"}, nil)
	rec = denied.do(t, http.MethodGet, path, nil)
	if rec.Code != http.StatusForbidden || denied.hits() != 0 {
		t.Fatalf("denied status=%d hits=%d", rec.Code, denied.hits())
	}
}

func TestShipperLegacyDocumentBypass(t *testing.T) {
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
		{http.MethodGet, "/api/v1/documents/" + id + "/attachments/" + att + "/signatures/sig-1"},
		{http.MethodGet, "/api/v1/signing-sessions/" + id},
		{http.MethodPost, "/api/v1/signing-sessions/" + id + "/signatures"},
	}
	for _, role := range []string{"SHIPPER_ADMIN", "SHIPPER_LOGIST"} {
		h := newShipperDocumentHarness(t, []string{role}, oneShipperMembership(shipperCompanyA, "SHIPPER", role))
		for _, route := range paths {
			before := h.hits()
			rec := h.do(t, route.method, route.path, nil)
			if rec.Code != http.StatusForbidden || h.hits() != before {
				t.Fatalf("%s %s %s status=%d hits=%d body=%s", role, route.method, route.path, rec.Code, h.hits(), readBody(t, rec))
			}
		}
	}

	h := newShipperDocumentHarness(t, []string{"PLATFORM_ADMIN"}, nil)
	h.tenantRole = []string{"PLATFORM_ADMIN"}
	for _, route := range paths {
		before := h.hits()
		rec := h.do(t, route.method, route.path, nil)
		if rec.Code != http.StatusOK || h.hits() != before+1 {
			t.Fatalf("operator %s %s status=%d hits=%d body=%s", route.method, route.path, rec.Code, h.hits(), readBody(t, rec))
		}
		h.document.mu.Lock()
		got := h.document.path
		h.document.mu.Unlock()
		if strings.Contains(got, "/v1/shipper/") {
			t.Fatalf("operator used shipper source %s", got)
		}
	}
}

func TestShipperShipmentRouteStaysOffDocumentService(t *testing.T) {
	h := newShipperDocumentHarness(t, []string{"DRIVER"}, oneShipperMembership(shipperCompanyA, "SHIPPER", "SHIPPER_ADMIN"))
	rec := h.do(t, http.MethodGet, "/api/v1/shipper/shipments?shipper_company_id="+shipperCompanyA, nil)
	if h.hits() != 0 {
		t.Fatal("shipment inbox reached document-service")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("shipment inbox status=%d body=%s", rec.Code, readBody(t, rec))
	}
}

func TestDriverPODUploadStaysInternal(t *testing.T) {
	var gotPath string
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/driver/me") {
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "driver-1", "companyId": shipperCompanyA})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(shipment.Close)
	documents := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uploadId":"u1"}`))
	}))
	t.Cleanup(documents.Close)

	cfg := testConfig()
	cfg.AuthEnabled = false
	cfg.DevTenantID = shipperReadTenantID
	cfg.MaxRequestBodyBytes = 1 << 20
	cfg.Services.Shipment = shipment.URL
	cfg.Services.Document = documents.URL
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, driver.NewHandler(testLogger(), cfg))
	body := bytes.NewBufferString(`{"mimeType":"application/pdf","fileName":"pod.pdf","idempotencyKey":"k1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/driver/me/shipments/"+shipperReadShipment+"/pod/uploads", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotPath != "/internal/v1/pod-uploads" {
		t.Fatalf("pod path=%s", gotPath)
	}
}
