package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type predictionStub struct {
	tenant uuid.UUID
	id     uuid.UUID
}

func (s *predictionStub) PredictionInput(_ context.Context, tenantID, id uuid.UUID) (*domain.ShipmentPredictionInput, error) {
	if tenantID != s.tenant || id != s.id {
		return nil, apperrors.NotFound("shipment not found")
	}
	return &domain.ShipmentPredictionInput{ID: id, TenantID: tenantID, Status: "IN_TRANSIT", Version: 1, DestinationLocationID: uuid.New()}, nil
}

func TestBNO50PredictionInputRequiresServiceToken(t *testing.T) {
	tenant := uuid.New()
	id := uuid.New()
	stub := &predictionStub{tenant: tenant, id: id}
	auth := internalauth.Config{Token: "platform-token"}
	handler := NewPredictionInputHandler(stub)
	router := chi.NewRouter()
	router.With(auth.Middleware).Get("/internal/v1/shipments/{shipmentId}/prediction-input", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+id.String()+"/prediction-input", nil)
	req.Header.Set("X-Tenant-ID", tenant.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("BNO50 status=%d", rec.Code)
	}

	req.Header.Set("X-Internal-Service-Token", "platform-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("BNO51 status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+id.String()+"/prediction-input", nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-Internal-Service-Token", "platform-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("BNO51 foreign status=%d", rec.Code)
	}
}
