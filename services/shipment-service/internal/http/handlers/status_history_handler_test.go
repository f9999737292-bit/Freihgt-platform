package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/service"
)

type statusHistoryHandlerStore struct {
	getFn     func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	listFn    func(ctx context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error)
	hasInitFn func(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error)
}

func (s *statusHistoryHandlerStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	return s.getFn(ctx, id, tenantID)
}
func (s *statusHistoryHandlerStore) ListStatusHistory(ctx context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
	return s.listFn(ctx, filter)
}
func (s *statusHistoryHandlerStore) HasInitialStatusHistory(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error) {
	return s.hasInitFn(ctx, tenantID, shipmentID)
}

func newStatusHistoryTestRouter(t *testing.T, store *statusHistoryHandlerStore) http.Handler {
	t.Helper()
	handler := NewStatusHistoryHandler(service.NewStatusHistoryService(store))
	r := chi.NewRouter()
	r.Get("/internal/v1/shipments/{shipmentId}/status-history", handler.List)
	return r
}

func TestStatusHistoryHandlerCompleteHistoryReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	actorID := uuid.New()
	now := time.Now().UTC()
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: id, TenantID: tenant, ShipmentNumber: "SHP-1", Status: domain.ShipmentStatusInTransit,
			}, nil
		},
		listFn: func(_ context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return []domain.ShipmentStatusHistory{{
				ID: uuid.New(), TenantID: filter.TenantID, ShipmentID: filter.ShipmentID,
				ShipmentVersion: 1, ToStatus: domain.ShipmentStatusCarrierAssigned,
				Source: string(domain.StatusHistorySourceShipmentService), ActorType: domain.ActorTypeUser,
				ActorID: &actorID, OccurredAt: now, RecordedAt: now,
			}}, 1, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+shipmentID+"/status-history", nil)
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
	if payload["complete"] != true {
		t.Fatalf("complete=%v", payload["complete"])
	}
	if _, ok := payload["tenant_id"]; ok {
		t.Fatal("response must not expose tenant_id")
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items=%v", payload["items"])
	}
}

func TestStatusHistoryHandlerPartialLegacyHistory(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	from := domain.ShipmentStatusCarrierAssigned
	now := time.Now().UTC()
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{ID: id, TenantID: tenant, ShipmentNumber: "SHP-2", Status: domain.ShipmentStatusInTransit}, nil
		},
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return []domain.ShipmentStatusHistory{{
				ID: uuid.New(), ShipmentVersion: 3, FromStatus: &from,
				ToStatus: domain.ShipmentStatusInTransit, OccurredAt: now, RecordedAt: now,
				ActorType: domain.ActorTypeSystem, Source: string(domain.StatusHistorySourceShipmentService),
			}}, 1, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+shipmentID+"/status-history", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["complete"] != false {
		t.Fatalf("complete=%v", payload["complete"])
	}
	warnings, _ := payload["warnings"].([]any)
	if len(warnings) != 1 || warnings[0] != domain.StatusHistoryWarningPartial {
		t.Fatalf("warnings=%v", payload["warnings"])
	}
}

func TestStatusHistoryHandlerMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) { return nil, nil },
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return nil, 0, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+uuid.NewString()+"/status-history", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStatusHistoryHandlerMalformedTenantReturns400(t *testing.T) {
	t.Parallel()
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) { return nil, nil },
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return nil, 0, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+uuid.NewString()+"/status-history", nil)
	req.Header.Set("X-Tenant-ID", "bad-tenant")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStatusHistoryHandlerForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return nil, 0, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+uuid.NewString()+"/status-history", nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStatusHistoryHandlerRepositoryFailureReturns500(t *testing.T) {
	t.Parallel()
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{ShipmentNumber: "SHP-3", Status: domain.ShipmentStatusInTransit}, nil
		},
		listFn: func(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			return nil, 0, apperrors.Internal("db failure", errors.New("boom"))
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+uuid.NewString()+"/status-history", nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatusHistoryHandlerIgnoresTenantQuerySpoofing(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	shipmentID := uuid.NewString()
	var lookupTenant uuid.UUID
	router := newStatusHistoryTestRouter(t, &statusHistoryHandlerStore{
		getFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			lookupTenant = tenant
			return &domain.Shipment{ID: id, TenantID: tenant, ShipmentNumber: "SHP-4", Status: domain.ShipmentStatusInTransit}, nil
		},
		listFn: func(_ context.Context, filter domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
			if filter.TenantID.String() != headerTenant {
				t.Fatalf("filter tenant=%s want header tenant", filter.TenantID)
			}
			return nil, 0, nil
		},
		hasInitFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil },
	})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+shipmentID+"/status-history?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if lookupTenant.String() != headerTenant {
		t.Fatalf("lookup tenant=%s want header tenant", lookupTenant)
	}
}
