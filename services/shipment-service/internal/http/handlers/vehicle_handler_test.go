package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/service"
)

type tenantScopedVehicleService struct {
	getFn func(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error)
}

func (s *tenantScopedVehicleService) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *tenantScopedVehicleService) Create(context.Context, uuid.UUID, domain.CreateVehicleInput) (*domain.Vehicle, error) {
	return nil, nil
}
func (s *tenantScopedVehicleService) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Vehicle, error) {
	if s.getFn != nil {
		return s.getFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (s *tenantScopedVehicleService) List(context.Context, uuid.UUID, domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
	return nil, 0, nil
}

func newVehicleGetByIDTestRouter(t *testing.T, store *tenantScopedVehicleService) http.Handler {
	t.Helper()
	handler := NewVehicleHandler(service.NewVehicleService(store))
	r := chi.NewRouter()
	r.Get("/v1/vehicles/{id}", handler.GetByID)
	return r
}

func TestGetVehicleByIDTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	vehicleID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(_ context.Context, tenant, id uuid.UUID) (*domain.Vehicle, error) {
			return &domain.Vehicle{ID: id, TenantID: tenant, PlateNumber: "A123", Status: domain.VehicleStatusActive}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/"+vehicleID, nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetVehicleByIDUnknownReturns404(t *testing.T) {
	t.Parallel()
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.NotFound("vehicle not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/cccccccc-cccc-cccc-cccc-cccccccccccc", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetVehicleByIDForeignReturns404(t *testing.T) {
	t.Parallel()
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.NotFound("vehicle not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/cccccccc-cccc-cccc-cccc-cccccccccccc", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetVehicleByIDMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/cccccccc-cccc-cccc-cccc-cccccccccccc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetVehicleByIDHeaderIgnoresConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	vehicleID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(_ context.Context, tenant, id uuid.UUID) (*domain.Vehicle, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return &domain.Vehicle{ID: id, TenantID: tenant, PlateNumber: "A123", Status: domain.VehicleStatusActive}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/"+vehicleID+"?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetVehicleByIDQueryOnlyReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/cccccccc-cccc-cccc-cccc-cccccccccccc?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetVehicleByIDInvalidUUIDReturns400(t *testing.T) {
	t.Parallel()
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/not-a-uuid", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetVehicleByIDRepositoryFailureReturns500(t *testing.T) {
	t.Parallel()
	router := newVehicleGetByIDTestRouter(t, &tenantScopedVehicleService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/cccccccc-cccc-cccc-cccc-cccccccccccc", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
