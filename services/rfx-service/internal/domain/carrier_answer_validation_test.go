package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestValidateCarrierAnswerPatchesRejectsInvalidType(t *testing.T) {
	qid := uuid.New()
	sid := uuid.New()
	rt := QuestionnaireRuntime{
		QuestionsByID: map[uuid.UUID]Question{
			qid: {ID: qid, SectionID: sid, QuestionCode: "Q1", QuestionType: QuestionTypeNumber, Label: "Amount"},
		},
		SectionByQID: map[uuid.UUID]Section{qid: {ID: sid}},
	}
	errs := ValidateCarrierAnswerPatches(rt, map[uuid.UUID]json.RawMessage{}, []AnswerPatchItem{
		{QuestionID: qid, Value: json.RawMessage(`"not-a-number"`)},
	}, false)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestValidateCarrierAnswersRequiredOnPreSubmit(t *testing.T) {
	qid := uuid.New()
	sid := uuid.New()
	rt := QuestionnaireRuntime{
		QuestionsByID: map[uuid.UUID]Question{
			qid: {ID: qid, SectionID: sid, QuestionCode: "Q1", QuestionType: QuestionTypeText, Label: "Name", Required: true},
		},
		SectionByQID: map[uuid.UUID]Section{qid: {ID: sid}},
	}
	errs := ValidateCarrierAnswers(rt, map[uuid.UUID]json.RawMessage{}, true)
	if len(errs) == 0 {
		t.Fatal("expected required validation error on pre-submit")
	}
	errs = ValidateCarrierAnswerPatches(rt, map[uuid.UUID]json.RawMessage{}, []AnswerPatchItem{}, false)
	if len(errs) != 0 {
		t.Fatalf("autosave should not require full form, got %d errors", len(errs))
	}
}

func TestComputeCompletionPercentIgnoresHiddenRequired(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()
	sid := uuid.New()
	rt := QuestionnaireRuntime{
		QuestionsByID: map[uuid.UUID]Question{
			sourceID: {ID: sourceID, SectionID: sid, QuestionCode: "SHOW", QuestionType: QuestionTypeYesNo, Label: "Show"},
			targetID: {ID: targetID, SectionID: sid, QuestionCode: "DETAIL", QuestionType: QuestionTypeText, Label: "Detail", Required: true},
		},
		SectionByQID: map[uuid.UUID]Section{
			sourceID: {ID: sid},
			targetID: {ID: sid},
		},
		Rules: []QuestionRule{{
			TargetQuestionID: &targetID,
			Action:           RuleActionShow,
			ConditionJSON:    mustCondition(t, `{"operator":"EQUALS","source_question_code":"SHOW","value":true}`),
		}},
	}
	byID := map[uuid.UUID]json.RawMessage{
		sourceID: json.RawMessage(`false`),
	}
	if pct := ComputeCompletionPercent(rt, byID); pct != 100 {
		t.Fatalf("expected 100%% when hidden required question not applicable, got %v", pct)
	}
}

func TestHiddenQuestionIDsPolicy(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()
	sid := uuid.New()
	rt := QuestionnaireRuntime{
		QuestionsByID: map[uuid.UUID]Question{
			sourceID: {ID: sourceID, SectionID: sid, QuestionCode: "SHOW", QuestionType: QuestionTypeYesNo},
			targetID: {ID: targetID, SectionID: sid, QuestionCode: "DETAIL", QuestionType: QuestionTypeText},
		},
		SectionByQID: map[uuid.UUID]Section{sourceID: {ID: sid}, targetID: {ID: sid}},
		Rules: []QuestionRule{{
			TargetQuestionID: &targetID,
			Action:           RuleActionShow,
			ConditionJSON:    mustCondition(t, `{"operator":"EQUALS","source_question_code":"SHOW","value":true}`),
		}},
	}
	hidden := HiddenQuestionIDs(rt, map[uuid.UUID]json.RawMessage{sourceID: json.RawMessage(`false`), targetID: json.RawMessage(`"old"`)})
	if len(hidden) != 1 || hidden[0] != targetID {
		t.Fatalf("expected target hidden, got %+v", hidden)
	}
}

func mustCondition(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	return json.RawMessage(raw)
}
