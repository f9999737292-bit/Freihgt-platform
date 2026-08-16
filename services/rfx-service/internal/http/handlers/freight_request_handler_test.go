package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
)

func newFreightRequestTestRouter(t *testing.T, store *mockFreightRequestHandlerStore) http.Handler {
	t.Helper()
	handler := NewFreightRequestHandler(service.NewFreightRequestService(store))
	r := chi.NewRouter()
	r.Get("/v1/freight-requests", handler.List)
	r.Get("/v1/freight-requests/{id}", handler.GetByID)
	return r
}

func sampleFreightRequest(id, tenantID uuid.UUID) *domain.FreightRequest {
	now := time.Now().UTC()
	return &domain.FreightRequest{
		ID:                   id,
		TenantID:             tenantID,
		FreightRequestNumber: "FR-1",
		RequestType:          "SPOT",
		ShipperCompanyID:     uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		Status:               domain.FreightRequestStatusDraft,
		CreatedAt:            now,
		UpdatedAt:            now,
		Version:              1,
	}
}

func TestListFreightRequestsTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		listFn: func(_ context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			if filter.TenantID.String() != tenantID {
				t.Fatalf("expected header tenant, got %s", filter.TenantID)
			}
			fr := sampleFreightRequest(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), filter.TenantID)
			return []domain.FreightRequest{*fr}, 1, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListFreightRequestsMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		listFn: func(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestListFreightRequestsQueryOnlyReturns403(t *testing.T) {
	t.Parallel()
	called := false
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		listFn: func(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestListFreightRequestsHeaderRejectsConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		listFn: func(_ context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			if filter.TenantID.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", filter.TenantID)
			}
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListFreightRequestsInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		listFn: func(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			called = true
			return nil, 0, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestListFreightRequestsFiltersRemainFunctional(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipperID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	requestType := "SPOT"
	status := domain.FreightRequestStatusDraft
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		listFn: func(_ context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			if filter.TenantID != tenantID {
				t.Fatalf("unexpected tenant %s", filter.TenantID)
			}
			if filter.Limit != 10 || filter.Offset != 5 {
				t.Fatalf("unexpected pagination limit=%d offset=%d", filter.Limit, filter.Offset)
			}
			if filter.RequestType == nil || *filter.RequestType != requestType {
				t.Fatalf("unexpected request_type %#v", filter.RequestType)
			}
			if filter.Status == nil || *filter.Status != status {
				t.Fatalf("unexpected status %#v", filter.Status)
			}
			if filter.ShipperCompanyID == nil || *filter.ShipperCompanyID != shipperID {
				t.Fatalf("unexpected shipper_company_id %#v", filter.ShipperCompanyID)
			}
			return nil, 0, nil
		},
	})
	url := "/v1/freight-requests?limit=10&offset=5&request_type=SPOT&status=DRAFT&shipper_company_id=" + shipperID.String()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetFreightRequestByIDTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	frID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return sampleFreightRequest(id, tenant), nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/"+frID, nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetFreightRequestByIDMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetFreightRequestByIDQueryOnlyReturns403(t *testing.T) {
	t.Parallel()
	called := false
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetFreightRequestByIDHeaderRejectsConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	frID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return sampleFreightRequest(id, tenant), nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/"+frID+"?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetFreightRequestByIDInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetFreightRequestByIDInvalidIDReturns400(t *testing.T) {
	t.Parallel()
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/not-a-uuid", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetFreightRequestByIDNotFoundReturns404(t *testing.T) {
	t.Parallel()
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return nil, apperrors.NotFound("freight request not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetFreightRequestByIDForeignTenantReturns404(t *testing.T) {
	t.Parallel()
	router := newFreightRequestTestRouter(t, &mockFreightRequestHandlerStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return nil, apperrors.NotFound("freight request not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type mockFreightRequestHandlerStore struct {
	getByIDFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error)
	listFn    func(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error)
}

func (m *mockFreightRequestHandlerStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockFreightRequestHandlerStore) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return domain.TransportOrderStatusReadyForSourcing, nil
}
func (m *mockFreightRequestHandlerStore) CreateFromTransportOrder(context.Context, domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestHandlerStore) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id, tenantID)
	}
	return nil, nil
}
func (m *mockFreightRequestHandlerStore) List(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return nil, 0, nil
}
func (m *mockFreightRequestHandlerStore) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.FreightRequest, error) {
	return nil, nil
}
