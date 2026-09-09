//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func beginHeldTemplateLock(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) (context.Context, pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	var locked uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM rfx.rfx_templates
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE`, templateID, fix.TenantID).Scan(&locked); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("lock template row: %v", err)
	}
	return ctx, tx
}

func waitForOtherLockWaiters(t *testing.T, env *testEnv, minWaiters int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := env.pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND pid <> pg_backend_pid()`).Scan(&waiting)
		if err == nil && waiting >= minWaiters {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d lock waiters", minWaiters)
}

func countTemplateSectionsByCode(t *testing.T, env *testEnv, templateID uuid.UUID, code string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_template_sections
		WHERE template_id = $1 AND section_code = $2 AND deleted_at IS NULL`,
		templateID, code).Scan(&count); err != nil {
		t.Fatalf("count sections: %v", err)
	}
	return count
}

func countAuditEventsByAction(t *testing.T, env *testEnv, fix buyerFixture, action string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id = $1 AND action = $2`, fix.TenantID, action).Scan(&count); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	return count
}

func publishTemplateInTx(t *testing.T, ctx context.Context, tx pgx.Tx, env *testEnv, fix buyerFixture, templateID uuid.UUID, summary string) *domain.RfxTemplateVersion {
	t.Helper()
	detail, err := env.templateSvc.GetTemplate(ctx, fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if detail.DraftVersion == nil {
		t.Fatal("expected draft version before publish")
	}
	tRepo := env.tmplRepo.WithTx(tx)
	result, err := tRepo.PublishTemplateVersionTx(ctx, templateID, fix.TenantID, detail.Template.Version, detail.DraftVersion.Version, summary, fix.BuyerA.UserID)
	if err != nil {
		t.Fatalf("publish in tx: %v", err)
	}
	return result.Published
}

func archiveTemplateInTx(t *testing.T, ctx context.Context, tx pgx.Tx, env *testEnv, fix buyerFixture, templateID uuid.UUID) {
	t.Helper()
	tRepo := env.tmplRepo.WithTx(tx)
	if _, err := tRepo.ArchiveTemplate(ctx, templateID, fix.TenantID); err != nil {
		t.Fatalf("archive in tx: %v", err)
	}
}

func loadPublishedTemplateQuestionnaire(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) *domain.TemplateQuestionnaireDefinition {
	t.Helper()
	published, err := env.tmplRepo.GetPublishedVersion(context.Background(), templateID, fix.TenantID)
	if err != nil {
		t.Fatalf("published version: %v", err)
	}
	if published == nil {
		t.Fatal("expected published version")
	}
	def, err := env.tmplQRepo.LoadQuestionnaire(context.Background(), templateID, published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load published questionnaire: %v", err)
	}
	return def
}

func questionLabelInPublished(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, questionCode string) string {
	t.Helper()
	def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
	for _, swq := range def.Sections {
		for _, q := range swq.Questions {
			if q.QuestionCode == questionCode {
				return q.Label
			}
		}
	}
	t.Fatalf("question %s not found in published graph", questionCode)
	return ""
}

func TestE4REM030CreateSectionVsPublishPublishWins(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-030", nil)
	templateID := detail.Template.ID

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	createErrCh := make(chan error, 1)
	go func() {
		_, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
			SectionCode: "RACE_LOST",
			Title:       "Race Lost",
		})
		createErrCh <- err
	}()
	waitForOtherLockWaiters(t, env, 1)
	publishTemplateInTx(t, ctx, tx, env, fix, templateID, "publish wins race")
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit publish tx: %v", err)
	}

	expectAppErrorCode(t, <-createErrCh, apperrors.CodeConflict)
	if count := countTemplateSectionsByCode(t, env, templateID, "RACE_LOST"); count != 0 {
		t.Fatalf("expected no race section, got %d", count)
	}
	if count := countAuditEventsByAction(t, env, fix, "rfx.template.section.created.v1"); count != 0 {
		t.Fatalf("expected no section create audit, got %d", count)
	}
}

func TestE4REM031CreateSectionVsPublishMutationWins(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-031", nil)
	templateID := detail.Template.ID

	if _, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "WINNER",
		Title:       "Winner",
	}); err != nil {
		t.Fatalf("create section: %v", err)
	}
	beforeAudit := countAuditEventsByAction(t, env, fix, "rfx.template.section.created.v1")
	published := publishTemplate(t, env, fix, templateID, "rem-031-pub")
	def, err := env.tmplQRepo.LoadQuestionnaire(context.Background(), templateID, published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load published graph: %v", err)
	}
	found := false
	for _, swq := range def.Sections {
		if swq.Section.SectionCode == "WINNER" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("published graph must include mutation section")
	}
	if after := countAuditEventsByAction(t, env, fix, "rfx.template.section.created.v1"); after != beforeAudit {
		t.Fatalf("expected exactly one section create audit, before=%d after=%d", beforeAudit, after)
	}
}

func TestE4REM032UpdateQuestionVsPublishConsistentSnapshot(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-032", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)

	t.Run("publish wins", func(t *testing.T) {
		ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
		updateErrCh := make(chan error, 1)
		go func() {
			questionID := loadDraftQuestionID(t, env, fix, templateID, "FLEET_SIZE")
			_, err := env.templateQSvc.UpdateQuestion(context.Background(), fix.BuyerA, templateID, questionID, domain.UpdateQuestionInput{
				Label:           strPtr("Updated fleet"),
				ExpectedVersion: 1,
			})
			updateErrCh <- err
		}()
		waitForOtherLockWaiters(t, env, 1)
		publishTemplateInTx(t, ctx, tx, env, fix, templateID, "publish wins update race")
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
		expectAppErrorCode(t, <-updateErrCh, apperrors.CodeConflict)
		if label := questionLabelInPublished(t, env, fix, templateID, "FLEET_SIZE"); label != "Fleet size" {
			t.Fatalf("expected original label, got %q", label)
		}
	})

	detail = createTemplate(t, env, fix, "rem-032b", nil)
	templateID = detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)

	t.Run("mutation wins", func(t *testing.T) {
		questionID := loadDraftQuestionID(t, env, fix, templateID, "FLEET_SIZE")
		if _, err := env.templateQSvc.UpdateQuestion(context.Background(), fix.BuyerA, templateID, questionID, domain.UpdateQuestionInput{
			Label:           strPtr("Updated fleet"),
			ExpectedVersion: 1,
		}); err != nil {
			t.Fatalf("update question: %v", err)
		}
		publishTemplate(t, env, fix, templateID, "rem-032b-pub")
		if label := questionLabelInPublished(t, env, fix, templateID, "FLEET_SIZE"); label != "Updated fleet" {
			t.Fatalf("expected updated label in published graph, got %q", label)
		}
	})
}

func TestE4REM033DeleteOptionRuleVsPublishConsistent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-033", nil)
	templateID := detail.Template.ID
	_, question, option, rule := seedOptionRuleGraph(t, env, fix, templateID)

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	deleteErrCh := make(chan error, 2)
	go func() {
		deleteErrCh <- env.templateQSvc.DeleteOption(context.Background(), fix.BuyerA, templateID, question.ID, option.ID, 1)
	}()
	go func() {
		deleteErrCh <- env.templateQSvc.DeleteRule(context.Background(), fix.BuyerA, templateID, rule.ID, 1)
	}()
	waitForOtherLockWaiters(t, env, 2)
	publishTemplateInTx(t, ctx, tx, env, fix, templateID, "publish wins delete race")
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	for i := 0; i < 2; i++ {
		expectAppErrorCode(t, <-deleteErrCh, apperrors.CodeConflict)
	}
	def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
	if len(def.Rules) != 1 {
		t.Fatalf("expected published rule retained, got %d", len(def.Rules))
	}
	for _, swq := range def.Sections {
		for _, q := range swq.Questions {
			if q.ID == question.ID && len(q.Options) != 1 {
				t.Fatalf("expected published option retained, got %d", len(q.Options))
			}
		}
	}
	err := env.templateQSvc.DeleteOption(context.Background(), fix.BuyerA, templateID, question.ID, option.ID, 1)
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4REM034ReorderVsPublishAtomicOrder(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-034", nil)
	templateID := detail.Template.ID
	sortA := 1
	sortB := 2
	secA, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{SectionCode: "A", Title: "A", SortOrder: &sortA})
	if err != nil {
		t.Fatalf("section A: %v", err)
	}
	secB, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{SectionCode: "B", Title: "B", SortOrder: &sortB})
	if err != nil {
		t.Fatalf("section B: %v", err)
	}

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	reorderErrCh := make(chan error, 1)
	go func() {
		reorderErrCh <- env.templateQSvc.ReorderSections(context.Background(), fix.BuyerA, templateID, []uuid.UUID{secB.ID, secA.ID})
	}()
	waitForOtherLockWaiters(t, env, 1)
	publishTemplateInTx(t, ctx, tx, env, fix, templateID, "publish wins reorder")
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	expectAppErrorCode(t, <-reorderErrCh, apperrors.CodeConflict)

	def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
	if len(def.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(def.Sections))
	}
	if def.Sections[0].Section.SectionCode != "A" || def.Sections[1].Section.SectionCode != "B" {
		t.Fatal("published order must remain fully A then B, no partial reorder")
	}
}

func TestE4REM035GraphMutationVsArchiveArchiveWins(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-035", nil)
	templateID := detail.Template.ID

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	createErrCh := make(chan error, 1)
	go func() {
		_, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
			SectionCode: "ARCHIVE_RACE",
			Title:       "Archive Race",
		})
		createErrCh <- err
	}()
	waitForOtherLockWaiters(t, env, 1)
	archiveTemplateInTx(t, ctx, tx, env, fix, templateID)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit archive tx: %v", err)
	}

	expectAppErrorCode(t, <-createErrCh, apperrors.CodeConflict)
	if count := countTemplateSectionsByCode(t, env, templateID, "ARCHIVE_RACE"); count != 0 {
		t.Fatalf("expected no archived-race section, got %d", count)
	}
}

func TestE4REM036GraphMutationWinsBeforeArchive(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-036", nil)
	templateID := detail.Template.ID

	if _, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "BEFORE_ARCHIVE",
		Title:       "Before Archive",
	}); err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, templateID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "AFTER_ARCHIVE",
		Title:       "After Archive",
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
	if count := countTemplateSectionsByCode(t, env, templateID, "BEFORE_ARCHIVE"); count != 1 {
		t.Fatalf("expected pre-archive section retained, got %d", count)
	}
	if count := countTemplateSectionsByCode(t, env, templateID, "AFTER_ARCHIVE"); count != 0 {
		t.Fatalf("expected no post-archive section, got %d", count)
	}
}

func TestE4REM037PublishArchiveGraphMutationNoDeadlock(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-037", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, 3)
	wg.Add(3)
	go func() {
		defer wg.Done()
		_, errs[0] = env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{SectionCode: "DL", Title: "DL"})
	}()
	go func() {
		defer wg.Done()
		reloaded, err := env.templateSvc.GetTemplate(ctx, fix.BuyerA, templateID)
		if err != nil {
			errs[1] = err
			return
		}
		if reloaded.DraftVersion == nil {
			errs[1] = apperrors.Conflict("no draft", nil)
			return
		}
		_, errs[1] = env.templateSvc.PublishTemplateVersion(ctx, fix.BuyerA, templateID, "rem-037-pub", domain.PublishTemplateVersionInput{
			ExpectedTemplateVersion: reloaded.Template.Version,
			ExpectedDraftVersion:    reloaded.DraftVersion.Version,
			ChangeSummary:           "deadlock probe publish",
		})
	}()
	go func() {
		defer wg.Done()
		reloaded, err := env.templateSvc.GetTemplate(ctx, fix.BuyerA, templateID)
		if err != nil {
			errs[2] = err
			return
		}
		_, errs[2] = env.templateSvc.UpdateTemplate(ctx, fix.BuyerA, templateID, domain.UpdateTemplateInput{
			NameI18n:        json.RawMessage(`{"en-US":"Deadlock probe"}`),
			ExpectedVersion: reloaded.Template.Version,
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
		t.Fatal("timed out waiting for concurrent publish/archive/graph operations")
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

func TestE4REM038DuplicateQuestionVsPublishNoPartialRows(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-038", nil)
	templateID := detail.Template.ID
	populateTemplateGraph(t, env, fix, templateID)
	sourceQuestionID := loadDraftQuestionID(t, env, fix, templateID, "FLEET_SIZE")

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	dupErrCh := make(chan error, 1)
	go func() {
		_, err := env.templateQSvc.DuplicateQuestion(context.Background(), fix.BuyerA, templateID, sourceQuestionID)
		dupErrCh <- err
	}()
	waitForOtherLockWaiters(t, env, 1)
	publishTemplateInTx(t, ctx, tx, env, fix, templateID, "publish wins duplicate")
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	expectAppErrorCode(t, <-dupErrCh, apperrors.CodeConflict)

	def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
	for _, swq := range def.Sections {
		if len(swq.Questions) != 1 {
			t.Fatalf("expected one published question, got %d", len(swq.Questions))
		}
	}
	var copyCount int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_template_questions q
		INNER JOIN rfx.rfx_template_sections s ON s.id = q.section_id
		INNER JOIN rfx.rfx_template_versions v ON v.id = s.rfx_template_version_id
		WHERE v.template_id = $1 AND v.status = 'PUBLISHED' AND q.question_code LIKE '%\_copy%' ESCAPE '\'`,
		templateID).Scan(&copyCount); err != nil {
		t.Fatalf("count duplicate questions: %v", err)
	}
	if copyCount != 0 {
		t.Fatalf("expected no duplicate question rows in published version, got %d", copyCount)
	}
}

func TestE4REM039AuditFailureConcurrentMutationRollsBackGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rem-039", nil)
	templateID := detail.Template.ID

	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	ctx, tx := beginHeldTemplateLock(t, env, fix, templateID)
	createErrCh := make(chan error, 1)
	publishErrCh := make(chan error, 1)
	go func() {
		_, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{
			SectionCode: "AUDIT_RACE",
			Title:       "Audit Race",
		})
		createErrCh <- err
	}()
	waitForOtherLockWaiters(t, env, 1)
	go func() {
		_, pubErr := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, templateID, "rem-039-pub", domain.PublishTemplateVersionInput{
			ExpectedTemplateVersion: detail.Template.Version,
			ExpectedDraftVersion:    detail.DraftVersion.Version,
			ChangeSummary:           "publish after audit race",
		})
		publishErrCh <- pubErr
	}()
	waitForOtherLockWaiters(t, env, 2)
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback held lock: %v", err)
	}

	createErr := <-createErrCh
	if createErr == nil {
		t.Fatal("expected audit failure on create section")
	}
	publishErr := <-publishErrCh
	if publishErr != nil {
		t.Fatalf("publish should succeed on clean graph: %v", publishErr)
	}
	if count := countTemplateSectionsByCode(t, env, templateID, "AUDIT_RACE"); count != 0 {
		t.Fatalf("expected rolled-back section absent, got %d", count)
	}
	def := loadPublishedTemplateQuestionnaire(t, env, fix, templateID)
	for _, swq := range def.Sections {
		if swq.Section.SectionCode == "AUDIT_RACE" {
			t.Fatal("published graph must not include rolled-back section")
		}
	}
}

func loadDraftQuestionID(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, questionCode string) uuid.UUID {
	t.Helper()
	detail, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if detail.DraftVersion == nil {
		t.Fatal("expected draft version")
	}
	def, err := env.tmplQRepo.LoadQuestionnaire(context.Background(), templateID, detail.DraftVersion.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load draft questionnaire: %v", err)
	}
	for _, swq := range def.Sections {
		for _, q := range swq.Questions {
			if q.QuestionCode == questionCode {
				return q.ID
			}
		}
	}
	t.Fatalf("question %s not found", questionCode)
	return uuid.Nil
}

func seedOptionRuleGraph(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) (*domain.TemplateSection, *domain.TemplateQuestion, *domain.TemplateQuestionOption, *domain.TemplateQuestionRule) {
	t.Helper()
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, templateID, domain.CreateSectionInput{SectionCode: "S", Title: "S"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, templateID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q", QuestionType: domain.QuestionTypeSingleSelect, Label: "Q", Required: true,
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	option, err := env.templateQSvc.CreateOption(context.Background(), fix.BuyerA, templateID, question.ID, domain.CreateQuestionOptionInput{
		OptionCode: "O1", Label: "One",
	})
	if err != nil {
		t.Fatalf("option: %v", err)
	}
	target := "Q"
	rule, err := env.templateQSvc.CreateRule(context.Background(), fix.BuyerA, templateID, domain.CreateQuestionRuleInput{
		RuleCode:           "R1",
		Action:             domain.RuleActionShow,
		TargetQuestionCode: &target,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q","value":"1"}`),
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	return section, question, option, rule
}
