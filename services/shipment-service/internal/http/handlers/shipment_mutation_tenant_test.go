package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

func newMutationTenantRouter(t *testing.T, store *mutationActorShipmentStore) http.Handler {
	t.Helper()
	handler := NewShipmentHandler(service.NewShipmentService(store, nil, nil))
	r := chi.NewRouter()
	r.Post("/v1/shipments/from-transport-order", handler.CreateFromTransportOrder)
	r.Post("/v1/shipments/from-bid", handler.CreateFromBid)
	r.Post("/v1/shipments/{id}/accept", handler.Accept)
	r.Patch("/v1/shipments/{id}/status", handler.UpdateStatus)
	r.Post("/v1/shipments/{id}/cancel", handler.Cancel)
	return r
}

func TestCreateShipmentBodyTenantWithoutHeaderReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{
		"tenant_id": uuid.NewString(), "shipment_number": "SHP-1",
		"transport_order_id": uuid.NewString(), "carrier_company_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-transport-order", bytes.NewReader(raw))
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called without trusted tenant header")
	}
}

func TestCreateShipmentBodyTenantWithHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{
		"tenant_id": "22222222-2222-2222-2222-222222222222", "shipment_number": "SHP-1",
		"transport_order_id": uuid.NewString(), "carrier_company_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-transport-order", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called when body contains tenant_id")
	}
}

func TestCreateShipmentVerifiedTenantPassedToService(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	var createTenant uuid.UUID
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		createFn: func(_ context.Context, params repository.CreateShipmentParams, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			createTenant = params.TenantID
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned}, nil
		},
	})
	body := map[string]any{
		"shipment_number": "SHP-1", "transport_order_id": uuid.NewString(),
		"carrier_company_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-transport-order", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", headerTenant)
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if createTenant.String() != headerTenant {
		t.Fatalf("create tenant=%s want header tenant", createTenant)
	}
}

func TestUpdateStatusBodyTenantReturns400(t *testing.T) {
	t.Parallel()
	called := false
	shipmentID := uuid.NewString()
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"tenant_id": uuid.NewString(), "status": domain.ShipmentStatusInTransit}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/shipments/"+shipmentID+"/status", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called when body contains tenant_id")
	}
}

func TestUpdateStatusMissingTenantReturns401BeforeDecode(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"tenant_id": uuid.NewString(), "status": domain.ShipmentStatusInTransit}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/shipments/"+uuid.NewString()+"/status", bytes.NewReader(raw))
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called without trusted tenant")
	}
}

func TestCancelBodyTenantConflictReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		cancelFn: func(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"tenant_id": "22222222-2222-2222-2222-222222222222", "reason": "CUSTOMER_REQUEST"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+uuid.NewString()+"/cancel", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called on tenant body conflict")
	}
}

func TestAcceptForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+uuid.NewString()+"/accept", nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateFromBidBodyTenantWithHeaderReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{
		"tenant_id": "22222222-2222-2222-2222-222222222222", "shipment_number": "SHP-2",
		"bid_id": uuid.NewString(), "transport_order_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-bid", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called when body contains tenant_id")
	}
}

func TestCreateFromBidVerifiedTenantPassedToService(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	var createTenant uuid.UUID
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		createFn: func(_ context.Context, params repository.CreateShipmentParams, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			createTenant = params.TenantID
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned}, nil
		},
	})
	body := map[string]any{
		"shipment_number": "SHP-2", "bid_id": uuid.NewString(),
		"transport_order_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-bid", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", headerTenant)
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if createTenant.String() != headerTenant {
		t.Fatalf("create tenant=%s want header tenant", createTenant)
	}
}

func TestUpdateStatusQueryOnlyTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"status": domain.ShipmentStatusInTransit}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/shipments/"+uuid.NewString()+"/status?tenant_id="+uuid.NewString(), bytes.NewReader(raw))
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called when tenant is only in query")
	}
}

func TestCancelQueryOnlyTenantReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		cancelFn: func(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"reason": "CUSTOMER_REQUEST"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+uuid.NewString()+"/cancel?tenant_id="+uuid.NewString(), bytes.NewReader(raw))
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called when tenant is only in query")
	}
}

func TestUpdateStatusForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
		updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"status": domain.ShipmentStatusInTransit}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/shipments/"+uuid.NewString()+"/status", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("repository update must not run for foreign shipment")
	}
}

func TestCancelForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
		cancelFn: func(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"reason": "CUSTOMER_REQUEST"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+uuid.NewString()+"/cancel", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("repository cancel must not run for foreign shipment")
	}
}

func TestAcceptBodyTenantReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationTenantRouter(t, &mutationActorShipmentStore{
		acceptFn: func(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"tenant_id": uuid.NewString()}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+uuid.NewString()+"/accept", bytes.NewReader(raw))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("accept must not run when body contains tenant_id")
	}
}
