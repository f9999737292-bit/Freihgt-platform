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
	handler := NewBidHandler(service.NewBidService(store, &mockFreightRequestStoreForBid{}, nil, nil))
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

func TestGetBidByIDQueryOnlyReturns403(t *testing.T) {
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
	if rec.Code != http.StatusForbidden || called {
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
	if rec.Code != http.StatusForbidden {
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

func newListBidsTestRouter(t *testing.T, bids *mockBidStore, requests *mockFreightRequestStoreForBid) http.Handler {
	t.Helper()
	handler := NewBidHandler(service.NewBidService(bids, requests, nil, nil))
	r := chi.NewRouter()
	r.Get("/v1/freight-requests/{id}/bids", handler.ListBids)
	return r
}

func newAcceptBidTestRouter(t *testing.T, bids *mockBidStore, requests *mockFreightRequestStoreForBid) http.Handler {
	t.Helper()
	handler := NewBidHandler(service.NewBidService(bids, requests, nil, nil))
	r := chi.NewRouter()
	r.Post("/v1/bids/{id}/accept", handler.AcceptBid)
	return r
}

func TestListBidsTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	frID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	bidID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(_ context.Context, freightRequestID, tenant uuid.UUID) ([]domain.Bid, error) {
			return []domain.Bid{{ID: bidID, TenantID: tenant, FreightRequestID: freightRequestID, BidNumber: "BID-1", Status: domain.BidStatusSubmitted, Items: []domain.BidItem{}}}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{ID: id, TenantID: tenant, Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/"+frID+"/bids", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListBidsMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/bids", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestListBidsQueryOnlyTenantReturns403(t *testing.T) {
	t.Parallel()
	called := false
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			called = true
			return nil, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/bids?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestListBidsHeaderRejectsConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	frID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(_ context.Context, _, tenant uuid.UUID) ([]domain.Bid, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return []domain.Bid{}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(_ context.Context, _, tenant uuid.UUID) (*domain.FreightRequest, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/"+frID+"/bids?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListBidsInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/bids", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestListBidsInvalidFreightRequestIDReturns400(t *testing.T) {
	t.Parallel()
	router := newListBidsTestRouter(t, &mockBidStore{}, &mockFreightRequestStoreForBid{})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/not-a-uuid/bids", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListBidsMissingFreightRequestReturns404(t *testing.T) {
	t.Parallel()
	listCalled := false
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
			listCalled = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return nil, apperrors.NotFound("freight request not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/bids", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || listCalled {
		t.Fatalf("status=%d listCalled=%v body=%s", rec.Code, listCalled, rec.Body.String())
	}
}

func TestListBidsForeignTenantReturns404(t *testing.T) {
	t.Parallel()
	listCalled := false
	router := newListBidsTestRouter(t, &mockBidStore{
		listByFreightRequestFn: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.Bid, error) {
			listCalled = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return nil, apperrors.NotFound("freight request not found")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/freight-requests/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/bids", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || listCalled {
		t.Fatalf("status=%d listCalled=%v body=%s", rec.Code, listCalled, rec.Body.String())
	}
}

func TestAcceptBidTrustedHeaderReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	bidID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: id, TenantID: tenant, Status: domain.BidStatusSubmitted}, nil
		},
		acceptBidFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: id, TenantID: tenant, Status: domain.BidStatusAccepted}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/"+bidID+"/accept", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAcceptBidMissingTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestAcceptBidQueryOnlyTenantReturns403(t *testing.T) {
	t.Parallel()
	called := false
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestAcceptBidHeaderRejectsConflictingQuery(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	bidID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			if tenant.String() != headerTenant {
				t.Fatalf("expected header tenant, got %s", tenant)
			}
			return &domain.Bid{ID: id, TenantID: tenant, Status: domain.BidStatusSubmitted}, nil
		},
		acceptBidFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: id, TenantID: tenant, Status: domain.BidStatusAccepted}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/"+bidID+"/accept?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAcceptBidInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			called = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, called, rec.Body.String())
	}
}

func TestAcceptBidInvalidIDReturns400(t *testing.T) {
	t.Parallel()
	router := newAcceptBidTestRouter(t, &mockBidStore{}, &mockFreightRequestStoreForBid{})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/not-a-uuid/accept", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAcceptBidNotFoundReturns404(t *testing.T) {
	t.Parallel()
	acceptCalled := false
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return nil, apperrors.NotFound("bid not found")
		},
		acceptBidFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			acceptCalled = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || acceptCalled {
		t.Fatalf("status=%d acceptCalled=%v body=%s", rec.Code, acceptCalled, rec.Body.String())
	}
}

func TestAcceptBidForeignTenantReturns404(t *testing.T) {
	t.Parallel()
	acceptCalled := false
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return nil, apperrors.NotFound("bid not found")
		},
		acceptBidFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			acceptCalled = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || acceptCalled {
		t.Fatalf("status=%d acceptCalled=%v body=%s", rec.Code, acceptCalled, rec.Body.String())
	}
}

func TestAcceptBidNonSubmittedReturns409(t *testing.T) {
	t.Parallel()
	acceptCalled := false
	router := newAcceptBidTestRouter(t, &mockBidStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: id, TenantID: tenant, Status: domain.BidStatusDraft}, nil
		},
		acceptBidFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			acceptCalled = true
			return nil, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/bids/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/accept", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || acceptCalled {
		t.Fatalf("status=%d acceptCalled=%v body=%s", rec.Code, acceptCalled, rec.Body.String())
	}
}

type mockBidStore struct {
	createFn               func(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	getByIDFn              func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
	listByFreightRequestFn func(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error)
	acceptBidFn            func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
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
func (m *mockBidStore) ListByFreightRequest(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error) {
	if m.listByFreightRequestFn != nil {
		return m.listByFreightRequestFn(ctx, freightRequestID, tenantID)
	}
	return nil, nil
}
func (m *mockBidStore) SubmitBid(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.Bid, error) {
	return nil, nil
}
func (m *mockBidStore) AcceptBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	if m.acceptBidFn != nil {
		return m.acceptBidFn(ctx, id, tenantID)
	}
	return nil, nil
}

type mockFreightRequestStoreForBid struct {
	getByIDFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error)
}

func (m *mockFreightRequestStoreForBid) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockFreightRequestStoreForBid) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockFreightRequestStoreForBid) CreateFromTransportOrder(context.Context, domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestStoreForBid) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id, tenantID)
	}
	return nil, nil
}
func (m *mockFreightRequestStoreForBid) List(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	return nil, 0, nil
}
func (m *mockFreightRequestStoreForBid) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.FreightRequest, error) {
	return nil, nil
}
