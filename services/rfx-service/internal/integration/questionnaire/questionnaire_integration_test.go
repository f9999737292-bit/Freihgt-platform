//go:build integration

package questionnaire

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestSectionCRUDAndReorder(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-SEC-1")

	sec1, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "HSE",
		Title:       "Health & Safety",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if sec1.SectionCode != "HSE" {
		t.Fatalf("unexpected section code %s", sec1.SectionCode)
	}

	def, err := env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire: %v", err)
	}
	if len(def.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(def.Sections))
	}

	title := "Updated HSE"
	updated, err := env.qSvc.UpdateSection(ctx, fix.BuyerA, event.ID, sec1.ID, domain.UpdateSectionInput{
		Title:           &title,
		ExpectedVersion: sec1.Version,
	})
	if err != nil {
		t.Fatalf("update section: %v", err)
	}
	if updated.Title != title {
		t.Fatalf("title not updated")
	}

	sec2, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "COMPLIANCE",
		Title:       "Compliance",
	})
	if err != nil {
		t.Fatalf("create second section: %v", err)
	}

	if err := env.qSvc.ReorderSections(ctx, fix.BuyerA, event.ID, []uuid.UUID{sec2.ID, sec1.ID}); err != nil {
		t.Fatalf("reorder sections: %v", err)
	}
	def, err = env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire after reorder: %v", err)
	}
	if len(def.Sections) != 2 {
		t.Fatalf("expected 2 sections")
	}
	if def.Sections[0].Section.SectionCode != "COMPLIANCE" || def.Sections[1].Section.SectionCode != "HSE" {
		t.Fatalf("unexpected section order: %s, %s", def.Sections[0].Section.SectionCode, def.Sections[1].Section.SectionCode)
	}

	sec2Version := def.Sections[0].Section.Version
	if err := env.qSvc.DeleteSection(ctx, fix.BuyerA, event.ID, sec2.ID, sec2Version); err != nil {
		t.Fatalf("delete empty section: %v", err)
	}
	def, err = env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire after delete: %v", err)
	}
	if len(def.Sections) != 1 {
		t.Fatalf("expected 1 section after delete, got %d", len(def.Sections))
	}
}

func TestQuestionCRUDDuplicateReorderDelete(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-Q-1")

	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN",
		Title:       "Main",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}

	q1, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES",
		QuestionType: domain.QuestionTypeText,
		Label:        "Notes",
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}

	help := "Updated help"
	label := "Notes updated"
	required := true
	q1, err = env.qSvc.UpdateQuestion(ctx, fix.BuyerA, event.ID, q1.ID, domain.UpdateQuestionInput{
		Label:           &label,
		HelpText:        &help,
		Required:        &required,
		ExpectedVersion: q1.Version,
	})
	if err != nil {
		t.Fatalf("update question: %v", err)
	}
	if q1.Label != label || q1.HelpText == nil || *q1.HelpText != help || !q1.Required {
		t.Fatalf("question update not persisted")
	}

	dup, err := env.qSvc.DuplicateQuestion(ctx, fix.BuyerA, event.ID, q1.ID)
	if err != nil {
		t.Fatalf("duplicate question: %v", err)
	}
	if dup.QuestionCode == q1.QuestionCode {
		t.Fatalf("duplicate should have new code")
	}

	def, err := env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire: %v", err)
	}
	if len(def.Sections[0].Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(def.Sections[0].Questions))
	}

	if err := env.qSvc.ReorderQuestions(ctx, fix.BuyerA, event.ID, sec.ID, []uuid.UUID{dup.ID, q1.ID}); err != nil {
		t.Fatalf("reorder questions: %v", err)
	}
	def, err = env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire after reorder: %v", err)
	}
	if def.Sections[0].Questions[0].ID != dup.ID {
		t.Fatalf("question reorder failed")
	}

	dupVersion := def.Sections[0].Questions[0].Version
	if err := env.qSvc.DeleteQuestion(ctx, fix.BuyerA, event.ID, dup.ID, dupVersion); err != nil {
		t.Fatalf("delete question: %v", err)
	}
	def, err = env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire after delete: %v", err)
	}
	if len(def.Sections[0].Questions) != 1 {
		t.Fatalf("expected 1 question after delete")
	}
	if def.Sections[0].Questions[0].TenantID != fix.TenantID {
		t.Fatalf("tenant_id not preserved")
	}
}

func TestOptionsAndDuplicateOptionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-OPT-1")

	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "SELECT",
		Title:       "Select",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	q, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "CHOICE",
		QuestionType: domain.QuestionTypeSingleSelect,
		Label:        "Choice",
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}

	for _, opt := range []struct{ code, label string }{{"YES", "Yes"}, {"NO", "No"}} {
		if _, err := env.qSvc.CreateOption(ctx, fix.BuyerA, event.ID, q.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.code,
			Label:      opt.label,
		}); err != nil {
			t.Fatalf("create option %s: %v", opt.code, err)
		}
	}

	def, err := env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get questionnaire: %v", err)
	}
	if len(def.Sections[0].Questions[0].Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(def.Sections[0].Questions[0].Options))
	}

	_, err = env.qSvc.CreateOption(ctx, fix.BuyerA, event.ID, q.ID, domain.CreateQuestionOptionInput{
		OptionCode: "YES",
		Label:      "Duplicate Yes",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestRuleEngineValidAndDeniedCases(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-RULE-1")

	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "HSE",
		Title:       "HSE",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}

	adrAvailable, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_AVAILABLE",
		QuestionType: domain.QuestionTypeYesNo,
		Label:        "ADR available?",
	})
	if err != nil {
		t.Fatalf("create ADR_AVAILABLE: %v", err)
	}
	adrNumber, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_NUMBER",
		QuestionType: domain.QuestionTypeText,
		Label:        "ADR number",
	})
	if err != nil {
		t.Fatalf("create ADR_NUMBER: %v", err)
	}
	adrExpiry, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_EXPIRY",
		QuestionType: domain.QuestionTypeDate,
		Label:        "ADR expiry",
	})
	if err != nil {
		t.Fatalf("create ADR_EXPIRY: %v", err)
	}

	targetNumber := "ADR_NUMBER"
	condAvailableTrue := json.RawMessage(`{"operator":"EQUALS","source_question_code":"ADR_AVAILABLE","value":true}`)
	if _, err := env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:           "REQ_ADR_NUMBER",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &targetNumber,
		ConditionJSON:      condAvailableTrue,
	}); err != nil {
		t.Fatalf("create valid rule 1: %v", err)
	}

	targetExpiry := "ADR_EXPIRY"
	if _, err := env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:           "REQ_ADR_EXPIRY",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &targetExpiry,
		ConditionJSON:      condAvailableTrue,
	}); err != nil {
		t.Fatalf("create valid rule 2: %v", err)
	}

	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:      "BAD_REF",
		Action:        domain.RuleActionShow,
		ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"MISSING_CODE","value":true}`),
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)

	selfTarget := adrNumber.QuestionCode
	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:           "SELF_REF",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &selfTarget,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"ADR_NUMBER","value":"x"}`),
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)

	// Cycle: ADR_NUMBER depends on ADR_EXPIRY, ADR_EXPIRY depends on ADR_NUMBER
	targetForCycleA := adrExpiry.QuestionCode
	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:           "CYCLE_A",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &targetForCycleA,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"ADR_NUMBER","value":"1"}`),
	})
	if err != nil {
		t.Fatalf("create cycle leg A: %v", err)
	}
	targetForCycleB := adrNumber.QuestionCode
	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:           "CYCLE_B",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &targetForCycleB,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"ADR_EXPIRY","value":"2026-01-01"}`),
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)

	_, err = env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode:      "TYPE_MISMATCH",
		Action:        domain.RuleActionShow,
		ConditionJSON: json.RawMessage(`{"operator":"GREATER_THAN","source_question_code":"ADR_AVAILABLE","value":1}`),
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)

	_ = adrAvailable
}

func TestInvalidDraftNotPersisted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-DRAFT-1")

	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN",
		Title:       "Main",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "VALID_Q",
		QuestionType: domain.QuestionTypeText,
		Label:        "Valid",
	}); err != nil {
		t.Fatalf("create valid question: %v", err)
	}

	before, err := env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get before invalid: %v", err)
	}
	versionBefore := before.RfxVersionID

	_, err = env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "BAD_Q",
		QuestionType: "NOT_A_TYPE",
		Label:        "Bad",
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)

	after, err := env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get after invalid: %v", err)
	}
	if after.RfxVersionID != versionBefore {
		t.Fatalf("draft version changed after failed mutation")
	}
	if len(after.Sections[0].Questions) != 1 || after.Sections[0].Questions[0].QuestionCode != "VALID_Q" {
		t.Fatalf("valid draft state not preserved")
	}

	ruleCount, err := countRules(ctx, env.pool, fix.TenantID, after.RfxVersionID)
	if err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if ruleCount != 0 {
		t.Fatalf("invalid rule should not persist, found %d", ruleCount)
	}
}

func TestSaveDraftIncrementsVersion(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-SAVE-1")

	studio, err := env.qSvc.GetStudio(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	if studio.DraftVersion == nil {
		t.Fatal("expected draft version")
	}
	ver, err := env.qSvc.SaveDraft(ctx, fix.BuyerA, event.ID, studio.DraftVersion.Version)
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if ver.Version <= studio.DraftVersion.Version {
		t.Fatalf("expected version increment")
	}
}

func TestPublishedVersionMutationDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-PUB-1")

	if _, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "PUBLISHED_SEC",
		Title:       "Published Section",
	}); err != nil {
		t.Fatalf("create section before publish: %v", err)
	}

	studio, err := env.qSvc.GetStudio(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	publishedVersionID := studio.DraftVersion.ID
	publishVersion(t, env, publishedVersionID)

	var publishedSectionCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_sections WHERE rfx_version_id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, publishedVersionID, fix.TenantID).Scan(&publishedSectionCount); err != nil {
		t.Fatalf("count published sections: %v", err)
	}
	if publishedSectionCount != 1 {
		t.Fatalf("expected 1 section on published version, got %d", publishedSectionCount)
	}

	studioAfter, err := env.qSvc.GetStudio(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get studio after publish: %v", err)
	}
	if studioAfter.DraftVersion == nil || studioAfter.DraftVersion.ID == publishedVersionID {
		t.Fatal("expected new draft version after publish")
	}
	if studioAfter.DraftVersion.Status != domain.RfxVersionStatusDraft {
		t.Fatalf("expected draft status, got %s", studioAfter.DraftVersion.Status)
	}

	if _, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "NEW_DRAFT",
		Title:       "New Draft Section",
	}); err != nil {
		t.Fatalf("create section on new draft: %v", err)
	}

	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_sections WHERE rfx_version_id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, publishedVersionID, fix.TenantID).Scan(&publishedSectionCount); err != nil {
		t.Fatalf("recount published sections: %v", err)
	}
	if publishedSectionCount != 1 {
		t.Fatalf("published version mutated: section count=%d", publishedSectionCount)
	}
}

func TestPublishReadinessPassAndFail(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-READY-1")

	studio, err := env.qSvc.GetStudio(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get studio: %v", err)
	}
	enableQuestionnaireByVersionID(t, env, fix.TenantID, studio.DraftVersion.ID)

	failResult, err := env.qSvc.ValidatePublish(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("validate empty: %v", err)
	}
	if failResult.Ready {
		t.Fatal("expected readiness fail for empty questionnaire")
	}

	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "HSE",
		Title:       "HSE",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "Q1",
		QuestionType: domain.QuestionTypeText,
		Label:        "Question 1",
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}

	passResult, err := env.qSvc.ValidatePublish(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("validate valid: %v", err)
	}
	if !passResult.Ready {
		t.Fatalf("expected readiness pass, items=%+v", passResult.Items)
	}
}

func TestLegacyRfxWithoutQuestionnaireCompatible(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-LEGACY-1")

	got, err := env.rfxSvc.GetEvent(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("get legacy event: %v", err)
	}
	if got.ID != event.ID {
		t.Fatalf("legacy event read failed")
	}

	_, err = env.qSvc.GetQuestionnaire(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("questionnaire optional for legacy event: %v", err)
	}
}

func TestMigration000065FreshAndLegacy(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	var exists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_versions'
		)`).Scan(&exists); err != nil {
		t.Fatalf("check rfx_versions: %v", err)
	}
	if !exists {
		t.Fatal("migration 000065: rfx_versions missing")
	}

	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-MIG-LEGACY")
	if _, err := env.qSvc.GetQuestionnaire(context.Background(), fix.BuyerA, event.ID); err != nil {
		t.Fatalf("warm draft version: %v", err)
	}
	if event.ID == uuid.Nil {
		t.Fatal("legacy event not preserved after migration")
	}

	var draftVersionID *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT draft_version_id FROM rfx.rfx_events WHERE id=$1`, event.ID).Scan(&draftVersionID); err != nil {
		t.Fatalf("read draft_version_id: %v", err)
	}
	if draftVersionID == nil {
		t.Fatal("expected draft_version_id after studio access path warms version")
	}
}
