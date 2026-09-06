package domain

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var carrierDateLayout = "2006-01-02"

type QuestionnaireRuntime struct {
	QuestionsByID   map[uuid.UUID]Question
	QuestionsByCode map[string]Question
	SectionByQID    map[uuid.UUID]Section
	Rules           []QuestionRule
}

func BuildQuestionnaireRuntime(def *QuestionnaireDefinition) QuestionnaireRuntime {
	rt := QuestionnaireRuntime{
		QuestionsByID:   map[uuid.UUID]Question{},
		QuestionsByCode: map[string]Question{},
		SectionByQID:    map[uuid.UUID]Section{},
		Rules:           def.Rules,
	}
	for _, sec := range def.Sections {
		for _, q := range sec.Questions {
			rt.QuestionsByID[q.ID] = q
			rt.QuestionsByCode[q.QuestionCode] = q
			rt.SectionByQID[q.ID] = sec.Section
		}
	}
	return rt
}

func EvaluateVisibleQuestions(rt QuestionnaireRuntime, answers map[string]json.RawMessage) map[uuid.UUID]bool {
	visible := make(map[uuid.UUID]bool, len(rt.QuestionsByID))
	for id, q := range rt.QuestionsByID {
		visible[id] = isQuestionVisible(q, rt, answers)
	}
	return visible
}

func isQuestionVisible(q Question, rt QuestionnaireRuntime, answers map[string]json.RawMessage) bool {
	show := true
	for _, rule := range rt.Rules {
		if rule.TargetQuestionID == nil || *rule.TargetQuestionID != q.ID {
			continue
		}
		match, err := EvaluateCondition(rule.ConditionJSON, answers)
		if err != nil {
			continue
		}
		switch rule.Action {
		case RuleActionHide:
			if match {
				show = false
			}
		case RuleActionShow:
			if !match {
				show = false
			}
		}
	}
	return show
}

func EvaluateRequiredQuestions(rt QuestionnaireRuntime, answers map[string]json.RawMessage, visible map[uuid.UUID]bool) map[uuid.UUID]bool {
	required := make(map[uuid.UUID]bool, len(rt.QuestionsByID))
	for id, q := range rt.QuestionsByID {
		if !visible[id] {
			continue
		}
		if q.Required {
			required[id] = true
		}
	}
	for _, rule := range rt.Rules {
		if rule.TargetQuestionID == nil || rule.Action != RuleActionRequire {
			continue
		}
		id := *rule.TargetQuestionID
		if !visible[id] {
			continue
		}
		match, err := EvaluateCondition(rule.ConditionJSON, answers)
		if err == nil && match {
			required[id] = true
		}
	}
	return required
}

func MergeAnswerMaps(existing map[uuid.UUID]json.RawMessage, patches []AnswerPatchItem) map[uuid.UUID]json.RawMessage {
	merged := make(map[uuid.UUID]json.RawMessage, len(existing)+len(patches))
	for k, v := range existing {
		merged[k] = v
	}
	for _, patch := range patches {
		merged[patch.QuestionID] = patch.Value
	}
	return merged
}

func AnswersByQuestionCode(rt QuestionnaireRuntime, byID map[uuid.UUID]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(byID))
	for qid, val := range byID {
		q, ok := rt.QuestionsByID[qid]
		if !ok {
			continue
		}
		out[q.QuestionCode] = val
	}
	return out
}

func ValidateCarrierAnswerPatches(rt QuestionnaireRuntime, existing map[uuid.UUID]json.RawMessage, patches []AnswerPatchItem, preSubmit bool) []ValidationErrorDetail {
	merged := MergeAnswerMaps(existing, patches)
	byCode := AnswersByQuestionCode(rt, merged)
	visible := EvaluateVisibleQuestions(rt, byCode)

	errors := make([]ValidationErrorDetail, 0)
	for _, patch := range patches {
		q, ok := rt.QuestionsByID[patch.QuestionID]
		if !ok {
			errors = append(errors, ValidationErrorDetail{
				QuestionID: patch.QuestionID, Field: "value", Rule: "unknown_question",
				MessageKey: "rfx.carrier.validation.unknown_question",
			})
			continue
		}
		sec := rt.SectionByQID[patch.QuestionID]
		if !visible[patch.QuestionID] {
			errors = append(errors, ValidationErrorDetail{
				SectionID: sec.ID, QuestionID: patch.QuestionID, Field: "value", Rule: "question_hidden",
				MessageKey: "rfx.carrier.validation.question_hidden",
			})
			continue
		}
		if fieldErrs := validateFieldValue(q, patch.Value); len(fieldErrs) > 0 {
			errors = append(errors, fieldErrs...)
		}
	}
	if preSubmit {
		errors = append(errors, validateFullCarrierAnswers(rt, merged, visible)...)
	}
	return errors
}

func ValidateCarrierAnswers(rt QuestionnaireRuntime, byID map[uuid.UUID]json.RawMessage, preSubmit bool) []ValidationErrorDetail {
	byCode := AnswersByQuestionCode(rt, byID)
	visible := EvaluateVisibleQuestions(rt, byCode)
	errors := validateFullCarrierAnswers(rt, byID, visible)
	if preSubmit {
		errors = append(errors, validateCrossFieldRules(rt, byCode, visible)...)
	}
	return errors
}

func validateFullCarrierAnswers(rt QuestionnaireRuntime, byID map[uuid.UUID]json.RawMessage, visible map[uuid.UUID]bool) []ValidationErrorDetail {
	byCode := AnswersByQuestionCode(rt, byID)
	required := EvaluateRequiredQuestions(rt, byCode, visible)

	errors := make([]ValidationErrorDetail, 0)
	for id, q := range rt.QuestionsByID {
		if !visible[id] {
			continue
		}
		val := byID[id]
		sec := rt.SectionByQID[id]
		if fieldErrs := validateFieldValue(q, val); len(fieldErrs) > 0 {
			errors = append(errors, fieldErrs...)
			continue
		}
		if required[id] && isAnswerEmpty(val) {
			errors = append(errors, ValidationErrorDetail{
				SectionID: sec.ID, QuestionID: id, Field: "value", Rule: "required",
				MessageKey: "rfx.carrier.validation.required",
				Params:     map[string]any{"question_code": q.QuestionCode},
			})
		}
	}
	return errors
}

func validateCrossFieldRules(rt QuestionnaireRuntime, answers map[string]json.RawMessage, visible map[uuid.UUID]bool) []ValidationErrorDetail {
	// L3 cross-field validation uses same rule engine preconditions for now.
	return nil
}

func validateFieldValue(q Question, raw json.RawMessage) []ValidationErrorDetail {
	secID := q.SectionID
	errs := make([]ValidationErrorDetail, 0)
	if isAnswerEmpty(raw) {
		return errs
	}
	switch q.QuestionType {
	case QuestionTypeText, QuestionTypeLongText:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return []ValidationErrorDetail{invalidType(q, secID, "text")}
		}
		var rules ValidationDefinition
		_ = json.Unmarshal(q.ValidationRuleJSON, &rules)
		if rules.MinLength != nil && len([]rune(s)) < *rules.MinLength {
			errs = append(errs, ValidationErrorDetail{
				SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "min_length",
				MessageKey: "rfx.carrier.validation.min_length", Params: map[string]any{"min": *rules.MinLength},
			})
		}
		if rules.MaxLength != nil && len([]rune(s)) > *rules.MaxLength {
			errs = append(errs, ValidationErrorDetail{
				SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "max_length",
				MessageKey: "rfx.carrier.validation.max_length", Params: map[string]any{"max": *rules.MaxLength},
			})
		}
		if rules.Pattern != nil && *rules.Pattern != "" {
			re, err := regexp.Compile(*rules.Pattern)
			if err == nil && !re.MatchString(s) {
				errs = append(errs, ValidationErrorDetail{
					SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "pattern",
					MessageKey: "rfx.carrier.validation.pattern", Params: map[string]any{},
				})
			}
		}
	case QuestionTypeNumber:
		if !jsonNumberValid(raw) {
			return []ValidationErrorDetail{invalidType(q, secID, "number")}
		}
	case QuestionTypeYesNo:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			var s string
			if err2 := json.Unmarshal(raw, &s); err2 != nil || !isYesNoString(s) {
				return []ValidationErrorDetail{invalidType(q, secID, "yes_no")}
			}
		}
	case QuestionTypeSingleSelect:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return []ValidationErrorDetail{invalidType(q, secID, "single_select")}
		}
		if !optionExists(q, s) {
			errs = append(errs, ValidationErrorDetail{
				SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "enum",
				MessageKey: "rfx.carrier.validation.invalid_option", Params: map[string]any{},
			})
		}
	case QuestionTypeMultiSelect:
		var items []string
		if err := json.Unmarshal(raw, &items); err != nil {
			return []ValidationErrorDetail{invalidType(q, secID, "multi_select")}
		}
		for _, item := range items {
			if !optionExists(q, item) {
				errs = append(errs, ValidationErrorDetail{
					SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "enum",
					MessageKey: "rfx.carrier.validation.invalid_option", Params: map[string]any{"value": item},
				})
			}
		}
	case QuestionTypeDate:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return []ValidationErrorDetail{invalidType(q, secID, "date")}
		}
		if _, err := time.Parse(carrierDateLayout, strings.TrimSpace(s)); err != nil {
			errs = append(errs, ValidationErrorDetail{
				SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "date_format",
				MessageKey: "rfx.carrier.validation.date_format", Params: map[string]any{"format": carrierDateLayout},
			})
		}
	default:
		errs = append(errs, ValidationErrorDetail{
			SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "unsupported_type",
			MessageKey: "rfx.carrier.validation.unsupported_type",
			Params:     map[string]any{"question_type": q.QuestionType},
		})
	}
	return errs
}

func ComputeCompletionPercent(rt QuestionnaireRuntime, byID map[uuid.UUID]json.RawMessage) float64 {
	byCode := AnswersByQuestionCode(rt, byID)
	visible := EvaluateVisibleQuestions(rt, byCode)
	required := EvaluateRequiredQuestions(rt, byCode, visible)
	if len(required) == 0 {
		return 100
	}
	answered := 0
	for id := range required {
		if !isAnswerEmpty(byID[id]) {
			answered++
		}
	}
	return float64(answered) / float64(len(required)) * 100
}

func HiddenQuestionIDs(rt QuestionnaireRuntime, byID map[uuid.UUID]json.RawMessage) []uuid.UUID {
	byCode := AnswersByQuestionCode(rt, byID)
	visible := EvaluateVisibleQuestions(rt, byCode)
	hidden := make([]uuid.UUID, 0)
	for id := range rt.QuestionsByID {
		if !visible[id] {
			hidden = append(hidden, id)
		}
	}
	return hidden
}

func ToValidationErrorItems(details []ValidationErrorDetail) []ValidationErrorDetail {
	return details
}

func invalidType(q Question, secID uuid.UUID, expected string) ValidationErrorDetail {
	return ValidationErrorDetail{
		SectionID: secID, QuestionID: q.ID, Field: "value", Rule: "type",
		MessageKey: "rfx.carrier.validation.invalid_type", Params: map[string]any{"expected": expected},
	}
}

func optionExists(q Question, code string) bool {
	for _, opt := range q.Options {
		if opt.OptionCode == code {
			return true
		}
	}
	return false
}

func isYesNoString(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "false", "yes", "no":
		return true
	default:
		return false
	}
}

func jsonNumberValid(raw json.RawMessage) bool {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		var n json.Number
		if err2 := json.Unmarshal(raw, &n); err2 != nil {
			return false
		}
	}
	return true
}
