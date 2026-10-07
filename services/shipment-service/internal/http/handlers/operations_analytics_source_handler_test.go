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
		for _, key := range []string{"sourceObservedAt", "OPS_SHIPMENTS_TOTAL", "OPS_ON_TIME_DELIVERY", "definitionVersion", "kpiId"} {
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
