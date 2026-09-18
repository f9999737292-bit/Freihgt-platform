//go:build integration

package erp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
)

func validCreatePayload() string {
	return `{
		"schema_version":"BINTRANS_RFX_ERP_JSON_V1",
		"requested_operation":"CREATE_DRAFT",
		"external":{"system":"SAP","object_id":"OBJ-135"},
		"event":{"type":"SPOT_RFQ","title":"ERP Preview","currency":"USD_EXT","timezone":"UTC_EXT"}
	}`
}

func TestE7P2INT133MissingPreviewScope(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int133-client")
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, []string{domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT135CreatePreviewValid(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int135-client")
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCreate)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	before := countERPAnalyses(t, env, tenantID)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ready_to_commit"] != true {
		t.Fatalf("ready_to_commit=%v", out["ready_to_commit"])
	}
	if out["analysis_id"] == nil || out["analysis_id"] == "" {
		t.Fatal("expected analysis_id")
	}
	if countERPAnalyses(t, env, tenantID) != before+1 {
		t.Fatal("expected analysis persisted")
	}
}

func TestE7P2INT136CreatePreviewValidationZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int136-client")
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCreate)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	before := countERPAnalyses(t, env, tenantID)
	payload := strings.Replace(validCreatePayload(), "BINTRANS_RFX_ERP_JSON_V1", "UNKNOWN_SCHEMA", 1)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if countERPAnalyses(t, env, tenantID) != before {
		t.Fatal("expected zero analysis rows")
	}
}

func TestE7P2INT146FeatureFlagDisabled(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int146-client")
	router := newERPPreviewRouter(t, env, config.Config{RfxErpIntegrationEnabled: false})
	before := countERPAnalyses(t, env, tenantID)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if countERPAnalyses(t, env, tenantID) != before {
		t.Fatal("expected no writes")
	}
}

func TestE7P2INT143MassAssignmentUnknownField(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int143-client")
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCreate)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"unexpected_field":true}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT195DeferredLanesRejected(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int195-client")
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCreate)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"lanes":[]}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
