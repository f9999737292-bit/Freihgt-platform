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
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type tenantScopedShipmentService struct {
	getFn  func(ctx context.Context, tenantID, id uuid.UUID) (*domain.Shipment, error)
	listFn func(ctx context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error)
}

func (s *tenantScopedShipmentService) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *tenantScopedShipmentService) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrderSnapshot, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) GetBid(context.Context, uuid.UUID, uuid.UUID) (*domain.BidSnapshot, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) CreateShipment(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	if s.getFn != nil {
		return s.getFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (s *tenantScopedShipmentService) List(ctx context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return nil, 0, nil
}
func (s *tenantScopedShipmentService) AssignDriver(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) AssignVehicle(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) Accept(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) Cancel(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *tenantScopedShipmentService) ListStatusHistory(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
	return nil, 0, nil
}
func (s *tenantScopedShipmentService) HasInitialStatusHistory(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func newGetByIDTestRouter(t *testing.T, store *tenantScopedShipmentService) http.Handler {
	t.Helper()
	handler := NewShipmentHandler(service.NewShipmentService(store, nil, nil))
	r := chi.NewRouter()
	r.Get("/v1/shipments/{id}", handler.GetByID)
	return r
}

func newListTestRouter(t *testing.T, store *tenantScopedShipmentService) http.Handler {
	t.Helper()
	handler := NewShipmentHandler(service.NewShipmentService(store, nil, nil))
	r := chi.NewRouter()
	r.Get("/v1/shipments", handler.List)
	return r
}

func TestGetByIDTrustedHeaderCurrentTenantReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(_ context.Context, tenant, id uuid.UUID) (*domain.Shipment, error) {
			if tenant.String() != tenantID || id.String() != shipmentID {
				t.Fatalf("unexpected lookup tenant=%s id=%s", tenant, id)
			}
			return &domain.Shipment{
				ID: id, TenantID: tenant, ShipmentNumber: "SHP-1", Status: domain.ShipmentStatusInTransit,
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/"+shipmentID, nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetByIDUnknownShipmentReturns404(t *testing.T) {
	t.Parallel()
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetByIDForeignTenantReturns404(t *testing.T) {
	t.Parallel()
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetByIDInvalidShipmentUUIDReturns400(t *testing.T) {
	t.Parallel()
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/not-a-uuid", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetByIDInvalidHeaderTenantReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called with invalid tenant header")
	}
}

func TestGetByIDQueryOnlyReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called for query-only tenant")
	}
}

func TestGetByIDMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called without tenant")
	}
}

func TestGetByIDHeaderIgnoresConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(_ context.Context, tenant, id uuid.UUID) (*domain.Shipment, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return &domain.Shipment{
				ID: id, TenantID: tenant, ShipmentNumber: "SHP-1", Status: domain.ShipmentStatusInTransit,
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/"+shipmentID+"?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetByIDRepositoryFailureReturns500(t *testing.T) {
	t.Parallel()
	router := newGetByIDTestRouter(t, &tenantScopedShipmentService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.Internal("db failure", errors.New("connection reset"))
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestShipmentListMissingTrustedTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(context.Context, domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called without trusted tenant")
	}
}

func TestShipmentListQueryOnlyTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(context.Context, domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called for query-only tenant")
	}
}

func TestShipmentListIgnoresForeignTenantQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(_ context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			if filter.TenantID.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", filter.TenantID)
			}
			return []domain.Shipment{{
				ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				TenantID: filter.TenantID, ShipmentNumber: "SHP-1", Status: domain.ShipmentStatusInTransit,
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}}, 1, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestShipmentListUsesTrustedTenant(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(_ context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			if filter.TenantID.String() != headerTenant {
				t.Fatalf("unexpected tenant %s", filter.TenantID)
			}
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments", nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestShipmentListRepositoryReceivesTrustedTenant(t *testing.T) {
	t.Parallel()
	headerTenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	foreignTenant := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	var gotTenant uuid.UUID
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(_ context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			gotTenant = filter.TenantID
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments?tenant_id="+foreignTenant.String(), nil)
	req.Header.Set("X-Tenant-ID", headerTenant.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotTenant != headerTenant {
		t.Fatalf("repository tenant=%s want=%s", gotTenant, headerTenant)
	}
}

func TestShipmentListDoesNotPerformUnscopedLookup(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	foreignTenant := "22222222-2222-2222-2222-222222222222"
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(_ context.Context, filter domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			if filter.TenantID.String() == foreignTenant {
				t.Fatal("unscoped foreign tenant lookup must not occur")
			}
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments?tenant_id="+foreignTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestShipmentListInvalidHeaderTenantReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newListTestRouter(t, &tenantScopedShipmentService{
		listFn: func(context.Context, domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/shipments", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called with invalid tenant header")
	}
}
