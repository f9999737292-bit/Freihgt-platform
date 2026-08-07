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

type tenantScopedDriverService struct {
	getFn func(ctx context.Context, tenantID, id uuid.UUID) (*domain.Driver, error)
}

func (s *tenantScopedDriverService) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *tenantScopedDriverService) Create(context.Context, uuid.UUID, domain.CreateDriverInput) (*domain.Driver, error) {
	return nil, nil
}
func (s *tenantScopedDriverService) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Driver, error) {
	if s.getFn != nil {
		return s.getFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (s *tenantScopedDriverService) List(context.Context, uuid.UUID, domain.ListDriversFilter) ([]domain.Driver, int, error) {
	return nil, 0, nil
}

func newDriverGetByIDTestRouter(t *testing.T, store *tenantScopedDriverService) http.Handler {
	t.Helper()
	handler := NewDriverHandler(service.NewDriverService(store))
	r := chi.NewRouter()
	r.Get("/v1/drivers/{id}", handler.GetByID)
	return r
}

func TestGetDriverByIDTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	driverID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(_ context.Context, tenant, id uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{ID: id, TenantID: tenant, FullName: "Driver A", Status: domain.DriverStatusActive}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/"+driverID, nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetDriverByIDUnknownReturns404(t *testing.T) {
	t.Parallel()
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.NotFound("driver not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetDriverByIDForeignReturns404(t *testing.T) {
	t.Parallel()
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.NotFound("driver not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetDriverByIDMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetDriverByIDHeaderIgnoresConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	driverID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(_ context.Context, tenant, id uuid.UUID) (*domain.Driver, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return &domain.Driver{ID: id, TenantID: tenant, FullName: "Driver A", Status: domain.DriverStatusActive}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/"+driverID+"?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetDriverByIDQueryOnlyReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetDriverByIDInvalidUUIDReturns400(t *testing.T) {
	t.Parallel()
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/not-a-uuid", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetDriverByIDRepositoryFailureReturns500(t *testing.T) {
	t.Parallel()
	router := newDriverGetByIDTestRouter(t, &tenantScopedDriverService{
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
