package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestScoreNumberLinear(t *testing.T) {
	norm := json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)
	raw, normScore, err := scoreNumberLinear(json.RawMessage(`50`), norm)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 50 || normScore != 50 {
		t.Fatalf("expected 50/50, got %v/%v", raw, normScore)
	}
}

func TestScoreBooleanMapKnockout(t *testing.T) {
	q := Question{ID: uuid.New(), QuestionCode: "ADR", QuestionType: QuestionTypeYesNo}
	criterion := ScoreCriterion{
		ID: uuid.New(), CriterionCode: "HSE", Weight: 40,
		NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
	}
	binding := ScoreBinding{
		KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`),
	}
	model := ScoreModel{ID: uuid.New(), ModelVersion: 1}

	result, err := ComputeAnswerScore(q, json.RawMessage(`false`), criterion, binding, model)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Knockout {
		t.Fatal("expected knockout")
	}
	if result.NormalizedScore != 0 {
		t.Fatalf("expected normalized 0, got %v", result.NormalizedScore)
	}
	if result.WeightedContribution != 0 {
		t.Fatalf("expected contribution 0, got %v", result.WeightedContribution)
	}
}

func TestDeterministicFixtureCarrierA(t *testing.T) {
	adrQ := Question{ID: uuid.New(), QuestionCode: "ADR_AVAILABLE", QuestionType: QuestionTypeYesNo}
	fleetQ := Question{ID: uuid.New(), QuestionCode: "FLEET_COUNT", QuestionType: QuestionTypeNumber}
	model := ScoreModel{ID: uuid.New(), ModelVersion: 1}

	hse := ScoreCriterion{
		ID: uuid.New(), CriterionCode: "HSE", Weight: 40,
		NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
	}
	capacity := ScoreCriterion{
		ID: uuid.New(), CriterionCode: "CAPACITY", Weight: 60,
		NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`),
	}
	adrBinding := ScoreBinding{KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)}
	fleetBinding := ScoreBinding{}

	adrScore, err := ComputeAnswerScore(adrQ, json.RawMessage(`true`), hse, adrBinding, model)
	if err != nil {
		t.Fatal(err)
	}
	fleetScore, err := ComputeAnswerScore(fleetQ, json.RawMessage(`50`), capacity, fleetBinding, model)
	if err != nil {
		t.Fatal(err)
	}
	total := SumWeightedContributions([]AnswerScoreComputation{adrScore, fleetScore})
	if adrScore.WeightedContribution != 40 {
		t.Fatalf("HSE contribution=%v want 40", adrScore.WeightedContribution)
	}
	if fleetScore.WeightedContribution != 30 {
		t.Fatalf("Capacity contribution=%v want 30", fleetScore.WeightedContribution)
	}
	if total != 70 {
		t.Fatalf("total=%v want 70", total)
	}
	if adrScore.Knockout {
		t.Fatal("carrier A should not knockout")
	}
}

func TestDeterministicFixtureCarrierB(t *testing.T) {
	adrQ := Question{ID: uuid.New(), QuestionCode: "ADR_AVAILABLE", QuestionType: QuestionTypeYesNo}
	fleetQ := Question{ID: uuid.New(), QuestionCode: "FLEET_COUNT", QuestionType: QuestionTypeNumber}
	model := ScoreModel{ID: uuid.New(), ModelVersion: 1}

	hse := ScoreCriterion{
		ID: uuid.New(), CriterionCode: "HSE", Weight: 40,
		NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
	}
	capacity := ScoreCriterion{
		ID: uuid.New(), CriterionCode: "CAPACITY", Weight: 60,
		NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`),
	}
	adrBinding := ScoreBinding{KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)}

	adrScore, err := ComputeAnswerScore(adrQ, json.RawMessage(`false`), hse, adrBinding, model)
	if err != nil {
		t.Fatal(err)
	}
	fleetScore, err := ComputeAnswerScore(fleetQ, json.RawMessage(`100`), capacity, ScoreBinding{}, model)
	if err != nil {
		t.Fatal(err)
	}
	total := SumWeightedContributions([]AnswerScoreComputation{adrScore, fleetScore})
	if total != 60 {
		t.Fatalf("total=%v want 60", total)
	}
	if !adrScore.Knockout {
		t.Fatal("expected knockout")
	}
	status, _ := AggregateQualificationStatus(total, true, []string{adrScore.KnockoutReason})
	if status != QualificationStatusRejected {
		t.Fatalf("status=%s want REJECTED", status)
	}
}

func TestCriteriaWeightInvalid(t *testing.T) {
	result := ValidateScoreModelReadiness(
		ScoreModel{},
		[]ScoreCriterion{{CriterionCode: "A", Weight: 30}, {CriterionCode: "B", Weight: 30}},
		nil, nil,
	)
	if result.Ready {
		t.Fatal("expected not ready")
	}
}
