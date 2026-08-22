//go:build integration

package ledger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/shared-go/internalauth"
)

func TestFC_B_SEC_001_MissingInternalTokenReturns401(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+fix.OrderID.String(), nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Company-ID", fix.BuyerID.String())
	req.Header.Set("X-Actor-Kind", security.ActorKindBuyer)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFC_B_SEC_002_TenantMismatchOnSourceEventReturns400(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	rec := postSourceEventHTTP(t, env, fix, map[string]any{
		"event_id":           uuid.NewString(),
		"tenant_id":          fix.OtherTenantID.String(),
		"transport_order_id": fix.OrderID.String(),
		"buyer_company_id":   fix.BuyerID.String(),
		"carrier_company_id": fix.CarrierID.String(),
		"entry_kind":         domain.EntryKindAccrualCostSnapshot,
		"source_service":     domain.SourceServiceBillingRegister,
		"source_type":        domain.SourceTypeFreightSettlement,
		"source_id":          settlementSourceID().String(),
		"source_revision":    1,
		"currency_code":      "RUB",
		"tax_basis":          "EX_VAT",
		"amount_availability": "AVAILABLE",
		"amount":             "1000.00",
		"occurred_at":        "2026-08-22T10:00:00Z",
		"event_origin":       domain.EventOriginLiveOutbox,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFC_B_SEC_003_CarrierMaskingHidesAccrual(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingest(t, env, baseIngestInput(fix, ingestOpts{
		entryKind: domain.EntryKindAccrualCostSnapshot, sourceRevision: 1, amount: decimalAmount("1000.00"),
	}))
	rec := getCostSummaryHTTP(t, env, fix, fix.TenantID, fix.CarrierID, security.ActorKindCarrier)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["accrued_amount"] != nil {
		t.Fatalf("carrier view leaked accrual: %v", payload["accrued_amount"])
	}
	assertActorKindCarrier(t)
}

func TestFC_B_SEC_004_WrongTenantCostReadReturns404(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	rec := getCostSummaryHTTP(t, env, fix, fix.OtherTenantID, fix.BuyerID, security.ActorKindBuyer)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFC_B_SEC_InternalTokenRequiredForRebuild(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/freight-cost/transport-orders/"+fix.OrderID.String()+"/rebuild", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	_ = internalauth.HeaderName
}
