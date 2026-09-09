//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func createTemplateAs(t *testing.T, env *testEnv, fix buyerFixture, actor domain.ActorContext, code string, owner *uuid.UUID) *domain.TemplateDetail {
	t.Helper()
	detail, err := env.templateSvc.CreateTemplate(context.Background(), actor, createTemplateInput(code, owner))
	if err != nil {
		t.Fatalf("create template %s: %v", code, err)
	}
	return detail
}

func containsTemplateID(items []domain.RfxTemplate, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func TestE4REM001BuyerAListExcludesCompanyBTemplate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ownedB := createTemplateAs(t, env, fix, fix.BuyerB, "company-b-owned", &fix.CompanyB)
	items, _, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if containsTemplateID(items, ownedB.Template.ID) {
		t.Fatal("buyer A list must not include company B template")
	}
}

func TestE4REM002BuyerASearchExactCodeBReturnsZero(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplateAs(t, env, fix, fix.BuyerB, "secret-b-code", &fix.CompanyB)
	search := "secret-b-code"
	items, total, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{Search: &search})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("expected zero search hits, len=%d total=%d", len(items), total)
	}
}

func TestE4REM003SearchCountAlsoZero(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplateAs(t, env, fix, fix.BuyerB, "count-hidden", &fix.CompanyB)
	search := "count-hidden"
	_, total, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{Search: &search})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total=0, got %d", total)
	}
}

func TestE4REM004TenantWideVisibleToBuyerAAndB(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	tenantWide := createTemplate(t, env, fix, "tenant-wide-rem", nil)
	for _, actor := range []domain.ActorContext{fix.BuyerA, fix.BuyerB} {
		items, _, err := env.templateSvc.ListTemplates(context.Background(), actor, domain.TemplateListFilter{})
		if err != nil {
			t.Fatalf("list for %s: %v", actor.UserID, err)
		}
		if !containsTemplateID(items, tenantWide.Template.ID) {
			t.Fatalf("tenant-wide template missing for buyer %s", actor.UserID)
		}
	}
}

func TestE4REM005OwnerCompanyFilterFromBuyerADoesNotRevealB(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ownedB := createTemplateAs(t, env, fix, fix.BuyerB, "filter-b-owned", &fix.CompanyB)
	items, total, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{OwnerCompanyID: &fix.CompanyB})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("expected empty filter result, len=%d total=%d", len(items), total)
	}
	if containsTemplateID(items, ownedB.Template.ID) {
		t.Fatal("company B template leaked via owner_company_id filter")
	}
}

func TestE4REM006CarrierList403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplate(t, env, fix, "carrier-list-rem", nil)
	_, _, err := env.templateSvc.ListTemplates(context.Background(), fix.CarrierAct, domain.TemplateListFilter{})
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func seedPlatformAdmin(t *testing.T, env *testEnv, fix buyerFixture, withCompanyMembership bool) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	adminID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		adminID, fix.TenantID, "platform-admin@test.local", "Platform Admin"); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	var adminRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PLATFORM_ADMIN' LIMIT 1`).Scan(&adminRoleID); err != nil {
		t.Fatalf("lookup platform admin role: %v", err)
	}
	companyID := fix.CompanyA
	if withCompanyMembership {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
			fix.TenantID, fix.CompanyA, adminID); err != nil {
			t.Fatalf("seed admin membership: %v", err)
		}
	} else {
		companyID = uuid.Nil
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, adminID, companyID, adminRoleID); err != nil {
		t.Fatalf("seed admin role: %v", err)
	}
	return domain.ActorContext{TenantID: fix.TenantID, UserID: adminID}
}

func TestE4REM007AdminAccessFollowsMembershipModel(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	tenantWide := createTemplate(t, env, fix, "admin-tenant-wide", nil)
	ownedA := createTemplateAs(t, env, fix, fix.BuyerA, "admin-owned-a", &fix.CompanyA)
	ownedB := createTemplateAs(t, env, fix, fix.BuyerB, "admin-owned-b", &fix.CompanyB)

	adminWithMembership := seedPlatformAdmin(t, env, fix, true)
	items, _, err := env.templateSvc.ListTemplates(context.Background(), adminWithMembership, domain.TemplateListFilter{})
	if err != nil {
		t.Fatalf("admin with membership list: %v", err)
	}
	if !containsTemplateID(items, tenantWide.Template.ID) || !containsTemplateID(items, ownedA.Template.ID) {
		t.Fatal("admin with company A membership should see tenant-wide and company A templates")
	}
	if containsTemplateID(items, ownedB.Template.ID) {
		t.Fatal("admin with company A membership must not see company B template")
	}

	adminNoMembership := seedPlatformAdmin(t, env, fix, false)
	_, _, err = env.templateSvc.ListTemplates(context.Background(), adminNoMembership, domain.TemplateListFilter{})
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE4REM008PaginationPreservesAccessPredicate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplate(t, env, fix, "page-a-1", nil)
	createTemplateAs(t, env, fix, fix.BuyerA, "page-a-owned", &fix.CompanyA)
	createTemplateAs(t, env, fix, fix.BuyerB, "page-b-hidden", &fix.CompanyB)
	page1, total, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{Limit: 1, Offset: 0})
	if err != nil || total < 2 {
		t.Fatalf("page1: err=%v total=%d", err, total)
	}
	page2, total2, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{Limit: 1, Offset: 1})
	if err != nil || total2 != total {
		t.Fatalf("page2: err=%v total=%d total2=%d", err, total, total2)
	}
	for _, page := range [][]domain.RfxTemplate{page1, page2} {
		for _, item := range page {
			if item.OwnerCompanyID != nil && *item.OwnerCompanyID == fix.CompanyB {
				t.Fatal("pagination leaked company B template")
			}
		}
	}
}

func TestE4REM009MetadataUpdateAuditFailureRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "audit-meta-rollback", nil)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err := env.templateSvc.UpdateTemplate(context.Background(), fix.BuyerA, detail.Template.ID, domain.UpdateTemplateInput{
		NameI18n:        json.RawMessage(`{"en-US":"Rollback name"}`),
		ExpectedVersion: detail.Template.Version,
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	reloaded, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Template.Version != detail.Template.Version {
		t.Fatalf("version changed on failed audit: before=%d after=%d", detail.Template.Version, reloaded.Template.Version)
	}
}

func TestE4REM010ArchiveAuditFailureRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "audit-archive-rollback", nil)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err == nil {
		t.Fatal("expected audit failure")
	}
	reloaded, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Template.Status != domain.RfxTemplateStatusActive {
		t.Fatalf("status changed on failed audit: %s", reloaded.Template.Status)
	}
}

func TestE4REM011SoftDeleteAuditFailureRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "audit-delete-rollback", nil)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	err := env.templateSvc.DeleteTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err == nil {
		t.Fatal("expected audit failure")
	}
	if _, getErr := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID); getErr != nil {
		t.Fatalf("template should remain readable after failed delete: %v", getErr)
	}
}

func TestE4REM012SectionCreateAuditFailureRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "audit-section-rollback", nil)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{
		SectionCode: "ROLLBACK",
		Title:       "Rollback",
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_template_sections
		WHERE template_id = $1 AND section_code = $2 AND deleted_at IS NULL`,
		detail.Template.ID, "ROLLBACK").Scan(&count); err != nil {
		t.Fatalf("count sections: %v", err)
	}
	if count != 0 {
		t.Fatalf("section persisted after audit failure, count=%d", count)
	}
}

func TestE4REM013QuestionUpdateAuditFailureRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "audit-question-rollback", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{
		SectionCode: "GENERAL",
		Title:       "General",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q1",
		QuestionType: domain.QuestionTypeText,
		Label:        "Original",
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	label := "Changed"
	_, err = env.templateQSvc.UpdateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, question.ID, domain.UpdateQuestionInput{
		Label:           &label,
		ExpectedVersion: question.Version,
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	reloaded, err := env.templateQSvc.GetQuestionnaire(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload questionnaire: %v", err)
	}
	if len(reloaded.Sections) != 1 || len(reloaded.Sections[0].Questions) != 1 || reloaded.Sections[0].Questions[0].Label != "Original" {
		t.Fatal("question label changed after failed audit")
	}
}

func TestE4REM014ReorderAuditFailureRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "audit-reorder-rollback", nil)
	secA, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{SectionCode: "A", Title: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	secB, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{SectionCode: "B", Title: "B"})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	err = env.templateQSvc.ReorderSections(context.Background(), fix.BuyerA, detail.Template.ID, []uuid.UUID{secB.ID, secA.ID})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	sections, err := env.tmplQRepo.ListSections(context.Background(), detail.Template.ID, detail.DraftVersion.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("list sections: %v", err)
	}
	if len(sections) != 2 || sections[0].SectionCode != "A" || sections[1].SectionCode != "B" {
		t.Fatalf("section order changed after failed audit: %+v", sections)
	}
}

func TestE4REM015CrossTenantQuestionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-q-tenant", nil)
	otherTenant := uuid.New()
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_questions (
			tenant_id, template_id, rfx_template_version_id, section_id, question_code, question_type, label
		)
		SELECT $1, template_id, rfx_template_version_id, id, 'BAD', 'TEXT', 'Bad'
		FROM rfx.rfx_template_sections
		WHERE template_id = $2 AND deleted_at IS NULL LIMIT 1`,
		otherTenant, detail.Template.ID)
	if err == nil {
		t.Fatal("expected cross-tenant question insert denied")
	}
}

func TestE4REM016CrossTemplateQuestionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detailA := createTemplate(t, env, fix, "fk-template-a", nil)
	detailB := createTemplate(t, env, fix, "fk-template-b", nil)
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_questions (
			tenant_id, template_id, rfx_template_version_id, section_id, question_code, question_type, label
		)
		SELECT s.tenant_id, $3, s.rfx_template_version_id, s.id, 'BAD', 'TEXT', 'Bad'
		FROM rfx.rfx_template_sections s
		WHERE s.template_id = $1 AND s.deleted_at IS NULL LIMIT 1`,
		detailA.Template.ID, detailB.Template.ID)
	if err == nil {
		t.Fatal("expected cross-template question insert denied")
	}
}

func TestE4REM017CrossVersionQuestionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-version", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, "rem-017-pub")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "rem-017-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_questions (
			tenant_id, template_id, rfx_template_version_id, section_id, question_code, question_type, label
		)
		SELECT tenant_id, template_id, $3, id, 'BAD', 'TEXT', 'Bad'
		FROM rfx.rfx_template_sections
		WHERE rfx_template_version_id = $2 AND deleted_at IS NULL LIMIT 1`,
		fix.TenantID, draft.ID, published.ID)
	if err == nil {
		t.Fatal("expected cross-version question insert denied")
	}
}

func TestE4REM018CrossTenantOptionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-opt-tenant", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{SectionCode: "S", Title: "S"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q", QuestionType: domain.QuestionTypeText, Label: "Q",
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	otherTenant := uuid.New()
	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_question_options (tenant_id, question_id, option_code, label)
		VALUES ($1, $2, 'BAD', 'Bad')`, otherTenant, question.ID)
	if err == nil {
		t.Fatal("expected cross-tenant option insert denied")
	}
}

func TestE4REM019OptionForForeignVersionQuestionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-opt-version", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, "rem-019-pub")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "rem-019-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	var draftSectionID uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `
		SELECT id FROM rfx.rfx_template_sections
		WHERE rfx_template_version_id = $1 AND deleted_at IS NULL LIMIT 1`, draft.ID).Scan(&draftSectionID); err != nil {
		t.Fatalf("lookup draft section: %v", err)
	}
	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_questions (
			tenant_id, template_id, rfx_template_version_id, section_id, question_code, question_type, label
		) VALUES ($1, $2, $3, $4, 'BAD', 'TEXT', 'Bad')`,
		fix.TenantID, detail.Template.ID, published.ID, draftSectionID)
	if err == nil {
		t.Fatal("expected cross-version question insert denied before option path")
	}
}

func insertRuleTarget(ctx context.Context, env *testEnv, fix buyerFixture, templateID, versionID, targetQuestionID uuid.UUID, ruleCode string) error {
	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_template_question_rules (
			tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action
		) VALUES ($1, $2, $3, $4, $5, 'SHOW')`,
		fix.TenantID, templateID, versionID, targetQuestionID, ruleCode)
	return err
}

func TestE4REM020RuleCrossTenantTargetDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-rule-tenant", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{SectionCode: "S", Title: "S"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q", QuestionType: domain.QuestionTypeText, Label: "Q",
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	otherTenant := uuid.New()
	_, err = env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_question_rules (
			tenant_id, template_id, rfx_template_version_id, target_question_id, rule_code, action
		) VALUES ($1, $2, $3, $4, $5, 'SHOW')`,
		otherTenant, detail.Template.ID, detail.DraftVersion.ID, question.ID, "R1")
	if err == nil {
		t.Fatal("expected cross-tenant rule target denied")
	}
}

func TestE4REM021RuleCrossTemplateTargetDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detailA := createTemplate(t, env, fix, "fk-rule-tpl-a", nil)
	detailB := createTemplate(t, env, fix, "fk-rule-tpl-b", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detailB.Template.ID, domain.CreateSectionInput{SectionCode: "S", Title: "S"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	foreignQuestion, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detailB.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q", QuestionType: domain.QuestionTypeText, Label: "Q",
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	err = insertRuleTarget(context.Background(), env, fix, detailA.Template.ID, detailA.DraftVersion.ID, foreignQuestion.ID, "R1")
	if err == nil {
		t.Fatal("expected cross-template rule target denied")
	}
}

func TestE4REM022RuleCrossVersionTargetDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-rule-version", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "rem-022-pub")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "rem-022-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	var publishedQuestionID uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `
		SELECT q.id FROM rfx.rfx_template_questions q
		INNER JOIN rfx.rfx_template_sections s ON s.id = q.section_id
		WHERE s.rfx_template_version_id = (
			SELECT id FROM rfx.rfx_template_versions WHERE template_id = $1 AND status = 'PUBLISHED' LIMIT 1
		) LIMIT 1`, detail.Template.ID).Scan(&publishedQuestionID); err != nil {
		t.Fatalf("lookup published question: %v", err)
	}
	err = insertRuleTarget(context.Background(), env, fix, detail.Template.ID, draft.ID, publishedQuestionID, "R1")
	if err == nil {
		t.Fatal("expected cross-version rule target denied")
	}
}

func TestE4REM023ValidSameVersionGraphAccepted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fk-valid-graph", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{SectionCode: "S", Title: "S"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q", QuestionType: domain.QuestionTypeText, Label: "Q",
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	if _, err := env.templateQSvc.CreateOption(context.Background(), fix.BuyerA, detail.Template.ID, question.ID, domain.CreateQuestionOptionInput{
		OptionCode: "O1", Label: "One",
	}); err != nil {
		t.Fatalf("option: %v", err)
	}
	target := "Q"
	if _, err := env.templateQSvc.CreateRule(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateQuestionRuleInput{
		RuleCode: "R1", Action: domain.RuleActionShow, TargetQuestionCode: &target, ConditionJSON: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("rule: %v", err)
	}
}

func TestE4REM024MigrationDownWithSearchPathPublic(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	if err := applyMigrationFile(ctx, env.pool, "000070_rfx_template_library_v3_0e4.up.sql"); err != nil {
		t.Fatalf("apply up: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `SET search_path = public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000070_rfx_template_library_v3_0e4.down.sql"); err != nil {
		t.Fatalf("apply down: %v", err)
	}
	var templatesExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_templates'
		)`).Scan(&templatesExists); err != nil {
		t.Fatalf("check templates removed: %v", err)
	}
	if templatesExists {
		t.Fatal("expected templates removed after down migration with search_path=public")
	}
}

func TestE4REM025ForkCreatedByIsForkActor(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplateAs(t, env, fix, fix.BuyerA, "fork-provenance", &fix.CompanyA)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "rem-025-pub")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerB, detail.Template.ID, "rem-025-fork")
	if err != nil {
		t.Fatalf("fork by buyer B: %v", err)
	}
	if draft.CreatedBy != fix.BuyerB.UserID {
		t.Fatalf("created_by=%s want fork actor %s", draft.CreatedBy, fix.BuyerB.UserID)
	}
}

func TestE4REM026ForkAuditActorMatchesForkActor(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fork-audit", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "rem-026-pub")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerB, detail.Template.ID, "rem-026-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	events, err := env.auditRepo.ListByEntity(context.Background(), fix.TenantID, "rfx_template_version", draft.ID, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(events) == 0 || events[0].ActorUserID == nil || *events[0].ActorUserID != fix.BuyerB.UserID {
		t.Fatalf("expected fork audit actor=%s events=%+v", fix.BuyerB.UserID, events)
	}
}

func TestE4REM027ForkReplayPreservesDraftAndCreatedBy(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fork-replay", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "rem-027-pub")
	key := "rem-027-fork"
	first, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerB, detail.Template.ID, key)
	if err != nil {
		t.Fatalf("first fork: %v", err)
	}
	second, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerB, detail.Template.ID, key)
	if err != nil {
		t.Fatalf("replay fork: %v", err)
	}
	if first.ID != second.ID || first.CreatedBy != second.CreatedBy || second.CreatedBy != fix.BuyerB.UserID {
		t.Fatalf("replay mismatch first=%+v second=%+v", first, second)
	}
}

func TestE4REM028SourceCreatedByUnchangedAfterFork(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplateAs(t, env, fix, fix.BuyerA, "source-provenance", &fix.CompanyA)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, "rem-028-pub")
	sourceCreatedBy := published.CreatedBy
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerB, detail.Template.ID, "rem-028-fork"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	reloaded, err := env.tmplRepo.GetVersionByID(context.Background(), published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload source version: %v", err)
	}
	if reloaded.CreatedBy != sourceCreatedBy {
		t.Fatalf("source created_by changed: before=%s after=%s", sourceCreatedBy, reloaded.CreatedBy)
	}
}

func TestE4REM029DuplicateQuestionGeneratesUniqueCode(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "dup-code-safe", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{SectionCode: "S", Title: "S"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET", QuestionType: domain.QuestionTypeText, Label: "Fleet",
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	first, err := env.templateQSvc.DuplicateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, question.ID)
	if err != nil {
		t.Fatalf("first duplicate: %v", err)
	}
	second, err := env.templateQSvc.DuplicateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, question.ID)
	if err != nil {
		t.Fatalf("second duplicate: %v", err)
	}
	if first.QuestionCode == second.QuestionCode {
		t.Fatalf("duplicate codes collided: %s", first.QuestionCode)
	}
}
