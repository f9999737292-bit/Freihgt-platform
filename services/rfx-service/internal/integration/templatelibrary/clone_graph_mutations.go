//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func mutateTemplateDraftGraph(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	detail, err := env.templateSvc.GetTemplate(ctx, fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if detail.DraftVersion == nil {
		t.Fatal("expected template draft after fork")
	}
	draftGraph, err := env.tmplQRepo.LoadQuestionnaire(ctx, templateID, detail.DraftVersion.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load template draft graph: %v", err)
	}
	mutatedSection, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "MUTATED",
		Title:       "Mutated Section",
	})
	if err != nil {
		t.Fatalf("add template section: %v", err)
	}
	if _, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, mutatedSection.ID, domain.CreateQuestionInput{
		QuestionCode: "EXTRA_NOTES",
		QuestionType: domain.QuestionTypeText,
		Label:        "Extra notes",
		Required:     false,
	}); err != nil {
		t.Fatalf("add template question: %v", err)
	}
	fleetQ := findTemplateQuestionByCode(t, draftGraph, "FLEET_SIZE")
	newLabel := "Fleet size (mutated)"
	if _, err := env.templateQSvc.UpdateQuestion(ctx, fix.BuyerA, templateID, fleetQ.ID, domain.UpdateQuestionInput{
		Label:           &newLabel,
		ExpectedVersion: fleetQ.Version,
	}); err != nil {
		t.Fatalf("update template question: %v", err)
	}
	coverageQ := findTemplateQuestionByCode(t, draftGraph, "COVERAGE")
	if _, err := env.templateQSvc.CreateOption(ctx, fix.BuyerA, templateID, coverageQ.ID, domain.CreateQuestionOptionInput{
		OptionCode: "NONE",
		Label:      "None",
	}); err != nil {
		t.Fatalf("add template option: %v", err)
	}
	target := "EXTRA_NOTES"
	if _, err := env.templateQSvc.CreateRule(ctx, fix.BuyerA, templateID, domain.CreateQuestionRuleInput{
		RuleCode:           "MUTATED_RULE",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &target,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":"99"}`),
	}); err != nil {
		t.Fatalf("add template rule: %v", err)
	}
}

func mutateEventDraftGraph(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	draftVersion, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, eventID)
	if err != nil {
		t.Fatalf("get event draft version: %v", err)
	}
	eventGraph, err := env.qRepo.LoadQuestionnaire(ctx, draftVersion.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load event draft graph: %v", err)
	}
	mutatedSection, err := env.qSvc.CreateSection(ctx, fix.BuyerA, eventID, domain.CreateSectionInput{
		SectionCode: "EVENT_MUTATED",
		Title:       "Event Mutated Section",
	})
	if err != nil {
		t.Fatalf("add event section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, eventID, mutatedSection.ID, domain.CreateQuestionInput{
		QuestionCode: "EXTRA_NOTES",
		QuestionType: domain.QuestionTypeText,
		Label:        "Extra notes",
		Required:     false,
	}); err != nil {
		t.Fatalf("add event question: %v", err)
	}
	fleetQ := findEventQuestionByCode(t, eventGraph, "FLEET_SIZE")
	newLabel := "Fleet size (event mutated)"
	if _, err := env.qSvc.UpdateQuestion(ctx, fix.BuyerA, eventID, fleetQ.ID, domain.UpdateQuestionInput{
		Label:           &newLabel,
		ExpectedVersion: fleetQ.Version,
	}); err != nil {
		t.Fatalf("update event question: %v", err)
	}
	coverageQ := findEventQuestionByCode(t, eventGraph, "COVERAGE")
	if _, err := env.qSvc.CreateOption(ctx, fix.BuyerA, eventID, coverageQ.ID, domain.CreateQuestionOptionInput{
		OptionCode: "NONE",
		Label:      "None",
	}); err != nil {
		t.Fatalf("add event option: %v", err)
	}
	target := "EXTRA_NOTES"
	if _, err := env.qSvc.CreateRule(ctx, fix.BuyerA, eventID, domain.CreateQuestionRuleInput{
		RuleCode:           "EVENT_MUTATED_RULE",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &target,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":"99"}`),
	}); err != nil {
		t.Fatalf("add event rule: %v", err)
	}
}

func findTemplateQuestionByCode(t *testing.T, graph *domain.TemplateQuestionnaireDefinition, code string) domain.TemplateQuestion {
	t.Helper()
	for _, section := range graph.Sections {
		for _, question := range section.Questions {
			if question.QuestionCode == code {
				return question
			}
		}
	}
	t.Fatalf("template question %s not found", code)
	return domain.TemplateQuestion{}
}

func findEventQuestionByCode(t *testing.T, graph *domain.QuestionnaireDefinition, code string) domain.Question {
	t.Helper()
	for _, section := range graph.Sections {
		for _, question := range section.Questions {
			if question.QuestionCode == code {
				return question
			}
		}
	}
	t.Fatalf("event question %s not found", code)
	return domain.Question{}
}
