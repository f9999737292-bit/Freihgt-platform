package controltower

import (
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

type readModelHandlerConfig struct {
	testHandlerConfig
	readModelMode  controltowerreadmodel.Mode
	readModelFn    func(w http.ResponseWriter, r *http.Request)
	readModelDelay time.Duration
}

func newReadModelSummaryHandler(t *testing.T, cfg readModelHandlerConfig) (http.Handler, *httptest.Server) {
	t.Helper()

	var readModelServer *httptest.Server
	if cfg.readModelMode.Enabled() {
		readModelServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.readModelDelay > 0 {
				time.Sleep(cfg.readModelDelay)
			}
			if cfg.readModelFn != nil {
				cfg.readModelFn(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"IN_TRANSIT":1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
		}))
		t.Cleanup(readModelServer.Close)
	}

	base := cfg.testHandlerConfig
	handler := newTestSummaryHandlerWithReadModel(t, base, readModelServer, cfg.readModelMode)
	return handler, readModelServer
}

func newTestSummaryHandlerWithReadModel(
	t *testing.T,
	cfg testHandlerConfig,
	readModelServer *httptest.Server,
	mode controltowerreadmodel.Mode,
) http.Handler {
	t.Helper()
	if cfg.companyURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.companyURL = server.URL
	}
	if cfg.transportURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.transportURL = server.URL
	}
	if cfg.documentURL == "" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		}))
		t.Cleanup(server.Close)
		cfg.documentURL = server.URL
	}

	readModelCfg := controltowerreadmodel.Config{
		Mode:                   mode,
		Timeout:                800 * time.Millisecond,
		RequireConsumerRunning: true,
		MaxResponseBytes:       256 * 1024,
	}
	if readModelServer != nil {
		readModelCfg.BaseURL = readModelServer.URL
	}

	h := NewHandler(slog.New(slog.DiscardHandler), config.Config{
		AuthEnabled:         cfg.authEnabled,
		DevTenantID:         cfg.devTenantID,
		ProxyTimeoutSeconds: 5,
		Services: config.ServiceURLs{
			Identity:       cfg.identityURL,
			Company:        cfg.companyURL,
			TransportOrder: cfg.transportURL,
			Shipment:       cfg.shipmentURL,
			Document:       cfg.documentURL,
		},
		ControlTower: config.ControlTowerConfig{
			MaxDownstreamFetchLimit: 200,
			ReadModel:               readModelCfg,
		},
	})
	return http.HandlerFunc(h.Summary)
}

func shipmentSummaryServer(items []map[string]any, total int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total})
	}))
}

func identityAdminServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"PLATFORM_ADMIN"}})
	}))
}

func decodeSummary(t *testing.T, body string) SummaryResponse {
	t.Helper()
	var resp SummaryResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode summary: %v body=%s", err, body)
	}
	return resp
}

func TestSummaryReadModelDisabledHasNoStatusSummary(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{
		{"id": "s1", "shipment_number": "SH-1", "status": "IN_TRANSIT"},
	}, 1)
	defer shipmentServer.Close()

	handler, readModelServer := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeDisabled,
	})
	if readModelServer != nil {
		t.Fatal("read-model server should not be created in disabled mode tests")
	}

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeSummary(t, rec.Body.String())
	if resp.StatusSummary != nil {
		t.Fatalf("expected no statusSummary in disabled mode, got %+v", resp.StatusSummary)
	}
}

func TestSummaryReadModelPrimaryUsesProjection(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{
		{"id": "s1", "shipment_number": "SH-1", "status": "DELIVERED"},
	}, 1)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModePrimary,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeSummary(t, rec.Body.String())
	if resp.StatusSummary == nil || resp.StatusSummary.Source != StatusSummarySourceReadModel {
		t.Fatalf("expected read-model source, got %+v", resp.StatusSummary)
	}
	if resp.StatusSummary.ByStatus["IN_TRANSIT"] != 1 {
		t.Fatalf("byStatus=%v", resp.StatusSummary.ByStatus)
	}
	if resp.KPI.Active == 0 && len(resp.Shipments.Items) == 0 {
		t.Fatal("legacy non-status fields should remain populated")
	}
}

func TestSummaryReadModelPrimaryFallbackOnTimeout(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{
		{"id": "s1", "shipment_number": "SH-1", "status": "IN_TRANSIT"},
		{"id": "s2", "shipment_number": "SH-2", "status": "IN_TRANSIT"},
	}, 2)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode:  controltowerreadmodel.ModePrimary,
		readModelDelay: 2 * time.Second,
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeSummary(t, rec.Body.String())
	if resp.StatusSummary == nil || resp.StatusSummary.Source != StatusSummarySourceLegacy {
		t.Fatalf("expected legacy fallback, got %+v", resp.StatusSummary)
	}
	if resp.StatusSummaryFreshness == nil || !resp.StatusSummaryFreshness.FallbackUsed {
		t.Fatal("expected fallbackUsed=true")
	}
	if !strings.Contains(rec.Body.String(), WarningReadModelUnavailable) {
		t.Fatalf("expected unavailable warning, body=%s", rec.Body.String())
	}
}

func TestSummaryReadModelPrimaryFallbackOn500(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{
		{"id": "s1", "shipment_number": "SH-1", "status": "LOADED"},
	}, 1)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModePrimary,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		},
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeSummary(t, rec.Body.String())
	if resp.StatusSummary.Source != StatusSummarySourceLegacy {
		t.Fatalf("expected legacy fallback on 500, got %+v", resp.StatusSummary)
	}
}

func TestSummaryReadModelPrimaryPartialProjectionWarning(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{
		{"id": "s1", "shipment_number": "SH-1", "status": "IN_TRANSIT"},
	}, 1)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModePrimary,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"IN_TRANSIT":1},"incompleteProjections":1,"freshness":{"consumerRunning":true}}`))
		},
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	resp := decodeSummary(t, rec.Body.String())
	if resp.StatusSummary.Source != StatusSummarySourceReadModel {
		t.Fatalf("expected read-model with partial projection, got %+v", resp.StatusSummary)
	}
	if !strings.Contains(rec.Body.String(), WarningReadModelPartial) {
		t.Fatalf("expected partial warning, body=%s", rec.Body.String())
	}
}

func TestSummaryReadModelShadowKeepsLegacyResponse(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	shipmentServer := shipmentSummaryServer([]map[string]any{
		{"id": "s1", "shipment_number": "SH-1", "status": "DELIVERED"},
	}, 1)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeShadow,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"totalShipments":99,"byStatus":{"IN_TRANSIT":99},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
		},
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	resp := decodeSummary(t, rec.Body.String())
	if resp.StatusSummary != nil {
		t.Fatalf("shadow mode must not expose statusSummary, got %+v", resp.StatusSummary)
	}
}

func TestSummaryReadModelForwardsVerifiedTenant(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"
	var readModelTenant string

	shipmentServer := shipmentSummaryServer([]map[string]any{}, 0)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeShadow,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			readModelTenant = r.Header.Get("X-Tenant-ID")
			_, _ = w.Write([]byte(`{"totalShipments":0,"byStatus":{},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
		},
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", tenantB)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if readModelTenant != tenantA {
		t.Fatalf("read-model tenant=%q want verified %q", readModelTenant, tenantA)
	}
}

func TestSummaryForbiddenRoleDoesNotCallReadModel(t *testing.T) {
	readModelCalled := false
	shipmentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("shipments should not be called")
	}))
	defer shipmentServer.Close()

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"FINANCE_MANAGER"}})
	}))
	defer identityServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityServer.URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModePrimary,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			readModelCalled = true
		},
	})

	tenantA := "11111111-1111-1111-1111-111111111111"
	token := signTestToken(t, "secret", "finance-user", tenantA, "finance@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if readModelCalled {
		t.Fatal("read-model must not be called for forbidden role")
	}
}

func TestSummaryReadModelVerifiedTenantHeaderOnly(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	var gotAuth string
	shipmentServer := shipmentSummaryServer([]map[string]any{}, 0)
	defer shipmentServer.Close()

	handler, _ := newReadModelSummaryHandler(t, readModelHandlerConfig{
		testHandlerConfig: testHandlerConfig{
			shipmentURL: shipmentServer.URL,
			identityURL: identityAdminServer().URL,
			authEnabled: true,
		},
		readModelMode: controltowerreadmodel.ModeShadow,
		readModelFn: func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"totalShipments":0,"byStatus":{},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
		},
	})

	token := signTestToken(t, "secret", "user-a", tenantA, "user@example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/control-tower/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(gwmiddleware.RequestIDHeader, "req-123")
	rec := serveThroughAuth(t, handler, req, "secret", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if gotAuth != "" {
		t.Fatalf("authorization must not be forwarded to read-model: %q", gotAuth)
	}
}
