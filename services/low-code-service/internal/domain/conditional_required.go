package domain

import (
	"encoding/json"
	"fmt"
	"strings"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type ValidationContext struct {
	EntityStatus string
	Role         string
}

type ValueSnapshot map[string]json.RawMessage

func BuildValueSnapshot(existing []CustomFieldValue, incoming []CustomFieldValueInput) ValueSnapshot {
	merged := make(ValueSnapshot, len(existing)+len(incoming))
	for _, item := range existing {
		merged[item.FieldCode] = item.ValueJSON
	}
	for _, item := range incoming {
		merged[item.FieldCode] = item.ValueJSON
	}
	return merged
}

func HasFieldValue(raw json.RawMessage) bool {
	if isJSONNull(raw) {
		return false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case nil:
		return false
	default:
		return true
	}
}

func IsConditionalRequiredRule(raw json.RawMessage) bool {
	if len(raw) == 0 || isJSONNull(raw) {
		return false
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	ifClause, ok := payload["if"]
	if !ok || isJSONNull(ifClause) {
		return false
	}
	var ifObj map[string]any
	if err := json.Unmarshal(ifClause, &ifObj); err != nil || len(ifObj) == 0 {
		return false
	}
	thenClause, ok := payload["then"]
	if !ok || isJSONNull(thenClause) {
		return false
	}
	var thenObj struct {
		Required json.RawMessage `json:"required"`
	}
	if err := json.Unmarshal(thenClause, &thenObj); err != nil {
		return false
	}
	if string(thenObj.Required) == "true" {
		return true
	}
	var requiredList []string
	if err := json.Unmarshal(thenObj.Required, &requiredList); err != nil {
		return false
	}
	for _, code := range requiredList {
		if strings.TrimSpace(code) != "" {
			return true
		}
	}
	return false
}

func CollectConditionallyRequiredFields(
	fields map[string]FieldDefinition,
	values ValueSnapshot,
	context ValidationContext,
) map[string]struct{} {
	required := map[string]struct{}{}
	for _, field := range fields {
		if !IsConditionalRequiredRule(field.ValidationRuleJSON) {
			continue
		}
		var payload struct {
			If   json.RawMessage `json:"if"`
			Then json.RawMessage `json:"then"`
		}
		if err := json.Unmarshal(field.ValidationRuleJSON, &payload); err != nil {
			continue
		}
		var ifClause map[string]any
		if err := json.Unmarshal(payload.If, &ifClause); err != nil {
			continue
		}
		if !EvaluateRuleCondition(ifClause, values, context) {
			continue
		}
		applyConditionalRequiredAction(payload.Then, field.Code, required)
	}
	return required
}

func ValidateConditionalRequiredFields(
	fields map[string]FieldDefinition,
	values ValueSnapshot,
	context ValidationContext,
) error {
	required := CollectConditionallyRequiredFields(fields, values, context)
	for code := range required {
		field, ok := fields[code]
		if !ok || field.SystemField {
			continue
		}
		raw, exists := values[code]
		if !exists || !HasFieldValue(raw) {
			return apperrors.ValidationRuleFailed(code, map[string]any{
				"reason": "conditionally required field",
			})
		}
	}
	return nil
}

func applyConditionalRequiredAction(thenClause json.RawMessage, sourceFieldCode string, target map[string]struct{}) {
	var payload struct {
		Required json.RawMessage `json:"required"`
	}
	if err := json.Unmarshal(thenClause, &payload); err != nil {
		return
	}
	if string(payload.Required) == "true" {
		target[sourceFieldCode] = struct{}{}
		return
	}
	var requiredList []string
	if err := json.Unmarshal(payload.Required, &requiredList); err != nil {
		return
	}
	for _, code := range requiredList {
		code = strings.TrimSpace(code)
		if code != "" {
			target[code] = struct{}{}
		}
	}
}

func EvaluateRuleCondition(
	condition map[string]any,
	values ValueSnapshot,
	context ValidationContext,
) bool {
	if len(condition) == 0 {
		return true
	}

	matched := true
	reserved := map[string]struct{}{
		"field": {}, "equals": {}, "not_equals": {}, "in": {}, "not_in": {},
	}

	if fieldCode, ok := condition["field"].(string); ok && strings.TrimSpace(fieldCode) != "" {
		actual := snapshotFieldValue(values, strings.TrimSpace(fieldCode))
		if expected, exists := condition["equals"]; exists {
			matched = matched && valuesEqual(actual, expected)
		}
		if expected, exists := condition["not_equals"]; exists {
			matched = matched && !valuesEqual(actual, expected)
		}
		if expected, exists := condition["in"]; exists {
			matched = matched && valueInList(actual, toAnySlice(expected))
		}
		if expected, exists := condition["not_in"]; exists {
			matched = matched && !valueInList(actual, toAnySlice(expected))
		}
	}

	for key, expected := range condition {
		if _, reservedKey := reserved[key]; reservedKey {
			continue
		}
		if strings.HasPrefix(key, "context.") {
			matched = matched && evaluateContextCondition(key, expected, context)
			continue
		}
		matched = matched && evaluateFieldCondition(key, expected, values)
	}

	return matched
}

func evaluateContextCondition(key string, expected any, context ValidationContext) bool {
	switch key {
	case "context.role":
		return valuesEqual(context.Role, expected)
	case "context.entity_status":
		if expectedMap, ok := expected.(map[string]any); ok {
			if inList, ok := expectedMap["in"]; ok {
				return valueInList(context.EntityStatus, toAnySlice(inList))
			}
		}
		return valuesEqual(context.EntityStatus, expected)
	default:
		return false
	}
}

func evaluateFieldCondition(fieldCode string, expected any, values ValueSnapshot) bool {
	actual := snapshotFieldValue(values, fieldCode)
	if expectedMap, ok := expected.(map[string]any); ok {
		if inList, ok := expectedMap["in"]; ok {
			return valueInList(actual, toAnySlice(inList))
		}
		if notInList, ok := expectedMap["not_in"]; ok {
			return !valueInList(actual, toAnySlice(notInList))
		}
	}
	return valuesEqual(actual, expected)
}

func snapshotFieldValue(values ValueSnapshot, fieldCode string) any {
	raw, ok := values[fieldCode]
	if !ok || isJSONNull(raw) {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

func valuesEqual(left, right any) bool {
	if left == nil || left == "" {
		return right == nil || right == ""
	}
	if right == nil {
		return false
	}
	return stringifyValue(left) == stringifyValue(right)
}

func valueInList(value any, list []any) bool {
	for _, item := range list {
		if valuesEqual(value, item) {
			return true
		}
	}
	return false
}

func stringifyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%g", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		bytes, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}

func toAnySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = item
		}
		return out
	default:
		return nil
	}
}
