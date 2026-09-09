//go:build integration

package templatelibrary

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func countIdempotencyRecords(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, key string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, templateID, key).Scan(&count); err != nil {
		t.Fatalf("count idempotency records: %v", err)
	}
	return count
}

func loadDraftSectionByCode(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, sectionCode string) *domain.TemplateSection {
	t.Helper()
	detail, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	def, err := env.tmplQRepo.LoadQuestionnaire(context.Background(), templateID, detail.DraftVersion.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load draft questionnaire: %v", err)
	}
	for _, swq := range def.Sections {
		if swq.Section.SectionCode == sectionCode {
			return &swq.Section
		}
	}
	t.Fatalf("section %s not found", sectionCode)
	return nil
}

func TestE4REM040PublishLocksGraphBeforeDestructiveMutation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-040", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)
	section := loadDraftSectionByCode(t, env, fix, templateID, "GENERAL")

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	deleteErrCh := make(chan error, 1)
	go func() {
		deleteErrCh <- env.templateQSvc.DeleteSection(context.Background(), fix.BuyerA, templateID, section.ID, section.Version)
	}()
	waitForOtherLockWaiters(t, env, 1)
	publishTemplateInTx(t, ctx, tx, env, fix, templateID, "publish locks graph first")
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit publish tx: %v", err)
	}

	expectAppErrorCode(t, <-deleteErrCh, apperrors.CodeConflict)
	def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
	if len(def.Sections) != 1 || def.Sections[0].Section.SectionCode != "GENERAL" {
		t.Fatal("published graph must retain validated pre-mutation section")
	}
}

func TestE4REM041PublishReadinessFailsAfterMutationMakesGraphUnready(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-041", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)
	questionID := loadDraftQuestionID(t, env, fix, templateID, "FLEET_SIZE")

	if err := env.templateQSvc.DeleteQuestion(context.Background(), fix.BuyerA, templateID, questionID, 1); err != nil {
		t.Fatalf("delete question: %v", err)
	}
	reloaded, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	_, err = env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, templateID, "rem-041-pub", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: reloaded.Template.Version,
		ExpectedDraftVersion:    reloaded.DraftVersion.Version,
		ChangeSummary:           "should fail readiness",
	})
	expectAppErrorCode(t, err, apperrors.CodeValidation)
	if published, err := env.tmplRepo.GetPublishedVersion(context.Background(), templateID, fix.TenantID); err != nil {
		t.Fatalf("published lookup: %v", err)
	} else if published != nil {
		t.Fatal("PUBLISHED version must not be created on readiness failure")
	}
}

func TestE4REM042PublishIncludesMutationReadyGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-042", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)

	if _, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "READY_WINNER",
		Title:       "Ready Winner",
	}); err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, templateID, loadDraftSectionByCode(t, env, fix, templateID, "READY_WINNER").ID, domain.CreateQuestionInput{
		QuestionCode: "READY_Q",
		QuestionType: domain.QuestionTypeText,
		Label:        "Ready Q",
		Required:     true,
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}

	published := publishTemplate(t, env, fix, templateID, "rem-042-pub")
	def, err := env.tmplQRepo.LoadQuestionnaire(context.Background(), templateID, published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load published graph: %v", err)
	}
	foundSection := false
	foundQuestion := false
	for _, swq := range def.Sections {
		if swq.Section.SectionCode == "READY_WINNER" {
			foundSection = true
		}
		for _, q := range swq.Questions {
			if q.QuestionCode == "READY_Q" {
				foundQuestion = true
			}
		}
	}
	if !foundSection || !foundQuestion {
		t.Fatal("published graph must include post-mutation ready section and question")
	}
}

func TestE4REM043ReadinessFailureRollsBackPublishIdempotencyAndAudit(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-043", nil)
	templateID := detail.Template.ID
	key := "rem-043-pub"

	beforeIdem := countIdempotencyRecords(t, env, fix, templateID, key)
	beforePublishAudit := countAuditEventsByAction(t, env, fix, "rfx.template.version.published.v1")

	_, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, templateID, key, domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    detail.DraftVersion.Version,
		ChangeSummary:           "empty graph readiness fail",
	})
	expectAppErrorCode(t, err, apperrors.CodeValidation)

	if afterIdem := countIdempotencyRecords(t, env, fix, templateID, key); afterIdem != beforeIdem {
		t.Fatalf("idempotency record must not persist on readiness failure, before=%d after=%d", beforeIdem, afterIdem)
	}
	if afterAudit := countAuditEventsByAction(t, env, fix, "rfx.template.version.published.v1"); afterAudit != beforePublishAudit {
		t.Fatalf("publish audit must not persist on readiness failure, before=%d after=%d", beforePublishAudit, afterAudit)
	}
	if published, err := env.tmplRepo.GetPublishedVersion(context.Background(), templateID, fix.TenantID); err != nil {
		t.Fatalf("published lookup: %v", err)
	} else if published != nil {
		t.Fatal("PUBLISHED version must not exist after readiness failure")
	}
}

func TestE4REM044ConcurrentPublishAndGraphMutationNoPartialState(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-044", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		reloaded, err := env.templateSvc.GetTemplate(ctx, fix.BuyerA, templateID)
		if err != nil {
			errs[0] = err
			return
		}
		_, errs[0] = env.templateSvc.PublishTemplateVersion(ctx, fix.BuyerA, templateID, "rem-044-pub", domain.PublishTemplateVersionInput{
			ExpectedTemplateVersion: reloaded.Template.Version,
			ExpectedDraftVersion:    reloaded.DraftVersion.Version,
			ChangeSummary:           "concurrent publish",
		})
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
			SectionCode: "RACE_044",
			Title:       "Race 044",
		})
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timed out waiting for concurrent publish and graph mutation")
	}

	published, _ := env.tmplRepo.GetPublishedVersion(context.Background(), templateID, fix.TenantID)
	raceSectionCount := countTemplateSectionsByCode(t, env, templateID, "RACE_044")
	for i, err := range errs {
		if err == nil {
			continue
		}
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("unexpected error[%d]: %v", i, err)
		}
		if appErr.Code != apperrors.CodeConflict && appErr.Code != apperrors.CodeValidation {
			t.Fatalf("unexpected error code[%d]: %s", i, appErr.Code)
		}
	}
	if published != nil {
		def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
		if raceSectionCount > 0 {
			for _, swq := range def.Sections {
				if swq.Section.SectionCode == "RACE_044" {
					t.Fatal("published graph must not include concurrent mutation section")
				}
			}
		}
		if raceSectionCount == 0 {
			if len(def.Sections) != 1 || def.Sections[0].Section.SectionCode != "GENERAL" {
				t.Fatal("published graph must be fully consistent with validated ready state")
			}
		}
	}
}

func TestE4REM045PublishArchiveGraphMutationNoDeadlock(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-045", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, 3)
	wg.Add(3)
	go func() {
		defer wg.Done()
		reloaded, err := env.templateSvc.GetTemplate(ctx, fix.BuyerA, templateID)
		if err != nil {
			errs[0] = err
			return
		}
		_, errs[0] = env.templateSvc.PublishTemplateVersion(ctx, fix.BuyerA, templateID, "rem-045-pub", domain.PublishTemplateVersionInput{
			ExpectedTemplateVersion: reloaded.Template.Version,
			ExpectedDraftVersion:    reloaded.DraftVersion.Version,
			ChangeSummary:           "archive race publish",
		})
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = env.templateSvc.ArchiveTemplate(ctx, fix.BuyerA, templateID)
	}()
	go func() {
		defer wg.Done()
		_, errs[2] = env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
			SectionCode: "ARCHIVE_RACE",
			Title:       "Archive Race",
		})
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timed out waiting for publish/archive/graph mutation concurrency")
	}
	for i, err := range errs {
		if err == nil {
			continue
		}
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("unexpected error[%d]: %v", i, err)
		}
		if appErr.Code != apperrors.CodeConflict && appErr.Code != apperrors.CodeValidation {
			t.Fatalf("unexpected error code[%d]: %s", i, appErr.Code)
		}
	}
}
