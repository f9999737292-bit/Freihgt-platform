package domain

import (
	"encoding/json"
	"fmt"
	"math"
)

const scoreWeightTotalTarget = 100.0
const scoreWeightTolerance = 0.01

type NumberLinearNormalization struct {
	Type string  `json:"type"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

type BooleanMapNormalization struct {
	Type       string  `json:"type"`
	TrueScore  float64 `json:"true_score"`
	FalseScore float64 `json:"false_score"`
}

type OptionMapNormalization struct {
	Type         string             `json:"type"`
	OptionScores map[string]float64 `json:"option_scores"`
	DefaultScore float64            `json:"default_score,omitempty"`
}

type MultiSelectNormalization struct {
	Type          string             `json:"type"`
	Aggregation   string             `json:"aggregation"`
	Cap           float64            `json:"cap,omitempty"`
	OptionScores  map[string]float64 `json:"option_scores"`
}

type BooleanEqualsKnockout struct {
	Type  string `json:"type"`
	Value bool   `json:"value"`
}

type OptionEqualsKnockout struct {
	Type       string `json:"type"`
	OptionCode string `json:"option_code"`
}

type NumberThresholdKnockout struct {
	Type      string  `json:"type"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
}

type AnswerScoreComputation struct {
	RawScore             float64
	NormalizedScore      float64
	WeightedContribution float64
	Knockout             bool
	KnockoutReason       string
	Rule                 string
	Explanation          ScoreExplanation
}

func roundScore(v float64) float64 {
	return math.Round(v*100) / 100
}

func ComputeAnswerScore(
	question Question,
	answerJSON json.RawMessage,
	criterion ScoreCriterion,
	binding ScoreBinding,
	model ScoreModel,
) (AnswerScoreComputation, error) {
	if isAnswerEmpty(answerJSON) {
		return AnswerScoreComputation{}, fmt.Errorf("answer is empty for question %s", question.QuestionCode)
	}

	normType, err := parseNormalizationType(criterion.NormalizationJSON)
	if err != nil {
		return AnswerScoreComputation{}, err
	}
	if err := assertQuestionTypeCompatible(question.QuestionType, normType); err != nil {
		return AnswerScoreComputation{}, err
	}

	var raw, normalized float64
	switch normType {
	case NormalizationNumberLinear:
		raw, normalized, err = scoreNumberLinear(answerJSON, criterion.NormalizationJSON)
	case NormalizationBooleanMap:
		raw, normalized, err = scoreBooleanMap(answerJSON, criterion.NormalizationJSON)
	case NormalizationOptionMap:
		raw, normalized, err = scoreOptionMap(answerJSON, criterion.NormalizationJSON)
	case NormalizationMultiSelect:
		raw, normalized, err = scoreMultiSelect(answerJSON, criterion.NormalizationJSON)
	default:
		return AnswerScoreComputation{}, fmt.Errorf("unsupported normalization type %q", normType)
	}
	if err != nil {
		return AnswerScoreComputation{}, err
	}

	contribution := roundScore(normalized * criterion.Weight / scoreWeightTotalTarget)
	knockout, reason, rule := evaluateKnockout(question, answerJSON, binding.KnockoutRuleJSON)

	input := map[string]any{"question_code": question.QuestionCode}
	var parsed any
	if json.Unmarshal(answerJSON, &parsed) == nil {
		input["value"] = parsed
	}

	explanation := ScoreExplanation{
		Source:               "PERSISTED_ANSWER",
		Input:                input,
		Rule:                 rule,
		ScoreModelID:         model.ID.String(),
		ScoreModelVersion:    model.ModelVersion,
		CriterionCode:        criterion.CriterionCode,
		CriterionWeight:      criterion.Weight,
		RawScore:             roundScore(raw),
		NormalizedScore:      roundScore(normalized),
		WeightedContribution: contribution,
		Knockout:             knockout,
		KnockoutReason:       reason,
	}

	return AnswerScoreComputation{
		RawScore:             roundScore(raw),
		NormalizedScore:      roundScore(normalized),
		WeightedContribution: contribution,
		Knockout:             knockout,
		KnockoutReason:       reason,
		Rule:                 rule,
		Explanation:          explanation,
	}, nil
}

func AggregateQualificationStatus(totalScore float64, anyKnockout bool, knockoutReasons []string) (string, json.RawMessage) {
	if anyKnockout {
		reason := map[string]any{"reasons": knockoutReasons}
		raw, _ := json.Marshal(reason)
		return QualificationStatusRejected, raw
	}
	return QualificationStatusQualified, nil
}

func scoreNumberLinear(answerJSON json.RawMessage, normJSON json.RawMessage) (raw, normalized float64, err error) {
	var cfg NumberLinearNormalization
	if err := json.Unmarshal(normJSON, &cfg); err != nil {
		return 0, 0, err
	}
	if cfg.Type != NormalizationNumberLinear || cfg.Max <= cfg.Min {
		return 0, 0, fmt.Errorf("invalid NUMBER_LINEAR normalization")
	}
	var value float64
	if err := json.Unmarshal(answerJSON, &value); err != nil {
		return 0, 0, err
	}
	raw = value
	if value <= cfg.Min {
		normalized = 0
		return raw, normalized, nil
	}
	if value >= cfg.Max {
		normalized = 100
		return raw, normalized, nil
	}
	normalized = ((value - cfg.Min) / (cfg.Max - cfg.Min)) * 100
	return raw, normalized, nil
}

func scoreBooleanMap(answerJSON json.RawMessage, normJSON json.RawMessage) (raw, normalized float64, err error) {
	var cfg BooleanMapNormalization
	if err := json.Unmarshal(normJSON, &cfg); err != nil {
		return 0, 0, err
	}
	if cfg.Type != NormalizationBooleanMap {
		return 0, 0, fmt.Errorf("invalid BOOLEAN_MAP normalization")
	}
	value, err := parseBooleanAnswer(answerJSON)
	if err != nil {
		return 0, 0, err
	}
	if value {
		raw, normalized = 1, cfg.TrueScore
	} else {
		raw, normalized = 0, cfg.FalseScore
	}
	return raw, normalized, nil
}

func scoreOptionMap(answerJSON json.RawMessage, normJSON json.RawMessage) (raw, normalized float64, err error) {
	var cfg OptionMapNormalization
	if err := json.Unmarshal(normJSON, &cfg); err != nil {
		return 0, 0, err
	}
	if cfg.Type != NormalizationOptionMap {
		return 0, 0, fmt.Errorf("invalid OPTION_MAP normalization")
	}
	var option string
	if err := json.Unmarshal(answerJSON, &option); err != nil {
		return 0, 0, err
	}
	score, ok := cfg.OptionScores[option]
	if !ok {
		score = cfg.DefaultScore
	}
	return score, score, nil
}

func scoreMultiSelect(answerJSON json.RawMessage, normJSON json.RawMessage) (raw, normalized float64, err error) {
	var cfg MultiSelectNormalization
	if err := json.Unmarshal(normJSON, &cfg); err != nil {
		return 0, 0, err
	}
	if cfg.Type != NormalizationMultiSelect {
		return 0, 0, fmt.Errorf("invalid MULTI_SELECT normalization")
	}
	var options []string
	if err := json.Unmarshal(answerJSON, &options); err != nil {
		return 0, 0, err
	}
	switch cfg.Aggregation {
	case MultiSelectAggregationSumCapped:
		sum := 0.0
		for _, opt := range options {
			sum += cfg.OptionScores[opt]
		}
		raw = sum
		if cfg.Cap > 0 && sum > cfg.Cap {
			normalized = cfg.Cap
		} else {
			normalized = sum
		}
	case MultiSelectAggregationMax:
		max := 0.0
		for _, opt := range options {
			if s := cfg.OptionScores[opt]; s > max {
				max = s
			}
		}
		raw, normalized = max, max
	case MultiSelectAggregationAverage:
		if len(options) == 0 {
			return 0, 0, nil
		}
		sum := 0.0
		for _, opt := range options {
			sum += cfg.OptionScores[opt]
		}
		raw = sum
		normalized = sum / float64(len(options))
	default:
		return 0, 0, fmt.Errorf("unsupported MULTI_SELECT aggregation %q", cfg.Aggregation)
	}
	return raw, normalized, nil
}

func evaluateKnockout(question Question, answerJSON json.RawMessage, knockoutJSON json.RawMessage) (bool, string, string) {
	if len(knockoutJSON) == 0 || string(knockoutJSON) == "null" {
		return false, "", ""
	}
	var typeProbe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(knockoutJSON, &typeProbe); err != nil {
		return false, "", typeProbe.Type
	}
	switch typeProbe.Type {
	case KnockoutBooleanEquals:
		var cfg BooleanEqualsKnockout
		_ = json.Unmarshal(knockoutJSON, &cfg)
		value, err := parseBooleanAnswer(answerJSON)
		if err != nil {
			return false, "", cfg.Type
		}
		if value == cfg.Value {
			reason := fmt.Sprintf("boolean equals %v", cfg.Value)
			return true, reason, cfg.Type
		}
	case KnockoutOptionEquals:
		var cfg OptionEqualsKnockout
		_ = json.Unmarshal(knockoutJSON, &cfg)
		var option string
		_ = json.Unmarshal(answerJSON, &option)
		if option == cfg.OptionCode {
			return true, fmt.Sprintf("option equals %s", cfg.OptionCode), cfg.Type
		}
	case KnockoutNumberThreshold:
		var cfg NumberThresholdKnockout
		_ = json.Unmarshal(knockoutJSON, &cfg)
		var value float64
		_ = json.Unmarshal(answerJSON, &value)
		triggered := false
		switch cfg.Operator {
		case "LT":
			triggered = value < cfg.Threshold
		case "LTE":
			triggered = value <= cfg.Threshold
		case "GT":
			triggered = value > cfg.Threshold
		case "GTE":
			triggered = value >= cfg.Threshold
		case "EQ":
			triggered = value == cfg.Threshold
		}
		if triggered {
			return true, fmt.Sprintf("number %s %v", cfg.Operator, cfg.Threshold), cfg.Type
		}
	}
	return false, "", typeProbe.Type
}

func parseBooleanAnswer(answerJSON json.RawMessage) (bool, error) {
	var b bool
	if err := json.Unmarshal(answerJSON, &b); err == nil {
		return b, nil
	}
	var s string
	if err := json.Unmarshal(answerJSON, &s); err != nil {
		return false, err
	}
	switch s {
	case "true", "yes", "YES", "True":
		return true, nil
	case "false", "no", "NO", "False":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean answer")
	}
}

func parseNormalizationType(normJSON json.RawMessage) (string, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(normJSON, &probe); err != nil {
		return "", err
	}
	return probe.Type, nil
}

func assertQuestionTypeCompatible(questionType, normType string) error {
	switch normType {
	case NormalizationNumberLinear:
		if questionType != QuestionTypeNumber && questionType != QuestionTypePercent {
			return fmt.Errorf("NUMBER_LINEAR requires NUMBER question, got %s", questionType)
		}
	case NormalizationBooleanMap:
		if questionType != QuestionTypeYesNo {
			return fmt.Errorf("BOOLEAN_MAP requires YES_NO question, got %s", questionType)
		}
	case NormalizationOptionMap:
		if questionType != QuestionTypeSingleSelect {
			return fmt.Errorf("OPTION_MAP requires SINGLE_SELECT question, got %s", questionType)
		}
	case NormalizationMultiSelect:
		if questionType != QuestionTypeMultiSelect {
			return fmt.Errorf("MULTI_SELECT normalization requires MULTI_SELECT question, got %s", questionType)
		}
	}
	return nil
}

func SumWeightedContributions(scores []AnswerScoreComputation) float64 {
	total := 0.0
	for _, s := range scores {
		total += s.WeightedContribution
	}
	return roundScore(total)
}
