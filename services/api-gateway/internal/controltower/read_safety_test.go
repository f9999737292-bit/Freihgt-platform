package controltower

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func TestSummaryReadsDoNotCreateWorkflowsRisksAutomationOrCases(t *testing.T) {
	plannedPickup := time.Now().UTC().Add(-3 * time.Hour)
	shipmentID := "11111111-1111-1111-1111-111111111111"
	var mu sync.Mutex
	var writeCalls []string
	var readModelTenants []string

	handler := summaryReadSafetyHandler(t, []map[string]any{pickupDelayShipment(shipmentID, tenantA, "carrier-a", plannedPickup)}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if path := r.URL.Path; summaryMutatingPath(path) {
			writeCalls = append(writeCalls, r.Method+" "+path)
		}
		if tenant := r.Header.Get("X-Tenant-ID"); tenant != "" {
			readModelTenants = append(readModelTenants, tenant)
		}
		mu.Unlock()
		writeSummaryReadModelStub(w, r, "")
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	spoofedTenant := "22222222-2222-2222-2222-222222222222"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary?tenant_id="+spoofedTenant, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Tenant-ID", spoofedTenant)
		rec := serveThroughAuth(t, handler, req, "secret", true)
		if rec.Code != http.StatusOK {
			t.Fatalf("repeat %d status=%d body=%s", i, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"status":"open"`) {
			t.Fatalf("repeat %d expected default open status, body=%s", i, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"shipmentRisks"`) {
			t.Fatalf("repeat %d expected stored shipment risks, body=%s", i, rec.Body.String())
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan string, 2)
	chain := gwmiddleware.Auth(true, "secret")(handler)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			chain.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				errCh <- rec.Body.String()
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Fatalf("concurrent summary failed: %s", msg)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(writeCalls) != 0 {
		t.Fatalf("summary issued mutating read-model calls: %v", writeCalls)
	}
	if len(readModelTenants) == 0 {
		t.Fatal("expected read-model calls to carry the verified tenant")
	}
	for _, tenant := range readModelTenants {
		if tenant != tenantA {
			t.Fatalf("read-model tenant=%q want %q", tenant, tenantA)
		}
	}
}

func TestSummaryOverlaysStoredWorkflowStatus(t *testing.T) {
	plannedPickup := time.Date(2026, 8, 10, 6, 0, 0, 0, time.UTC)
	shipmentID := "11111111-1111-1111-1111-111111111111"
	eventID := deterministicEventID(shipmentID, EventTypePickupDelay, canonicalEventAnchor(ControlTowerShipment{
		ID:              shipmentID,
		PlannedPickupAt: &plannedPickup,
	}, EventTypePickupDelay))

	var mu sync.Mutex
	var writeCalls []string
	handler := summaryReadSafetyHandler(t, []map[string]any{pickupDelayShipment(shipmentID, tenantA, "carrier-a", plannedPickup)}, func(w http.ResponseWriter, r *http.Request) {
		if summaryMutatingPath(r.URL.Path) {
			mu.Lock()
			writeCalls = append(writeCalls, r.Method+" "+r.URL.Path)
			mu.Unlock()
		}
		writeSummaryReadModelStub(w, r, eventID)
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body SummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	found := false
	for _, event := range body.CriticalEvents {
		if event.ID != eventID {
			continue
		}
		found = true
		if event.Status != WorkflowStatusAcknowledged {
			t.Fatalf("status=%q want acknowledged", event.Status)
		}
	}
	if !found {
		t.Fatalf("stored event %s missing from %s", eventID, rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(writeCalls) != 0 {
		t.Fatalf("summary issued mutating read-model calls: %v", writeCalls)
	}
}

func TestSummaryCompanyFilterStaysInsideVerifiedTenant(t *testing.T) {
	plannedPickup := time.Now().UTC().Add(-3 * time.Hour)
	shipperA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	shipperB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	carrierA := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	carrierB := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	shipmentA := "11111111-1111-1111-1111-111111111111"
	shipmentB := "22222222-2222-2222-2222-222222222222"
	var mu sync.Mutex
	var writes int
	var foreignTenants []string

	handler := summaryReadSafetyHandler(t, []map[string]any{
		pickupDelayShipment(shipmentA, shipperA, carrierA, plannedPickup),
		pickupDelayShipment(shipmentB, shipperB, carrierB, plannedPickup),
	}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if summaryMutatingPath(r.URL.Path) {
			writes++
		}
		if tenant := r.Header.Get("X-Tenant-ID"); tenant != tenantA {
			foreignTenants = append(foreignTenants, tenant)
		}
		mu.Unlock()
		writeSummaryReadModelStub(w, r, "")
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	shipperRec := serveThroughAuth(t, handler, authorizedSummaryRequest(token, "/api/v1/control-tower/summary?shipper_id="+shipperA), "secret", true)
	assertSingleShipment(t, shipperRec, shipmentA)
	carrierRec := serveThroughAuth(t, handler, authorizedSummaryRequest(token, "/api/v1/control-tower/summary?carrier_id="+carrierB), "secret", true)
	assertSingleShipment(t, carrierRec, shipmentB)
	mu.Lock()
	defer mu.Unlock()
	if writes != 0 {
		t.Fatalf("company filters issued %d mutating calls", writes)
	}
	if len(foreignTenants) != 0 {
		t.Fatalf("read-model tenants=%v", foreignTenants)
	}
}

func TestDirectCaseHandlersRejectDisabledReadModel(t *testing.T) {
	hits := 0
	readModel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer readModel.Close()

	handler := newCaseReadSafetyHandler(t, controltowerreadmodel.ModeDisabled, readModel.URL)
	spoofed := "22222222-2222-2222-2222-222222222222"
	for _, call := range directCaseCalls() {
		req := httptest.NewRequest(call.method, call.target, nil)
		req.Header.Set("X-Tenant-ID", spoofed)
		req = withChiParams(req, call.params)
		rec := httptest.NewRecorder()
		call.invoke(handler, rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d body=%s", call.name, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "CONTROL_TOWER_READ_MODEL_UNAVAILABLE") {
			t.Fatalf("%s body=%s", call.name, rec.Body.String())
		}
	}
	if hits != 0 {
		t.Fatalf("disabled case handlers called read model %d times", hits)
	}
}

func TestDirectCaseHandlersUseVerifiedTenantWhenReadModelEnabled(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var tenants []string
	readModel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		tenants = append(tenants, r.Header.Get("X-Tenant-ID"))
		mu.Unlock()
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer readModel.Close()

	handler := newCaseReadSafetyHandler(t, controltowerreadmodel.ModeShadow, readModel.URL)
	spoofed := "22222222-2222-2222-2222-222222222222"
	for _, call := range directCaseCalls() {
		req := httptest.NewRequest(call.method, call.target, nil)
		req.Header.Set("X-Tenant-ID", spoofed)
		req = withChiParams(req, call.params)
		rec := httptest.NewRecorder()
		call.invoke(handler, rec, req)
		if rec.Code != call.wantStatus {
			t.Fatalf("%s status=%d body=%s", call.name, rec.Code, rec.Body.String())
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 4 {
		t.Fatalf("calls=%v", calls)
	}
	for _, tenant := range tenants {
		if tenant != tenantA {
			t.Fatalf("tenant=%q want %q calls=%v", tenant, tenantA, calls)
		}
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"DELETE /internal/v1/control-tower/cases/case-1/links/link-1",
		"GET /internal/v1/control-tower/cases/case-1/timeline",
		"GET /internal/v1/control-tower/cases/duplicates",
		"DELETE /internal/v1/control-tower/cases/case-1/participants/user-1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
}

type directCaseCall struct {
	name       string
	method     string
	target     string
	params     map[string]string
	wantStatus int
	invoke     func(*Handler, http.ResponseWriter, *http.Request)
}

func directCaseCalls() []directCaseCall {
	return []directCaseCall{
		{
			name: "RemoveCaseLink", method: http.MethodDelete, target: "/cases/case-1/links/link-1",
			params: map[string]string{"caseId": "case-1", "linkId": "link-1"}, wantStatus: http.StatusNoContent,
			invoke: (*Handler).RemoveCaseLink,
		},
		{
			name: "GetCaseTimeline", method: http.MethodGet, target: "/cases/case-1/timeline",
			params: map[string]string{"caseId": "case-1"}, wantStatus: http.StatusOK,
			invoke: (*Handler).GetCaseTimeline,
		},
		{
			name: "FindCaseDuplicates", method: http.MethodGet, target: "/cases/duplicates",
			wantStatus: http.StatusOK,
			invoke:     (*Handler).FindCaseDuplicates,
		},
		{
			name: "RemoveCaseParticipant", method: http.MethodDelete, target: "/cases/case-1/participants/user-1",
			params: map[string]string{"caseId": "case-1", "userId": "user-1"}, wantStatus: http.StatusNoContent,
			invoke: (*Handler).RemoveCaseParticipant,
		},
	}
}

func summaryReadSafetyHandler(t *testing.T, shipments []map[string]any, readModelFn func(http.ResponseWriter, *http.Request)) http.Handler {
	t.Helper()
	shipmentServer := shipmentSummaryServer(shipments, len(shipments))
	t.Cleanup(shipmentServer.Close)
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	t.Cleanup(identityServer.Close)
	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityServer.URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeShadow,
		readModelFn:   readModelFn,
	})
	return handler
}

func newCaseReadSafetyHandler(t *testing.T, mode controltowerreadmodel.Mode, readModelURL string) *Handler {
	t.Helper()
	return NewHandler(slog.New(slog.DiscardHandler), config.Config{
		AuthEnabled:         false,
		DevTenantID:         tenantA,
		ProxyTimeoutSeconds: 5,
		ControlTower: config.ControlTowerConfig{
			ReadModel: controltowerreadmodel.Config{
				BaseURL:          readModelURL,
				Mode:             mode,
				Timeout:          time.Second,
				MaxResponseBytes: 256 * 1024,
			},
		},
	})
}

func pickupDelayShipment(id, shipperID, carrierID string, plannedPickup time.Time) map[string]any {
	return map[string]any{
		"id":                 id,
		"shipment_number":    "SH-" + id[:8],
		"shipper_company_id": shipperID,
		"carrier_company_id": carrierID,
		"status":             "PICKUP_SLOT_BOOKED",
		"planned_pickup_at":  plannedPickup.Format(time.RFC3339),
		"updated_at":         plannedPickup.Add(time.Hour).Format(time.RFC3339),
	}
}

func writeSummaryReadModelStub(w http.ResponseWriter, r *http.Request, acknowledgedEventID string) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/status-summary"):
		_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"PICKUP_SLOT_BOOKED":1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
	case strings.HasSuffix(r.URL.Path, "/workflows/lookup"):
		if acknowledgedEventID == "" {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		payload, _ := json.Marshal(map[string]any{
			"items": []map[string]string{{
				"eventId": acknowledgedEventID,
				"status":  "acknowledged",
			}},
		})
		_, _ = w.Write(payload)
	case strings.HasSuffix(r.URL.Path, "/workflows/list-open"):
		_, _ = w.Write([]byte(`{"items":[]}`))
	case strings.HasSuffix(r.URL.Path, "/risks/kpi"):
		_, _ = w.Write([]byte(`{"active":1}`))
	case strings.HasSuffix(r.URL.Path, "/risks"):
		_, _ = io.WriteString(w, `{"items":[{"riskId":"risk-1","shipmentId":"11111111-1111-1111-1111-111111111111","predictedExceptionType":"PICKUP_DELAY","score":10,"level":"high","status":"active","firstDetectedAt":"2026-08-10T06:00:00Z","evaluatedAt":"2026-08-10T07:00:00Z","signals":[]}]}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func summaryMutatingPath(path string) bool {
	switch {
	case strings.HasSuffix(path, "/workflows/ensure"):
		return true
	case strings.HasSuffix(path, "/risks/sync"):
		return true
	case strings.Contains(path, "/automation/evaluate"):
		return true
	case strings.Contains(path, "/cases"):
		return true
	default:
		return false
	}
}

func authorizedSummaryRequest(token, target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func assertSingleShipment(t *testing.T, rec *httptest.ResponseRecorder, shipmentID string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body SummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if body.Shipments.Total != 1 || len(body.Shipments.Items) != 1 || body.Shipments.Items[0].ID != shipmentID {
		t.Fatalf("shipments=%+v want only %s", body.Shipments, shipmentID)
	}
	if len(body.CriticalEvents) == 0 {
		t.Fatal("expected critical events for the filtered shipment")
	}
	for _, event := range body.CriticalEvents {
		if event.ShipmentID != shipmentID {
			t.Fatalf("event shipment=%s want %s", event.ShipmentID, shipmentID)
		}
	}
}

func withChiParams(req *http.Request, params map[string]string) *http.Request {
	routeCtx := chi.NewRouteContext()
	for key, value := range params {
		routeCtx.URLParams.Add(key, value)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	return req.WithContext(ctx)
}
