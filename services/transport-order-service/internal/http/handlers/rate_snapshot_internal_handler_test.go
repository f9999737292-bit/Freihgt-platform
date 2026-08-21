package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/shared-go/internalauth"
)

const testInternalToken = "test-internal-token"

type stubSnapshotReader struct {
	snapshot *domain.RateSnapshot
	err      error
}

func (s *stubSnapshotReader) GetRateSnapshotByTransportOrder(_ context.Context, _, _ uuid.UUID) (*domain.RateSnapshot, error) {
	return s.snapshot, s.err
}

func newInternalTestRouter(handler *RateSnapshotInternalHandler) http.Handler {
	r := chi.NewRouter()
	auth := internalauth.Config{Token: testInternalToken, Environment: "test"}
	r.Route("/internal/v1", func(r chi.Router) {
		r.Use(auth.Middleware)
		r.Get("/transport-orders/{transportOrderId}/rate-snapshot", handler.GetRateSnapshot)
	})
	return r
}

func TestTO_CR_001_MissingInternalTokenReturns401(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_002_MissingTenantIDReturns400(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestTO_CR_003_PricedSnapshotReturns200(t *testing.T) {
	tenantID := uuid.New()
	transportOrderID := uuid.New()
	snapshot := &domain.RateSnapshot{
		ID:               uuid.New(),
		TenantID:         tenantID,
		TransportOrderID: transportOrderID,
		BuyerCompanyID:   uuid.New(),
		CarrierCompanyID: uuid.New(),
		CurrencyCode:     "RUB",
		TotalAmount:      decimal.RequireFromString("150000.00"),
		PricingSource:    "CONTRACT_RATE",
	}
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{snapshot: snapshot})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+transportOrderID.String()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp rateSnapshotInternalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalAmount != "150000.00" {
		t.Fatalf("total_amount = %q", resp.TotalAmount)
	}
	if resp.PricingModelVersion != domain.PricingModelVersionSnapshotV1 {
		t.Fatalf("pricing_model_version = %q", resp.PricingModelVersion)
	}
}

func TestTO_CR_004_NotFoundReturns404(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{err: apperrors.NotFound("transport order not found")})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_006_UnpricedTransportOrderReturns409(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{
		err: apperrors.Conflict("transport order has no pricing snapshot", map[string]any{"field": "pricing_model_version"}),
	})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_005_InvalidTransportOrderIDReturns400(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/not-a-uuid/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_007_MissingSnapshotReturns409(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{
		err: apperrors.Conflict("pricing snapshot missing for priced transport order", map[string]any{"field": "rate_snapshot"}),
	})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_009_InvalidTenantIDReturns400(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_010_WrongInternalTokenReturns401(t *testing.T) {
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+uuid.NewString()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, "wrong-token")
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTO_CR_008_ZeroTotalSnapshotReturnsZeroString(t *testing.T) {
	tenantID := uuid.New()
	transportOrderID := uuid.New()
	snapshot := &domain.RateSnapshot{
		ID:               uuid.New(),
		TenantID:         tenantID,
		TransportOrderID: transportOrderID,
		BuyerCompanyID:   uuid.New(),
		CarrierCompanyID: uuid.New(),
		CurrencyCode:     "RUB",
		TotalAmount:      decimal.Zero,
		PricingSource:    "CONTRACT_RATE",
	}
	handler := NewRateSnapshotInternalHandler(&stubSnapshotReader{snapshot: snapshot})
	router := newInternalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/transport-orders/"+transportOrderID.String()+"/rate-snapshot", nil)
	req.Header.Set(internalauth.HeaderName, testInternalToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp rateSnapshotInternalResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.TotalAmount != "0.00" {
		t.Fatalf("total_amount = %q", resp.TotalAmount)
	}
}
