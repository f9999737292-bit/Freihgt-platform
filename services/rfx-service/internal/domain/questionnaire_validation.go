package domain

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func ValidateValidationDefinition(questionType string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var def ValidationDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		return apperrors.Validation("invalid validation_rule_json", map[string]any{"field": "validation_rule_json"})
	}
	if def.MinLength != nil && *def.MinLength < 0 {
		return apperrors.Validation("min_length cannot be negative", map[string]any{"field": "validation_rule_json.min_length"})
	}
	if def.MaxLength != nil && *def.MaxLength < 0 {
		return apperrors.Validation("max_length cannot be negative", map[string]any{"field": "validation_rule_json.max_length"})
	}
	if def.MinLength != nil && def.MaxLength != nil && *def.MaxLength < *def.MinLength {
		return apperrors.Validation("max_length must be >= min_length", map[string]any{"field": "validation_rule_json.max_length"})
	}
	if def.MinValue != nil && def.MaxValue != nil && *def.MaxValue < *def.MinValue {
		return apperrors.Validation("max_value must be >= min_value", map[string]any{"field": "validation_rule_json.max_value"})
	}
	if def.Pattern != nil {
		if _, err := regexp.Compile(strings.TrimSpace(*def.Pattern)); err != nil {
			return apperrors.Validation("invalid validation pattern", map[string]any{"field": "validation_rule_json.pattern"})
		}
	}
	if def.MaxFileSize != nil && *def.MaxFileSize < 0 {
		return apperrors.Validation("max_file_size cannot be negative", map[string]any{"field": "validation_rule_json.max_file_size"})
	}
	if questionType == QuestionTypeFile && def.MaxFileSize == nil && len(def.AllowedMIME) == 0 {
		// file questions may omit constraints in draft; publish readiness will warn
	}
	return nil
}

func ValidateQuestionnaireDefinition(sections []SectionWithQuestions, rules []QuestionRule) error {
	questionCodes := make(map[string]struct{})
	questionCodeByID := make(map[uuid.UUID]string)
	questionTypeByCode := make(map[string]string)
	for _, swq := range sections {
		if err := ValidateSectionCode(swq.Section.SectionCode); err != nil {
			return err
		}
		if strings.TrimSpace(swq.Section.Title) == "" {
			return apperrors.Validation("section title is required", map[string]any{"field": "title", "section_code": swq.Section.SectionCode})
		}
		seenQuestionCodes := make(map[string]struct{})
		for _, q := range swq.Questions {
			if err := ValidateQuestionCode(q.QuestionCode); err != nil {
				return err
			}
			if _, dup := seenQuestionCodes[q.QuestionCode]; dup {
				return apperrors.Validation("duplicate question_code in section", map[string]any{"field": "question_code", "question_code": q.QuestionCode})
			}
			seenQuestionCodes[q.QuestionCode] = struct{}{}
			if _, dup := questionCodes[q.QuestionCode]; dup {
				return apperrors.Validation("duplicate question_code in questionnaire", map[string]any{"field": "question_code", "question_code": q.QuestionCode})
			}
			questionCodes[q.QuestionCode] = struct{}{}
			questionCodeByID[q.ID] = q.QuestionCode
			questionTypeByCode[q.QuestionCode] = q.QuestionType
			if err := ValidateQuestionType(q.QuestionType); err != nil {
				return err
			}
			if strings.TrimSpace(q.Label) == "" {
				return apperrors.Validation("question label is required", map[string]any{"field": "label", "question_code": q.QuestionCode})
			}
			if err := ValidateValidationDefinition(q.QuestionType, q.ValidationRuleJSON); err != nil {
				return err
			}
			seenOptionCodes := make(map[string]struct{})
			for _, opt := range q.Options {
				if err := ValidateOptionCode(opt.OptionCode); err != nil {
					return err
				}
				if _, dup := seenOptionCodes[opt.OptionCode]; dup {
					return apperrors.Validation("duplicate option_code", map[string]any{"field": "option_code", "question_code": q.QuestionCode})
				}
				seenOptionCodes[opt.OptionCode] = struct{}{}
				if strings.TrimSpace(opt.Label) == "" {
					return apperrors.Validation("option label is required", map[string]any{"field": "label", "option_code": opt.OptionCode})
				}
			}
		}
	}
	return ValidateRuleSet(rules, questionCodes, questionCodeByID, questionTypeByCode)
}

func ValidateQuestionnaireStructure(sections []SectionWithQuestions, rules []QuestionRule) error {
	if err := ValidateQuestionnaireDefinition(sections, rules); err != nil {
		return err
	}
	for _, swq := range sections {
		for _, q := range swq.Questions {
			if QuestionTypeRequiresOptions(q.QuestionType) && len(q.Options) == 0 {
				return apperrors.Validation("select questions require options", map[string]any{"field": "options", "question_code": q.QuestionCode})
			}
		}
	}
	return nil
}

func EvaluatePublishReadiness(version RfxVersion, sections []SectionWithQuestions, rules []QuestionRule) PublishReadinessResult {
	result := PublishReadinessResult{Ready: true, Items: []PublishReadinessItem{}}
	if !version.QuestionnaireEnabled {
		result.Items = append(result.Items, PublishReadinessItem{
			Code:    "QUESTIONNAIRE_DISABLED",
			Status:  PublishCheckPass,
			Message: "Questionnaire is disabled for this version",
		})
		return result
	}
	if len(sections) == 0 {
		result.Ready = false
		result.BlockingFail++
		result.Items = append(result.Items, PublishReadinessItem{
			Code:    "NO_SECTIONS",
			Status:  PublishCheckFail,
			Message: "At least one section is required",
		})
	}
	questionCount := 0
	for _, swq := range sections {
		if len(swq.Questions) == 0 {
			result.WarningCount++
			result.Items = append(result.Items, PublishReadinessItem{
				Code:    "EMPTY_SECTION",
				Status:  PublishCheckWarn,
				Message: "Section has no questions",
				Details: map[string]any{"section_code": swq.Section.SectionCode},
			})
		}
		questionCount += len(swq.Questions)
	}
	if questionCount == 0 {
		result.Ready = false
		result.BlockingFail++
		result.Items = append(result.Items, PublishReadinessItem{
			Code:    "NO_QUESTIONS",
			Status:  PublishCheckFail,
			Message: "At least one question is required",
		})
	}
	if err := ValidateQuestionnaireStructure(sections, rules); err != nil {
		result.Ready = false
		result.BlockingFail++
		result.Items = append(result.Items, PublishReadinessItem{
			Code:    "INVALID_STRUCTURE",
			Status:  PublishCheckFail,
			Message: err.Error(),
		})
	}
	if result.BlockingFail == 0 {
		result.Items = append(result.Items, PublishReadinessItem{
			Code:    "STRUCTURE_VALID",
			Status:  PublishCheckPass,
			Message: "Questionnaire structure is valid",
		})
	}
	return result
}
