package handlers

import (
	"bytes"
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
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type mutationActorShipmentStore struct {
	createFn            func(ctx context.Context, params repository.CreateShipmentParams, transition domain.StatusTransitionContext) (*domain.Shipment, error)
	updateStatusFn      func(ctx context.Context, id, tenantID uuid.UUID, fromStatus, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error)
	getTransportOrderFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrderSnapshot, error)
	getBidFn            func(ctx context.Context, id, tenantID uuid.UUID) (*domain.BidSnapshot, error)
	acceptFn            func(ctx context.Context, id, tenantID uuid.UUID, fromStatus string, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error)
	cancelFn            func(ctx context.Context, id, tenantID uuid.UUID, fromStatus string, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error)
	getByIDAndTenantFn  func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
}

func (s *mutationActorShipmentStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *mutationActorShipmentStore) GetTransportOrder(ctx context.Context, id, tenantID uuid.UUID) (*domain.TransportOrderSnapshot, error) {
	if s.getTransportOrderFn != nil {
		return s.getTransportOrderFn(ctx, id, tenantID)
	}
	return &domain.TransportOrderSnapshot{
		Status:           domain.TransportOrderStatusAssigned,
		ShipperCompanyID: uuid.New(), ConsigneeCompanyID: uuid.New(),
		OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
		TransportMode: "ROAD",
	}, nil
}
func (s *mutationActorShipmentStore) GetBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.BidSnapshot, error) {
	if s.getBidFn != nil {
		return s.getBidFn(ctx, id, tenantID)
	}
	return &domain.BidSnapshot{
		Status: domain.BidStatusAccepted, CarrierCompanyID: uuid.New(),
	}, nil
}
func (s *mutationActorShipmentStore) CreateShipment(ctx context.Context, params repository.CreateShipmentParams, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
	if s.createFn != nil {
		return s.createFn(ctx, params, transition)
	}
	return nil, errors.New("unexpected create")
}
func (s *mutationActorShipmentStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	if s.getByIDAndTenantFn != nil {
		return s.getByIDAndTenantFn(ctx, id, tenantID)
	}
	return &domain.Shipment{ID: id, TenantID: tenantID, Status: domain.ShipmentStatusCarrierAssigned, Version: 1}, nil
}
func (s *mutationActorShipmentStore) List(context.Context, domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
	return nil, 0, nil
}
func (s *mutationActorShipmentStore) AssignDriver(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *mutationActorShipmentStore) AssignVehicle(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
	return nil, nil
}
func (s *mutationActorShipmentStore) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, fromStatus, newStatus string, actualPickupAt, actualDeliveryAt *time.Time, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, id, tenantID, fromStatus, newStatus, actualPickupAt, actualDeliveryAt, expectedVersion, transition)
	}
	return nil, errors.New("unexpected update")
}
func (s *mutationActorShipmentStore) Accept(ctx context.Context, id, tenantID uuid.UUID, fromStatus string, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
	if s.acceptFn != nil {
		return s.acceptFn(ctx, id, tenantID, fromStatus, expectedVersion, transition)
	}
	return nil, nil
}
func (s *mutationActorShipmentStore) Cancel(ctx context.Context, id, tenantID uuid.UUID, fromStatus string, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
	if s.cancelFn != nil {
		return s.cancelFn(ctx, id, tenantID, fromStatus, expectedVersion, transition)
	}
	return nil, nil
}
func (s *mutationActorShipmentStore) ListStatusHistory(context.Context, domain.ListStatusHistoryFilter) ([]domain.ShipmentStatusHistory, int, error) {
	return nil, 0, nil
}
func (s *mutationActorShipmentStore) HasInitialStatusHistory(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func newMutationActorRouter(t *testing.T, store *mutationActorShipmentStore) http.Handler {
	t.Helper()
	handler := NewShipmentHandler(service.NewShipmentService(store, nil, nil))
	r := chi.NewRouter()
	r.Post("/v1/shipments/from-transport-order", handler.CreateFromTransportOrder)
	r.Patch("/v1/shipments/{id}/status", handler.UpdateStatus)
	return r
}

func TestCreateShipmentVerifiedUserPassesActorToService(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationActorRouter(t, &mutationActorShipmentStore{
		createFn: func(_ context.Context, _ repository.CreateShipmentParams, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			if transition.ActorType != domain.ActorTypeUser {
				t.Fatalf("actorType=%s", transition.ActorType)
			}
			if transition.ActorID == nil || transition.ActorID.String() != testVerifiedUserID {
				t.Fatalf("actorID=%v", transition.ActorID)
			}
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned}, nil
		},
	})
	body := map[string]any{
		"shipment_number":    "SHP-1",
		"transport_order_id": uuid.NewString(), "carrier_company_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-transport-order", bytes.NewReader(raw))
	setVerifiedTenantHeader(req)
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("service must be called")
	}
}

func TestCreateShipmentMissingUserReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationActorRouter(t, &mutationActorShipmentStore{
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{
		"shipment_number":    "SHP-1",
		"transport_order_id": uuid.NewString(), "carrier_company_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-transport-order", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called without verified user")
	}
}

func TestUpdateStatusMissingUserReturns401(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationActorRouter(t, &mutationActorShipmentStore{
		updateStatusFn: func(context.Context, uuid.UUID, uuid.UUID, string, string, *time.Time, *time.Time, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{"status": domain.ShipmentStatusAcceptedByCarrier}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/v1/shipments/"+uuid.NewString()+"/status", bytes.NewReader(raw))
	setVerifiedTenantHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called without verified user")
	}
}

func TestCreateShipmentBodyActorIDReturns400(t *testing.T) {
	t.Parallel()
	called := false
	router := newMutationActorRouter(t, &mutationActorShipmentStore{
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	})
	body := map[string]any{
		"shipment_number":    "SHP-2",
		"transport_order_id": uuid.NewString(), "carrier_company_id": uuid.NewString(),
		"actor_id": uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/from-transport-order", bytes.NewReader(raw))
	setVerifiedTenantHeader(req)
	setVerifiedUserHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("service must not be called when body contains actor_id")
	}
}
