package domain

import (
	"encoding/json"
	"testing"
)

func TestEvaluateConditionGroup_Matching(t *testing.T) {
	delay := int64(3600)
	ctx := AutomationContext{
		Attributes: TriggerAttributes{
			RiskLevel:             "high",
			ProjectedDelaySeconds: &delay,
		},
	}
	group := ConditionGroup{
		Logic: "ALL",
		Conditions: []ConditionClause{
			{Field: "riskLevel", Operator: "eq", Value: json.RawMessage(`"high"`)},
			{Field: "projectedDelaySeconds", Operator: "gte", Value: json.RawMessage(`1800`)},
		},
	}
	ok, results := EvaluateConditionGroup(group, ctx)
	if !ok {
		t.Fatalf("expected match, got false: %#v", results)
	}
}

func TestEvaluateConditionGroup_NonMatching(t *testing.T) {
	ctx := AutomationContext{Attributes: TriggerAttributes{RiskLevel: "low"}}
	group := ConditionGroup{
		Logic: "ALL",
		Conditions: []ConditionClause{
			{Field: "riskLevel", Operator: "eq", Value: json.RawMessage(`"high"`)},
		},
	}
	ok, _ := EvaluateConditionGroup(group, ctx)
	if ok {
		t.Fatal("expected non-match")
	}
}

func TestEvaluateConditionGroup_ANY(t *testing.T) {
	ctx := AutomationContext{Attributes: TriggerAttributes{ETAStatus: "stale"}}
	group := ConditionGroup{
		Logic: "ANY",
		Conditions: []ConditionClause{
			{Field: "etaStatus", Operator: "eq", Value: json.RawMessage(`"available"`)},
			{Field: "etaStatus", Operator: "eq", Value: json.RawMessage(`"stale"`)},
		},
	}
	ok, _ := EvaluateConditionGroup(group, ctx)
	if !ok {
		t.Fatal("expected ANY match")
	}
}

func TestEvaluateConditionGroup_InvalidOperator(t *testing.T) {
	ctx := AutomationContext{Attributes: TriggerAttributes{RiskLevel: "high"}}
	group := ConditionGroup{
		Logic: "ALL",
		Conditions: []ConditionClause{
			{Field: "riskLevel", Operator: "contains", Value: json.RawMessage(`"high"`)},
		},
	}
	ok, _ := EvaluateConditionGroup(group, ctx)
	if ok {
		t.Fatal("invalid operator should not match")
	}
}

func TestValidateConditionGroup_FailClosed(t *testing.T) {
	err := ValidateConditionGroup(ConditionGroup{Logic: "ALL", Conditions: []ConditionClause{
		{Field: "unknown_field", Operator: "eq", Value: json.RawMessage(`"x"`)},
	}}, 0)
	if err == nil {
		t.Fatal("expected validation error for unknown field")
	}
}
