//go:build integration

package carrierresponse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/http/handlers"
)

func TestHeaderSpoofTenantQueryDenied(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	crHandler := handlers.NewCarrierResponseHandler(env.crSvc)
	r := chi.NewRouter()
	r.Get("/v1/rfx-events/{id}/carrier-response", crHandler.GetCarrierResponse)

	req := httptest.NewRequest(http.MethodGet, "/v1/rfx-events/"+event.ID.String()+"/carrier-response?tenant_id="+uuid.NewString(), nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.CarrierAct.UserID.String())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for tenant_id query spoof, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCrossCompanyCarrierDeny(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()

	carrierB := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		carrierB, fix.TenantID, "Carrier B", "CARRIER"); err != nil {
		t.Fatalf("seed carrier B: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: carrierB, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add carrier B participant: %v", err)
	}

	_, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, carrierB)
	if err == nil {
		t.Fatal("expected cross-company deny when carrier A requests carrier B company")
	}
}

func TestCarrierResumeRequiresStart(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()

	_, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err == nil {
		t.Fatal("expected not found before start")
	}

	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	resumed, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("resume get: %v", err)
	}
	if resumed.Response.ID != ws.Response.ID {
		t.Fatal("resume returned different response")
	}
}

func TestBatchAtomicSuccessAndRollback(t *testing.T) {
	env, fix, event, textQ, numQ := seedPublishedQuestionnaireWithTextAndNumber(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: textQ.ID, Value: json.RawMessage(`"alpha"`)},
			{QuestionID: numQ.ID, Value: json.RawMessage(`10`)},
		},
	})
	if err != nil {
		t.Fatalf("batch atomic success: %v", err)
	}
	if saved.SaveVersion != ws.Response.SaveVersion+1 {
		t.Fatalf("expected save_version increment, got %d", saved.SaveVersion)
	}

	reloaded, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: reloaded.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: textQ.ID, Value: json.RawMessage(`"good"`)},
			{QuestionID: numQ.ID, Value: json.RawMessage(`"not-a-number"`)},
		},
	})
	assertValidationFailed(t, err)
	after, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.Response.SaveVersion != reloaded.Response.SaveVersion {
		t.Fatal("batch rollback must not advance save_version")
	}
	if len(after.Answers) != 2 {
		t.Fatalf("batch rollback must preserve both prior valid answers, got %d", len(after.Answers))
	}
}

func TestRequiredRulePreSubmitAndSubmitDenied(t *testing.T) {
	env, fix, event, q := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	pre, err := env.crSvc.ValidateResponse(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pre.Valid {
		t.Fatal("expected pre-submit fail for missing required answer")
	}

	_, err = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion)
	if err == nil {
		t.Fatal("expected submit with errors denied")
	}
}

func TestPreSubmitPassAfterValidSave(t *testing.T) {
	env, fix, event, q := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"done"`) }},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.CompletionPercent < 100 {
		t.Fatalf("expected 100%% completion, got %v", saved.CompletionPercent)
	}
	pre, err := env.crSvc.ValidateResponse(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !pre.Valid {
		t.Fatalf("expected pre-submit pass, errors=%+v", pre.Errors)
	}
}

func TestConditionalRequiredRule(t *testing.T) {
	env, fix, event, triggerQ, detailQ := seedConditionalRequiredQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: triggerQ.ID, Value: json.RawMessage(`true`)},
		},
	})
	if err != nil {
		t.Fatalf("save trigger: %v", err)
	}
	pre, err := env.crSvc.ValidateResponse(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pre.Valid {
		t.Fatal("expected conditional required detail missing")
	}
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: detailQ.ID, Value: json.RawMessage(`"detail"`) }},
	})
	if err != nil {
		t.Fatalf("save detail: %v", err)
	}
	pre, err = env.crSvc.ValidateResponse(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("validate after detail: %v", err)
	}
	if !pre.Valid {
		t.Fatalf("expected pass after conditional required filled, errors=%+v", pre.Errors)
	}
}

func TestHiddenQuestionPolicyAndDeleteAtomic(t *testing.T) {
	env, fix, event, showQ, detailQ := seedHiddenQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Fill required NOTES via questionnaire lookup
	var notesQID uuid.UUID
	for _, sec := range ws.Questionnaire.Sections {
		for _, q := range sec.Questions {
			if q.QuestionCode == "NOTES" {
				notesQID = q.ID
				break
			}
		}
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: notesQID, Value: json.RawMessage(`"filled"`)},
			{QuestionID: showQ.ID, Value: json.RawMessage(`true`)},
			{QuestionID: detailQ.ID, Value: json.RawMessage(`"visible-detail"`)},
		},
	})
	if err != nil {
		t.Fatalf("save visible answers: %v", err)
	}
	hiddenSave, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: showQ.ID, Value: json.RawMessage(`false`) }},
	})
	if err != nil {
		t.Fatalf("hide detail question: %v", err)
	}
	after, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, a := range after.Answers {
		if a.QuestionID == detailQ.ID {
			t.Fatal("hidden answer must be deleted atomically on save")
		}
	}
	if hiddenSave.CompletionPercent < 100 {
		t.Fatalf("hidden required must not block completion, got %v", hiddenSave.CompletionPercent)
	}
}

func seedPublishedQuestionnaireWithTextAndNumber(t *testing.T) (*testEnv, buyerFixture, *domain.RfxEvent, *domain.Question, *domain.Question) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Batch Atomic",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-CR-BAT-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{SectionCode: "MAIN", Title: "Main"})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	required := true
	textQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes", Required: &required,
	})
	if err != nil {
		t.Fatalf("create text question: %v", err)
	}
	numQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "AMOUNT", QuestionType: domain.QuestionTypeNumber, Label: "Amount", Required: &required,
	})
	if err != nil {
		t.Fatalf("create number question: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return env, fix, event, textQ, numQ
}

func seedConditionalRequiredQuestionnaire(t *testing.T) (*testEnv, buyerFixture, *domain.RfxEvent, *domain.Question, *domain.Question) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Conditional Required",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-CR-COND-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{SectionCode: "COND", Title: "Conditional"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	triggerQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NEED_DETAIL", QuestionType: domain.QuestionTypeYesNo, Label: "Need detail?",
	})
	if err != nil {
		t.Fatalf("trigger question: %v", err)
	}
	detailQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "DETAIL", QuestionType: domain.QuestionTypeText, Label: "Detail",
	})
	if err != nil {
		t.Fatalf("detail question: %v", err)
	}
	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, &detailQ.ID, domain.CreateQuestionRuleInput{
		RuleCode:      "REQ_DETAIL",
		Action:        domain.RuleActionRequire,
		ConditionJSON: conditionEquals("NEED_DETAIL", true),
	})
	if err != nil {
		t.Fatalf("require rule: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return env, fix, event, triggerQ, detailQ
}

func seedHiddenQuestionnaire(t *testing.T) (*testEnv, buyerFixture, *domain.RfxEvent, *domain.Question, *domain.Question) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Hidden Policy",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-CR-HID-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{SectionCode: "HID", Title: "Hidden"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	required := true
	textQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes", Required: &required,
	})
	if err != nil {
		t.Fatalf("notes question: %v", err)
	}
	_ = textQ
	showQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "SHOW_DETAIL", QuestionType: domain.QuestionTypeYesNo, Label: "Show?",
	})
	if err != nil {
		t.Fatalf("show question: %v", err)
	}
	detailQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "DETAIL_TEXT", QuestionType: domain.QuestionTypeText, Label: "Detail",
	})
	if err != nil {
		t.Fatalf("detail question: %v", err)
	}
	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, &detailQ.ID, domain.CreateQuestionRuleInput{
		RuleCode:      "SHOW_DETAIL",
		Action:        domain.RuleActionShow,
		ConditionJSON: conditionEquals("SHOW_DETAIL", true),
	})
	if err != nil {
		t.Fatalf("show rule: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return env, fix, event, showQ, detailQ
}

func conditionEquals(sourceCode string, value any) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"operator":             domain.ConditionOperatorEquals,
		"source_question_code": sourceCode,
		"value":                value,
	})
	return b
}
