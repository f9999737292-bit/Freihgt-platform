package domain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

var allowedOperators = map[string]struct{}{
	"eq": {}, "neq": {}, "in": {}, "not_in": {},
	"gt": {}, "gte": {}, "lt": {}, "lte": {},
	"exists": {}, "not_exists": {},
}

var allowedConditionFields = map[string]struct{}{
	"itemType": {}, "workflowStatus": {}, "priority": {}, "businessImpact": {},
	"exceptionCategory": {}, "riskLevel": {}, "riskStatus": {}, "predictedExceptionType": {},
	"slaStatus": {}, "escalationLevel": {}, "trackingStatus": {}, "trackingQuality": {},
	"etaStatus": {}, "arrivalProjection": {}, "projectedDelaySeconds": {},
	"slotType": {}, "slotArrivalProjection": {}, "slotProjectedLateSeconds": {},
	"caseStatus": {}, "caseSeverity": {}, "assigned": {}, "hasActiveCase": {},
}

var allowedActionCodes = map[string]struct{}{
	"contact_carrier": {}, "contact_driver": {}, "contact_shipper": {},
	"request_eta_update": {}, "request_documents": {}, "check_tracking": {},
	"review_slot": {}, "request_slot_reschedule": {},
	"review_vehicle_assignment": {}, "review_driver_assignment": {},
	"create_case": {}, "create_action_item": {}, "monitor": {}, "other": {},
}

func ValidateConditionGroup(group ConditionGroup, depth int) error {
	if depth > MaxConditionNesting {
		return apperrors.Validation("condition nesting exceeds limit", map[string]any{"max": MaxConditionNesting})
	}
	logic := strings.ToUpper(strings.TrimSpace(group.Logic))
	if logic != "ALL" && logic != "ANY" {
		return apperrors.Validation("condition logic must be ALL or ANY", map[string]any{"field": "logic"})
	}
	total := len(group.Conditions)
	for _, g := range group.Groups {
		total += countConditions(g)
	}
	if total == 0 {
		return apperrors.Validation("at least one condition is required", map[string]any{"field": "conditions"})
	}
	if total > MaxConditionsPerRule {
		return apperrors.Validation("condition count exceeds limit", map[string]any{"max": MaxConditionsPerRule})
	}
	for _, c := range group.Conditions {
		if err := validateClause(c); err != nil {
			return err
		}
	}
	for _, g := range group.Groups {
		if err := ValidateConditionGroup(g, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func countConditions(group ConditionGroup) int {
	n := len(group.Conditions)
	for _, g := range group.Groups {
		n += countConditions(g)
	}
	return n
}

func validateClause(c ConditionClause) error {
	field := strings.TrimSpace(c.Field)
	if _, ok := allowedConditionFields[field]; !ok {
		return apperrors.Validation("unknown condition field", map[string]any{"field": field})
	}
	op := strings.TrimSpace(c.Operator)
	if _, ok := allowedOperators[op]; !ok {
		return apperrors.Validation("unknown condition operator", map[string]any{"operator": op})
	}
	switch op {
	case "exists", "not_exists":
		return nil
	case "in", "not_in":
		var arr []any
		if err := json.Unmarshal(c.Value, &arr); err != nil || len(arr) == 0 {
			return apperrors.Validation("in/not_in requires non-empty array value", map[string]any{"field": field})
		}
	default:
		if len(c.Value) == 0 {
			return apperrors.Validation("condition value is required", map[string]any{"field": field})
		}
	}
	return nil
}

func ValidateActionCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	if _, ok := allowedActionCodes[code]; !ok {
		return apperrors.Validation("unknown action code", map[string]any{"actionCode": code})
	}
	return nil
}

func ValidateExecutionMode(mode string) error {
	mode = strings.TrimSpace(mode)
	switch mode {
	case ExecutionModeObserve, ExecutionModeRecommend, ExecutionModeGuardedAuto:
		return nil
	default:
		return apperrors.Validation("invalid execution mode", map[string]any{"executionMode": mode})
	}
}

func ValidateTriggerType(triggerType string) error {
	if _, ok := AllowedTriggerTypes[strings.TrimSpace(triggerType)]; !ok {
		return apperrors.Validation("invalid trigger type", map[string]any{"triggerType": triggerType})
	}
	return nil
}

func EvaluateConditionGroup(group ConditionGroup, ctx AutomationContext) (bool, []MatchedCondition) {
	logic := strings.ToUpper(strings.TrimSpace(group.Logic))
	var results []MatchedCondition
	for _, c := range group.Conditions {
		m := evaluateClause(c, ctx)
		results = append(results, m)
	}
	for _, g := range group.Groups {
		ok, sub := EvaluateConditionGroup(g, ctx)
		results = append(results, sub...)
		if logic == "ALL" && !ok {
			return false, results
		}
		if logic == "ANY" && ok {
			return true, results
		}
	}
	if logic == "ALL" {
		for _, r := range results {
			if !r.Matched {
				return false, results
			}
		}
		return len(results) > 0, results
	}
	for _, r := range results {
		if r.Matched {
			return true, results
		}
	}
	return false, results
}

func evaluateClause(c ConditionClause, ctx AutomationContext) MatchedCondition {
	actual := fieldValue(c.Field, ctx)
	op := strings.TrimSpace(c.Operator)
	m := MatchedCondition{Field: c.Field, Operator: op, Actual: actual}

	switch op {
	case "exists":
		m.Matched = !isEmpty(actual)
		m.Expected = true
	case "not_exists":
		m.Matched = isEmpty(actual)
		m.Expected = false
	case "eq":
		var expected any
		_ = json.Unmarshal(c.Value, &expected)
		m.Expected = expected
		m.Matched = compareEqual(actual, expected)
	case "neq":
		var expected any
		_ = json.Unmarshal(c.Value, &expected)
		m.Expected = expected
		m.Matched = !compareEqual(actual, expected)
	case "in":
		var expected []any
		_ = json.Unmarshal(c.Value, &expected)
		m.Expected = expected
		m.Matched = inSlice(actual, expected)
	case "not_in":
		var expected []any
		_ = json.Unmarshal(c.Value, &expected)
		m.Expected = expected
		m.Matched = !inSlice(actual, expected)
	case "gt", "gte", "lt", "lte":
		var expected any
		_ = json.Unmarshal(c.Value, &expected)
		m.Expected = expected
		m.Matched = compareNumeric(actual, expected, op)
	}
	return m
}

func fieldValue(field string, ctx AutomationContext) any {
	a := ctx.Attributes
	switch field {
	case "itemType":
		return a.ItemType
	case "workflowStatus":
		return a.WorkflowStatus
	case "priority":
		return a.Priority
	case "businessImpact":
		return a.BusinessImpact
	case "exceptionCategory":
		return a.ExceptionCategory
	case "riskLevel":
		return a.RiskLevel
	case "riskStatus":
		return a.RiskStatus
	case "predictedExceptionType":
		return a.PredictedExceptionType
	case "slaStatus":
		return a.SLAStatus
	case "escalationLevel":
		return a.EscalationLevel
	case "trackingStatus":
		return a.TrackingStatus
	case "trackingQuality":
		return a.TrackingQuality
	case "etaStatus":
		return a.ETAStatus
	case "arrivalProjection":
		return a.ArrivalProjection
	case "projectedDelaySeconds":
		if a.ProjectedDelaySeconds == nil {
			return nil
		}
		return *a.ProjectedDelaySeconds
	case "slotType":
		return a.SlotType
	case "slotArrivalProjection":
		return a.SlotArrivalProjection
	case "slotProjectedLateSeconds":
		if a.SlotProjectedLateSeconds == nil {
			return nil
		}
		return *a.SlotProjectedLateSeconds
	case "caseStatus":
		return a.CaseStatus
	case "caseSeverity":
		return a.CaseSeverity
	case "assigned":
		if a.Assigned == nil {
			return nil
		}
		return *a.Assigned
	case "hasActiveCase":
		if a.HasActiveCase == nil {
			return nil
		}
		return *a.HasActiveCase
	default:
		return nil
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case bool:
		return false
	default:
		return reflect.ValueOf(v).IsZero()
	}
}

func compareEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case int64:
		return toFloat64(b) == float64(av)
	case float64:
		return toFloat64(b) == av
	case string:
		return strings.EqualFold(av, fmt.Sprint(b))
	default:
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
}

func inSlice(actual any, expected []any) bool {
	for _, e := range expected {
		if compareEqual(actual, e) {
			return true
		}
	}
	return false
}

func compareNumeric(actual, expected any, op string) bool {
	a := toFloat64(actual)
	b := toFloat64(expected)
	switch op {
	case "gt":
		return a > b
	case "gte":
		return a >= b
	case "lt":
		return a < b
	case "lte":
		return a <= b
	default:
		return false
	}
}

func toFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	default:
		f, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f
	}
}

func BuildContextFromTrigger(trigger AutomationTrigger) AutomationContext {
	return AutomationContext{
		Trigger:    trigger,
		Attributes: trigger.Attributes,
	}
}

func FormatMatchedExplanation(results []MatchedCondition) string {
	var parts []string
	for _, r := range results {
		if r.Matched {
			parts = append(parts, fmt.Sprintf("%s %s %v", r.Field, r.Operator, r.Expected))
		}
	}
	return strings.Join(parts, "; ")
}
