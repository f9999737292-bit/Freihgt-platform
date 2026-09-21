//go:build integration

package carrierresponse

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestStartResponseIdempotentAndResume(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()

	ws1, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	ws2, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if ws1.Response.ID != ws2.Response.ID {
		t.Fatal("expected same response id")
	}
	if ws2.Response.RfxVersionID == nil {
		t.Fatal("expected pinned version")
	}
}

func TestAnswerValidSaveInvalid422AndLastValidPreserved(t *testing.T) {
	env, fix, event, q := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	valid := json.RawMessage(`"hello"`)
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: valid}},
	})
	if err != nil {
		t.Fatalf("valid save: %v", err)
	}
	reloaded, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(reloaded.Answers))
	}
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: reloaded.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`123`)}},
	})
	assertValidationFailed(t, err)
	after, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("reload after invalid: %v", err)
	}
	if after.Response.SaveVersion != reloaded.Response.SaveVersion {
		t.Fatalf("save_version advanced on invalid save: before=%d after=%d", reloaded.Response.SaveVersion, after.Response.SaveVersion)
	}
	if len(after.Answers) != 1 || string(after.Answers[0].AnswerValueJSON) != `"hello"` {
		t.Fatalf("valid answer not preserved: %+v", after.Answers)
	}
}

func TestStaleSaveVersion409(t *testing.T) {
	env, fix, event, q := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"a"`) }},
	})
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"b"`) }},
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestSubmitSuccessAndPostSubmitEditDenied(t *testing.T) {
	env, fix, event, q := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"ready"`) }},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	_, err = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, saved.SaveVersion, "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion + 1,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"blocked"`) }},
	})
	if err == nil {
		t.Fatal("expected post-submit edit denied")
	}
}

func TestCrossTenantAndNonParticipantDeny(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	_, err := env.crSvc.StartOrResume(ctx, fix.CrossTenant, event.ID, fix.CarrierID)
	if err == nil {
		t.Fatal("expected cross tenant deny")
	}
	otherCarrier := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	_, err = env.crSvc.StartOrResume(ctx, otherCarrier, event.ID, fix.CarrierID)
	if err == nil {
		t.Fatal("expected non-participant deny")
	}
	_, err = env.crSvc.StartOrResume(ctx, fix.BuyerA, event.ID, fix.CarrierID)
	if err == nil {
		t.Fatal("expected buyer mutation deny")
	}
}

func TestLegacyResponseCompatibility(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	legacy, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("legacy create: %v", err)
	}
	if legacy.RfxVersionID != nil {
		t.Fatal("legacy response should not have version yet")
	}
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start pins legacy: %v", err)
	}
	if ws.Response.ID != legacy.ID {
		t.Fatalf("expected same legacy response id")
	}
	if ws.Response.RfxVersionID == nil {
		t.Fatal("expected legacy response pinned to published version")
	}
}

func TestLegacyCommercialOfferPinsOnStart(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	published, err := env.qRepo.GetPublishedVersionForEvent(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("published version: %v", err)
	}

	lot, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Main lot",
	})
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}

	legacy, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("legacy commercial create: %v", err)
	}
	if legacy.RfxVersionID != nil {
		t.Fatal("commercial response must start unbound")
	}
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, legacy.ID, []domain.UpsertOfferLineInput{
		{RfxLotID: lot.ID, Amount: 15000, CurrencyCode: "RUB"},
	}); err != nil {
		t.Fatalf("save offer: %v", err)
	}

	_, getErr := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	var getApp *apperrors.AppError
	if !errors.As(getErr, &getApp) || getApp.Code != apperrors.CodeUnprocessable {
		t.Fatalf("expected unbound GET Unprocessable, got %v", getErr)
	}
	if getApp.Details["field"] != "rfx_version_id" {
		t.Fatalf("expected details.field=rfx_version_id, got %#v", getApp.Details)
	}

	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start pins commercial: %v", err)
	}
	if ws.Response.ID != legacy.ID {
		t.Fatalf("response id drifted: before=%s after=%s", legacy.ID, ws.Response.ID)
	}
	if ws.Response.ParticipantCompanyID != fix.CarrierID || ws.Response.TenantID != fix.TenantID || ws.Response.RfxEventID != event.ID {
		t.Fatal("tenant/event/participant drifted after pin")
	}
	if ws.Response.Status != domain.RfxResponseStatusDraft {
		t.Fatalf("expected DRAFT, got %s", ws.Response.Status)
	}
	if ws.Response.RfxVersionID == nil || *ws.Response.RfxVersionID != published.ID {
		t.Fatalf("expected pin to published %s, got %v", published.ID, ws.Response.RfxVersionID)
	}
	if len(ws.Questionnaire.Sections) == 0 {
		t.Fatal("expected published questionnaire sections")
	}

	lines, err := env.rfxRepo.ListOfferLinesByResponse(ctx, legacy.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("list offer lines: %v", err)
	}
	if len(lines) != 1 || lines[0].RfxLotID != lot.ID || lines[0].Amount != 15000 || lines[0].CurrencyCode != "RUB" {
		t.Fatalf("offer not preserved: %+v", lines)
	}

	var responseCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_responses WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, event.ID, fix.TenantID).Scan(&responseCount); err != nil {
		t.Fatalf("count responses: %v", err)
	}
	if responseCount != 1 {
		t.Fatalf("expected one response row, got %d", responseCount)
	}

	v2 := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	publishVersion(t, env, v2.ID)
	again, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("second start: %v", err)
	}
	if again.Response.ID != legacy.ID {
		t.Fatal("second start created another response")
	}
	if again.Response.RfxVersionID == nil || *again.Response.RfxVersionID != published.ID {
		t.Fatalf("already pinned response was rebound: want %s got %v", published.ID, again.Response.RfxVersionID)
	}
	reloaded, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("GET after pin: %v", err)
	}
	if reloaded.Response.ID != legacy.ID || reloaded.Response.RfxVersionID == nil || *reloaded.Response.RfxVersionID != published.ID {
		t.Fatal("GET after pin drifted")
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_responses WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, event.ID, fix.TenantID).Scan(&responseCount); err != nil {
		t.Fatalf("recount responses: %v", err)
	}
	if responseCount != 1 {
		t.Fatalf("expected one response row after resume, got %d", responseCount)
	}

	carrierB := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		carrierB, fix.TenantID, "Carrier B", "CARRIER"); err != nil {
		t.Fatalf("seed carrier B: %v", err)
	}
	if _, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, carrierB); err == nil {
		t.Fatal("foreign carrier company must not bind existing response")
	}
}

func TestStartOrResumeUnboundWithoutPublishedVersion(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "No published questionnaire",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-NOQ-" + uuid.NewString()[:8],
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
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	if _, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	}); err != nil {
		t.Fatalf("commercial create: %v", err)
	}
	_, err = env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestStartOrResumeUnboundQuestionnaireDisabled(t *testing.T) {
	env, fix, event, _ := seedPublishedQuestionnaire(t)
	ctx := context.Background()
	published, err := env.qRepo.GetPublishedVersionForEvent(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("published version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled = FALSE WHERE id = $1 AND tenant_id = $2`,
		published.ID, fix.TenantID); err != nil {
		t.Fatalf("disable questionnaire: %v", err)
	}
	if _, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	}); err != nil {
		t.Fatalf("commercial create: %v", err)
	}
	_, err = env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected questionnaire-disabled validation, got %v", err)
	}
	if appErr.Details["field"] != "questionnaire_enabled" {
		t.Fatalf("expected details.field=questionnaire_enabled, got %#v", appErr.Details)
	}
}

func seedPublishedQuestionnaire(t *testing.T) (*testEnv, buyerFixture, *domain.RfxEvent, *domain.Question) {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Carrier Response Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-CR-" + uuid.NewString()[:8],
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
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	required := true
	q, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes", Required: required,
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return env, fix, event, q
}

func assertValidationFailed(t *testing.T, err error) {
	t.Helper()
	assertAppErrorCode(t, err, apperrors.CodeValidationFailed)
}
