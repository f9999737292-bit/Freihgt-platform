package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/analytics-service/internal/config"
	"github.com/freight-platform/analytics-service/internal/kpi"
	"github.com/freight-platform/analytics-service/internal/source"
)

const (
	testToken   = "gateway-caller-token"
	sourceToken = "source-caller-token"
	sharedToken = "shared-internal-token"
	testTenant  = "11111111-1111-1111-1111-111111111111"
)

type stubSource struct {
	snap  kpi.Snapshot
	err   error
	ready error
}

func (s stubSource) Fetch(context.Context, string, kpi.ReadScope) (kpi.Snapshot, error) {
	if s.snap.DispositionScope == "" {
		s.snap.DispositionScope = kpi.DispositionScopeTenant
	}
	return s.snap, s.err
}

func (s stubSource) Ready(context.Context) error { return s.ready }

func testRouter(t *testing.T, src FoundationSource) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(log, config.Config{
		Environment:        "test",
		ShipmentURL:        "http://shipment.test",
		GatewayCallerToken: testToken,
		SourceCallerToken:  sourceToken,
		SourceTimeout:      time.Second,
	}, src)
}

func TestIngressAndKPIRoute(t *testing.T) {
	router := testRouter(t, stubSource{snap: kpi.Snapshot{ShipmentTotal: 4, OnTimeDeliveryDenominator: 3, OnTimeDeliveryNumerator: 1}})
	ok := request(t, router, http.MethodGet, "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", testToken, "api-gateway", testTenant, "")
	if ok.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", ok.Code, ok.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(ok.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["definitionVersion"].(float64) != 1 {
		t.Fatalf("%v", payload)
	}
	freshness := payload["dataFreshness"].(map[string]any)
	if freshness["status"] != "UNKNOWN" || freshness["sourceObservedAt"] != nil {
		t.Fatalf("freshness=%v", freshness)
	}

	cases := []struct {
		name   string
		token  string
		caller string
		tenant string
		query  string
		path   string
		want   int
	}{
		{"no token", "", "api-gateway", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusUnauthorized},
		{"wrong token", "other", "api-gateway", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusUnauthorized},
		{"shared token with gateway name", sharedToken, "api-gateway", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusUnauthorized},
		{"source token with gateway name", sourceToken, "api-gateway", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusUnauthorized},
		{"no caller", testToken, "", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusForbidden},
		{"wrong caller", testToken, "shipment-service", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusForbidden},
		{"missing tenant", testToken, "api-gateway", "", "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusUnauthorized},
		{"invalid tenant", testToken, "api-gateway", "not-a-uuid", "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusBadRequest},
		{"filter rejected", testToken, "api-gateway", testTenant, "?carrier=x", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusBadRequest},
		{"query tenant spoof", testToken, "api-gateway", testTenant, "?tenant_id=22222222-2222-2222-2222-222222222222", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusBadRequest},
		{"unknown kpi", testToken, "api-gateway", testTenant, "", "/v1/analytics/kpis/OPS_OTIF", http.StatusNotFound},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := request(t, router, http.MethodGet, tt.path+tt.query, tt.token, tt.caller, tt.tenant, "")
			if rec.Code != tt.want {
				t.Fatalf("status=%d want %d body=%s", rec.Code, tt.want, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), testToken) || strings.Contains(rec.Body.String(), sourceToken) || strings.Contains(rec.Body.String(), sharedToken) {
				t.Fatalf("response disclosed a credential: %s", rec.Body.String())
			}
		})
	}
}

func TestSourceFailuresFailClosed(t *testing.T) {
	router := testRouter(t, stubSource{err: &source.Error{Reason: "http_5xx"}})
	rec := request(t, router, http.MethodGet, "/v1/analytics/kpis/OPS_RETURN_CASES", testToken, "api-gateway", testTenant, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "4") && strings.Contains(rec.Body.String(), `"value"`) {
		t.Fatalf("returned a number: %s", rec.Body.String())
	}
}

func TestOmittedSourceFieldReturnsNoKPI(t *testing.T) {
	fields := []string{
		"tenantId",
		"shipmentTotal",
		"onTimePickupDenominator",
		"onTimePickupNumerator",
		"onTimeDeliveryDenominator",
		"onTimeDeliveryNumerator",
		"returnCaseCount",
		"redirectCaseCount",
		"carriers",
		"dispositionScope",
	}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			payload := map[string]any{
				"tenantId":                  testTenant,
				"shipmentTotal":             0,
				"onTimePickupDenominator":   0,
				"onTimePickupNumerator":     0,
				"onTimeDeliveryDenominator": 0,
				"onTimeDeliveryNumerator":   0,
				"returnCaseCount":           0,
				"redirectCaseCount":         0,
				"carriers":                  []any{},
				"dispositionScope":          "TENANT",
			}
			delete(payload, field)
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(raw)
			}))
			defer server.Close()
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			router := NewRouter(log, config.Config{
				Environment:        "test",
				ShipmentURL:        server.URL,
				GatewayCallerToken: testToken,
				SourceCallerToken:  sourceToken,
				SourceTimeout:      time.Second,
			}, source.NewClient(config.Config{ShipmentURL: server.URL, SourceCallerToken: sourceToken, SourceTimeout: time.Second}))
			rec := request(t, router, http.MethodGet, "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", testToken, "api-gateway", testTenant, "")
			if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), `"kpiId"`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandlerMapsLiveSourceFailures(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		delay  time.Duration
	}{
		{name: "source 500", status: http.StatusInternalServerError, body: `{"error":"boom"}`},
		{name: "malformed", status: http.StatusOK, body: `{`},
		{name: "wrong tenant", status: http.StatusOK, body: `{"tenantId":"22222222-2222-2222-2222-222222222222","shipmentTotal":1,"onTimePickupDenominator":0,"onTimePickupNumerator":0,"onTimeDeliveryDenominator":0,"onTimeDeliveryNumerator":0,"returnCaseCount":0,"redirectCaseCount":0,"carriers":[],"dispositionScope":"TENANT"}`},
		{name: "inconsistent", status: http.StatusOK, body: `{"tenantId":"` + testTenant + `","shipmentTotal":1,"onTimePickupDenominator":0,"onTimePickupNumerator":0,"onTimeDeliveryDenominator":2,"onTimeDeliveryNumerator":1,"returnCaseCount":0,"redirectCaseCount":0,"carriers":[],"dispositionScope":"TENANT"}`},
		{name: "timeout", status: http.StatusOK, body: `{"tenantId":"` + testTenant + `"}`, delay: time.Second},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.delay > 0 {
					time.Sleep(tt.delay)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			router := NewRouter(log, config.Config{
				Environment:        "test",
				ShipmentURL:        server.URL,
				GatewayCallerToken: testToken,
				SourceCallerToken:  sourceToken,
				SourceTimeout:      40 * time.Millisecond,
			}, source.NewClient(config.Config{ShipmentURL: server.URL, SourceCallerToken: sourceToken, SourceTimeout: 40 * time.Millisecond}))
			rec := request(t, router, http.MethodGet, "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", testToken, "api-gateway", testTenant, "")
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"kpiId"`) {
				t.Fatalf("returned a kpi: %s", rec.Body.String())
			}
		})
	}
}

func TestReadyUsesTechnicalCheck(t *testing.T) {
	router := testRouter(t, stubSource{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready=%d", rec.Code)
	}
	down := testRouter(t, stubSource{ready: &source.Error{Reason: "unreachable"}})
	rec = httptest.NewRecorder()
	down.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("down ready=%d", rec.Code)
	}
	health := httptest.NewRecorder()
	down.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health=%d", health.Code)
	}
}

func TestCompanyScopeDeniesUnverifiedReturnAndRedirect(t *testing.T) {
	company := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	other := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	var got kpi.ReadScope
	router := testRouter(t, recordingSource{snap: kpi.Snapshot{
		ShipmentTotal:    2,
		DispositionScope: kpi.DispositionScopeUnavailable,
		Carriers: []kpi.CarrierSnapshot{{
			CarrierCompanyID:          company,
			OnTimePickupDenominator:   1,
			OnTimePickupNumerator:     1,
			OnTimeDeliveryDenominator: 1,
			OnTimeDeliveryNumerator:   1,
		}},
	}, got: &got})

	denied := companyRequest(t, router, "/v1/analytics/kpis/OPS_RETURN_CASES", kpi.ScopeCarrier, company, other)
	if denied.Code != http.StatusForbidden || strings.Contains(denied.Body.String(), `"value"`) || strings.Contains(denied.Body.String(), `"shipmentTotal"`) {
		t.Fatalf("status=%d body=%s", denied.Code, denied.Body.String())
	}
	redirect := companyRequest(t, router, "/v1/analytics/kpis/OPS_REDIRECT_CASES", kpi.ScopeShipper, company, other)
	if redirect.Code != http.StatusForbidden {
		t.Fatalf("redirect status=%d body=%s", redirect.Code, redirect.Body.String())
	}
	total := companyRequest(t, router, "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", kpi.ScopeCarrier, company, other)
	if total.Code != http.StatusOK || got.Kind != kpi.ScopeCarrier || got.CompanyID != company {
		t.Fatalf("status=%d scope=%+v body=%s", total.Code, got, total.Body.String())
	}
	if strings.Contains(total.Body.String(), other) {
		t.Fatalf("response used the client company header: %s", total.Body.String())
	}

	missing := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil)
	req.Header.Set("X-Internal-Service-Token", testToken)
	req.Header.Set("X-Internal-Service-Name", "api-gateway")
	req.Header.Set("X-Tenant-ID", testTenant)
	req.Header.Set("X-Company-ID", company)
	router.ServeHTTP(missing, req)
	if missing.Code != http.StatusForbidden {
		t.Fatalf("missing scope status=%d body=%s", missing.Code, missing.Body.String())
	}

	tenantCase := companyRequest(t, router, "/v1/analytics/kpis/OPS_RETURN_CASES", kpi.ScopeTenant, "", "")
	if tenantCase.Code != http.StatusServiceUnavailable || strings.Contains(tenantCase.Body.String(), `"value"`) || strings.Contains(tenantCase.Body.String(), `"kpiId"`) {
		t.Fatalf("tenant request against unavailable disposition status=%d body=%s", tenantCase.Code, tenantCase.Body.String())
	}
}

func TestCompanyScopeWithTenantDispositionFailsClosed(t *testing.T) {
	company := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	router := testRouter(t, stubSource{snap: kpi.Snapshot{
		ShipmentTotal:    2,
		ReturnCaseCount:  1,
		DispositionScope: kpi.DispositionScopeTenant,
	}})
	rec := companyRequest(t, router, "/v1/analytics/kpis/OPS_RETURN_CASES", kpi.ScopeCarrier, company, "")
	if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), `"value"`) || strings.Contains(rec.Body.String(), `"kpiId"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type recordingSource struct {
	snap kpi.Snapshot
	got  *kpi.ReadScope
}

func (s recordingSource) Fetch(_ context.Context, _ string, scope kpi.ReadScope) (kpi.Snapshot, error) {
	if s.got != nil {
		*s.got = scope
	}
	return s.snap, nil
}

func (recordingSource) Ready(context.Context) error { return nil }

func companyRequest(t *testing.T, handler http.Handler, path, scope, company, clientCompany string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Internal-Service-Token", testToken)
	req.Header.Set("X-Internal-Service-Name", "api-gateway")
	req.Header.Set("X-Tenant-ID", testTenant)
	req.Header.Set("X-Analytics-Scope", scope)
	if company != "" {
		req.Header.Set("X-Analytics-Company-ID", company)
	}
	if clientCompany != "" {
		req.Header.Set("X-Company-ID", clientCompany)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func request(t *testing.T, handler http.Handler, method, path, token, caller, tenant, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	if caller != "" {
		req.Header.Set("X-Internal-Service-Name", caller)
	}
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	req.Header.Set("X-Analytics-Scope", kpi.ScopeTenant)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
