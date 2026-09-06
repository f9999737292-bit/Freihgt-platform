package domain

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	ConditionOperatorAnd        = "AND"
	ConditionOperatorOr         = "OR"
	ConditionOperatorEquals     = "EQUALS"
	ConditionOperatorNotEquals  = "NOT_EQUALS"
	ConditionOperatorIn         = "IN"
	ConditionOperatorNotIn      = "NOT_IN"
	ConditionOperatorIsEmpty    = "IS_EMPTY"
	ConditionOperatorIsNotEmpty = "IS_NOT_EMPTY"
	ConditionOperatorGreaterThan = "GREATER_THAN"
	ConditionOperatorLessThan    = "LESS_THAN"
)

var allowedConditionOperators = map[string]struct{}{
	ConditionOperatorAnd:         {},
	ConditionOperatorOr:          {},
	ConditionOperatorEquals:      {},
	ConditionOperatorNotEquals:   {},
	ConditionOperatorIn:          {},
	ConditionOperatorNotIn:       {},
	ConditionOperatorIsEmpty:     {},
	ConditionOperatorIsNotEmpty:  {},
	ConditionOperatorGreaterThan: {},
	ConditionOperatorLessThan:    {},
}

func ParseConditionalExpression(raw json.RawMessage) (*ConditionalExpression, error) {
	if len(raw) == 0 {
		return nil, apperrors.Validation("condition_json is required", map[string]any{"field": "condition_json"})
	}
	var expr ConditionalExpression
	if err := json.Unmarshal(raw, &expr); err != nil {
		return nil, apperrors.Validation("invalid condition_json", map[string]any{"field": "condition_json"})
	}
	if err := validateConditionalExpression(&expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func ValidateConditionalExpression(raw json.RawMessage) error {
	_, err := ParseConditionalExpression(raw)
	return err
}

func validateConditionalExpression(expr *ConditionalExpression) error {
	if expr == nil {
		return apperrors.Validation("condition_json is required", map[string]any{"field": "condition_json"})
	}
	op := strings.TrimSpace(expr.Operator)
	if op == "" {
		return apperrors.Validation("condition operator is required", map[string]any{"field": "condition_json.operator"})
	}
	if _, ok := allowedConditionOperators[op]; !ok {
		return apperrors.Validation("invalid condition operator", map[string]any{"field": "condition_json.operator", "value": op})
	}
	switch op {
	case ConditionOperatorAnd, ConditionOperatorOr:
		if len(expr.Children) == 0 {
			return apperrors.Validation("logical condition requires children", map[string]any{"field": "condition_json.children"})
		}
		for i := range expr.Children {
			if err := validateConditionalExpression(&expr.Children[i]); err != nil {
				return err
			}
		}
	default:
		if strings.TrimSpace(expr.SourceQuestionCode) == "" {
			return apperrors.Validation("source_question_code is required", map[string]any{"field": "condition_json.source_question_code"})
		}
	}
	return nil
}

func CollectConditionSourceQuestionCodes(raw json.RawMessage) ([]string, error) {
	expr, err := ParseConditionalExpression(raw)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0)
	collectSourceCodes(expr, &codes)
	return codes, nil
}

func collectSourceCodes(expr *ConditionalExpression, codes *[]string) {
	if expr == nil {
		return
	}
	op := strings.TrimSpace(expr.Operator)
	switch op {
	case ConditionOperatorAnd, ConditionOperatorOr:
		for i := range expr.Children {
			collectSourceCodes(&expr.Children[i], codes)
		}
	default:
		code := strings.TrimSpace(expr.SourceQuestionCode)
		if code != "" {
			*codes = append(*codes, code)
		}
	}
}


func DetectRuleCycles(rules []QuestionRule, questionCodeByID map[uuid.UUID]string) error {
	graph := make(map[string][]string)
	for _, rule := range rules {
		targetCode := ""
		if rule.TargetQuestionID != nil {
			targetCode = questionCodeByID[*rule.TargetQuestionID]
		}
		if targetCode == "" {
			continue
		}
		sources, err := CollectConditionSourceQuestionCodes(rule.ConditionJSON)
		if err != nil {
			return err
		}
		for _, source := range sources {
			if source == targetCode {
				return apperrors.Validation("rule references itself", map[string]any{
					"field":         "condition_json",
					"question_code": targetCode,
				})
			}
			graph[targetCode] = append(graph[targetCode], source)
		}
	}
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	var visit func(string) error
	visit = func(node string) error {
		if stack[node] {
			return apperrors.Validation("rule dependency cycle detected", map[string]any{"field": "rules", "question_code": node})
		}
		if visited[node] {
			return nil
		}
		visited[node] = true
		stack[node] = true
		for _, dep := range graph[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(stack, node)
		return nil
	}
	for node := range graph {
		if err := visit(node); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRuleSet validates structural integrity of a rule set against known question codes.
func ValidateRuleSet(rules []QuestionRule, questionCodes map[string]struct{}, questionCodeByID map[uuid.UUID]string, questionTypeByCode map[string]string) error {
	for _, rule := range rules {
		if err := ValidateRuleAction(rule.Action); err != nil {
			return err
		}
		if err := ValidateConditionalExpression(rule.ConditionJSON); err != nil {
			return err
		}
		if err := validateConditionOperatorsForQuestionTypes(rule.ConditionJSON, questionTypeByCode); err != nil {
			return err
		}
		sources, err := CollectConditionSourceQuestionCodes(rule.ConditionJSON)
		if err != nil {
			return err
		}
		for _, source := range sources {
			if _, ok := questionCodes[source]; !ok {
				return apperrors.Validation("condition references unknown question", map[string]any{
					"field":                "condition_json.source_question_code",
					"source_question_code": source,
				})
			}
		}
		if rule.TargetQuestionID != nil {
			targetCode := questionCodeByID[*rule.TargetQuestionID]
			if targetCode == "" {
				return apperrors.Validation("target question not found", map[string]any{"field": "target_question_id"})
			}
		}
	}
	return DetectRuleCycles(rules, questionCodeByID)
}

func validateConditionOperatorsForQuestionTypes(raw json.RawMessage, questionTypeByCode map[string]string) error {
	if len(questionTypeByCode) == 0 {
		return nil
	}
	expr, err := ParseConditionalExpression(raw)
	if err != nil {
		return err
	}
	return validateExpressionOperatorsForQuestionTypes(expr, questionTypeByCode)
}

func validateExpressionOperatorsForQuestionTypes(expr *ConditionalExpression, questionTypeByCode map[string]string) error {
	if expr == nil {
		return nil
	}
	op := strings.TrimSpace(expr.Operator)
	switch op {
	case ConditionOperatorAnd, ConditionOperatorOr:
		for i := range expr.Children {
			if err := validateExpressionOperatorsForQuestionTypes(&expr.Children[i], questionTypeByCode); err != nil {
				return err
			}
		}
	default:
		qType, ok := questionTypeByCode[strings.TrimSpace(expr.SourceQuestionCode)]
		if !ok {
			return nil
		}
		switch op {
		case ConditionOperatorGreaterThan, ConditionOperatorLessThan:
			if !QuestionTypeSupportsNumericValidation(qType) {
				return apperrors.Validation("condition operator incompatible with question type", map[string]any{
					"field":                "condition_json.operator",
					"operator":             op,
					"source_question_code": expr.SourceQuestionCode,
					"question_type":        qType,
				})
			}
		}
	}
	return nil
}

// EvaluateCondition evaluates a condition against answer values keyed by question code.
func EvaluateCondition(raw json.RawMessage, answers map[string]json.RawMessage) (bool, error) {
	expr, err := ParseConditionalExpression(raw)
	if err != nil {
		return false, err
	}
	return evaluateExpression(expr, answers)
}

func evaluateExpression(expr *ConditionalExpression, answers map[string]json.RawMessage) (bool, error) {
	op := strings.TrimSpace(expr.Operator)
	switch op {
	case ConditionOperatorAnd:
		for i := range expr.Children {
			ok, err := evaluateExpression(&expr.Children[i], answers)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case ConditionOperatorOr:
		for i := range expr.Children {
			ok, err := evaluateExpression(&expr.Children[i], answers)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case ConditionOperatorIsEmpty:
		return isAnswerEmpty(answers[expr.SourceQuestionCode]), nil
	case ConditionOperatorIsNotEmpty:
		return !isAnswerEmpty(answers[expr.SourceQuestionCode]), nil
	case ConditionOperatorEquals:
		return compareAnswer(answers[expr.SourceQuestionCode], expr.Value, op)
	case ConditionOperatorNotEquals:
		ok, err := compareAnswer(answers[expr.SourceQuestionCode], expr.Value, ConditionOperatorEquals)
		return !ok, err
	case ConditionOperatorIn, ConditionOperatorNotIn:
		ok, err := compareAnswer(answers[expr.SourceQuestionCode], expr.Value, op)
		if op == ConditionOperatorNotIn {
			return !ok, err
		}
		return ok, err
	case ConditionOperatorGreaterThan, ConditionOperatorLessThan:
		return compareAnswer(answers[expr.SourceQuestionCode], expr.Value, op)
	default:
		return false, apperrors.Validation("unsupported condition operator", map[string]any{"operator": op})
	}
}

func isAnswerEmpty(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return true
	}
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	default:
		return false
	}
}

func compareAnswer(actual json.RawMessage, expected json.RawMessage, operator string) (bool, error) {
	if isAnswerEmpty(actual) {
		return false, nil
	}
	var actualVal any
	if err := json.Unmarshal(actual, &actualVal); err != nil {
		return false, nil
	}
	var expectedVal any
	if err := json.Unmarshal(expected, &expectedVal); err != nil {
		return false, apperrors.Validation("invalid condition value", map[string]any{"field": "condition_json.value"})
	}
	switch operator {
	case ConditionOperatorEquals:
		return jsonEqual(actualVal, expectedVal), nil
	case ConditionOperatorIn, ConditionOperatorNotIn:
		items, ok := expectedVal.([]any)
		if !ok {
			return false, apperrors.Validation("IN condition requires array value", map[string]any{"field": "condition_json.value"})
		}
		for _, item := range items {
			if jsonEqual(actualVal, item) {
				return true, nil
			}
		}
		return false, nil
	case ConditionOperatorGreaterThan, ConditionOperatorLessThan:
		actualNum, okA := toFloat(actualVal)
		expectedNum, okE := toFloat(expectedVal)
		if !okA || !okE {
			return false, nil
		}
		if operator == ConditionOperatorGreaterThan {
			return actualNum > expectedNum, nil
		}
		return actualNum < expectedNum, nil
	default:
		return false, nil
	}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
