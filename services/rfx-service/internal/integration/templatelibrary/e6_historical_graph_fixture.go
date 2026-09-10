//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

// e6HistoricalGraphExpectation captures stable codes and presentation order for E6 graph tests.
// Questionnaire graph rules support SHOW/HIDE/REQUIRE only (frozen E4); knockout lives in scoring bindings.
type e6HistoricalGraphExpectation struct {
	Sections []e6ExpectedSection
	Rules    []e6ExpectedRule
}

type e6ExpectedSection struct {
	Code      string
	SortOrder int
	Questions []e6ExpectedQuestion
}

type e6ExpectedQuestion struct {
	Code               string
	Type               string
	SortOrder          int
	ValidationRuleJSON string
	OptionCodes        []string
}

type e6ExpectedRule struct {
	RuleCode           string
	Action             string
	SortOrder          int
	TargetQuestionCode string
	ConditionJSON      string
}

func e6IntSortOrder(v int) *int {
	return &v
}

func populateE6HistoricalTemplateGraph(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) e6HistoricalGraphExpectation {
	t.Helper()
	ctx := context.Background()

	expect := e6HistoricalGraphExpectation{
		Sections: []e6ExpectedSection{
			{
				Code:      "COVERAGE",
				SortOrder: 10,
				Questions: []e6ExpectedQuestion{
					{Code: "SERVICE_LEVEL", Type: domain.QuestionTypeSingleSelect, SortOrder: 10, OptionCodes: []string{"PREMIUM", "BASIC"}},
					{Code: "NOTES", Type: domain.QuestionTypeLongText, SortOrder: 20},
				},
			},
			{
				Code:      "LOGISTICS",
				SortOrder: 20,
				Questions: []e6ExpectedQuestion{
					{Code: "FLEET_SIZE", Type: domain.QuestionTypeNumber, SortOrder: 10, ValidationRuleJSON: `{"min_value":1}`},
					{Code: "HAS_ADR", Type: domain.QuestionTypeYesNo, SortOrder: 20},
				},
			},
		},
		Rules: []e6ExpectedRule{
			{
				RuleCode:           "SHOW_NOTES",
				Action:             domain.RuleActionShow,
				SortOrder:          10,
				TargetQuestionCode: "NOTES",
				ConditionJSON:      `{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":10}`,
			},
			{
				RuleCode:           "REQ_ADR_DETAILS",
				Action:             domain.RuleActionRequire,
				SortOrder:          20,
				TargetQuestionCode: "HAS_ADR",
				ConditionJSON:      `{"operator":"EQUALS","source_question_code":"SERVICE_LEVEL","value":"PREMIUM"}`,
			},
		},
	}

	coverageSection, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "COVERAGE",
		Title:       "Coverage Requirements",
		SortOrder:   e6IntSortOrder(10),
	})
	if err != nil {
		t.Fatalf("create COVERAGE section: %v", err)
	}
	logisticsSection, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "LOGISTICS",
		Title:       "Logistics",
		SortOrder:   e6IntSortOrder(20),
	})
	if err != nil {
		t.Fatalf("create LOGISTICS section: %v", err)
	}

	serviceQ, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, coverageSection.ID, domain.CreateQuestionInput{
		QuestionCode: "SERVICE_LEVEL",
		QuestionType: domain.QuestionTypeSingleSelect,
		Label:        "Service level",
		Required:     true,
		SortOrder:    e6IntSortOrder(10),
	})
	if err != nil {
		t.Fatalf("create SERVICE_LEVEL: %v", err)
	}
	for _, opt := range []struct {
		code      string
		label     string
		sortOrder int
	}{
		{"PREMIUM", "Premium", 10},
		{"BASIC", "Basic", 20},
	} {
		if _, err := env.templateQSvc.CreateOption(ctx, fix.BuyerA, templateID, serviceQ.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.code,
			Label:      opt.label,
			SortOrder:  e6IntSortOrder(opt.sortOrder),
		}); err != nil {
			t.Fatalf("create option %s: %v", opt.code, err)
		}
	}
	if _, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, coverageSection.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES",
		QuestionType: domain.QuestionTypeLongText,
		Label:        "Additional notes",
		SortOrder:    e6IntSortOrder(20),
	}); err != nil {
		t.Fatalf("create NOTES: %v", err)
	}

	if _, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, logisticsSection.ID, domain.CreateQuestionInput{
		QuestionCode:       "FLEET_SIZE",
		QuestionType:       domain.QuestionTypeNumber,
		Label:              "Fleet size",
		Required:           true,
		ValidationRuleJSON: json.RawMessage(`{"min_value":1}`),
		SortOrder:          e6IntSortOrder(10),
	}); err != nil {
		t.Fatalf("create FLEET_SIZE: %v", err)
	}
	if _, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, logisticsSection.ID, domain.CreateQuestionInput{
		QuestionCode: "HAS_ADR",
		QuestionType: domain.QuestionTypeYesNo,
		Label:        "ADR certified",
		Required:     false,
		SortOrder:    e6IntSortOrder(20),
	}); err != nil {
		t.Fatalf("create HAS_ADR: %v", err)
	}

	targetNotes := "NOTES"
	if _, err := env.templateQSvc.CreateRule(ctx, fix.BuyerA, templateID, domain.CreateQuestionRuleInput{
		RuleCode:           "SHOW_NOTES",
		Action:             domain.RuleActionShow,
		TargetQuestionCode: &targetNotes,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":10}`),
		SortOrder:          e6IntSortOrder(10),
	}); err != nil {
		t.Fatalf("create SHOW_NOTES rule: %v", err)
	}
	targetADR := "HAS_ADR"
	if _, err := env.templateQSvc.CreateRule(ctx, fix.BuyerA, templateID, domain.CreateQuestionRuleInput{
		RuleCode:           "REQ_ADR_DETAILS",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &targetADR,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"SERVICE_LEVEL","value":"PREMIUM"}`),
		SortOrder:          e6IntSortOrder(20),
	}); err != nil {
		t.Fatalf("create REQ_ADR_DETAILS rule: %v", err)
	}

	return expect
}

func setupPublishedE6HistoricalTemplate(t *testing.T, env *testEnv, fix buyerFixture, code string, owner *uuid.UUID) (*domain.TemplateDetail, *domain.RfxTemplateVersion, e6HistoricalGraphExpectation) {
	t.Helper()
	detail := createTemplate(t, env, fix, code, owner)
	expect := populateE6HistoricalTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	return detail, published, expect
}

func reloadTemplateVersionByID(t *testing.T, env *testEnv, fix buyerFixture, templateID, versionID uuid.UUID) *domain.RfxTemplateVersion {
	t.Helper()
	detail, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	for i := range detail.Versions {
		if detail.Versions[i].ID == versionID {
			v := detail.Versions[i]
			return &v
		}
	}
	t.Fatalf("template version %s not found", versionID)
	return nil
}

func setupE6HistoricalTemplateDraft(t *testing.T, env *testEnv, fix buyerFixture, code string, owner *uuid.UUID) (*domain.TemplateDetail, e6HistoricalGraphExpectation) {
	t.Helper()
	detail := createTemplate(t, env, fix, code, owner)
	expect := populateE6HistoricalTemplateGraph(t, env, fix, detail.Template.ID)
	detail, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload template after graph populate: %v", err)
	}
	if detail.DraftVersion == nil {
		t.Fatal("expected draft version after graph populate")
	}
	return detail, expect
}
