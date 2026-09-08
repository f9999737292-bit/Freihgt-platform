//go:build integration

package versionlifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE2INT01CompareV1WithV2(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-01")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-01-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-01-v2", "e2-int-01-fork")

	result, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if result.SourceVersionNumber != 1 || result.TargetVersionNumber != 2 {
		t.Fatalf("unexpected version numbers: %+v", result)
	}
	if result.Summary.AddedCount == 0 {
		t.Fatalf("expected added items, summary=%+v", result.Summary)
	}
}

func TestE2INT02CompareStableHash(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-02")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-02-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-02-v2", "e2-int-02-fork")
	in := domain.CompareVersionsInput{SourceVersionID: v1.ID, TargetVersionID: v2.ID}
	first, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, in)
	if err != nil {
		t.Fatalf("first compare: %v", err)
	}
	second, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, in)
	if err != nil {
		t.Fatalf("second compare: %v", err)
	}
	if first.CanonicalDiffHash != second.CanonicalDiffHash {
		t.Fatalf("hash unstable: %s vs %s", first.CanonicalDiffHash, second.CanonicalDiffHash)
	}
}

func TestE2INT03CompareCrossEvent404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	eventA := createDraftEvent(t, env, fix, "RFX-E2-03A")
	eventB := createDraftEvent(t, env, fix, "RFX-E2-03B")
	vA := ensurePublishedVersion(t, env, fix, eventA.ID, "e2-int-03-a")
	vB := ensurePublishedVersion(t, env, fix, eventB.ID, "e2-int-03-b")

	_, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, eventA.ID, domain.CompareVersionsInput{
		SourceVersionID: vA.ID,
		TargetVersionID: vB.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE2INT04CompareCrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-04")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-04-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-04-v2", "e2-int-04-fork")

	_, err := env.versionSvc.CompareVersions(context.Background(), fix.CrossTenant, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE2INT05CompareCarrier403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-05")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-05-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-05-v2", "e2-int-05-fork")

	_, err := env.versionSvc.CompareVersions(context.Background(), fix.CarrierAct, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE2INT06RestorePublishedAsDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-06")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-06-pub")
	sourceGraph, err := env.qRepo.LoadQuestionnaire(context.Background(), published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load source graph: %v", err)
	}

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-06-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Restore published",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if draft.Status != domain.RfxVersionStatusDraft || draft.VersionNumber != 2 {
		t.Fatalf("unexpected draft: %+v", draft)
	}
	restoredGraph, err := env.qRepo.LoadQuestionnaire(context.Background(), draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load restored graph: %v", err)
	}
	assertQuestionnaireGraphEqual(t, sourceGraph, restoredGraph)

	state, err := env.qRepo.GetEventVersionState(context.Background(), event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.PublishedVersionID == nil || *state.PublishedVersionID != published.ID {
		t.Fatal("published pointer changed")
	}
}

func TestE2INT07RestoreSupersededAsDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-07")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-07-v1")
	_ = publishSecondVersion(t, env, fix, event.ID, "e2-int-07-v2", "e2-int-07-fork")
	v1Reload, err := env.qRepo.GetVersionByID(context.Background(), v1.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v1: %v", err)
	}
	if v1Reload.Status != domain.RfxVersionStatusSuperseded {
		t.Fatalf("v1 status=%s", v1Reload.Status)
	}

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, v1.ID, "e2-int-07-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Restore superseded",
	})
	if err != nil {
		t.Fatalf("restore superseded: %v", err)
	}
	if draft.VersionNumber != 3 {
		t.Fatalf("expected version_number=3, got %d", draft.VersionNumber)
	}
}

func TestE2INT08RestoreExistingDraft409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-08")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-08-pub")
	if _, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, event.ID, "e2-int-08-fork"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	_, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-08-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Should fail",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE2INT09RestoreIdempotentReplay(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-09")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-09-pub")
	in := domain.RestoreVersionAsDraftInput{ChangeSummary: "Restore once"}
	first, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-09-key", in)
	if err != nil {
		t.Fatalf("first restore: %v", err)
	}
	second, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-09-key", in)
	if err != nil {
		t.Fatalf("replay restore: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent replay diverged")
	}
}

func TestE2INT10RestoreSameKeyDifferentBody409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-10")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-10-pub")
	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-10-key", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "First body",
	}); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	_, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-10-key", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Different body",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE2INT11RestoreConcurrentSingleDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-11")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-11-pub")
	in := domain.RestoreVersionAsDraftInput{ChangeSummary: "Concurrent restore"}
	key := "e2-int-11-key"

	var wg sync.WaitGroup
	results := make([]*domain.RfxVersion, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, in)
		}(i)
	}
	wg.Wait()

	success := 0
	conflict := 0
	var draftID uuid.UUID
	for i, callErr := range errs {
		if callErr == nil {
			success++
			if results[i] == nil {
				t.Fatal("nil draft on success")
			}
			if draftID == uuid.Nil {
				draftID = results[i].ID
			} else if results[i].ID != draftID {
				t.Fatalf("concurrent restore created multiple drafts")
			}
			continue
		}
		var appErr *apperrors.AppError
		if errors.As(callErr, &appErr) && appErr.Code == apperrors.CodeConflict {
			conflict++
			continue
		}
		t.Fatalf("unexpected error: %v", callErr)
	}
	if success != 1 {
		t.Fatalf("expected one success, got success=%d conflict=%d", success, conflict)
	}
}

func TestE2INT12RestoreAuditOnce(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-12")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-12-pub")
	in := domain.RestoreVersionAsDraftInput{ChangeSummary: "Audit once"}
	key := "e2-int-12-key"
	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, in)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, in); err != nil {
		t.Fatalf("replay: %v", err)
	}
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM core.audit_events
		WHERE tenant_id = $1 AND event_type = 'rfx.version.restored_as_draft.v1' AND resource_id = $2`,
		fix.TenantID, draft.ID).Scan(&count); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one audit event, got %d", count)
	}
}

func TestE2INT13RestorePreservesResponsePinAndScoreHistory(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-13")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-13-pub")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	if ws.Response.RfxVersionID == nil {
		t.Fatal("response not pinned")
	}
	pinnedVersion := *ws.Response.RfxVersionID
	insertQualificationForResponse(t, env, fix.TenantID, &ws.Response)

	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-13-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Restore while response exists",
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}

	reloaded, err := env.rfxRepo.GetResponseByID(context.Background(), ws.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	if reloaded.RfxVersionID == nil || *reloaded.RfxVersionID != pinnedVersion {
		t.Fatalf("response pin changed: got %v want %s", reloaded.RfxVersionID, pinnedVersion)
	}
	var scoreCount int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results WHERE tenant_id = $1 AND rfx_response_id = $2`,
		fix.TenantID, ws.Response.ID).Scan(&scoreCount); err != nil {
		t.Fatalf("count scores: %v", err)
	}
	if scoreCount != 1 {
		t.Fatalf("score history changed: count=%d", scoreCount)
	}
}

func publishSecondVersion(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, publishKey, forkKey string) *domain.RfxVersion {
	t.Helper()
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, eventID, forkKey)
	if err != nil {
		t.Fatalf("fork for v2: %v", err)
	}
	section, err := env.qSvc.CreateSection(context.Background(), fix.BuyerA, eventID, domain.CreateSectionInput{
		SectionCode: "V2",
		Title:       "Version 2",
	})
	if err != nil {
		t.Fatalf("create v2 section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(context.Background(), fix.BuyerA, eventID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTE",
		QuestionType: domain.QuestionTypeText,
		Label:        "Note",
		Required:     true,
	}); err != nil {
		t.Fatalf("create v2 question: %v", err)
	}
	draft, err = env.qRepo.GetVersionByID(context.Background(), draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	event, err := env.rfxRepo.GetEventByID(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	published, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, eventID, publishKey, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Second publish",
	})
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	return published
}

func assertQuestionnaireGraphEqual(t *testing.T, left, right *domain.QuestionnaireDefinition) {
	t.Helper()
	if len(left.Sections) != len(right.Sections) {
		t.Fatalf("section count mismatch: %d vs %d", len(left.Sections), len(right.Sections))
	}
	leftSections := map[string]domain.SectionWithQuestions{}
	for _, section := range left.Sections {
		leftSections[section.Section.SectionCode] = section
	}
	for _, section := range right.Sections {
		match, ok := leftSections[section.Section.SectionCode]
		if !ok {
			t.Fatalf("missing section code %s on restored graph", section.Section.SectionCode)
		}
		if match.Section.Title != section.Section.Title {
			t.Fatalf("section title mismatch for %s", section.Section.SectionCode)
		}
		if len(match.Questions) != len(section.Questions) {
			t.Fatalf("question count mismatch for section %s", section.Section.SectionCode)
		}
		leftQuestions := map[string]domain.Question{}
		for _, question := range match.Questions {
			leftQuestions[question.QuestionCode] = question
		}
		for _, question := range section.Questions {
			sourceQuestion, ok := leftQuestions[question.QuestionCode]
			if !ok {
				t.Fatalf("missing question %s", question.QuestionCode)
			}
			if sourceQuestion.Label != question.Label || sourceQuestion.Required != question.Required {
				t.Fatalf("question payload mismatch for %s", question.QuestionCode)
			}
		}
	}
}
