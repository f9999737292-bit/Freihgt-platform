//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE6EV01HistoricalVersionQuestionnaireReadOnlyNoWrite(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-ev-01", &fix.CompanyA)

	before := captureTemplateVersionQuestionnaireWriteSnapshot(t, env, fix, published.TemplateID, published.ID)

	q, err := env.templateQSvc.GetVersionQuestionnaire(context.Background(), fix.BuyerA, published.TemplateID, published.ID)
	if err != nil {
		t.Fatalf("PG get version questionnaire: %v", err)
	}
	assertE6HistoricalGraphPG(t, q, expect)

	afterPG := captureTemplateVersionQuestionnaireWriteSnapshot(t, env, fix, published.TemplateID, published.ID)
	assertTemplateVersionQuestionnaireWriteSnapshotEqual(t, before, afterPG)

	rec := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, published.TemplateID, published.ID)
	assertE6HistoricalGraphHTTP(t, rec, published, expect)

	afterHTTP := captureTemplateVersionQuestionnaireWriteSnapshot(t, env, fix, published.TemplateID, published.ID)
	assertTemplateVersionQuestionnaireWriteSnapshotEqual(t, before, afterHTTP)
}

func TestE6EV02HistoricalVersionQuestionnaireDeterministicRepeatHTTP(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published, expect := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-ev-02", &fix.CompanyA)

	before := captureTemplateVersionQuestionnaireWriteSnapshot(t, env, fix, published.TemplateID, published.ID)

	rec1 := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, published.TemplateID, published.ID)
	rec2 := getTemplateVersionQuestionnaireHTTP(t, env, e6EnabledConfig(), fix, published.TemplateID, published.ID)
	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("expected 200/200, got %d/%d", rec1.Code, rec2.Code)
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Fatalf("repeat GET bodies differ:\nfirst=%s\nsecond=%s", rec1.Body.String(), rec2.Body.String())
	}
	assertE6HistoricalGraphHTTP(t, rec1, published, expect)

	after := captureTemplateVersionQuestionnaireWriteSnapshot(t, env, fix, published.TemplateID, published.ID)
	assertTemplateVersionQuestionnaireWriteSnapshotEqual(t, before, after)
}

func TestE6EV04EventGetProvenanceReadOnlyNoWrite(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published, _ := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-ev-04", &fix.CompanyA)
	cloneResult := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-EV-04", fix.CompanyA), uuid.NewString())

	before := captureEventGetWriteSnapshot(t, env, fix, cloneResult.Event.ID)

	prov, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, cloneResult.Event.ID)
	if err != nil {
		t.Fatalf("PG get event provenance: %v", err)
	}
	if prov == nil {
		t.Fatal("expected PG provenance for cloned event")
	}
	assertProvenanceExact(t, prov, published, detail.Template.TemplateCode, false)

	afterPG := captureEventGetWriteSnapshot(t, env, fix, cloneResult.Event.ID)
	assertEventGetWriteSnapshotEqual(t, before, afterPG)

	rec := getEventHTTP(t, env, e6EnabledConfig(), fix, cloneResult.Event.ID)
	assertHTTPEventProvenanceExact(t, rec, published, detail.Template.TemplateCode, false)

	afterHTTP := captureEventGetWriteSnapshot(t, env, fix, cloneResult.Event.ID)
	assertEventGetWriteSnapshotEqual(t, before, afterHTTP)
}

func TestE6EV03EventProvenanceLifecycleAndNoLeakage(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published, _ := setupPublishedE6HistoricalTemplate(t, env, fix, "e6-ev-03", &fix.CompanyA)

	cloneResult := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-EV-03", fix.CompanyA), uuid.NewString())

	prov, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, cloneResult.Event.ID)
	if err != nil {
		t.Fatalf("PG provenance after clone: %v", err)
	}
	if prov == nil {
		t.Fatal("expected PG provenance after clone")
	}
	assertProvenanceExact(t, prov, published, detail.Template.TemplateCode, false)

	cloneRec := getEventHTTP(t, env, e6EnabledConfig(), fix, cloneResult.Event.ID)
	if cloneRec.Code != http.StatusOK {
		t.Fatalf("HTTP clone event GET: %d body=%s", cloneRec.Code, cloneRec.Body.String())
	}
	assertHTTPEventProvenanceExact(t, cloneRec, published, detail.Template.TemplateCode, false)

	manualEvent := createDraftEvent(t, env, fix, "RFQ-E6-EV-03-MANUAL")
	manualProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, manualEvent.ID)
	if err != nil {
		t.Fatalf("PG manual provenance: %v", err)
	}
	if manualProv != nil {
		t.Fatalf("manual event PG provenance must be nil, got %+v", manualProv)
	}
	manualRec := getEventHTTP(t, env, e6EnabledConfig(), fix, manualEvent.ID)
	assertHTTPEventOmitsProvenance(t, manualRec)

	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e6-ev-03-fork"); err != nil {
		t.Fatalf("fork draft: %v", err)
	}
	publishTemplate(t, env, fix, detail.Template.ID, "e6-ev-03-pub2")
	supersededClone := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E6-EV-03-SUP", fix.CompanyA), uuid.NewString())
	supProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.BuyerA, supersededClone.Event.ID)
	if err != nil {
		t.Fatalf("PG superseded provenance: %v", err)
	}
	if supProv == nil || !supProv.SourceVersionWarning || supProv.SourceVersionStatus != domain.RfxVersionStatusSuperseded {
		t.Fatalf("expected superseded provenance warning, got %+v", supProv)
	}
	supRec := getEventHTTP(t, env, e6EnabledConfig(), fix, supersededClone.Event.ID)
	assertHTTPEventProvenanceExact(t, supRec, published, detail.Template.TemplateCode, true)

	carrierRec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.CarrierAct, cloneResult.Event.ID)
	expectHTTPErrorCode(t, carrierRec, http.StatusNotFound, apperrors.CodeNotFound)

	carrierProv, err := env.rfxSvc.GetEventProvenance(context.Background(), fix.CarrierAct, cloneResult.Event.ID)
	if err == nil && carrierProv != nil {
		t.Fatalf("carrier PG provenance leak: %+v", carrierProv)
	}
	if err != nil {
		expectAppErrorCode(t, err, apperrors.CodeNotFound)
	}

	crossRec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.CrossTenant, cloneResult.Event.ID)
	expectHTTPErrorCode(t, crossRec, http.StatusNotFound, apperrors.CodeNotFound)

	nonOwnerRec := getEventHTTPWithActor(t, env, e6EnabledConfig(), fix.BuyerB, cloneResult.Event.ID)
	expectHTTPErrorCode(t, nonOwnerRec, http.StatusNotFound, apperrors.CodeNotFound)
}

func assertProvenanceExact(t *testing.T, prov *domain.RfxEventProvenance, published *domain.RfxTemplateVersion, templateCode string, wantWarning bool) {
	t.Helper()
	if prov.SourceTemplateID != published.TemplateID {
		t.Fatalf("source_template_id: got %s want %s", prov.SourceTemplateID, published.TemplateID)
	}
	if prov.SourceTemplateVersionID != published.ID {
		t.Fatalf("source_template_version_id: got %s want %s", prov.SourceTemplateVersionID, published.ID)
	}
	if prov.SourceVersionNumber != published.VersionNumber {
		t.Fatalf("source_version_number: got %d want %d", prov.SourceVersionNumber, published.VersionNumber)
	}
	if prov.TemplateCode != templateCode {
		t.Fatalf("template_code: got %s want %s", prov.TemplateCode, templateCode)
	}
	if prov.SourceVersionWarning != wantWarning {
		t.Fatalf("source_version_warning: got %v want %v", prov.SourceVersionWarning, wantWarning)
	}
	if wantWarning && prov.SourceVersionStatus != domain.RfxVersionStatusSuperseded {
		t.Fatalf("source_version_status: got %s want SUPERSEDED", prov.SourceVersionStatus)
	}
	if !wantWarning && prov.SourceVersionStatus != domain.RfxVersionStatusPublished {
		t.Fatalf("source_version_status: got %s want PUBLISHED", prov.SourceVersionStatus)
	}
}

func assertHTTPEventProvenanceExact(t *testing.T, rec *httptest.ResponseRecorder, published *domain.RfxTemplateVersion, templateCode string, wantWarning bool) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode event response: %v", err)
	}
	if got, _ := payload["source_template_id"].(string); got != published.TemplateID.String() {
		t.Fatalf("source_template_id: got %s want %s", got, published.TemplateID)
	}
	if got, _ := payload["source_template_version_id"].(string); got != published.ID.String() {
		t.Fatalf("source_template_version_id: got %s want %s", got, published.ID)
	}
	versionNumber, ok := payload["source_version_number"].(float64)
	if !ok || int(versionNumber) != published.VersionNumber {
		t.Fatalf("source_version_number: got %#v want %d", payload["source_version_number"], published.VersionNumber)
	}
	if got, _ := payload["source_template_code"].(string); got != templateCode {
		t.Fatalf("source_template_code: got %s want %s", got, templateCode)
	}
	warning, ok := payload["source_version_warning"].(bool)
	if !ok || warning != wantWarning {
		t.Fatalf("source_version_warning: got %#v want %v", payload["source_version_warning"], wantWarning)
	}
	wantStatus := domain.RfxVersionStatusPublished
	if wantWarning {
		wantStatus = domain.RfxVersionStatusSuperseded
	}
	if got, _ := payload["source_version_status"].(string); got != wantStatus {
		t.Fatalf("source_version_status: got %s want %s", got, wantStatus)
	}
}

func assertHTTPEventOmitsProvenance(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode event response: %v", err)
	}
	for _, key := range []string{
		"source_template_id",
		"source_template_version_id",
		"source_version_number",
		"source_version_status",
		"source_version_warning",
		"source_template_code",
		"source_template_name_i18n",
	} {
		if _, ok := payload[key]; ok {
			t.Fatalf("response must not expose provenance field %s", key)
		}
	}
}
