//go:build integration

package versionlifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE1INT01FirstPublish(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-01")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-01")
	if published.Status != domain.RfxVersionStatusPublished {
		t.Fatalf("status=%s", published.Status)
	}
	if !published.IsCurrentPublished {
		t.Fatal("expected current published flag")
	}
}

func TestE1INT02PublishedGraphMutationDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-02")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-02")
	_, err := env.qSvc.CreateSection(context.Background(), fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "BLOCKED",
		Title:       "Should fail",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE1INT03ForkFromPublished(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-03")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-03-pub")
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-03-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if draft.Status != domain.RfxVersionStatusDraft {
		t.Fatalf("status=%s", draft.Status)
	}
	if !draft.IsActiveDraft {
		t.Fatal("expected active draft flag")
	}
}

func TestE1INT04SecondDraft409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-04")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-04-pub")
	if _, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-04-fork-1"); err != nil {
		t.Fatalf("first fork: %v", err)
	}
	_, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-04-fork-2")
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE1INT05PublishV2WithoutResponsesSupersedesV1(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-05")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-05-v1")
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-05-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	section, err := env.qSvc.CreateSection(context.Background(), fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "V2",
		Title:       "Version 2",
	})
	if err != nil {
		t.Fatalf("create section on draft: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(context.Background(), fix.BuyerA, event.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTE",
		QuestionType: domain.QuestionTypeText,
		Label:        "Note",
		Required:     true,
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	draft, err = env.qRepo.GetVersionByID(context.Background(), draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	event, err = env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	v2, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e1-int-05-v2", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Second publish",
	})
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	v1Reload, err := env.qRepo.GetVersionByID(context.Background(), v1.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v1: %v", err)
	}
	if v1Reload.Status != domain.RfxVersionStatusSuperseded {
		t.Fatalf("v1 status=%s", v1Reload.Status)
	}
	if v2.Status != domain.RfxVersionStatusPublished || !v2.IsCurrentPublished {
		t.Fatalf("v2 not current published: %+v", v2)
	}
}

func TestE1INT06RepeatPublishWithResponses422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-06")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-06-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-06-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	event, err = env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e1-int-06-v2", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Blocked republish",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation/422 impact error, got %v", err)
	}
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactRequired {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE1INT07PublishIdempotencySameKeySameBody(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-07")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	in := domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Idempotent publish",
	}
	first, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e1-int-07-key", in)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	second, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e1-int-07-key", in)
	if err != nil {
		t.Fatalf("replay publish: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent replay returned different version ids")
	}
}

func TestE1INT08PublishIdempotencySameKeyDifferentBody409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-08")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e1-int-08-key", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "First body",
	}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	draft2, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e1-int-08-fork")
	if err != nil {
		t.Fatalf("fork draft: %v", err)
	}
	event2, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event2: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e1-int-08-key", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event2.Version,
		ExpectedDraftVersion: draft2.Version,
		ChangeSummary:        "Different body",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE1INT09CrossTenantVersionList404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-09")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-09")
	_, err := env.versionSvc.ListVersions(context.Background(), fix.CrossTenant, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE1INT10SameTenantNonOwnerPublish403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-10")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerB, event.ID, "e1-int-10", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Forbidden owner",
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE1INT11CarrierPublish403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-11")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.CarrierAct, event.ID, "e1-int-11", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Carrier forbidden",
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE1INT12OldDraftResponseSavesAfterV2Publish(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-12")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-12-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if ws.Response.RfxVersionID == nil || *ws.Response.RfxVersionID != v1.ID {
		t.Fatalf("expected pin to v1")
	}
	promoteVersionForContinuityFixture(t, env, fix, event.ID, v1.ID)
	qID := firstQuestionID(t, ws)
	_, err = env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qID, Value: json.RawMessage(`"still-v1"`) }},
	})
	if err != nil {
		t.Fatalf("save after v2 publish: %v", err)
	}
}

func TestE1INT13OldDraftResponseSubmitsAgainstPinnedV1(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-13")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-13-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	qID := firstQuestionID(t, ws)
	_, err = env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qID, Value: json.RawMessage(`"answer"`) }},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	promoteVersionForContinuityFixture(t, env, fix, event.ID, v1.ID)
	ws2, err := env.crSvc.GetWorkspace(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if _, err := env.crSvc.Submit(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, ws2.Response.SaveVersion); err != nil {
		t.Fatalf("submit against pinned v1: %v", err)
	}
	reloaded, err := env.rfxRepo.GetResponseByEventAndCompany(context.Background(), event.ID, fix.CarrierID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	if reloaded.RfxVersionID == nil || *reloaded.RfxVersionID != v1.ID {
		t.Fatalf("submit pinned wrong version")
	}
}

func TestE1INT14NewResponsePinsV2(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-14")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-14-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	v2 := publishV2WithoutResponses(t, env, fix, event.ID, "e1-int-14-v2", "e1-int-14-fork")
	carrierB := uuid.New()
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		carrierB, fix.TenantID, "Carrier B", "CARRIER"); err != nil {
		t.Fatalf("seed carrier b: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(context.Background(), fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID:        fix.TenantID,
		RfxEventID:      event.ID,
		CompanyID:       carrierB,
		ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	carrierAct := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`,
		carrierAct.UserID, fix.TenantID, "carrier-b@test.local", "carrier-b@test.local"); err != nil {
		t.Fatalf("seed carrier b user: %v", err)
	}
	var carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`,
		fix.TenantID, carrierB, carrierAct.UserID); err != nil {
		t.Fatalf("membership: %v", err)
	}
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.TenantID, carrierAct.UserID, carrierB, carrierRoleID); err != nil {
		t.Fatalf("role: %v", err)
	}
	ws, err := env.crSvc.StartOrResume(context.Background(), carrierAct, event.ID, carrierB)
	if err != nil {
		t.Fatalf("start new carrier: %v", err)
	}
	if ws.Response.RfxVersionID == nil || *ws.Response.RfxVersionID != v2.ID {
		t.Fatalf("new response should pin v2, got %v want %s", ws.Response.RfxVersionID, v2.ID)
	}
}

func TestE1INT15ResponseVersionPinImmutable(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-15")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-15-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if ws.Response.RfxVersionID == nil || *ws.Response.RfxVersionID != v1.ID {
		t.Fatalf("expected response pinned to v1 before immutability check")
	}
	otherVersionID := uuid.New()
	reloaded, err := env.rfxRepo.PinResponseVersion(context.Background(), ws.Response.ID, fix.TenantID, otherVersionID)
	if err != nil {
		t.Fatalf("pin attempt: %v", err)
	}
	if reloaded.RfxVersionID == nil || *reloaded.RfxVersionID != v1.ID {
		t.Fatalf("expected immutable pin to v1, got %v", reloaded.RfxVersionID)
	}
}

func TestE1INT16ScoreHistoryUnchangedAfterV2Publish(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-16")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-16-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	insertQualificationForResponse(t, env, fix.TenantID, &ws.Response)
	var before int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results qr
		INNER JOIN rfx.rfx_responses rr ON rr.id = qr.rfx_response_id
		WHERE rr.rfx_event_id = $1`, event.ID).Scan(&before); err != nil {
		t.Fatalf("count scores: %v", err)
	}
	promoteVersionForContinuityFixture(t, env, fix, event.ID, v1.ID)
	var after int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results qr
		INNER JOIN rfx.rfx_responses rr ON rr.id = qr.rfx_response_id
		WHERE rr.rfx_event_id = $1`, event.ID).Scan(&after); err != nil {
		t.Fatalf("count scores after: %v", err)
	}
	if before != after || before != 1 {
		t.Fatalf("score history changed: before=%d after=%d", before, after)
	}
}

func TestE1INT17ConcurrentPublishOneWinner(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-17")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	in := domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Concurrent publish",
	}
	var wg sync.WaitGroup
	results := make([]*domain.RfxVersion, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "e1-int-17-" + string(rune('a'+idx))
			results[idx], errs[idx] = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, key, in)
		}(i)
	}
	wg.Wait()
	success := 0
	for i, err := range errs {
		if err == nil {
			success++
			if results[i] == nil {
				t.Fatal("nil result on success")
			}
			continue
		}
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
			t.Fatalf("unexpected error[%d]: %v", i, err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly one successful publish, got %d", success)
	}
}

func TestE1INT18ConcurrentForkOneDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-18")
	ensurePublishedVersion(t, env, fix, event.ID, "e1-int-18-pub")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "e1-int-18-fork-" + string(rune('a'+idx))
			_, errs[idx] = env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, key)
		}(i)
	}
	wg.Wait()
	success := 0
	for i, err := range errs {
		if err == nil {
			success++
			continue
		}
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
			t.Fatalf("unexpected fork error[%d]: %v", i, err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly one successful fork, got %d", success)
	}
}

func firstQuestionID(t *testing.T, ws *domain.CarrierResponseWorkspace) uuid.UUID {
	t.Helper()
	for _, section := range ws.Questionnaire.Sections {
		if len(section.Questions) > 0 {
			return section.Questions[0].ID
		}
	}
	t.Fatal("expected at least one question in workspace")
	return uuid.Nil
}

func publishV2WithoutResponses(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, publishKey, forkKey string) *domain.RfxVersion {
	t.Helper()
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, eventID, forkKey)
	if err != nil {
		t.Fatalf("fork for v2: %v", err)
	}
	section, err := env.qSvc.CreateSection(context.Background(), fix.BuyerA, eventID, domain.CreateSectionInput{
		SectionCode: "V2SEC",
		Title:       "V2 Section",
	})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(context.Background(), fix.BuyerA, eventID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "V2Q",
		QuestionType: domain.QuestionTypeText,
		Label:        "V2 Question",
		Required:     true,
	}); err != nil {
		t.Fatalf("question: %v", err)
	}
	draft, err = env.qRepo.GetVersionByID(context.Background(), draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	event, err := env.rfxRepo.GetEventByID(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	v2, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, eventID, publishKey, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Version 2 publish",
	})
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	return v2
}
