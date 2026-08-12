package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
)

func newBidGetByIDTestRouter(t *testing.T, store *mockBidStore) http.Handler {
	t.Helper()
	handler := NewBidHandler(service.NewBidService(store, &mockFreightRequestStoreForBid{}))
	r := chi.NewRouter()
	r.Get("/v1/bids/{id}", handler.GetByID)
	return r
}

func TestGetBidByIDTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	bidID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{
				ID: id, TenantID: tenant, BidNumber: "BID-1", Status: domain.BidStatusDraft,
				Items: []domain.BidItem{},
			}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/"+bidID, nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetBidByIDMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetBidByIDQueryOnlyReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetBidByIDHeaderIgnoresConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	bidID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return &domain.Bid{ID: id, TenantID: tenant, BidNumber: "BID-1", Status: domain.BidStatusDraft}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/"+bidID+"?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetBidByIDNotFoundReturns404(t *testing.T) {
	t.Parallel()
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return nil, apperrors.NotFound("bid not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetBidByIDForeignTenantReturns404(t *testing.T) {
	t.Parallel()
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return nil, apperrors.NotFound("bid not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetBidByIDInvalidIDReturns400(t *testing.T) {
	t.Parallel()
	router := newBidGetByIDTestRouter(t, &mockBidStore{})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/not-a-uuid", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetBidByIDInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestGetBidByIDServiceReceivesScopedTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	bidID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	router := newBidGetByIDTestRouter(t, &mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			if id != bidID || tenant != tenantID {
				t.Fatalf("unexpected scoped lookup id=%s tenant=%s", id, tenant)
			}
			return &domain.Bid{ID: id, TenantID: tenant, BidNumber: "BID-1", Status: domain.BidStatusDraft}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/"+bidID.String(), nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type mockBidStore struct {
	createFn  func(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	getByIDFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
}

func (m *mockBidStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockBidStore) CreateBid(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error) {
	if m.createFn != nil {
		return m.createFn(ctx, in)
	}
	return nil, nil
}
func (m *mockBidStore) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id, tenantID)
	}
	return nil, nil
}
func (m *mockBidStore) ListByFreightRequest(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
	return nil, nil
}
func (m *mockBidStore) SubmitBid(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.Bid, error) {
	return nil, nil
}
func (m *mockBidStore) AcceptBid(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
	return nil, nil
}

type mockFreightRequestStoreForBid struct{}

func (m *mockFreightRequestStoreForBid) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockFreightRequestStoreForBid) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockFreightRequestStoreForBid) CreateFromTransportOrder(context.Context, domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestStoreForBid) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestStoreForBid) List(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	return nil, 0, nil
}
func (m *mockFreightRequestStoreForBid) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.FreightRequest, error) {
	return nil, nil
}
