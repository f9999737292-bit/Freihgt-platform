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
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type assignShipmentStore struct {
	getByIDAndTenantFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error)
	assignDriverFn     func(ctx context.Context, id, tenantID, driverID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error)
	assignVehicleFn    func(ctx context.Context, id, tenantID, vehicleID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error)
}

func (s *assignShipmentStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *assignShipmentStore) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrderSnapshot, error) {
	return nil, nil
}
func (s *assignShipmentStore) GetBid(context.Context, uuid.UUID, uuid.UUID) (*domain.BidSnapshot, error) {
	return nil, nil
}
func (s *assignShipmentStore) CreateShipment(context.Context, repository.CreateShipmentParams) (*domain.Shipment, error) {
	return nil, nil
}
func (s *assignShipmentStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Shipment, error) {
	if s.getByIDAndTenantFn != nil {
		return s.getByIDAndTenantFn(ctx, id, tenantID)
	}
	return nil, nil
}
func (s *assignShipmentStore) List(context.Context, domain.ListShipmentsFilter) ([]domain.Shipment, int, error) {
	return nil, 0, nil
}
func (s *assignShipmentStore) AssignDriver(ctx context.Context, id, tenantID, driverID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error) {
	if s.assignDriverFn != nil {
		return s.assignDriverFn(ctx, id, tenantID, driverID, newStatus, expectedVersion)
	}
	return nil, nil
}
func (s *assignShipmentStore) AssignVehicle(ctx context.Context, id, tenantID, vehicleID uuid.UUID, newStatus string, expectedVersion int) (*domain.Shipment, error) {
	if s.assignVehicleFn != nil {
		return s.assignVehicleFn(ctx, id, tenantID, vehicleID, newStatus, expectedVersion)
	}
	return nil, nil
}
func (s *assignShipmentStore) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, *time.Time, *time.Time, int) (*domain.Shipment, error) {
	return nil, nil
}
func (s *assignShipmentStore) Accept(context.Context, uuid.UUID, uuid.UUID, int) (*domain.Shipment, error) {
	return nil, nil
}
func (s *assignShipmentStore) Cancel(context.Context, uuid.UUID, uuid.UUID, int) (*domain.Shipment, error) {
	return nil, nil
}

type assignDriverLookup struct {
	getByIDAndTenantFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Driver, error)
}

func (m *assignDriverLookup) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Driver, error) {
	if m.getByIDAndTenantFn != nil {
		return m.getByIDAndTenantFn(ctx, id, tenantID)
	}
	return nil, nil
}

type assignVehicleLookup struct {
	getByIDAndTenantFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Vehicle, error)
}

func (m *assignVehicleLookup) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Vehicle, error) {
	if m.getByIDAndTenantFn != nil {
		return m.getByIDAndTenantFn(ctx, id, tenantID)
	}
	return nil, nil
}

func newAssignDriverTestRouter(t *testing.T, store *assignShipmentStore, drivers *assignDriverLookup) http.Handler {
	t.Helper()
	handler := NewShipmentHandler(service.NewShipmentService(store, drivers, &assignVehicleLookup{}))
	r := chi.NewRouter()
	r.Post("/v1/shipments/{id}/assign-driver", handler.AssignDriver)
	return r
}

func newAssignVehicleTestRouter(t *testing.T, store *assignShipmentStore, vehicles *assignVehicleLookup) http.Handler {
	t.Helper()
	handler := NewShipmentHandler(service.NewShipmentService(store, &assignDriverLookup{}, vehicles))
	r := chi.NewRouter()
	r.Post("/v1/shipments/{id}/assign-vehicle", handler.AssignVehicle)
	return r
}

func TestAssignDriverTrustedHeaderAndDriverIDReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	driverID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	carrierID := uuid.New()

	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			if tenant.String() != tenantID {
				t.Fatalf("unexpected tenant %s", tenant)
			}
			return &domain.Shipment{
				ID: id, TenantID: tenant, Status: domain.ShipmentStatusCarrierAssigned,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignDriverFn: func(_ context.Context, id, tenant, gotDriverID uuid.UUID, _ string, _ int) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: id, TenantID: tenant, Status: domain.ShipmentStatusAcceptedByCarrier, DriverID: &gotDriverID,
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}, nil
		},
	}, &assignDriverLookup{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{ID: id, TenantID: tenant, CarrierCompanyID: carrierID}, nil
		},
	})

	body, _ := json.Marshal(map[string]string{"driver_id": driverID})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+shipmentID+"/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverBodyTenantWithoutHeaderReturns401(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id": "22222222-2222-2222-2222-222222222222",
		"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	})
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called without trusted header")
			return nil, nil
		},
	}, &assignDriverLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverBodyTenantWithHeaderReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id": "22222222-2222-2222-2222-222222222222",
		"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	})
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called when body contains tenant_id")
			return nil, nil
		},
	}, &assignDriverLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverMissingHeaderSkipsService(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"})
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called without header")
			return nil, nil
		},
	}, &assignDriverLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"})
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called with invalid tenant header")
			return nil, nil
		},
	}, &assignDriverLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverInvalidDriverIDReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{"driver_id": "not-a-uuid"})
	router := newAssignDriverTestRouter(t, &assignShipmentStore{}, &assignDriverLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverForeignDriverReturns404(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				Status: domain.ShipmentStatusCarrierAssigned, CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
	}, &assignDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return nil, apperrors.NotFound("driver not found")
		},
	})
	body, _ := json.Marshal(map[string]string{"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	}, &assignDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			t.Fatal("driver lookup must not run for foreign shipment")
			return nil, nil
		},
	})
	body, _ := json.Marshal(map[string]string{"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignDriverInternalErrorReturns500(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	router := newAssignDriverTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	}, &assignDriverLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Driver, error) {
			return &domain.Driver{CarrierCompanyID: carrierID}, nil
		},
	})
	body, _ := json.Marshal(map[string]string{"driver_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-driver", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleTrustedHeaderAndVehicleIDReturns200(t *testing.T) {
	t.Parallel()
	tenantID := "11111111-1111-1111-1111-111111111111"
	shipmentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	vehicleID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	carrierID := uuid.New()

	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: id, TenantID: tenant, Status: domain.ShipmentStatusAcceptedByCarrier,
				CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
		assignVehicleFn: func(_ context.Context, id, tenant, gotVehicleID uuid.UUID, _ string, _ int) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: id, TenantID: tenant, Status: domain.ShipmentStatusVehicleAssigned, VehicleID: &gotVehicleID,
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}, nil
		},
	}, &assignVehicleLookup{
		getByIDAndTenantFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Vehicle, error) {
			return &domain.Vehicle{ID: id, TenantID: tenant, CarrierCompanyID: carrierID}, nil
		},
	})

	body, _ := json.Marshal(map[string]string{"vehicle_id": vehicleID})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/"+shipmentID+"/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleBodyTenantWithoutHeaderReturns401(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id":  "22222222-2222-2222-2222-222222222222",
		"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc",
	})
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called without trusted header")
			return nil, nil
		},
	}, &assignVehicleLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleBodyTenantWithHeaderReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{
		"tenant_id":  "22222222-2222-2222-2222-222222222222",
		"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc",
	})
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called when body contains tenant_id")
			return nil, nil
		},
	}, &assignVehicleLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleMissingHeaderSkipsService(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"})
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called without header")
			return nil, nil
		},
	}, &assignVehicleLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleInvalidTenantHeaderReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"})
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			t.Fatal("service must not be called with invalid tenant header")
			return nil, nil
		},
	}, &assignVehicleLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleInvalidVehicleIDReturns400(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]string{"vehicle_id": "not-a-uuid"})
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{}, &assignVehicleLookup{})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleForeignVehicleReturns404(t *testing.T) {
	t.Parallel()
	carrierID := uuid.New()
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				Status: domain.ShipmentStatusAcceptedByCarrier, CarrierCompanyID: &carrierID, Version: 1,
			}, nil
		},
	}, &assignVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			return nil, apperrors.NotFound("vehicle not found")
		},
	})
	body, _ := json.Marshal(map[string]string{"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
	}, &assignVehicleLookup{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Vehicle, error) {
			t.Fatal("vehicle lookup must not run for foreign shipment")
			return nil, nil
		},
	})
	body, _ := json.Marshal(map[string]string{"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssignVehicleInternalErrorReturns500(t *testing.T) {
	t.Parallel()
	router := newAssignVehicleTestRouter(t, &assignShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.Internal("db failure", errors.New("boom"))
		},
	}, &assignVehicleLookup{})
	body, _ := json.Marshal(map[string]string{"vehicle_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"})
	req := httptest.NewRequest(http.MethodPost, "/v1/shipments/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/assign-vehicle", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
