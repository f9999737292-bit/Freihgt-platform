//go:build integration

package versionlifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestGetOrCreateDraftVersionStartsDisabled(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-QE-01")

	draft, err := env.qRepo.GetOrCreateDraftVersion(context.Background(), fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("get or create draft: %v", err)
	}
	if draft.QuestionnaireEnabled {
		t.Fatal("GetOrCreateDraftVersion must insert questionnaire_enabled=false")
	}
	if draft.Status != domain.RfxVersionStatusDraft {
		t.Fatalf("status=%s", draft.Status)
	}

	again, err := env.qRepo.GetOrCreateDraftVersion(context.Background(), fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("repeat get or create: %v", err)
	}
	if again.ID != draft.ID {
		t.Fatal("repeat GetOrCreateDraftVersion created another version")
	}
	if again.QuestionnaireEnabled {
		t.Fatal("repeat GetOrCreateDraftVersion must not enable questionnaire")
	}
}

func TestFailedPublishQuestionnaireKeepsDisabled(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-QE-02")
	ctx := context.Background()

	draft, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("get or create draft: %v", err)
	}
	if draft.QuestionnaireEnabled {
		t.Fatal("draft must start disabled")
	}
	event, err = env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	_, err = env.versionSvc.PublishQuestionnaire(ctx, fix.BuyerA, event.ID, "qe-fail-empty", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "empty draft must not enable",
	})
	assertAppErrorCode(t, err, apperrors.CodeValidationFailed)

	reloaded, err := env.qRepo.GetVersionByID(ctx, draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft after failed publish: %v", err)
	}
	if reloaded.QuestionnaireEnabled {
		t.Fatal("failed PublishQuestionnaire must leave questionnaire_enabled=false")
	}
	if reloaded.Status != domain.RfxVersionStatusDraft {
		t.Fatalf("failed publish changed status to %s", reloaded.Status)
	}
}

func TestPublishQuestionnaireEnablesDisabledDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-QE-03")
	ctx := context.Background()

	draft, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("get or create draft: %v", err)
	}
	if draft.QuestionnaireEnabled {
		t.Fatal("studio draft must start disabled")
	}

	section, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN",
		Title:       "Main",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES",
		QuestionType: domain.QuestionTypeText,
		Label:        "Notes",
		Required:     true,
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}

	draft, err = env.qRepo.GetVersionByID(ctx, draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft after content: %v", err)
	}
	if draft.QuestionnaireEnabled {
		t.Fatal("adding sections must not enable questionnaire")
	}
	event, err = env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	in := domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "explicit publish enables questionnaire",
	}
	published, err := env.versionSvc.PublishQuestionnaire(ctx, fix.BuyerA, event.ID, "qe-enable-pub", in)
	if err != nil {
		t.Fatalf("publish questionnaire: %v", err)
	}
	if published.ID != draft.ID {
		t.Fatalf("publish created a new version: draft=%s published=%s", draft.ID, published.ID)
	}
	if published.Status != domain.RfxVersionStatusPublished {
		t.Fatalf("status=%s", published.Status)
	}
	if !published.QuestionnaireEnabled {
		t.Fatal("successful PublishQuestionnaire must set questionnaire_enabled=true")
	}

	definition, err := env.qRepo.LoadQuestionnaire(ctx, published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load published questionnaire: %v", err)
	}
	if len(definition.Sections) == 0 || len(definition.Sections[0].Questions) == 0 {
		t.Fatal("published version must contain sections and questions")
	}

	replay, err := env.versionSvc.PublishQuestionnaire(ctx, fix.BuyerA, event.ID, "qe-enable-pub", in)
	if err != nil {
		t.Fatalf("repeat publish: %v", err)
	}
	if replay.ID != published.ID {
		t.Fatal("repeat PublishQuestionnaire created another version")
	}

	var versionCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_versions WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, event.ID, fix.TenantID).Scan(&versionCount); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("expected one version row, got %d", versionCount)
	}
}

func TestExistingPublishedRowsUnchangedByNewDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	historical := createDraftEvent(t, env, fix, "RFX-QE-04A")
	published := ensurePublishedVersion(t, env, fix, historical.ID, "qe-hist-pub")
	if !published.QuestionnaireEnabled {
		t.Fatal("fixture published row should stay enabled")
	}

	var beforeEnabled bool
	var beforeStatus string
	var beforeVersion int
	var beforePublishedAt time.Time
	if err := env.pool.QueryRow(ctx, `
		SELECT questionnaire_enabled, status, version, published_at
		FROM rfx.rfx_versions
		WHERE id = $1 AND tenant_id = $2
	`, published.ID, fix.TenantID).Scan(&beforeEnabled, &beforeStatus, &beforeVersion, &beforePublishedAt); err != nil {
		t.Fatalf("snapshot historical version: %v", err)
	}

	fresh := createDraftEvent(t, env, fix, "RFX-QE-04B")
	draft, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, fresh.ID)
	if err != nil {
		t.Fatalf("create unrelated draft: %v", err)
	}
	if draft.QuestionnaireEnabled {
		t.Fatal("new draft must stay disabled")
	}
	if draft.ID == published.ID {
		t.Fatal("new draft reused historical version")
	}

	fresh, err = env.rfxRepo.GetEventByID(ctx, fresh.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload fresh event: %v", err)
	}
	if _, err := env.versionSvc.PublishQuestionnaire(ctx, fix.BuyerA, fresh.ID, "qe-hist-fail", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: fresh.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "must fail without questions",
	}); err == nil {
		t.Fatal("empty draft publish must fail")
	}

	var afterEnabled bool
	var afterStatus string
	var afterVersion int
	var afterPublishedAt time.Time
	if err := env.pool.QueryRow(ctx, `
		SELECT questionnaire_enabled, status, version, published_at
		FROM rfx.rfx_versions
		WHERE id = $1 AND tenant_id = $2
	`, published.ID, fix.TenantID).Scan(&afterEnabled, &afterStatus, &afterVersion, &afterPublishedAt); err != nil {
		t.Fatalf("reload historical version: %v", err)
	}
	if afterEnabled != beforeEnabled || afterStatus != beforeStatus || afterVersion != beforeVersion || !afterPublishedAt.Equal(beforePublishedAt) {
		t.Fatalf("historical published row changed: before=(%t %s %d %s) after=(%t %s %d %s)",
			beforeEnabled, beforeStatus, beforeVersion, beforePublishedAt,
			afterEnabled, afterStatus, afterVersion, afterPublishedAt)
	}
}

func TestPublishQuestionnaireDoesNotCreateVersionOnStudioOpen(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-QE-05")
	ctx := context.Background()

	first, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("studio open: %v", err)
	}
	second, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("repeat studio open: %v", err)
	}
	if first.ID != second.ID || second.QuestionnaireEnabled {
		t.Fatal("studio GET/open must not enable or fork a new draft")
	}
	if _, err := env.qRepo.GetPublishedVersionForEvent(ctx, event.ID, fix.TenantID); err == nil {
		t.Fatal("studio open must not publish a version")
	} else {
		assertAppErrorCode(t, err, apperrors.CodeNotFound)
	}
}
