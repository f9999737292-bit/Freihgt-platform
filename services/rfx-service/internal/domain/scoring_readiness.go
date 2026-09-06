package domain

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/google/uuid"
)

func ValidateScoreModelReadiness(
	model ScoreModel,
	criteria []ScoreCriterion,
	bindings []ScoreBinding,
	questions []Question,
) ScoreModelReadinessResult {
	errs := make([]ScoreModelReadinessError, 0)

	if len(criteria) == 0 {
		errs = append(errs, ScoreModelReadinessError{
			Code: "CRITERIA_REQUIRED", Field: "criteria", Message: "at least one criterion is required",
		})
	}

	codeSet := map[string]struct{}{}
	totalWeight := 0.0
	for _, c := range criteria {
		if c.CriterionCode == "" {
			errs = append(errs, ScoreModelReadinessError{
				Code: "CRITERION_CODE_REQUIRED", Field: "criteria", Message: "criterion code is required",
			})
			continue
		}
		if _, dup := codeSet[c.CriterionCode]; dup {
			errs = append(errs, ScoreModelReadinessError{
				Code: "CRITERION_CODE_DUPLICATE", Field: "criteria", Message: "duplicate criterion code",
				Params: map[string]any{"criterion_code": c.CriterionCode},
			})
		}
		codeSet[c.CriterionCode] = struct{}{}
		if !isFiniteWeight(c.Weight) || c.Weight < 0 {
			errs = append(errs, ScoreModelReadinessError{
				Code: "CRITERION_WEIGHT_INVALID", Field: "criteria", Message: "criterion weight must be >= 0",
				Params: map[string]any{"criterion_code": c.CriterionCode},
			})
		}
		totalWeight += c.Weight
		if normErr := validateNormalizationJSON(c.NormalizationJSON); normErr != nil {
			errs = append(errs, ScoreModelReadinessError{
				Code: "NORMALIZATION_INVALID", Field: "criteria", Message: normErr.Error(),
				Params: map[string]any{"criterion_code": c.CriterionCode},
			})
		}
	}
	if len(criteria) > 0 && math.Abs(totalWeight-scoreWeightTotalTarget) > scoreWeightTolerance {
		errs = append(errs, ScoreModelReadinessError{
			Code: "CRITERIA_WEIGHT_TOTAL_INVALID", Field: "criteria",
			Message: fmt.Sprintf("criterion weights must sum to %.0f", scoreWeightTotalTarget),
			Params:  map[string]any{"total_weight": totalWeight},
		})
	}

	questionByID := map[uuid.UUID]Question{}
	questionByCode := map[string]Question{}
	for _, q := range questions {
		questionByID[q.ID] = q
		questionByCode[q.QuestionCode] = q
	}

	criterionByID := map[uuid.UUID]ScoreCriterion{}
	for _, c := range criteria {
		criterionByID[c.ID] = c
	}

	bindingKeys := map[string]struct{}{}
	for _, b := range bindings {
		criterion, ok := criterionByID[b.CriterionID]
		if !ok {
			errs = append(errs, ScoreModelReadinessError{
				Code: "BINDING_CRITERION_INVALID", Field: "bindings", Message: "binding references unknown criterion",
			})
			continue
		}
		question, ok := questionByID[b.QuestionID]
		if !ok {
			errs = append(errs, ScoreModelReadinessError{
				Code: "BINDING_QUESTION_INVALID", Field: "bindings", Message: "binding references unknown question",
				Params: map[string]any{"criterion_code": criterion.CriterionCode},
			})
			continue
		}
		key := b.CriterionID.String() + ":" + b.QuestionID.String()
		if _, dup := bindingKeys[key]; dup {
			errs = append(errs, ScoreModelReadinessError{
				Code: "BINDING_DUPLICATE", Field: "bindings", Message: "duplicate binding",
				Params: map[string]any{"criterion_code": criterion.CriterionCode, "question_code": question.QuestionCode},
			})
		}
		bindingKeys[key] = struct{}{}

		normType, _ := parseNormalizationType(criterion.NormalizationJSON)
		if err := assertQuestionTypeCompatible(question.QuestionType, normType); err != nil {
			errs = append(errs, ScoreModelReadinessError{
				Code: "BINDING_TYPE_INCOMPATIBLE", Field: "bindings", Message: err.Error(),
				Params: map[string]any{"criterion_code": criterion.CriterionCode, "question_code": question.QuestionCode},
			})
		}
		if err := validateOptionScoresAgainstQuestion(criterion.NormalizationJSON, question); err != nil {
			errs = append(errs, ScoreModelReadinessError{
				Code: "OPTION_SCORE_INVALID", Field: "bindings", Message: err.Error(),
				Params: map[string]any{"criterion_code": criterion.CriterionCode, "question_code": question.QuestionCode},
			})
		}
		if err := validateMultiSelectAggregation(criterion.NormalizationJSON); err != nil {
			errs = append(errs, ScoreModelReadinessError{
				Code: "MULTI_SELECT_AGGREGATION_INVALID", Field: "criteria", Message: err.Error(),
				Params: map[string]any{"criterion_code": criterion.CriterionCode},
			})
		}
		if len(b.KnockoutRuleJSON) > 0 && string(b.KnockoutRuleJSON) != "null" {
			if err := validateKnockoutCompatibility(question, b.KnockoutRuleJSON); err != nil {
				errs = append(errs, ScoreModelReadinessError{
					Code: "KNOCKOUT_INCOMPATIBLE", Field: "bindings", Message: err.Error(),
					Params: map[string]any{"criterion_code": criterion.CriterionCode, "question_code": question.QuestionCode},
				})
			}
		}
	}

	for _, c := range criteria {
		hasBinding := false
		for _, b := range bindings {
			if b.CriterionID == c.ID {
				hasBinding = true
				break
			}
		}
		if !hasBinding {
			errs = append(errs, ScoreModelReadinessError{
				Code: "CRITERION_UNBOUND", Field: "criteria", Message: "criterion has no question binding",
				Params: map[string]any{"criterion_code": c.CriterionCode},
			})
		}
	}

	return ScoreModelReadinessResult{Ready: len(errs) == 0, Errors: errs}
}

func validateNormalizationJSON(raw json.RawMessage) error {
	t, err := parseNormalizationType(raw)
	if err != nil {
		return err
	}
	switch t {
	case NormalizationNumberLinear:
		var cfg NumberLinearNormalization
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return err
		}
		if cfg.Max <= cfg.Min {
			return fmt.Errorf("NUMBER_LINEAR max must be greater than min")
		}
	case NormalizationBooleanMap:
		var cfg BooleanMapNormalization
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return err
		}
	case NormalizationOptionMap:
		var cfg OptionMapNormalization
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return err
		}
		if len(cfg.OptionScores) == 0 {
			return fmt.Errorf("OPTION_MAP requires option_scores")
		}
	case NormalizationMultiSelect:
		var cfg MultiSelectNormalization
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return err
		}
		if cfg.Aggregation == "" || len(cfg.OptionScores) == 0 {
			return fmt.Errorf("MULTI_SELECT requires aggregation and option_scores")
		}
		if err := validateMultiSelectAggregation(raw); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported normalization type %q", t)
	}
	return nil
}

func validateKnockoutCompatibility(question Question, knockoutJSON json.RawMessage) error {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(knockoutJSON, &probe); err != nil {
		return err
	}
	switch probe.Type {
	case KnockoutBooleanEquals:
		if question.QuestionType != QuestionTypeYesNo {
			return fmt.Errorf("BOOLEAN_EQUALS knockout requires YES_NO question")
		}
	case KnockoutOptionEquals:
		if question.QuestionType != QuestionTypeSingleSelect {
			return fmt.Errorf("OPTION_EQUALS knockout requires SINGLE_SELECT question")
		}
	case KnockoutNumberThreshold:
		if question.QuestionType != QuestionTypeNumber && question.QuestionType != QuestionTypePercent {
			return fmt.Errorf("NUMBER_THRESHOLD knockout requires NUMBER question")
		}
	default:
		return fmt.Errorf("unsupported knockout type %q", probe.Type)
	}
	return nil
}

func isFiniteWeight(w float64) bool {
	return !math.IsNaN(w) && !math.IsInf(w, 0)
}

func validateMultiSelectAggregation(normJSON json.RawMessage) error {
	t, err := parseNormalizationType(normJSON)
	if err != nil || t != NormalizationMultiSelect {
		return nil
	}
	var cfg MultiSelectNormalization
	if err := json.Unmarshal(normJSON, &cfg); err != nil {
		return err
	}
	switch cfg.Aggregation {
	case MultiSelectAggregationSumCapped, MultiSelectAggregationMax, MultiSelectAggregationAverage:
		return nil
	default:
		return fmt.Errorf("unsupported MULTI_SELECT aggregation %q", cfg.Aggregation)
	}
}

func validateOptionScoresAgainstQuestion(normJSON json.RawMessage, question Question) error {
	t, err := parseNormalizationType(normJSON)
	if err != nil {
		return nil
	}
	var optionScores map[string]float64
	switch t {
	case NormalizationOptionMap:
		var cfg OptionMapNormalization
		if err := json.Unmarshal(normJSON, &cfg); err != nil {
			return err
		}
		optionScores = cfg.OptionScores
	case NormalizationMultiSelect:
		var cfg MultiSelectNormalization
		if err := json.Unmarshal(normJSON, &cfg); err != nil {
			return err
		}
		optionScores = cfg.OptionScores
	default:
		return nil
	}
	if len(optionScores) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, opt := range question.Options {
		allowed[opt.OptionCode] = struct{}{}
	}
	for code := range optionScores {
		if _, ok := allowed[code]; !ok {
			return fmt.Errorf("option score references unknown option %q", code)
		}
	}
	return nil
}
