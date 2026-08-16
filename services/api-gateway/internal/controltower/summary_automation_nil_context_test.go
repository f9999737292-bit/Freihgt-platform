package controltower

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
)

func TestFireAutomationTriggerNilContextDoesNotPanic(t *testing.T) {
	var evaluateCalled bool
	readModelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/automation/evaluate") {
			evaluateCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer readModelServer.Close()

	svc := NewService(config.Config{
		ControlTower: config.ControlTowerConfig{
			ReadModel: controltowerreadmodel.Config{
				BaseURL: readModelServer.URL,
				Mode:    controltowerreadmodel.ModeShadow,
				Timeout: time.Second,
			},
		},
	}, nil, slog.New(slog.DiscardHandler))

	svc.fireAutomationTrigger(nil, RequestContext{
		TenantID:  tenantA,
		UserID:    "user-a",
		RequestID: "req-nil-ctx",
	}, []byte(`{"triggerType":"exception_created","triggerId":"x","persist":true}`))

	if !evaluateCalled {
		t.Fatal("expected automation evaluate to be called with nil parent context")
	}
}

func TestSummarySuccessWithReadModelAutomationTriggers(t *testing.T) {
	plannedPickup := time.Now().UTC().Add(-3 * time.Hour)
	shipmentID := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{{
		"id":                 shipmentID,
		"shipment_number":    "SH-1",
		"shipper_company_id": tenantA,
		"status":             "PICKUP_SLOT_BOOKED",
		"planned_pickup_at":  plannedPickup.Format(time.RFC3339),
		"updated_at":         time.Now().UTC().Format(time.RFC3339),
	}}, 1)
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityServer.URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeShadow,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/status-summary"):
				_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"PICKUP_SLOT_BOOKED":1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
			case strings.HasSuffix(r.URL.Path, "/workflows/ensure"):
				_, _ = w.Write([]byte(`{"createdEventIds":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`))
			case strings.HasSuffix(r.URL.Path, "/workflows/lookup"):
				_, _ = w.Write([]byte(`{"items":[]}`))
			case strings.HasSuffix(r.URL.Path, "/workflows/list-open"):
				_, _ = w.Write([]byte(`{"items":[]}`))
			case strings.HasSuffix(r.URL.Path, "/automation/evaluate"):
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"items":[]}`))
			case strings.HasSuffix(r.URL.Path, "/risks/sync"):
				_, _ = w.Write([]byte(`{"items":[]}`))
			case strings.HasSuffix(r.URL.Path, "/risks"):
				_, _ = w.Write([]byte(`{"items":[],"total":0}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		},
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = req.WithContext(context.Background())

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestMergeDriverCriticalEventsAppendsOpenDriverWorkflows(t *testing.T) {
	readModelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/workflows/list-open") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"eventId":           "3e51404909f54e0db0328dd935980fc5",
				"shipmentId":        "22222222-2222-2222-2222-222222222222",
				"eventType":         "vehicle_breakdown",
				"source":            "driver",
				"occurredAt":        time.Now().UTC().Format(time.RFC3339),
				"status":            "open",
				"priority":          "p1",
				"exceptionCategory": "vehicle_breakdown",
				"businessImpact":    "none",
			}},
		})
	}))
	defer readModelServer.Close()

	svc := NewService(config.Config{
		ControlTower: config.ControlTowerConfig{
			ReadModel: controltowerreadmodel.Config{
				BaseURL: readModelServer.URL,
				Mode:    controltowerreadmodel.ModePrimary,
				Timeout: time.Second,
			},
		},
	}, nil, slog.New(slog.DiscardHandler))

	events := []ControlTowerEvent{{
		ID:         "existingeventexistingeventexistingev",
		ShipmentID: "22222222-2222-2222-2222-222222222222",
		Type:       EventTypePickupDelay,
		Source:     EventSourceControlTower,
	}}
	svc.mergeDriverCriticalEvents(context.Background(), RequestContext{
		TenantID:  tenantA,
		RequestID: "req-driver-merge",
	}, &events, map[string]ControlTowerShipment{
		"22222222-2222-2222-2222-222222222222": {ID: "22222222-2222-2222-2222-222222222222", ShipmentNumber: "SH-DRIVER"},
	})

	if len(events) != 2 {
		t.Fatalf("events=%d want 2", len(events))
	}
	if events[1].ID != "3e51404909f54e0db0328dd935980fc5" {
		t.Fatalf("driver event id=%q", events[1].ID)
	}
	if events[1].Source != EventSourceDriver {
		t.Fatalf("source=%q want driver", events[1].Source)
	}
	if events[1].ShipmentNumber != "SH-DRIVER" {
		t.Fatalf("shipment number=%q", events[1].ShipmentNumber)
	}
}

func TestSummaryTenantIsolationUsesVerifiedTenantForDriverWorkflowList(t *testing.T) {
	var capturedTenant string
	readModelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/workflows/list-open") {
			capturedTenant = r.Header.Get("X-Tenant-ID")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/status-summary"):
			_, _ = w.Write([]byte(`{"totalShipments":0,"byStatus":{},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
		case strings.HasSuffix(r.URL.Path, "/workflows/list-open"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		case strings.HasSuffix(r.URL.Path, "/risks/sync"), strings.HasSuffix(r.URL.Path, "/risks"):
			_, _ = w.Write([]byte(`{"items":[],"total":0}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"items":[]}`))
		}
	}))
	defer readModelServer.Close()

	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
	defer identityServer.Close()

	handler := newTestSummaryHandlerWithReadModel(t, testHandlerConfig{
		shipmentURL: shipmentServer.URL,
		identityURL: identityServer.URL,
		authEnabled: true,
	}, readModelServer, controltowerreadmodel.ModePrimary)

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(gwmiddleware.RequestIDHeader, "req-tenant-isolation")

	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if capturedTenant != tenantA {
		t.Fatalf("driver workflow tenant=%q want %q", capturedTenant, tenantA)
	}
}
