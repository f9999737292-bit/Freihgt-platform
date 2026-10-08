package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const analyticsSourcePath = "/internal/v1/analytics/operations-foundation"

type analyticsSourceStub struct {
	tenant uuid.UUID
	snap   domain.OperationsAnalyticsSourceSnapshot
}

func (s *analyticsSourceStub) OperationsFoundation(_ context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error) {
	s.tenant = tenantID
	if s.snap.TenantID == uuid.Nil {
		s.snap.TenantID = tenantID
	}
	return s.snap, nil
}

func analyticsSourceRouter(stub *analyticsSourceStub, token string) http.Handler {
	handler := NewOperationsAnalyticsSourceHandler(stub)
	auth := internalauth.Config{Token: token}
	r := chi.NewRouter()
	r.With(auth.Middleware, RequireAnalyticsCaller).Get(analyticsSourcePath, handler.Get)
	r.With(auth.Middleware, RequireAnalyticsCaller).Get(OperationsFoundationV2Path, handler.GetExtended)
	return r
}

func TestOperationsAnalyticsSourceAuth(t *testing.T) {
	tenant := uuid.New()
	foreign := uuid.New()
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{ShipmentTotal: 2, OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 1}}
	router := analyticsSourceRouter(stub, "analytics-token")

	t.Run("missing token", func(t *testing.T) {
		rec := analyticsSourceCall(router, "", "", tenant.String(), "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("wrong token", func(t *testing.T) {
		rec := analyticsSourceCall(router, "other-token", AuthorizedAnalyticsCaller, tenant.String(), "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("missing caller", func(t *testing.T) {
		rec := analyticsSourceCall(router, "analytics-token", "", tenant.String(), "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("wrong caller", func(t *testing.T) {
		rec := analyticsSourceCall(router, "analytics-token", "network-optimizer-service", tenant.String(), "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("missing tenant", func(t *testing.T) {
		rec := analyticsSourceCall(router, "analytics-token", AuthorizedAnalyticsCaller, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid tenant", func(t *testing.T) {
		rec := analyticsSourceCall(router, "analytics-token", AuthorizedAnalyticsCaller, "not-a-uuid", "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("authorized", func(t *testing.T) {
		rec := analyticsSourceCall(router, "analytics-token", AuthorizedAnalyticsCaller, tenant.String(), foreign.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["tenantId"] != tenant.String() || body["tenantId"] == foreign.String() {
			t.Fatalf("%v", body)
		}
		if stub.tenant != tenant {
			t.Fatalf("reader tenant %s", stub.tenant)
		}
		for _, key := range []string{"sourceObservedAt", "OPS_SHIPMENTS_TOTAL", "OPS_ON_TIME_DELIVERY", "definitionVersion", "kpiId", "onTimePickupDenominator", "onTimePickupNumerator", "carriers", "latePickupCount", "lateDeliveryCount"} {
			if _, ok := body[key]; ok {
				t.Fatalf("unexpected %s", key)
			}
		}
		if _, ok := body["shipmentTotal"]; !ok {
			t.Fatalf("%v", body)
		}
	})
	t.Run("public route absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/analytics/operations-foundation", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func TestOperationsAnalyticsSourceEmptyTenantIsOK(t *testing.T) {
	tenant := uuid.New()
	stub := &analyticsSourceStub{}
	router := analyticsSourceRouter(stub, "analytics-token")
	rec := analyticsSourceCall(router, "analytics-token", AuthorizedAnalyticsCaller, tenant.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var snap domain.OperationsAnalyticsSourceSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.TenantID != tenant || snap.ShipmentTotal != 0 || snap.OnTimeDeliveryDenominator != 0 || snap.OnTimeDeliveryNumerator != 0 || snap.ReturnCaseCount != 0 || snap.RedirectCaseCount != 0 {
		t.Fatalf("%+v", snap)
	}
}

func TestRequireAnalyticsCallerDoesNotReplaceTokenFailure(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler ran")
	})
	auth := internalauth.Config{Token: "analytics-token"}
	router := chi.NewRouter()
	router.With(auth.Middleware, RequireAnalyticsCaller).Get(analyticsSourcePath, handler)
	rec := analyticsSourceCall(router, "", AuthorizedAnalyticsCaller, uuid.NewString(), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), string(apperrors.CodeUnauthorized)) && !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
		t.Fatalf("%s", rec.Body.String())
	}
}

func TestOperationsAnalyticsSourceV1StaysCompatible(t *testing.T) {
	tenant := uuid.New()
	carrier := uuid.New()
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 3, OnTimePickupDenominator: 2, OnTimePickupNumerator: 1,
		OnTimeDeliveryDenominator: 2, OnTimeDeliveryNumerator: 1, ReturnCaseCount: 1, RedirectCaseCount: 0,
		Carriers: []domain.OperationsAnalyticsCarrierSource{{
			CarrierCompanyID: carrier, OnTimePickupDenominator: 2, OnTimePickupNumerator: 1,
			OnTimeDeliveryDenominator: 2, OnTimeDeliveryNumerator: 1,
		}},
	}}
	router := analyticsSourceRouter(stub, "analytics-token")
	rec := analyticsSourceCall(router, "analytics-token", AuthorizedAnalyticsCaller, tenant.String(), uuid.NewString())
	decoder := json.NewDecoder(strings.NewReader(rec.Body.String()))
	decoder.DisallowUnknownFields()
	var payload struct {
		TenantID                  string `json:"tenantId"`
		ShipmentTotal             int64  `json:"shipmentTotal"`
		OnTimeDeliveryDenominator int64  `json:"onTimeDeliveryDenominator"`
		OnTimeDeliveryNumerator   int64  `json:"onTimeDeliveryNumerator"`
		ReturnCaseCount           int64  `json:"returnCaseCount"`
		RedirectCaseCount         int64  `json:"redirectCaseCount"`
	}
	if err := decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.TenantID != tenant.String() || payload.ShipmentTotal != 3 || payload.OnTimeDeliveryDenominator != 2 || payload.OnTimeDeliveryNumerator != 1 || payload.ReturnCaseCount != 1 {
		t.Fatalf("%+v", payload)
	}
}

func TestOperationsAnalyticsSourceV2AuthAndFacts(t *testing.T) {
	tenant := uuid.New()
	foreign := uuid.New()
	carrier := uuid.New()
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 2, OnTimePickupDenominator: 2, OnTimePickupNumerator: 1,
		OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 1,
		Carriers: []domain.OperationsAnalyticsCarrierSource{{
			CarrierCompanyID: carrier, OnTimePickupDenominator: 2, OnTimePickupNumerator: 1,
			OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 1,
		}},
	}}
	router := analyticsSourceRouter(stub, "analytics-token")
	call := func(token, caller, headerTenant, queryTenant string) *httptest.ResponseRecorder {
		path := OperationsFoundationV2Path
		if queryTenant != "" {
			path += "?tenant_id=" + queryTenant
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set("X-Internal-Service-Token", token)
		}
		if caller != "" {
			req.Header.Set("X-Internal-Service-Name", caller)
		}
		if headerTenant != "" {
			req.Header.Set("X-Tenant-ID", headerTenant)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := call("", AuthorizedAnalyticsCaller, tenant.String(), ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token %d", rec.Code)
	}
	if rec := call("analytics-token", "network-optimizer-service", tenant.String(), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong caller %d", rec.Code)
	}
	rec := call("analytics-token", AuthorizedAnalyticsCaller, tenant.String(), foreign.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var snap domain.OperationsAnalyticsSourceSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.TenantID != tenant || stub.tenant != tenant || snap.OnTimePickupDenominator != 2 || snap.OnTimePickupNumerator != 1 || len(snap.Carriers) != 1 || snap.Carriers[0].CarrierCompanyID != carrier {
		t.Fatalf("%+v reader %s", snap, stub.tenant)
	}
	for _, key := range []string{"latePickupCount", "lateDeliveryCount", "sourceObservedAt"} {
		if strings.Contains(rec.Body.String(), key) {
			t.Fatalf("body %s", rec.Body.String())
		}
	}
}

func analyticsSourceCall(router http.Handler, token, caller, tenant, queryTenant string) *httptest.ResponseRecorder {
	path := analyticsSourcePath
	if queryTenant != "" {
		path += "?tenant_id=" + queryTenant
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	if caller != "" {
		req.Header.Set("X-Internal-Service-Name", caller)
	}
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
