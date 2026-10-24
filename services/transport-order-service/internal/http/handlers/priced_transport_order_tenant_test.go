package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
)

type recordingPricedOrders struct {
	mu    sync.Mutex
	calls []domain.CreatePricedTransportOrderInput
	award []domain.CreateFromAwardScopeInput
	byKey map[string]uuid.UUID
}

func (s *recordingPricedOrders) CreatePricedTransportOrder(_ context.Context, in domain.CreatePricedTransportOrderInput) (*domain.PricedTransportOrderResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, in)
	if s.byKey == nil {
		s.byKey = map[string]uuid.UUID{}
	}
	orderID, ok := s.byKey[in.TenantID.String()+"|"+in.Actor.CompanyID.String()+"|"+in.IdempotencyKey]
	if !ok {
		orderID = uuid.New()
		s.byKey[in.TenantID.String()+"|"+in.Actor.CompanyID.String()+"|"+in.IdempotencyKey] = orderID
	}
	return &domain.PricedTransportOrderResult{
		Order: &domain.TransportOrder{
			ID:            orderID,
			TenantID:      in.TenantID,
			OrderNumber:   in.OrderNumber,
			TransportMode: in.TransportMode,
			Status:        domain.TransportOrderStatusDraft,
		},
		RateSnapshotID: uuid.New(),
	}, nil
}

func (s *recordingPricedOrders) CreateFromAwardScope(_ context.Context, in domain.CreateFromAwardScopeInput) (*domain.PricedTransportOrderResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.award = append(s.award, in)
	return &domain.PricedTransportOrderResult{
		Order: &domain.TransportOrder{
			ID:       uuid.New(),
			TenantID: in.TenantID,
			Status:   domain.TransportOrderStatusDraft,
		},
		RateSnapshotID: uuid.New(),
	}, nil
}

func newPricedTenantRouter(store *recordingPricedOrders) http.Handler {
	handler := &PricedTransportOrderHandler{priced: store}
	r := chi.NewRouter()
	r.Post("/v1/transport-orders", handler.CreatePricedTransportOrder)
	r.Post("/internal/v1/transport-orders/from-award-scope", handler.CreateFromAwardScope)
	return r
}

func pricedCreateBody(tenantID string) map[string]any {
	body := map[string]any{
		"order_number":            "TO-E2E-007",
		"shipper_company_id":      uuid.NewString(),
		"consignee_company_id":    uuid.NewString(),
		"origin_location_id":      uuid.NewString(),
		"destination_location_id": uuid.NewString(),
		"cargo_id":                uuid.NewString(),
		"transport_mode":          "ROAD",
		"equipment_type":          "TILT",
		"pricing_context": map[string]any{
			"carrier_company_id": uuid.NewString(),
			"pricing_source":     "CONTRACT",
		},
	}
	if tenantID != "" {
		body["tenant_id"] = tenantID
	}
	return body
}

func postPricedCreate(t *testing.T, router http.Handler, tenantHeader, bodyTenant, companyHeader, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(pricedCreateBody(bodyTenant))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if companyHeader == "" {
		companyHeader = uuid.NewString()
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/transport-orders", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Company-ID", companyHeader)
	req.Header.Set("X-Actor-Kind", "BUYER")
	if tenantHeader != "" {
		req.Header.Set("X-Tenant-ID", tenantHeader)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPublicCreateMatchingTenantUsesVerifiedTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.NewString()
	store := &recordingPricedOrders{}
	rec := postPricedCreate(t, newPricedTenantRouter(store), tenantID, tenantID, "", "idem-match")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.calls) != 1 || store.calls[0].TenantID.String() != tenantID || store.calls[0].Actor.TenantID.String() != tenantID {
		t.Fatalf("create tenant=%+v", store.calls)
	}
}

func TestPublicCreateMismatchedBodyTenantDenied(t *testing.T) {
	t.Parallel()
	store := &recordingPricedOrders{}
	rec := postPricedCreate(t, newPricedTenantRouter(store), uuid.NewString(), uuid.NewString(), "", "idem-mismatch")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.calls) != 0 {
		t.Fatal("mismatched body tenant reached order create")
	}
}

func TestPublicCreateMissingVerifiedTenantFailClosed(t *testing.T) {
	t.Parallel()
	store := &recordingPricedOrders{}
	rec := postPricedCreate(t, newPricedTenantRouter(store), "", uuid.NewString(), "", "idem-missing")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.calls) != 0 {
		t.Fatal("missing verified tenant reached order create")
	}
}

func TestPublicCreateInvalidVerifiedTenantFailClosed(t *testing.T) {
	t.Parallel()
	store := &recordingPricedOrders{}
	rec := postPricedCreate(t, newPricedTenantRouter(store), "not-a-uuid", uuid.NewString(), "", "idem-invalid")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.calls) != 0 {
		t.Fatal("invalid verified tenant reached order create")
	}
}

func TestPublicCreateRepeatedIdempotencyKeyReturnsOneOrder(t *testing.T) {
	t.Parallel()
	tenantID := uuid.NewString()
	companyID := uuid.NewString()
	userID := uuid.NewString()
	store := &recordingPricedOrders{}
	router := newPricedTenantRouter(store)
	raw, err := json.Marshal(pricedCreateBody(tenantID))
	if err != nil {
		t.Fatal(err)
	}
	post := func() *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/transport-orders", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "idem-once")
		req.Header.Set("X-Tenant-ID", tenantID)
		req.Header.Set("X-User-ID", userID)
		req.Header.Set("X-Company-ID", companyID)
		req.Header.Set("X-Actor-Kind", "BUYER")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	first := post()
	second := post()
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("statuses=%d/%d", first.Code, second.Code)
	}
	var firstBody, secondBody map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatal(err)
	}
	if firstBody["id"] == "" || firstBody["id"] != secondBody["id"] {
		t.Fatalf("order ids=%v/%v", firstBody["id"], secondBody["id"])
	}
	if len(store.byKey) != 1 {
		t.Fatalf("stored orders=%d", len(store.byKey))
	}
}

func TestAwardScopeCreateStillUsesVerifiedHeaderTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	store := &recordingPricedOrders{}
	body := map[string]any{
		"rfx_event_id":            uuid.NewString(),
		"order_number":            "TO-AWD",
		"shipper_company_id":      uuid.NewString(),
		"consignee_company_id":    uuid.NewString(),
		"origin_location_id":      uuid.NewString(),
		"destination_location_id": uuid.NewString(),
		"cargo_id":                uuid.NewString(),
		"transport_mode":          "ROAD",
		"equipment_type":          "TILT",
		"carrier_company_id":      uuid.NewString(),
		"actor_user_id":           uuid.NewString(),
		"actor_company_id":        uuid.NewString(),
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/transport-orders/from-award-scope", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "award-conv-test")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	newPricedTenantRouter(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.award) != 1 || store.award[0].TenantID != tenantID {
		t.Fatalf("award tenant=%+v", store.award)
	}
}
