//go:build integration

package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
)

func newStudioTestRouter(env *testEnv) http.Handler {
	h := handlers.NewQuestionnaireHandler(env.qSvc)
	r := chi.NewRouter()
	r.Route("/v1/rfx-events", func(r chi.Router) {
		r.Get("/{id}/studio", h.GetStudio)
		r.Get("/{id}/questionnaire", h.GetQuestionnaire)
		r.Post("/{id}/save-draft", h.SaveDraft)
		r.Post("/{id}/validate-publish", h.ValidatePublish)
		r.Post("/{id}/sections", h.CreateSection)
		r.Post("/{id}/questions", h.CreateQuestion)
		r.Post("/{id}/questions/{question_id}/options", h.CreateOption)
		r.Post("/{id}/rules", h.CreateRule)
	})
	return r
}

func studioRequest(
	t *testing.T,
	router http.Handler,
	method, path string,
	fix buyerFixture,
	body any,
) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var parsed map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("decode response (%d): %v body=%s", rec.Code, err, rec.Body.String())
		}
	}
	return rec.Code, parsed
}

// TestStudioQuestionnaireAPIFlow_E2E exercises the buyer studio API sequence:
// create event → studio → sections → questions → options → rules → save-draft → validate-publish.
func TestStudioQuestionnaireAPIFlow_E2E(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-STUDIO-E2E-1")
	router := newStudioTestRouter(env)
	base := "/v1/rfx-events/" + event.ID.String()

	status, studio := studioRequest(t, router, http.MethodGet, base+"/studio", fix, nil)
	if status != http.StatusOK {
		t.Fatalf("get studio: status=%d body=%v", status, studio)
	}
	draftVersion, ok := studio["draft_version"].(map[string]any)
	if !ok || draftVersion == nil {
		t.Fatal("expected draft_version in studio response")
	}
	versionID, _ := draftVersion["id"].(string)
	if versionID == "" {
		t.Fatal("draft version id missing")
	}
	parsedVersionID, err := uuid.Parse(versionID)
	if err != nil {
		t.Fatalf("parse version id: %v", err)
	}
	enableQuestionnaireByVersionID(t, env, fix.TenantID, parsedVersionID)

	status, section := studioRequest(t, router, http.MethodPost, base+"/sections", fix, map[string]any{
		"section_code": "HSE",
		"title":        "Health & Safety",
		"sort_order":   1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create section: status=%d body=%v", status, section)
	}
	sectionID, _ := section["id"].(string)
	if sectionID == "" {
		t.Fatal("section id missing")
	}

	status, question := studioRequest(t, router, http.MethodPost, base+"/questions", fix, map[string]any{
		"section_id":    sectionID,
		"question_code": "ADR_AVAILABLE",
		"question_type": "YES_NO",
		"label":         "ADR available?",
		"sort_order":    1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create question: status=%d body=%v", status, question)
	}
	questionID, _ := question["id"].(string)
	if questionID == "" {
		t.Fatal("question id missing")
	}

	status, followUp := studioRequest(t, router, http.MethodPost, base+"/questions", fix, map[string]any{
		"section_id":    sectionID,
		"question_code": "ADR_NUMBER",
		"question_type": "TEXT",
		"label":         "ADR certificate number",
		"sort_order":    2,
	})
	if status != http.StatusCreated {
		t.Fatalf("create follow-up question: status=%d body=%v", status, followUp)
	}

	status, rule := studioRequest(t, router, http.MethodPost, base+"/rules", fix, map[string]any{
		"rule_code":            "REQ_ADR_NUMBER",
		"action":               "REQUIRE",
		"target_question_code": "ADR_NUMBER",
		"condition_json": map[string]any{
			"operator":             "EQUALS",
			"source_question_code": "ADR_AVAILABLE",
			"value":                true,
		},
		"sort_order": 1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create rule: status=%d body=%v", status, rule)
	}

	draftVersionNum, _ := draftVersion["version"].(float64)
	status, saved := studioRequest(t, router, http.MethodPost, base+"/save-draft", fix, map[string]any{
		"expected_version": int(draftVersionNum),
	})
	if status != http.StatusOK {
		t.Fatalf("save draft: status=%d body=%v", status, saved)
	}
	if saved["version"] == nil {
		t.Fatal("save-draft response missing version")
	}

	status, readiness := studioRequest(t, router, http.MethodPost, base+"/validate-publish", fix, nil)
	if status != http.StatusOK {
		t.Fatalf("validate publish: status=%d body=%v", status, readiness)
	}
	ready, _ := readiness["ready"].(bool)
	if !ready {
		t.Fatalf("expected publish readiness pass, got %+v", readiness)
	}

	status, questionnaire := studioRequest(t, router, http.MethodGet, base+"/questionnaire", fix, nil)
	if status != http.StatusOK {
		t.Fatalf("get questionnaire: status=%d body=%v", status, questionnaire)
	}
	enabled, _ := questionnaire["questionnaire_enabled"].(bool)
	if !enabled {
		t.Fatal("expected questionnaire_enabled=true on definition")
	}

	def, err := env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("service get questionnaire: %v", err)
	}
	if len(def.Sections) != 1 || len(def.Sections[0].Questions) != 2 {
		t.Fatalf("unexpected persisted structure: sections=%d questions=%d",
			len(def.Sections), len(def.Sections[0].Questions))
	}
	if len(def.Rules) != 1 {
		t.Fatalf("expected 1 rule persisted, got %d", len(def.Rules))
	}
}

func TestStudioQuestionnaireValidationHTTP400(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-STUDIO-VAL-1")
	router := newStudioTestRouter(env)
	base := "/v1/rfx-events/" + event.ID.String()

	status, body := studioRequest(t, router, http.MethodPost, base+"/questions", fix, map[string]any{
		"section_id":    uuid.New().String(),
		"question_code": "ORPHAN",
		"question_type": "TEXT",
		"label":         "Orphan",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 validation, got %d body=%v", status, body)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %v", body)
	}
	if code, _ := errObj["code"].(string); code == "" {
		t.Fatalf("expected error code in 400 response, got %v", body)
	}
}
