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
	testToken  = "internal-token"
	testTenant = "11111111-1111-1111-1111-111111111111"
)

type stubSource struct {
	snap  kpi.Snapshot
	err   error
	ready error
}

func (s stubSource) Fetch(context.Context, string) (kpi.Snapshot, error) {
	return s.snap, s.err
}

func (s stubSource) Ready(context.Context) error { return s.ready }

func testRouter(t *testing.T, src FoundationSource) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(log, config.Config{
		Environment:   "test",
		ShipmentURL:   "http://shipment.test",
		InternalToken: testToken,
		SourceTimeout: time.Second,
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
		{"no caller", testToken, "", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusForbidden},
		{"wrong caller", testToken, "shipment-service", testTenant, "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusForbidden},
		{"missing tenant", testToken, "api-gateway", "", "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusUnauthorized},
		{"invalid tenant", testToken, "api-gateway", "not-a-uuid", "", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusBadRequest},
		{"filter rejected", testToken, "api-gateway", testTenant, "?carrier=x", "/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", http.StatusBadRequest},
		{"unknown kpi", testToken, "api-gateway", testTenant, "", "/v1/analytics/kpis/OPS_OTIF", http.StatusNotFound},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := request(t, router, http.MethodGet, tt.path+tt.query, tt.token, tt.caller, tt.tenant, "")
			if rec.Code != tt.want {
				t.Fatalf("status=%d want %d body=%s", rec.Code, tt.want, rec.Body.String())
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
		"onTimeDeliveryDenominator",
		"onTimeDeliveryNumerator",
		"returnCaseCount",
		"redirectCaseCount",
	}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			payload := map[string]any{
				"tenantId":                  testTenant,
				"shipmentTotal":             0,
				"onTimeDeliveryDenominator": 0,
				"onTimeDeliveryNumerator":   0,
				"returnCaseCount":           0,
				"redirectCaseCount":         0,
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
				Environment:   "test",
				ShipmentURL:   server.URL,
				InternalToken: testToken,
				SourceTimeout: time.Second,
			}, source.NewClient(config.Config{ShipmentURL: server.URL, InternalToken: testToken, SourceTimeout: time.Second}))
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
		{name: "wrong tenant", status: http.StatusOK, body: `{"tenantId":"22222222-2222-2222-2222-222222222222","shipmentTotal":1,"onTimeDeliveryDenominator":0,"onTimeDeliveryNumerator":0,"returnCaseCount":0,"redirectCaseCount":0}`},
		{name: "inconsistent", status: http.StatusOK, body: `{"tenantId":"` + testTenant + `","shipmentTotal":1,"onTimeDeliveryDenominator":2,"onTimeDeliveryNumerator":1,"returnCaseCount":0,"redirectCaseCount":0}`},
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
				Environment:   "test",
				ShipmentURL:   server.URL,
				InternalToken: testToken,
				SourceTimeout: 40 * time.Millisecond,
			}, source.NewClient(config.Config{ShipmentURL: server.URL, InternalToken: testToken, SourceTimeout: 40 * time.Millisecond}))
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
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
