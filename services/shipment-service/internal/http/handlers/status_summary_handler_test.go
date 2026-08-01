package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type statusSummaryHandlerRepo struct {
	getFn func(ctx context.Context, tenantID uuid.UUID) ([]repository.StatusSummaryRow, error)
}

func (s *statusSummaryHandlerRepo) GetStatusSummary(ctx context.Context, tenantID uuid.UUID) ([]repository.StatusSummaryRow, error) {
	return s.getFn(ctx, tenantID)
}

func newStatusSummaryTestRouter(t *testing.T, repo *statusSummaryHandlerRepo) http.Handler {
	t.Helper()
	handler := NewStatusSummaryHandler(service.NewStatusSummaryService(repo))
	r := chi.NewRouter()
	r.Get("/internal/v1/shipments/status-summary", handler.GetStatusSummary)
	return r
}

func TestStatusSummaryHandlerReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	router := newStatusSummaryTestRouter(t, &statusSummaryHandlerRepo{
		getFn: func(_ context.Context, tenant uuid.UUID) ([]repository.StatusSummaryRow, error) {
			if tenant.String() != tenantID {
				t.Fatalf("tenant=%s want header tenant", tenant)
			}
			return []repository.StatusSummaryRow{
				{Status: domain.ShipmentStatusCarrierAssigned, ShipmentCount: 2, TotalShipments: 9},
				{Status: domain.ShipmentStatusInTransit, ShipmentCount: 3, TotalShipments: 9},
				{Status: domain.ShipmentStatusDelivered, ShipmentCount: 4, TotalShipments: 9},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/status-summary", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["totalShipments"] != float64(9) {
		t.Fatalf("totalShipments=%v", payload["totalShipments"])
	}
	if payload["countedShipments"] != float64(9) {
		t.Fatalf("countedShipments=%v", payload["countedShipments"])
	}
	if payload["complete"] != true {
		t.Fatalf("complete=%v", payload["complete"])
	}
	if _, ok := payload["tenant_id"]; ok {
		t.Fatal("response must not expose tenant_id")
	}
	if _, ok := payload["tenantId"]; ok {
		t.Fatal("response must not expose tenantId")
	}
	byStatus, ok := payload["byStatus"].(map[string]any)
	if !ok || len(byStatus) != 3 {
		t.Fatalf("byStatus=%v", payload["byStatus"])
	}
}

func TestStatusSummaryHandlerMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	router := newStatusSummaryTestRouter(t, &statusSummaryHandlerRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			t.Fatal("repository must not be called without tenant")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/status-summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatusSummaryHandlerMalformedTenantReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusSummaryTestRouter(t, &statusSummaryHandlerRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			t.Fatal("repository must not be called for malformed tenant")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/status-summary", nil)
	req.Header.Set("X-Tenant-ID", "bad-tenant")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatusSummaryHandlerZeroUUIDTenantReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusSummaryTestRouter(t, &statusSummaryHandlerRepo{
		getFn: func(context.Context, uuid.UUID) ([]repository.StatusSummaryRow, error) {
			t.Fatal("repository must not be called for zero tenant")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/status-summary", nil)
	req.Header.Set("X-Tenant-ID", uuid.Nil.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
