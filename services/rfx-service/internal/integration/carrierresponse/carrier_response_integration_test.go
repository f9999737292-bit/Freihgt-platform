//go:build integration

package carrierresponse

import (
	"context"
	"encoding/json"
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
	_, err = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, saved.SaveVersion)
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
