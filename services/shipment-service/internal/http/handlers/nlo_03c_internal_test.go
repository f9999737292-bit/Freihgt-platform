package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestNLO03C_006_DRIVER_CANNOT_SUPPLY_CARGO_ID(t *testing.T) {
	if _, ok := reflect.TypeOf(driverOperationalEventRequest{}).FieldByName("CargoID"); ok {
		t.Fatal("driver request body accepts cargo_id")
	}
}

type evidenceReaderStub struct {
	owner    uuid.UUID
	shipment domain.Shipment
	rows     []domain.ShipmentCargoEvidence
}

func (s evidenceReaderStub) ExecutionContext(_ context.Context, tenantID, shipmentID uuid.UUID) (domain.ShipmentExecutionContext, error) {
	if tenantID != s.owner || shipmentID != s.shipment.ID {
		return domain.ShipmentExecutionContext{}, apperrors.NotFound("shipment not found")
	}
	return domain.ExecutionContextFromShipment(s.shipment), nil
}

func (s evidenceReaderStub) OnboardCargo(_ context.Context, tenantID, shipmentID uuid.UUID) (domain.OnboardCargoView, error) {
	if tenantID != s.owner || shipmentID != s.shipment.ID {
		return domain.OnboardCargoView{}, apperrors.NotFound("shipment not found")
	}
	return domain.ResolveOnboardCargo(s.shipment, s.rows), nil
}

func evidenceRouter(stub evidenceReaderStub, token string) http.Handler {
	handler := NewExecutionEvidenceHandler(stub)
	auth := internalauth.Config{Token: token}
	r := chi.NewRouter()
	r.Route("/internal/v1/shipments", func(r chi.Router) {
		r.With(auth.Middleware).Get("/{shipmentId}/execution-context", handler.GetExecutionContext)
		r.With(auth.Middleware).Get("/{shipmentId}/onboard-cargo", handler.GetOnboardCargo)
	})
	return r
}

func TestNLO03C_InternalReads(t *testing.T) {
	tenant := uuid.New()
	foreign := uuid.New()
	cargo := uuid.New()
	vehicle := uuid.New()
	shipment := domain.Shipment{
		ID: uuid.New(), TenantID: tenant, Version: 6, Status: domain.ShipmentStatusLoaded,
		VehicleID: &vehicle, OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(), CargoID: &cargo,
	}
	occurred := time.Now().UTC().Truncate(time.Second)
	stub := evidenceReaderStub{owner: tenant, shipment: shipment, rows: []domain.ShipmentCargoEvidence{{
		ID: uuid.New(), ShipmentID: shipment.ID, CargoID: cargo, State: domain.CargoEvidenceConfirmedOnboard,
		StateVersion: 6, OccurredAt: occurred, Source: domain.CargoEvidenceSourceDriver, SourceEventType: "PICKUP_COMPLETED",
	}}}
	router := evidenceRouter(stub, "internal-token")
	execPath := "/internal/v1/shipments/" + shipment.ID.String() + "/execution-context"
	onboardPath := "/internal/v1/shipments/" + shipment.ID.String() + "/onboard-cargo"

	t.Run("NLO03C_024_INTERNAL_TOKEN_REQUIRED", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, execPath, nil)
		req.Header.Set("X-Tenant-ID", tenant.String())
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatal(rec.Code)
		}
	})
	t.Run("NLO03C_016_EXECUTION_CONTEXT_TENANT_SCOPED", func(t *testing.T) {
		body := callInternal(t, router, execPath, tenant)
		if body["shipment_status"] != domain.ShipmentStatusLoaded || body["shipment_version"].(float64) != 6 {
			t.Fatalf("%v", body)
		}
		if _, ok := body["optimizer"]; ok {
			t.Fatal("optimizer field")
		}
	})
	t.Run("NLO03C_017_FOREIGN_EXECUTION_CONTEXT_NOT_FOUND", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, execPath+"?tenant_id="+foreign.String(), nil)
		req.Header.Set("X-Internal-Service-Token", "internal-token")
		req.Header.Set("X-Tenant-ID", foreign.String())
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatal(rec.Code)
		}
	})
	t.Run("NLO03C_018_ONBOARD_READ_TENANT_SCOPED", func(t *testing.T) {
		body := callInternal(t, router, onboardPath, tenant)
		if body["resolution"] != domain.CargoEvidenceConfirmedOnboard {
			t.Fatalf("%v", body)
		}
	})
	t.Run("NLO03C_019_FOREIGN_ONBOARD_READ_NOT_FOUND", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, onboardPath, nil)
		req.Header.Set("X-Internal-Service-Token", "internal-token")
		req.Header.Set("X-Tenant-ID", foreign.String())
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatal(rec.Code)
		}
	})
	t.Run("NLO03C_021_NO_EVIDENCE_RETURNS_UNPROVEN_NOT_ONBOARD_FALSE", func(t *testing.T) {
		empty := stub
		empty.rows = nil
		body := callInternal(t, evidenceRouter(empty, "internal-token"), onboardPath, tenant)
		if body["resolution"] != domain.OnboardCargoUnproven {
			t.Fatalf("%v", body)
		}
		items, _ := body["items"].([]any)
		if len(items) != 0 {
			t.Fatal(items)
		}
	})
}

func callInternal(t *testing.T, router http.Handler, path string, tenant uuid.UUID) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Internal-Service-Token", "internal-token")
	req.Header.Set("X-Tenant-ID", tenant.String())
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}
