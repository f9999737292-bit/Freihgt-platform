//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE6INT01PublishedVersionQuestionnaireGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-int-01", &fix.CompanyA)

	q, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, published.TemplateID, published.ID)
	if err != nil {
		t.Fatalf("get published version questionnaire: %v", err)
	}
	assertE6HistoricalGraphPG(t, q, expect)
}

func TestE6INT02SupersededVersionQuestionnaireGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, firstPublished, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-int-02", &fix.CompanyA)

	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, uuid.NewString())
	if err != nil {
		t.Fatalf("fork draft: %v", err)
	}
	detail, err = env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if _, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, "e6-int-02-pub2", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    draft.Version,
		ChangeSummary:           "Second publish",
	}); err != nil {
		t.Fatalf("publish second version: %v", err)
	}

	q, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, firstPublished.TemplateID, firstPublished.ID)
	if err != nil {
		t.Fatalf("get superseded version questionnaire: %v", err)
	}
	if q.VersionStatus != domain.RfxVersionStatusSuperseded {
		t.Fatalf("expected superseded version status, got %s", q.VersionStatus)
	}
	assertE6HistoricalGraphPG(t, q, expect)
}

func TestE6INT03VersionQuestionnaireCrossTenantNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-03", &fix.CompanyA)

	_, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.CrossTenant, published.TemplateID, published.ID)
	expectAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE6INT04VersionQuestionnaireCarrierForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-04", &fix.CompanyA)

	_, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.CarrierAct, published.TemplateID, published.ID)
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE6INT05EventProvenanceAfterClone(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-05", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-05", fix.CompanyA), uuid.NewString())

	prov, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, result.Event.ID)
	if err != nil {
		t.Fatalf("get event provenance: %v", err)
	}
	if prov == nil || prov.SourceTemplateVersionID != published.ID {
		t.Fatalf("expected provenance for clone source, got %+v", prov)
	}
}

func TestE6INT06ManualEventProvenanceNull(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFQ-E6-06")

	prov, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get event provenance: %v", err)
	}
	if prov != nil {
		t.Fatalf("manual event must not expose provenance, got %+v", prov)
	}
}

func TestE6INT07EventProvenanceHiddenFromCarrier(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-07", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-07", fix.CompanyA), uuid.NewString())

	prov, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.CarrierAct, result.Event.ID)
	if err == nil && prov != nil {
		t.Fatal("carrier must not receive template provenance")
	}
}

func TestE6INT08HTTPGetVersionQuestionnaire(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-int-08", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, published.TemplateID, published.ID)
	assertE6HistoricalGraphHTTP(t, rec, published, expect)
}

func TestE6INT09HTTPGetEventProvenanceFields(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-09", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-09", fix.CompanyA), uuid.NewString())

	rec := getEventHTTP(t, env, e6EnabledConfig(), fix, result.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, _ := payload["source_template_version_id"].(string); got != published.ID.String() {
		t.Fatalf("expected source_template_version_id=%s got %#v", published.ID, payload["source_template_version_id"])
	}
	if payload["source_version_number"] == nil {
		t.Fatal("expected source_version_number in event GET response")
	}
}

func TestE6INT10HTTPCrossTenantVersionQuestionnaireNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-10", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTPWithActor(t, env, e6EnabledConfig(), fix.CrossTenant, published.TemplateID, published.ID)
	expectHTTPErrorCode(t, rec, http.StatusNotFound, apperrors.CodeNotFound)
}

func TestE6INT11CrossTemplateVersionQuestionnaireNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detailA := createTemplate(t, env, fix, "e6-int-11-a", &fix.CompanyA)
	_, publishedB, _ := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-int-11-b", &fix.CompanyA)

	_, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, detailA.Template.ID, publishedB.ID)
	expectAppErrorCode(t, err, apperrors.CodeNotFound)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, detailA.Template.ID, publishedB.ID)
	expectHTTPErrorCode(t, rec, http.StatusNotFound, apperrors.CodeNotFound)
}

func TestE6INT12SupersededVersionQuestionnaireGraphHTTP(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, firstPublished, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-int-12", &fix.CompanyA)

	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e6-int-12-fork")
	if err != nil {
		t.Fatalf("fork draft: %v", err)
	}
	detail, err = env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if _, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, "e6-int-12-pub2", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    draft.Version,
		ChangeSummary:           "Second publish",
	}); err != nil {
		t.Fatalf("publish second version: %v", err)
	}
	superseded := reloadTemplateVersionByID(t, env, fix, detail.Template.ID, firstPublished.ID)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, superseded.TemplateID, superseded.ID)
	assertE6HistoricalGraphHTTP(t, rec, superseded, expect)
}

func TestE6INT13DraftVersionQuestionnaireGraphPG(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, expect := setupE6HistoricalTemplateDraft(t, env, fix, "e6-int-13", &fix.CompanyA)

	q, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, detail.Template.ID, detail.DraftVersion.ID)
	if err != nil {
		t.Fatalf("get draft version questionnaire: %v", err)
	}
	if q.VersionStatus != domain.RfxVersionStatusDraft {
		t.Fatalf("expected draft version status, got %s", q.VersionStatus)
	}
	assertE6HistoricalGraphPG(t, q, expect)
}

func TestE6INT14DraftVersionQuestionnaireGraphHTTP(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, expect := setupE6HistoricalTemplateDraft(t, env, fix, "e6-int-14", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, detail.Template.ID, detail.DraftVersion.ID)
	assertE6HistoricalGraphHTTP(t, rec, detail.DraftVersion, expect)
}

func TestE6INT15UnknownVersionQuestionnaireNotFoundPG(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "e6-int-15", &fix.CompanyA)

	_, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, detail.Template.ID, uuid.New())
	expectAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE6INT16UnknownVersionQuestionnaireNotFoundHTTP(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "e6-int-16", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, detail.Template.ID, uuid.New())
	expectHTTPErrorCode(t, rec, http.StatusNotFound, apperrors.CodeNotFound)
}

func TestE6INT17CompanyOwnedVersionQuestionnaireForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-17", &fix.CompanyA)

	_, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerB, published.TemplateID, published.ID)
	expectAppErrorCode(t, err, apperrors.CodeForbidden)

	rec := getTemplateVersionQuestionnaireHTTPWithActor(t, env, e6EnabledConfig(), fix.BuyerB, published.TemplateID, published.ID)
	expectHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
}

func TestE6INT18HTTPManualEventOmitsProvenanceFields(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFQ-E6-18")

	rec := getEventHTTP(t, env, e6EnabledConfig(), fix, event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{
		"source_template_id",
		"source_template_version_id",
		"source_version_number",
		"source_version_status",
		"source_template_code",
	} {
		if _, ok := payload[key]; ok {
			t.Fatalf("manual event must not expose provenance field %s", key)
		}
	}
}

func TestE6INT19HTTPCrossTenantEventNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFQ-E6-19")

	rec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.CrossTenant, event.ID)
	expectHTTPErrorCode(t, rec, http.StatusNotFound, apperrors.CodeNotFound)
}

func TestE6INT20SupersededCloneProvenanceWarningPG(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, v1 := setupPublishedTemplate(t, env, fix, "e6-int-20", &fix.CompanyA)
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e6-int-20-fork"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	publishTemplate(t, env, fix, detail.Template.ID, "e6-int-20-pub2")
	result := cloneFromTemplate(t, env, fix.BuyerA, v1.ID, defaultCloneEventInput("RFQ-E6-20", fix.CompanyA), uuid.NewString())

	prov, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, result.Event.ID)
	if err != nil {
		t.Fatalf("get event provenance: %v", err)
	}
	if prov == nil || !prov.SourceVersionWarning || prov.SourceVersionStatus != domain.RfxVersionStatusSuperseded {
		t.Fatalf("expected superseded provenance warning, got %+v", prov)
	}
}

func TestE6INT21HTTPCarrierVersionQuestionnaireForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-21", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTPWithActor(t, env, e6EnabledConfig(), fix.CarrierAct, published.TemplateID, published.ID)
	expectHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
}

func TestE6INT22HTTPBuyerNonOwnerEventNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFQ-E6-22")

	rec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.BuyerB, event.ID)
	expectHTTPErrorCode(t, rec, http.StatusNotFound, apperrors.CodeNotFound)
}

func TestE6INT23HTTPFeatureFlagDisabledVersionQuestionnaireNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-23", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, config.Config{RfxVersioningV3Enabled: false}, fix, published.TemplateID, published.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE6INT24OpenAPIEventProvenanceAndVersionQuestionnaireParity(t *testing.T) {
	root := repoRoot(t)
	openapiPath := filepath.Join(root, "packages", "openapi", "rfx-service.yaml")
	rfxRouterPath := filepath.Join(root, "services", "rfx-service", "internal", "http", "router.go")
	gatewayPath := filepath.Join(root, "services", "api-gateway", "internal", "http", "router.go")
	openapiBody, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	rfxBody, err := os.ReadFile(rfxRouterPath)
	if err != nil {
		t.Fatalf("read rfx router: %v", err)
	}
	gatewayBody, err := os.ReadFile(gatewayPath)
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}
	openapi := string(openapiBody)
	gateway := string(gatewayBody)
	rfxRouter := string(rfxBody)

	for _, fragment := range []string{
		"/api/v1/rfx-templates/{id}/versions/{version_id}/questionnaire",
		"/api/v1/rfx-events/{id}",
		"source_template_id",
		"source_template_version_id",
		"source_version_number",
		"source_version_status",
		"source_version_warning",
		"source_template_code",
		"RfxEventDetailResponse",
		"RfxTemplateQuestionnaireDefinition",
		"version_status",
	} {
		if !strings.Contains(openapi, fragment) {
			t.Fatalf("openapi missing fragment %s", fragment)
		}
	}
	for _, path := range []string{
		"/api/v1/rfx-templates/{id}/versions/{version_id}/questionnaire",
		"/api/v1/rfx-events/{id}",
	} {
		if !strings.Contains(gateway, path) {
			t.Fatalf("gateway missing path %s", path)
		}
	}
	for _, fragment := range []string{
		`Get("/{id}/versions/{version_id}/questionnaire"`,
		`Get("/{id}", rfxHandler.GetEvent)`,
	} {
		if !strings.Contains(rfxRouter, fragment) {
			t.Fatalf("rfx router missing fragment %s", fragment)
		}
	}
}

func TestE6INT25TenantWidePublishedVersionReadableByBuyerB(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-int-25", nil)

	q, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerB, published.TemplateID, published.ID)
	if err != nil {
		t.Fatalf("tenant-wide published graph for buyer B PG: %v", err)
	}
	assertE6HistoricalGraphPG(t, q, expect)

	rec := getTemplateVersionQuestionnaireHTTPWithActor(t, env, e6EnabledConfig(), fix.BuyerB, published.TemplateID, published.ID)
	assertE6HistoricalGraphHTTP(t, rec, published, expect)
}

func e6EnabledConfig() config.Config {
	return config.Config{RfxVersioningV3Enabled: true}
}

func getTemplateVersionQuestionnaireHTTP(t *testing.T, env *testEnv, cfg config.Config, fix buyerFixture, templateID, versionID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return getTemplateVersionQuestionnaireHTTPWithActor(t, env, cfg, fix.BuyerA, templateID, versionID)
}

func getTemplateVersionQuestionnaireHTTPWithActor(t *testing.T, env *testEnv, cfg config.Config, actor domain.ActorContext, templateID, versionID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, cfg, env.rfxSvc, env.qSvc, env.versionSvc, env.templateSvc, env.templateQSvc, env.cloneSvc, env.crSvc, nil, env.scoreModelSvc, nil, nil, nil, nil)
	path := "/v1/rfx-templates/" + templateID.String() + "/versions/" + versionID.String() + "/questionnaire"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Tenant-ID", actor.TenantID.String())
	req.Header.Set("X-User-ID", actor.UserID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func getEventHTTP(t *testing.T, env *testEnv, cfg config.Config, fix buyerFixture, eventID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return getEventHTTPWithActor(t, env, cfg, fix.BuyerA, eventID)
}

func getEventHTTPWithActor(t *testing.T, env *testEnv, cfg config.Config, actor domain.ActorContext, eventID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, cfg, env.rfxSvc, env.qSvc, env.versionSvc, env.templateSvc, env.templateQSvc, env.cloneSvc, env.crSvc, nil, env.scoreModelSvc, nil, nil, nil, nil)
	path := "/v1/rfx-events/" + eventID.String()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Tenant-ID", actor.TenantID.String())
	req.Header.Set("X-User-ID", actor.UserID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func expectHTTPErrorCode(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode apperrors.Code) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", wantStatus, rec.Code, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v raw=%s", err, rec.Body.String())
	}
	if payload.Error.Code != string(wantCode) {
		t.Fatalf("expected error code=%s got=%s body=%s", wantCode, payload.Error.Code, rec.Body.String())
	}
}
