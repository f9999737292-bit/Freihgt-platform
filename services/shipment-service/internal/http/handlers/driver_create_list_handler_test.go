package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

type createListDriverService struct {
	createFn func(ctx context.Context, tenantID uuid.UUID, in domain.CreateDriverInput) (*domain.Driver, error)
	listFn   func(ctx context.Context, tenantID uuid.UUID, filter domain.ListDriversFilter) ([]domain.Driver, int, error)
}

func (s *createListDriverService) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *createListDriverService) Create(ctx context.Context, tenantID uuid.UUID, in domain.CreateDriverInput) (*domain.Driver, error) {
	if s.createFn != nil {
		return s.createFn(ctx, tenantID, in)
	}
	return nil, nil
}
func (s *createListDriverService) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
	return nil, nil
}
func (s *createListDriverService) List(ctx context.Context, tenantID uuid.UUID, filter domain.ListDriversFilter) ([]domain.Driver, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, filter)
	}
	return nil, 0, nil
}

func newDriverCreateRouter(t *testing.T, store *createListDriverService) http.Handler {
	t.Helper()
	handler := NewDriverHandler(service.NewDriverService(store))
	r := chi.NewRouter()
	r.Post("/v1/drivers", handler.Create)
	return r
}

func newDriverListRouter(t *testing.T, store *createListDriverService) http.Handler {
	t.Helper()
	handler := NewDriverHandler(service.NewDriverService(store))
	r := chi.NewRouter()
	r.Get("/v1/drivers", handler.List)
	return r
}

func TestCreateDriverTrustedHeaderReturns201(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	companyID := "22222222-2222-2222-2222-222222222222"
	router := newDriverCreateRouter(t, &createListDriverService{
		createFn: func(_ context.Context, tenant uuid.UUID, in domain.CreateDriverInput) (*domain.Driver, error) {
			return &domain.Driver{
				ID: uuid.New(), TenantID: tenant, CarrierCompanyID: in.CarrierCompanyID,
				FullName: in.FullName, Status: domain.DriverStatusActive,
			}, nil
		},
	})
	body, _ := json.Marshal(map[string]string{
		"carrier_company_id": companyID,
		"full_name":          "Driver A",
		"license_country":    "RU",
		"preferred_locale":   "ru-RU",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/drivers", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateDriverBodyTenantReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id":          "22222222-2222-2222-2222-222222222222",
		"carrier_company_id": "33333333-3333-3333-3333-333333333333",
		"full_name":          "Driver A",
	})
	router := newDriverCreateRouter(t, &createListDriverService{
		createFn: func(context.Context, uuid.UUID, domain.CreateDriverInput) (*domain.Driver, error) {
			t.Fatal("service must not be called when body contains tenant_id")
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/drivers", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateDriverBodyTenantWithoutHeaderReturns401(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id":          "11111111-1111-1111-1111-111111111111",
		"carrier_company_id": "33333333-3333-3333-3333-333333333333",
		"full_name":          "Driver A",
	})
	router := newDriverCreateRouter(t, &createListDriverService{
		createFn: func(context.Context, uuid.UUID, domain.CreateDriverInput) (*domain.Driver, error) {
			t.Fatal("service must not be called without trusted header")
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/drivers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateDriverMissingHeaderReturns401(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"carrier_company_id": "33333333-3333-3333-3333-333333333333",
		"full_name":          "Driver A",
	})
	router := newDriverCreateRouter(t, &createListDriverService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/drivers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateDriverInternalErrorReturns500(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"carrier_company_id": "33333333-3333-3333-3333-333333333333",
		"full_name":          "Driver A",
	})
	router := newDriverCreateRouter(t, &createListDriverService{
		createFn: func(context.Context, uuid.UUID, domain.CreateDriverInput) (*domain.Driver, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/drivers", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListDriversTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	router := newDriverListRouter(t, &createListDriverService{
		listFn: func(_ context.Context, tenant uuid.UUID, _ domain.ListDriversFilter) ([]domain.Driver, int, error) {
			if tenant.String() != tenantID {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return []domain.Driver{{FullName: "A"}}, 1, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListDriversQueryOnlyTenantReturns401(t *testing.T) {
	t.Parallel()
	router := newDriverListRouter(t, &createListDriverService{
		listFn: func(context.Context, uuid.UUID, domain.ListDriversFilter) ([]domain.Driver, int, error) {
			t.Fatal("service must not be called for query-only tenant")
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListDriversHeaderIgnoresConflictingQueryTenant(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	router := newDriverListRouter(t, &createListDriverService{
		listFn: func(_ context.Context, tenant uuid.UUID, _ domain.ListDriversFilter) ([]domain.Driver, int, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return []domain.Driver{{FullName: "A"}}, 1, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/drivers?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type createListVehicleService struct {
	createFn func(ctx context.Context, tenantID uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error)
	listFn   func(ctx context.Context, tenantID uuid.UUID, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error)
}

func (s *createListVehicleService) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *createListVehicleService) Create(ctx context.Context, tenantID uuid.UUID, in domain.CreateVehicleInput) (*domain.Vehicle, error) {
	if s.createFn != nil {
		return s.createFn(ctx, tenantID, in)
	}
	return nil, nil
}
func (s *createListVehicleService) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
	return nil, nil
}
func (s *createListVehicleService) List(ctx context.Context, tenantID uuid.UUID, filter domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, filter)
	}
	return nil, 0, nil
}

func newVehicleCreateRouter(t *testing.T, store *createListVehicleService) http.Handler {
	t.Helper()
	handler := NewVehicleHandler(service.NewVehicleService(store))
	r := chi.NewRouter()
	r.Post("/v1/vehicles", handler.Create)
	return r
}

func newVehicleListRouter(t *testing.T, store *createListVehicleService) http.Handler {
	t.Helper()
	handler := NewVehicleHandler(service.NewVehicleService(store))
	r := chi.NewRouter()
	r.Get("/v1/vehicles", handler.List)
	return r
}

func TestCreateVehicleBodyTenantReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id":          "22222222-2222-2222-2222-222222222222",
		"carrier_company_id": "33333333-3333-3333-3333-333333333333",
		"plate_number":       "A123",
	})
	router := newVehicleCreateRouter(t, &createListVehicleService{
		createFn: func(context.Context, uuid.UUID, domain.CreateVehicleInput) (*domain.Vehicle, error) {
			t.Fatal("service must not be called when body contains tenant_id")
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/vehicles", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListVehiclesQueryOnlyTenantReturns401(t *testing.T) {
	t.Parallel()
	router := newVehicleListRouter(t, &createListVehicleService{
		listFn: func(context.Context, uuid.UUID, domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
			t.Fatal("service must not be called for query-only tenant")
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListVehiclesHeaderIgnoresConflictingQueryTenant(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	router := newVehicleListRouter(t, &createListVehicleService{
		listFn: func(_ context.Context, tenant uuid.UUID, _ domain.ListVehiclesFilter) ([]domain.Vehicle, int, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return []domain.Vehicle{{PlateNumber: "A123"}}, 1, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles?tenant_id=22222222-2222-2222-2222-222222222222", nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
