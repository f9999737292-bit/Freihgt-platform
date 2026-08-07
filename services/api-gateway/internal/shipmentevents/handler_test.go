package shipmentevents

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/platform/sla"
)

func TestEventsForeignTenantReturns404(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tenant_id"); got != "" {
			t.Fatalf("downstream must not receive tenant_id query, got %q", got)
		}
		if got := r.Header.Get("X-Tenant-ID"); got != tenantA {
			t.Fatalf("downstream X-Tenant-ID=%q want %q", got, tenantA)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer shipmentServer.Close()

	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		authEnabled: true,
	})

	token := signEventsTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveEventsThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEventsIgnoresSpoofedTenantHeader(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": shipmentID, "tenant_id": tenantA, "shipment_number": "SHP-1", "status": "IN_TRANSIT",
			"planned_pickup_at": "2026-08-01T10:00:00Z", "created_at": "2026-07-31T10:00:00Z", "updated_at": "2026-07-31T11:00:00Z",
		})
	}))
	defer shipmentServer.Close()

	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		authEnabled: true,
	})

	token := signEventsTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", tenantB)

	rec := serveEventsThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEventsMissingVerifiedTenantReturns401WithoutDownstream(t *testing.T) {
	downstreamCalled := false
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer shipmentServer.Close()

	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		authEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/events", nil)
	rec := serveEventsThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if downstreamCalled {
		t.Fatal("downstream should not be called without verified tenant")
	}
}

func TestEventsShipmentUnavailableReturns503(t *testing.T) {
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer shipmentServer.Close()

	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		devTenantID: "11111111-1111-1111-1111-111111111111",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SHIPMENT_EVENTS_SHIPMENT_UNAVAILABLE") {
		t.Fatalf("expected shipment unavailable code, body=%s", rec.Body.String())
	}
}

func TestEventsDocumentsUnavailableReturns200Partial(t *testing.T) {
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tenantID := "11111111-1111-1111-1111-111111111111"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": shipmentID, "tenant_id": tenantID, "shipment_number": "SHP-1", "status": "IN_TRANSIT",
			"planned_pickup_at": "2026-08-01T10:00:00Z", "created_at": "2026-07-31T10:00:00Z", "updated_at": "2026-07-31T11:00:00Z",
		})
	}))
	defer shipmentServer.Close()

	documentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer documentServer.Close()

	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		documentURL: documentServer.URL,
		devTenantID: tenantID,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload EventsResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.DataFreshness.Partial {
		t.Fatal("expected partial response")
	}
	found := false
	for _, warning := range payload.DataFreshness.Warnings {
		if warning == WarningDocumentEventsUnavailable {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected DOCUMENT_EVENTS_UNAVAILABLE warning, got %#v", payload.DataFreshness.Warnings)
	}
}

func TestEventsInvalidShipmentIDReturns400(t *testing.T) {
	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		devTenantID: "11111111-1111-1111-1111-111111111111",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/not-a-uuid/events", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEventsInvalidDateRangeReturns400(t *testing.T) {
	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		devTenantID: "11111111-1111-1111-1111-111111111111",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/events?date_from=2026-08-02&date_to=2026-08-01", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEventsPaginationValidation(t *testing.T) {
	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		devTenantID: "11111111-1111-1111-1111-111111111111",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/events?page=0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("page=0 status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/events?limit=201", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("limit=201 status=%d", rec.Code)
	}
}

func TestEventsRBACAllowAndDeny(t *testing.T) {
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tenantID := "11111111-1111-1111-1111-111111111111"

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": shipmentID, "tenant_id": tenantID, "shipment_number": "SHP-1", "status": "IN_TRANSIT",
			"planned_pickup_at": "2026-08-01T10:00:00Z", "created_at": "2026-07-31T10:00:00Z", "updated_at": "2026-07-31T11:00:00Z",
		})
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"UNKNOWN_ROLE"}})
	}))
	defer identityServer.Close()

	handler := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	})

	token := signEventsTestToken(t, "secret", "user-a", tenantID, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveEventsThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("deny status=%d body=%s", rec.Code, rec.Body.String())
	}

	identityAllow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityAllow.Close()

	handlerAllow := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityAllow.URL,
		authEnabled: true,
	})
	req = httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = serveEventsThroughAuth(t, handlerAllow, req, "secret", true)
	identityDeny := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
	}))
	defer identityDeny.Close()

	handlerDeny := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityDeny.URL,
		authEnabled: true,
	})
	req = httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = serveEventsThroughAuth(t, handlerDeny, req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("finance deny status=%d body=%s", rec.Code, rec.Body.String())
	}

	procurementIdentity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PROCUREMENT_MANAGER"}})
	}))
	defer procurementIdentity.Close()

	handlerProcurement := newTestEventsHandler(t, testEventsHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: procurementIdentity.URL,
		authEnabled: true,
	})
	req = httptest.NewRequest(http.MethodGet, "/api/v1/shipments/"+shipmentID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = serveEventsThroughAuth(t, handlerProcurement, req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("procurement deny status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBuildSLACriticalEvent(t *testing.T) {
	now := time.Date(2026, 7, 31, 18, 0, 0, 0, time.UTC)
	shipment := rawShipment{
		ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ShipmentNumber: "SHP-1", Status: "IN_TRANSIT",
		PlannedPickupAt: mustEventsTime("2026-07-29T08:00:00Z"), PlannedDeliveryAt: mustEventsTime("2026-07-29T12:00:00Z"),
		ActualPickupAt: mustEventsTime("2026-07-29T08:30:00Z"), UpdatedAt: mustEventsTime("2026-07-29T13:00:00Z"),
	}
	event := buildSLAEvent(shipment, defaultTestThresholds(), now)
	if event == nil || event.Type != EventTypeSLACritical {
		t.Fatalf("expected SLA critical event, got %#v", event)
	}
}

func TestBuildSLANoEventForOnTime(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	shipment := rawShipment{
		ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ShipmentNumber: "SHP-1", Status: "IN_TRANSIT",
		PlannedPickupAt: mustEventsTime("2026-07-31T14:00:00Z"), PlannedDeliveryAt: mustEventsTime("2026-08-01T10:00:00Z"),
		ActualPickupAt: mustEventsTime("2026-07-31T13:00:00Z"), UpdatedAt: mustEventsTime("2026-07-31T11:30:00Z"),
	}
	if event := buildSLAEvent(shipment, defaultTestThresholds(), now); event != nil {
		t.Fatalf("expected no SLA event for on-time shipment, got %#v", event)
	}
}

type testEventsHandlerConfig struct {
	shipmentURL string
	documentURL string
	billingURL  string
	identityURL string
	authEnabled bool
	devTenantID string
}

func newTestEventsHandler(t *testing.T, cfg testEventsHandlerConfig) http.Handler {
	t.Helper()
	if cfg.documentURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.documentURL = server.URL
	}
	if cfg.billingURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.billingURL = server.URL
	}
	if cfg.identityURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
		}))
		t.Cleanup(server.Close)
		cfg.identityURL = server.URL
	}
	if cfg.shipmentURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "tenant_id": cfg.devTenantID,
				"shipment_number": "SHP-1", "status": "IN_TRANSIT",
				"planned_pickup_at": "2026-08-01T10:00:00Z", "created_at": "2026-07-31T10:00:00Z", "updated_at": "2026-07-31T11:00:00Z",
			})
		}))
		t.Cleanup(server.Close)
		cfg.shipmentURL = server.URL
	}

	h := NewHandler(slog.New(slog.DiscardHandler), config.Config{
		AuthEnabled:         cfg.authEnabled,
		DevTenantID:         cfg.devTenantID,
		ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{
			Identity:        cfg.identityURL,
			Shipment:        cfg.shipmentURL,
			Document:        cfg.documentURL,
			BillingRegister: cfg.billingURL,
		},
		ControlTower: config.ControlTowerConfig{MaxDownstreamFetchLimit: 200},
	})
	r := chi.NewRouter()
	r.Get("/api/v1/shipments/{shipmentId}/events", h.Events)
	return r
}

func serveEventsThroughAuth(t *testing.T, handler http.Handler, req *http.Request, secret string, enabled bool) *httptest.ResponseRecorder {
	t.Helper()
	chain := gwmiddleware.Auth(enabled, secret)(handler)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	return rec
}

func signEventsTestToken(t *testing.T, secret, userID, tenantID, email string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"email":     email,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func mustEventsTime(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	utc := parsed.UTC()
	return &utc
}

func defaultTestThresholds() sla.Thresholds {
	return sla.Thresholds{
		AtRiskMinutes: 120, CriticalDelayMinutes: 240, StaleWarningMinutes: 120, StaleCriticalMinutes: 360,
	}
}

func readEventsBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}
