package controltower

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func TestAcknowledgeCriticalEventInvalidEventIDReturns400(t *testing.T) {
	handler := newTestAcknowledgeHandler(t, testAcknowledgeConfig{})
	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/control-tower/critical-events/not-valid/acknowledge", nil)
	req = withEventID(req, "not-valid")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestAcknowledgeCriticalEventRejectsNonEmptyBody(t *testing.T) {
	handler := newTestAcknowledgeHandler(t, testAcknowledgeConfig{})
	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/control-tower/critical-events/abc/acknowledge", strings.NewReader(`{"tenant_id":"x"}`))
	req = withEventID(req, "abc")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestAcknowledgeCriticalEventUnknownEventReturns404(t *testing.T) {
	plannedPickup := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"id":                 "11111111-1111-1111-1111-111111111111",
				"shipment_number":    "SH-1",
				"shipper_company_id": tenantA,
				"status":             "PICKUP_SLOT_BOOKED",
				"planned_pickup_at":  plannedPickup.Add(-2 * time.Hour).Format(time.RFC3339),
				"updated_at":         plannedPickup.Format(time.RFC3339),
			}},
			"total": 1,
		})
	}))
	defer shipmentServer.Close()

	readModelServer := httptest.NewServer(http.NotFoundHandler())
	defer readModelServer.Close()

	handler := newTestAcknowledgeHandler(t, testAcknowledgeConfig{
		shipmentURL:   shipmentServer.URL,
		readModelURL:  readModelServer.URL,
		readModelMode: controltowerreadmodel.ModePrimary,
	})
	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/control-tower/critical-events/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/acknowledge", nil)
	req = withEventID(req, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestValidateAcknowledgeRequestBodyAllowsEmptyObject(t *testing.T) {
	if err := validateAcknowledgeRequestBody([]byte(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAcknowledgeRequestBodyRejectsProperties(t *testing.T) {
	if err := validateAcknowledgeRequestBody([]byte(`{"foo":"bar"}`)); err == nil {
		t.Fatal("expected validation error")
	}
}

type testAcknowledgeConfig struct {
	shipmentURL   string
	identityURL   string
	readModelURL  string
	readModelMode controltowerreadmodel.Mode
}

func newTestAcknowledgeHandler(t *testing.T, cfg testAcknowledgeConfig) http.Handler {
	t.Helper()

	if cfg.shipmentURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.shipmentURL = server.URL
	}
	if cfg.identityURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
		}))
		t.Cleanup(server.Close)
		cfg.identityURL = server.URL
	}
	if cfg.readModelURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"eventId":        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"shipmentId":     "11111111-1111-1111-1111-111111111111",
				"eventType":      "PICKUP_DELAY",
				"occurredAt":     time.Now().UTC().Format(time.RFC3339),
				"source":         "control-tower",
				"acknowledgedAt": time.Now().UTC().Format(time.RFC3339),
				"acknowledgedBy": map[string]string{"userId": "user-a"},
			})
		}))
		t.Cleanup(server.Close)
		cfg.readModelURL = server.URL
	}
	if cfg.readModelMode == "" {
		cfg.readModelMode = controltowerreadmodel.ModePrimary
	}

	h := NewHandler(slog.New(slog.DiscardHandler), config.Config{
		AuthEnabled:         true,
		ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{
			Identity: cfg.identityURL,
			Company:  "http://127.0.0.1:1",
			Shipment: cfg.shipmentURL,
			Document: "http://127.0.0.1:1",
		},
		ControlTower: config.ControlTowerConfig{
			MaxDownstreamFetchLimit: 200,
			ReadModel: controltowerreadmodel.Config{
				BaseURL: cfg.readModelURL,
				Mode:    cfg.readModelMode,
				Timeout: 2 * time.Second,
			},
		},
	})
	return http.HandlerFunc(h.AcknowledgeCriticalEvent)
}

func withEventID(req *http.Request, eventID string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("eventId", eventID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

const tenantA = "11111111-1111-1111-1111-111111111111"

func TestAcknowledgeHandlerUsesVerifiedTenantInReadModelCall(t *testing.T) {
	plannedPickupAt := time.Date(2026, 8, 10, 6, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	shipmentID := "11111111-1111-1111-1111-111111111111"
	eventID := deterministicEventID(shipmentID, EventTypePickupDelay, canonicalEventAnchor(ControlTowerShipment{
		ID:              shipmentID,
		PlannedPickupAt: &plannedPickupAt,
	}, EventTypePickupDelay))

	var capturedTenant string
	readModelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = r.Header.Get("X-Tenant-ID")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"eventId":        eventID,
			"shipmentId":     shipmentID,
			"eventType":      EventTypePickupDelay,
			"occurredAt":     plannedPickupAt.UTC().Format(time.RFC3339),
			"source":         EventSourceControlTower,
			"acknowledgedAt": time.Now().UTC().Format(time.RFC3339),
			"acknowledgedBy": map[string]string{"userId": "user-a"},
		})
	}))
	defer readModelServer.Close()

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"id":                 shipmentID,
				"shipment_number":    "SH-1",
				"shipper_company_id": tenantA,
				"status":             "PICKUP_SLOT_BOOKED",
				"planned_pickup_at":  plannedPickupAt.Format(time.RFC3339),
				"updated_at":         updatedAt.Format(time.RFC3339),
			}},
			"total": 1,
		})
	}))
	defer shipmentServer.Close()

	handler := newTestAcknowledgeHandler(t, testAcknowledgeConfig{
		shipmentURL:   shipmentServer.URL,
		readModelURL:  readModelServer.URL,
		readModelMode: controltowerreadmodel.ModePrimary,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/control-tower/critical-events/"+eventID+"/acknowledge", bytes.NewReader([]byte("{}")))
	req = withEventID(req, eventID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "22222222-2222-2222-2222-222222222222")

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	if capturedTenant != tenantA {
		t.Fatalf("read model tenant=%q want %q", capturedTenant, tenantA)
	}

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), eventID) {
		t.Fatalf("expected event id in response, body=%s", string(body))
	}
}

func TestAcknowledgeHandlerMissingAuthReturns401(t *testing.T) {
	handler := newTestAcknowledgeHandler(t, testAcknowledgeConfig{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/control-tower/critical-events/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/acknowledge", nil)
	rec := httptest.NewRecorder()
	gwmiddleware.Auth(true, "secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}
