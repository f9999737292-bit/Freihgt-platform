//go:build integration

package erp

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestE7P2INT163AnalysisStatusPreviewed(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT163")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT163")
	rec := getERP(t, router, "/v1/integrations/erp/analyses/"+analysisID.String(), tenantID, companyID, principal.ID, statusReadScopes())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeERPJSON(t, rec.Result(), rec.Body.Bytes())
	if out["status"] != domain.ImportAnalysisStatusPreviewed {
		t.Fatalf("status=%v", out["status"])
	}
	if out["expires_at"] == nil || out["expires_at"] == "" {
		t.Fatal("expires_at required")
	}
	if out["ready_to_commit"] != true {
		t.Fatalf("ready_to_commit=%v", out["ready_to_commit"])
	}
	if _, ok := out["actor_id"]; ok {
		t.Fatal("analysis status must not leak actor_id")
	}
}

func TestE7P2INT164AnalysisStatusConsumed(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT164")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT164")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-164")
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit status=%d body=%s", rec.Code, rec.Body.String())
	}
	statusRec := getERP(t, router, "/v1/integrations/erp/analyses/"+analysisID.String(), tenantID, companyID, principal.ID, statusReadScopes())
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", statusRec.Code, statusRec.Body.String())
	}
	out := decodeERPJSON(t, statusRec.Result(), statusRec.Body.Bytes())
	if out["status"] != domain.ImportAnalysisStatusConsumed {
		t.Fatalf("status=%v", out["status"])
	}
	if out["consumed_at"] == nil || out["consumed_at"] == "" {
		t.Fatal("consumed_at required")
	}
	if out["ready_to_commit"] != false {
		t.Fatalf("consumed analysis ready_to_commit=%v", out["ready_to_commit"])
	}
}

func TestE6AnalysisStatusForeignPrincipal404(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principalA, router := seedCreateCommitReady(t, env, "E6ANL")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principalA.ID, "E6ANL")
	principalB := seedIntegrationPrincipal(t, env, tenantID, companyID, "e6-anl-b")
	rec := getERP(t, router, "/v1/integrations/erp/analyses/"+analysisID.String(), tenantID, companyID, principalB.ID, statusReadScopes())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE6AnalysisStatusUnknown404(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "E6ANL-MISS")
	rec := getERP(t, router, "/v1/integrations/erp/analyses/"+uuid.NewString(), tenantID, companyID, principal.ID, statusReadScopes())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE6AnalysisStatusFeatureFlagDisabled404(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, enabled := seedCreateCommitReady(t, env, "E6ANL-FLAG")
	analysisID := previewCreateAnalysis(t, enabled, tenantID, companyID, principal.ID, "E6ANL-FLAG")
	disabled := newERPPreviewRouter(t, env, config.Config{RfxErpIntegrationEnabled: false})
	rec := getERP(t, disabled, "/v1/integrations/erp/analyses/"+analysisID.String(), tenantID, companyID, principal.ID, statusReadScopes())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
