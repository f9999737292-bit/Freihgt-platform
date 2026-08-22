//go:build integration

package variance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/shared-go/internalauth"
)

func TestFC_C_SEC_005_TenantAMappingInvisibleToTenantB(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	_, err := env.mappings.UpsertMapping(ctx, repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "TENANT_ONLY",
		TargetCategory: "FUEL",
		EffectiveFrom:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("upsert tenant mapping: %v", err)
	}
	_, tenantB, _, err := env.mappings.LoadActiveMappings(ctx, nil, fix.OtherTenantID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load tenant B: %v", err)
	}
	for _, m := range tenantB {
		if m.SourceChargeCodeNormalized == "TENANT_ONLY" {
			t.Fatal("tenant A mapping must be invisible to tenant B")
		}
	}
}

func TestFC_C_SEC_006_CrossTenantProjectionDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	rec := getCostSummaryHTTP(t, env, fix, fix.OtherTenantID, fix.BuyerID, "BUYER")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("wrong tenant must return 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFC_C_SEC_007_InternalTokenRequiredForRebuild(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/freight-cost/transport-orders/"+fix.OrderID.String()+"/rebuild", nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token must return 401, got %d", rec.Code)
	}
	req.Header.Set(internalauth.HeaderName, testToken)
	rec = httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid token expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFC_C_SEC_008_CarrierCostReadMasksVariance(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	rec := getCostSummaryHTTP(t, env, fix, fix.TenantID, fix.CarrierID, "CARRIER")
	if rec.Code != http.StatusOK {
		t.Fatalf("carrier read status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSONBody(t, rec)
	if body["current_variance_amount"] != nil {
		t.Fatalf("carrier must not see current_variance_amount: %v", body["current_variance_amount"])
	}
	if body["forecast_exposure"] != nil {
		t.Fatalf("carrier must not see forecast_exposure: %v", body["forecast_exposure"])
	}
	_ = uuid.New()
}
