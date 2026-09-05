package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestDetectRuleCycles_SelfReferenceDenied(t *testing.T) {
	qID := uuid.New()
	questions := map[uuid.UUID]string{qID: "ADR"}
	cond := mustRaw(`{"operator":"EQUALS","source_question_code":"ADR","value":true}`)
	rules := []QuestionRule{{
		TargetQuestionID: &qID,
		RuleCode:         "R1",
		Action:           RuleActionRequire,
		ConditionJSON:    cond,
	}}
	err := DetectRuleCycles(rules, questions)
	if err == nil {
		t.Fatal("expected self-reference error")
	}
}

func TestDetectRuleCycles_CycleDenied(t *testing.T) {
	qA, qB := uuid.New(), uuid.New()
	questions := map[uuid.UUID]string{qA: "A", qB: "B"}
	rules := []QuestionRule{
		{TargetQuestionID: &qB, RuleCode: "R1", Action: RuleActionShow, ConditionJSON: mustRaw(`{"operator":"EQUALS","source_question_code":"A","value":true}`)},
		{TargetQuestionID: &qA, RuleCode: "R2", Action: RuleActionShow, ConditionJSON: mustRaw(`{"operator":"IS_NOT_EMPTY","source_question_code":"B"}`)},
	}
	if err := DetectRuleCycles(rules, questions); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateQuestionType_Invalid(t *testing.T) {
	if err := ValidateQuestionType("INVALID"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEvaluatePublishReadiness_Pass(t *testing.T) {
	version := RfxVersion{QuestionnaireEnabled: true}
	sections := []SectionWithQuestions{{
		Section: Section{SectionCode: "S1", Title: "Section 1"},
		Questions: []Question{{
			ID: uuid.New(), QuestionCode: "Q1", QuestionType: QuestionTypeText, Label: "Name",
		}},
	}}
	result := EvaluatePublishReadiness(version, sections, nil)
	if !result.Ready {
		t.Fatalf("expected ready, blocking=%d items=%+v", result.BlockingFail, result.Items)
	}
}

func TestEvaluatePublishReadiness_FailNoSections(t *testing.T) {
	version := RfxVersion{QuestionnaireEnabled: true}
	result := EvaluatePublishReadiness(version, nil, nil)
	if result.Ready {
		t.Fatal("expected not ready")
	}
}

func mustRaw(s string) []byte { return []byte(s) }
