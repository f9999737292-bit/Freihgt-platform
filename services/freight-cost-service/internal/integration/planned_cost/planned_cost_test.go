package planned_cost

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/client/transport_order"
	"github.com/freight-platform/freight-cost-service/internal/config"
	httpserver "github.com/freight-platform/freight-cost-service/internal/http"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

const testToken = "test-token"

var newTestRouterMu sync.Mutex

func TestFC_A_E2E_001_BuyerSameCompanyPlannedReadSuccess(t *testing.T) {

	tenantID := uuid.New()
	buyerCompanyID := uuid.New()
	carrierCompanyID := uuid.New()
	transportOrderID := uuid.New()

	router := newTestRouter(t, tenantID, buyerCompanyID, carrierCompanyID, transportOrderID, "150000.00")
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+transportOrderID.String(), nil)
	setActorHeaders(req, tenantID, buyerCompanyID, security.ActorKindBuyer)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	payload := decodeBody(t, rec)
	if payload["planned_amount"] != "150000.00" {
		t.Fatalf("planned_amount = %v", payload["planned_amount"])
	}
	if payload["data_stage"] != "PLANNED_ONLY" {
		t.Fatalf("data_stage = %v", payload["data_stage"])
	}
	if payload["financial_finality"] != "NOT_EVALUATED" {
		t.Fatalf("financial_finality = %v", payload["financial_finality"])
	}
}

func TestFC_A_E2E_002_CarrierSameCompanyPlannedReadSuccess(t *testing.T) {

	tenantID := uuid.New()
	buyerCompanyID := uuid.New()
	carrierCompanyID := uuid.New()
	transportOrderID := uuid.New()

	router := newTestRouter(t, tenantID, buyerCompanyID, carrierCompanyID, transportOrderID, "150000.00")
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+transportOrderID.String(), nil)
	setActorHeaders(req, tenantID, carrierCompanyID, security.ActorKindCarrier)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	payload := decodeBody(t, rec)
	if payload["planned_amount"] != "150000.00" {
		t.Fatalf("planned_amount = %v", payload["planned_amount"])
	}
	if payload["accrued_amount"] != nil {
		t.Fatalf("accrued_amount = %v", payload["accrued_amount"])
	}
}

func TestFC_A_E2E_003_WrongTenantIndistinguishableFromNotFound(t *testing.T) {

	tenantID := uuid.New()
	otherTenant := uuid.New()
	buyerCompanyID := uuid.New()
	carrierCompanyID := uuid.New()
	transportOrderID := uuid.New()

	router := newTestRouter(t, tenantID, buyerCompanyID, carrierCompanyID, transportOrderID, "150000.00")
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+transportOrderID.String(), nil)
	setActorHeaders(req, otherTenant, buyerCompanyID, security.ActorKindBuyer)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestFC_A_SEC_004_MissingInternalTokenReturns401(t *testing.T) {

	tenantID := uuid.New()
	buyerCompanyID := uuid.New()
	carrierCompanyID := uuid.New()
	transportOrderID := uuid.New()

	router := newTestRouter(t, tenantID, buyerCompanyID, carrierCompanyID, transportOrderID, "150000.00")
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+transportOrderID.String(), nil)
	setActorHeaders(req, tenantID, buyerCompanyID, security.ActorKindBuyer)
	req.Header.Del(internalauth.HeaderName)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestFC_A_SEC_005_MissingActorHeadersReturns400(t *testing.T) {

	tenantID := uuid.New()
	buyerCompanyID := uuid.New()
	carrierCompanyID := uuid.New()
	transportOrderID := uuid.New()

	router := newTestRouter(t, tenantID, buyerCompanyID, carrierCompanyID, transportOrderID, "150000.00")
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+transportOrderID.String(), nil)
	req.Header.Set(internalauth.HeaderName, testToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func newTestRouter(
	t *testing.T,
	tenantID, buyerCompanyID, carrierCompanyID, transportOrderID uuid.UUID,
	totalAmount string,
) http.Handler {
	t.Helper()
	newTestRouterMu.Lock()
	defer newTestRouterMu.Unlock()

	mockTransport := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerTenant, err := uuid.Parse(r.Header.Get("X-Tenant-ID"))
		if err != nil || headerTenant != tenantID {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transport_order_id":    transportOrderID.String(),
			"tenant_id":             tenantID.String(),
			"buyer_company_id":      buyerCompanyID.String(),
			"carrier_company_id":    carrierCompanyID.String(),
			"snapshot_id":           uuid.NewString(),
			"currency_code":         "RUB",
			"total_amount":          totalAmount,
			"pricing_source":        "CONTRACT_RATE",
			"pricing_model_version": "SNAPSHOT_V1",
			"resolved_at":           "2026-08-21T12:00:00Z",
		})
	}))
	t.Cleanup(mockTransport.Close)

	cfg := config.Config{
		ServiceName:          "freight-cost-service",
		Environment:          "test",
		InternalServiceToken: testToken,
		TransportOrderURL:    mockTransport.URL,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	transportClient := transport_order.NewClient(cfg.TransportOrderURL, cfg.InternalServiceToken, fcmetrics.New())
	costSvc := service.NewCostService(transportClient, nil)
	workspaceSvc := service.NewWorkspaceService(nil, nil, costSvc, transportClient)
	return httpserver.NewRouter(log, nil, cfg, costSvc, nil, nil, nil, workspaceSvc, nil, nil, nil, nil, nil, fcmetrics.New())
}

func setActorHeaders(req *http.Request, tenantID, companyID uuid.UUID, actorKind string) {
	userID := uuid.New()
	req.Header.Set(internalauth.HeaderName, testToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("X-Company-ID", companyID.String())
	req.Header.Set("X-Actor-Kind", actorKind)
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return payload
}
