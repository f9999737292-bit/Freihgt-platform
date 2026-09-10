//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-01", &fix.CompanyA)

	q, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, published.TemplateID, published.ID)
	if err != nil {
		t.Fatalf("get published version questionnaire: %v", err)
	}
	if len(q.Sections) == 0 {
		t.Fatal("expected non-empty questionnaire graph for published version")
	}
}

func TestE6INT02SupersededVersionQuestionnaireGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, firstPublished := setupPublishedTemplate(t, env, fix, "e6-int-02", &fix.CompanyA)

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
	if len(q.Sections) == 0 {
		t.Fatal("expected non-empty questionnaire graph for superseded version")
	}
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
	if err != nil {
		t.Fatalf("carrier event access: %v", err)
	}
	if prov != nil {
		t.Fatal("carrier must not receive template provenance")
	}
}

func TestE6INT08HTTPGetVersionQuestionnaire(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-08", &fix.CompanyA)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, fix, published.TemplateID, published.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	sections, ok := payload["sections"].([]any)
	if !ok || len(sections) == 0 {
		t.Fatalf("expected sections in HTTP response, got %#v", payload["sections"])
	}
}

func TestE6INT09HTTPGetEventProvenanceFields(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e6-int-09", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-09", fix.CompanyA), uuid.NewString())

	rec := getEventHTTP(t, env, fix, result.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["source_template_version_id"] != published.ID.String() {
		t.Fatalf("expected source_template_version_id=%s got %#v", published.ID, payload["source_template_version_id"])
	}
	if payload["source_version_number"] == nil {
		t.Fatal("expected source_version_number in event GET response")
	}
}

func getTemplateVersionQuestionnaireHTTP(t *testing.T, env *testEnv, fix buyerFixture, templateID, versionID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, config.Config{}, env.rfxSvc, env.qSvc, env.versionSvc, env.templateSvc, env.templateQSvc, env.cloneSvc, env.crSvc, env.scoreModelSvc, nil, nil, nil, nil)
	path := "/v1/rfx-templates/" + templateID.String() + "/versions/" + versionID.String() + "/questionnaire"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func getEventHTTP(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, config.Config{}, env.rfxSvc, env.qSvc, env.versionSvc, env.templateSvc, env.templateQSvc, env.cloneSvc, env.crSvc, env.scoreModelSvc, nil, nil, nil, nil)
	path := "/v1/rfx-events/" + eventID.String()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
