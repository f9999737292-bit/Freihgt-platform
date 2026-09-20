//go:build integration

package erp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
)

func commitCreatedEvent(t *testing.T, router http.Handler, tenantID, companyID, principalID uuid.UUID, objectID string) uuid.UUID {
	t.Helper()
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principalID, objectID)
	rec := postERPCreateCommit(t, router, tenantID, companyID, principalID, commitScopes(), analysisID, "e6-"+objectID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	return uuid.MustParse(out["rfx_event_id"].(string))
}

func decodeERPJSON(t *testing.T, rec *http.Response, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode json: %v body=%s", err, string(body))
	}
	return out
}

func externalLookupPath(system, objectID, revision string) string {
	q := url.Values{}
	q.Set("external_system", system)
	q.Set("external_object_id", objectID)
	if revision != "" {
		q.Set("external_revision", revision)
	}
	return "/v1/integrations/erp/rfx/by-external-id?" + q.Encode()
}

func assertErpRfxDraftSummaryAllowlist(t *testing.T, out map[string]any) {
	t.Helper()
	required := []string{
		"rfx_event_id", "status", "rfx_type", "title", "timezone", "creation_channel",
		"publish_readiness_summary", "lot_count", "questionnaire_counts",
		"event_row_version", "draft_row_version",
	}
	for _, key := range required {
		if _, ok := out[key]; !ok {
			t.Fatalf("missing allowlisted field %s", key)
		}
	}
	if out["status"] != domain.RfxStatusDraft {
		t.Fatalf("status=%v", out["status"])
	}
	readiness, _ := out["publish_readiness_summary"].(map[string]any)
	if readiness == nil {
		t.Fatal("publish_readiness_summary must be an object")
	}
	if _, ok := readiness["items"]; ok {
		t.Fatal("publish_readiness_summary must not include items")
	}
	for _, key := range []string{"ready", "blocking_fail_count", "warning_count"} {
		if _, ok := readiness[key]; !ok {
			t.Fatalf("publish_readiness_summary missing %s", key)
		}
	}
	forbidden := []string{
		"participants", "carriers", "bids", "scores", "awards", "actor_id", "user_id",
		"password", "secret", "token", "invited_carriers", "responses",
	}
	for _, key := range forbidden {
		if _, ok := out[key]; ok {
			t.Fatalf("forbidden field %s leaked", key)
		}
	}
}

func TestE7P2INT160GetByStableExternalID(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT160")
	eventID := commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "INT160")
	rec := getERP(t, router, externalLookupPath("sap", "INT160", ""), tenantID, companyID, principal.ID, readScopes())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeERPJSON(t, rec.Result(), rec.Body.Bytes())
	if out["rfx_event_id"] != eventID.String() {
		t.Fatalf("rfx_event_id=%v want %s", out["rfx_event_id"], eventID)
	}
	link, _ := out["external_link"].(map[string]any)
	if link["system"] != "SAP" || link["object_id"] != "INT160" || link["revision"] != "1" {
		t.Fatalf("external_link=%v", link)
	}
}

func TestE7P2INT161GetUnknownExternal404(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT161")
	_ = commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "INT161")
	rec := getERP(t, router, externalLookupPath("SAP", "MISSING-161", ""), tenantID, companyID, principal.ID, readScopes())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT162GetInternalDraftSummary(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT162")
	eventID := commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "INT162")
	rec := getERP(t, router, "/v1/integrations/erp/rfx-events/"+eventID.String(), tenantID, companyID, principal.ID, readScopes())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeERPJSON(t, rec.Result(), rec.Body.Bytes())
	assertErpRfxDraftSummaryAllowlist(t, out)
	if out["creation_channel"] != domain.CreationChannelERP {
		t.Fatalf("creation_channel=%v", out["creation_channel"])
	}
	if out["timezone"] != "UTC" {
		t.Fatalf("timezone=%v", out["timezone"])
	}
	if out["draft_row_version"] == nil {
		t.Fatal("draft_row_version required")
	}
}

func TestE7P2INT188CompetitorDataAbsent(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT188")
	eventID := commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "INT188")
	rec := getERP(t, router, "/v1/integrations/erp/rfx-events/"+eventID.String(), tenantID, companyID, principal.ID, readScopes())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertErpRfxDraftSummaryAllowlist(t, decodeERPJSON(t, rec.Result(), rec.Body.Bytes()))
}

func TestE7P2INT192GetWithoutRevisionDeterministic(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT192")
	eventID := commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "INT192")
	rec1 := getERP(t, router, externalLookupPath("SAP", "INT192", ""), tenantID, companyID, principal.ID, readScopes())
	rec2 := getERP(t, router, externalLookupPath("SAP", "INT192", ""), tenantID, companyID, principal.ID, readScopes())
	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("status=%d/%d", rec1.Code, rec2.Code)
	}
	out1 := decodeERPJSON(t, rec1.Result(), rec1.Body.Bytes())
	out2 := decodeERPJSON(t, rec2.Result(), rec2.Body.Bytes())
	if out1["rfx_event_id"] != eventID.String() || out2["rfx_event_id"] != eventID.String() {
		t.Fatalf("lookup must return the same event, got %v / %v", out1["rfx_event_id"], out2["rfx_event_id"])
	}
	current := getERP(t, router, externalLookupPath("SAP", "INT192", "1"), tenantID, companyID, principal.ID, readScopes())
	if current.Code != http.StatusOK {
		t.Fatalf("current E4 revision without history must be 200, status=%d body=%s", current.Code, current.Body.String())
	}
	unknown := getERP(t, router, externalLookupPath("SAP", "INT192", "never-recorded"), tenantID, companyID, principal.ID, readScopes())
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown revision must be 404, status=%d body=%s", unknown.Code, unknown.Body.String())
	}
}

func TestE6PublishedEventReturns409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "E6PUB")
	eventID := commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "E6PUB")
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_events SET status = 'PUBLISHED' WHERE id = $1`, eventID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	byID := getERP(t, router, "/v1/integrations/erp/rfx-events/"+eventID.String(), tenantID, companyID, principal.ID, readScopes())
	if byID.Code != http.StatusConflict || errorMachineCode(byID.Body.Bytes()) != domain.MachineCodeEventNotDraft {
		t.Fatalf("GET by id published status=%d machine=%s body=%s", byID.Code, errorMachineCode(byID.Body.Bytes()), byID.Body.String())
	}
	byExt := getERP(t, router, externalLookupPath("SAP", "E6PUB", ""), tenantID, companyID, principal.ID, readScopes())
	if byExt.Code != http.StatusConflict || errorMachineCode(byExt.Body.Bytes()) != domain.MachineCodeEventNotDraft {
		t.Fatalf("GET by external published status=%d machine=%s body=%s", byExt.Code, errorMachineCode(byExt.Body.Bytes()), byExt.Body.String())
	}
}

func TestE6ReadFeatureFlagDisabled404(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, enabled := seedCreateCommitReady(t, env, "E6FLAG")
	eventID := commitCreatedEvent(t, enabled, tenantID, companyID, principal.ID, "E6FLAG")
	disabled := newERPPreviewRouter(t, env, config.Config{RfxErpIntegrationEnabled: false})
	paths := []string{
		"/v1/integrations/erp/rfx-events/" + eventID.String(),
		externalLookupPath("SAP", "E6FLAG", ""),
		"/v1/integrations/erp/capabilities",
	}
	for _, path := range paths {
		rec := getERP(t, disabled, path, tenantID, companyID, principal.ID, append(readScopes(), statusReadScopes()...))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestE6ReadMissingScope403(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "E6SCOPE")
	eventID := commitCreatedEvent(t, router, tenantID, companyID, principal.ID, "E6SCOPE")
	rec := getERP(t, router, "/v1/integrations/erp/rfx-events/"+eventID.String(), tenantID, companyID, principal.ID, statusReadScopes())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE6ReadCrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, companyA, principalA, router := seedCreateCommitReady(t, env, "E6XT")
	eventID := commitCreatedEvent(t, router, tenantA, companyA, principalA.ID, "E6XT")
	tenantB, companyB := seedTenantCompany(t, env)
	principalB := seedIntegrationPrincipal(t, env, tenantB, companyB, "e6-xt-b")
	rec := getERP(t, router, "/v1/integrations/erp/rfx-events/"+eventID.String(), tenantB, companyB, principalB.ID, readScopes())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
