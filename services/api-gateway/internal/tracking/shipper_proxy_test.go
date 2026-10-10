package tracking

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func TestShipperProxyUsesSafeSource(t *testing.T) {
	var gotPath, gotQuery, gotTenant, gotCompany, gotActor string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("shipper_company_id")
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotCompany = r.Header.Get("X-Company-ID")
		gotActor = r.Header.Get("X-Actor-Kind")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"shipmentId":"s1","trackingStatus":"ok"}`))
	}))
	t.Cleanup(upstream.Close)

	cfg := config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services:            config.ServiceURLs{Tracking: upstream.URL},
	}
	handler := NewHandler(nil, cfg)
	r := chi.NewRouter()
	r.Get("/api/v1/shipper/shipments/{id}/tracking", handler.GetShipperTracking)
	r.Get("/api/v1/shipper/shipments/{id}/eta", handler.GetShipperETA)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipper/shipments/s1/tracking?shipper_company_id=aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Company-ID", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	req.Header.Set("X-Actor-Kind", "BUYER")
	req = req.WithContext(gwmiddleware.WithAuthContext(req.Context(), gwmiddleware.AuthContext{
		TenantID: "11111111-1111-1111-1111-111111111111",
		UserID:   "22222222-2222-2222-2222-222222222222",
	}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotPath != "/v1/shipper/shipments/s1/tracking" || gotQuery != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("path=%s query=%s", gotPath, gotQuery)
	}
	if gotTenant != "11111111-1111-1111-1111-111111111111" || gotCompany != gotQuery || gotActor != "BUYER" {
		t.Fatalf("tenant=%s company=%s actor=%s", gotTenant, gotCompany, gotActor)
	}

	tamper := httptest.NewRequest(http.MethodGet, "/api/v1/shipper/shipments/s1/eta?plannedPickupAt=2026-01-01T00:00:00Z", nil)
	tamper = tamper.WithContext(gwmiddleware.WithAuthContext(tamper.Context(), gwmiddleware.AuthContext{
		TenantID: "11111111-1111-1111-1111-111111111111",
		UserID:   "22222222-2222-2222-2222-222222222222",
	}))
	tamperRec := httptest.NewRecorder()
	r.ServeHTTP(tamperRec, tamper)
	if tamperRec.Code != http.StatusBadRequest {
		t.Fatalf("tamper status=%d", tamperRec.Code)
	}
	if gotPath != "/v1/shipper/shipments/s1/tracking" {
		t.Fatalf("tamper called upstream path=%s", gotPath)
	}
}
