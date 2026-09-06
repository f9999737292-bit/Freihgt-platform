package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestReadinessOptionScoreInvalid(t *testing.T) {
	criterionID := uuid.New()
	question := Question{
		ID: uuid.New(), QuestionCode: "SERVICE_LEVEL", QuestionType: QuestionTypeSingleSelect,
		Options: []QuestionOption{
			{OptionCode: "GOLD"}, {OptionCode: "SILVER"},
		},
	}
	result := ValidateScoreModelReadiness(
		ScoreModel{},
		[]ScoreCriterion{{
			ID: criterionID, CriterionCode: "SERVICE", Weight: 100,
			NormalizationJSON: json.RawMessage(`{"type":"OPTION_MAP","option_scores":{"PLATINUM":100}}`),
		}},
		[]ScoreBinding{{CriterionID: criterionID, QuestionID: question.ID}},
		[]Question{question},
	)
	if result.Ready || !hasReadinessErrorCode(result, "OPTION_SCORE_INVALID") {
		t.Fatalf("expected OPTION_SCORE_INVALID, ready=%v errors=%+v", result.Ready, result.Errors)
	}
}

func TestReadinessMultiSelectAggregationInvalid(t *testing.T) {
	result := ValidateScoreModelReadiness(
		ScoreModel{},
		[]ScoreCriterion{{
			ID: uuid.New(), CriterionCode: "FEATURES", Weight: 100,
			NormalizationJSON: json.RawMessage(`{"type":"MULTI_SELECT","aggregation":"MEDIAN","option_scores":{"A":10}}`),
		}},
		nil, nil,
	)
	if result.Ready || !hasReadinessErrorCode(result, "NORMALIZATION_INVALID") {
		t.Fatalf("expected NORMALIZATION_INVALID, errors=%+v", result.Errors)
	}
}

func hasReadinessErrorCode(result ScoreModelReadinessResult, code string) bool {
	for _, e := range result.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}
