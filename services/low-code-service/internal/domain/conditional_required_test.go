package domain

import (
	"encoding/json"
	"testing"
)

func TestCollectConditionallyRequiredFieldsCargoClass(t *testing.T) {
	fields := map[string]FieldDefinition{
		"cargo_class": {
			Code:      "cargo_class",
			FieldType: "SELECT",
			ValidationRuleJSON: json.RawMessage(
				`{"if":{"field":"cargo_class","in":["A","B","C"]},"then":{"required":["loading_window_note"]}}`,
			),
		},
		"loading_window_note": {Code: "loading_window_note", FieldType: "TEXT"},
	}

	withNote := ValueSnapshot{
		"cargo_class":         json.RawMessage(`"A"`),
		"loading_window_note": json.RawMessage(`"note"`),
	}
	required := CollectConditionallyRequiredFields(fields, withNote, ValidationContext{})
	if len(required) != 1 {
		t.Fatalf("expected 1 required field, got %d", len(required))
	}
	if _, ok := required["loading_window_note"]; !ok {
		t.Fatal("expected loading_window_note to be required")
	}

	general := ValueSnapshot{"cargo_class": json.RawMessage(`"GENERAL"`)}
	required = CollectConditionallyRequiredFields(fields, general, ValidationContext{})
	if len(required) != 0 {
		t.Fatalf("expected no required fields for GENERAL, got %d", len(required))
	}
}

func TestValidateConditionalRequiredFieldsFailsWhenMissing(t *testing.T) {
	fields := map[string]FieldDefinition{
		"cargo_class": {
			Code:      "cargo_class",
			FieldType: "SELECT",
			ValidationRuleJSON: json.RawMessage(
				`{"if":{"field":"cargo_class","in":["A","B","C"]},"then":{"required":["loading_window_note"]}}`,
			),
		},
		"loading_window_note": {Code: "loading_window_note", FieldType: "TEXT"},
	}
	values := ValueSnapshot{"cargo_class": json.RawMessage(`"A"`)}
	if err := ValidateConditionalRequiredFields(fields, values, ValidationContext{}); err == nil {
		t.Fatal("expected validation failure")
	}
}

func TestValidateConditionalRequiredFieldsPassesWithValue(t *testing.T) {
	fields := map[string]FieldDefinition{
		"cargo_class": {
			Code:      "cargo_class",
			FieldType: "SELECT",
			ValidationRuleJSON: json.RawMessage(
				`{"if":{"field":"cargo_class","in":["A","B","C"]},"then":{"required":["loading_window_note"]}}`,
			),
		},
		"loading_window_note": {Code: "loading_window_note", FieldType: "TEXT"},
	}
	values := ValueSnapshot{
		"cargo_class":         json.RawMessage(`"A"`),
		"loading_window_note": json.RawMessage(`"window"`),
	}
	if err := ValidateConditionalRequiredFields(fields, values, ValidationContext{}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestEvaluateRuleConditionEntityStatus(t *testing.T) {
	condition := map[string]any{
		"context.entity_status": map[string]any{"in": []any{"APPROVED", "PAID"}},
	}
	if !EvaluateRuleCondition(condition, ValueSnapshot{}, ValidationContext{EntityStatus: "APPROVED"}) {
		t.Fatal("expected APPROVED to match")
	}
	if EvaluateRuleCondition(condition, ValueSnapshot{}, ValidationContext{EntityStatus: "DRAFT"}) {
		t.Fatal("expected DRAFT not to match")
	}
}

func TestHasFieldValue(t *testing.T) {
	if HasFieldValue(json.RawMessage(`null`)) {
		t.Fatal("null should be empty")
	}
	if HasFieldValue(json.RawMessage(`""`)) {
		t.Fatal("empty string should be empty")
	}
	if !HasFieldValue(json.RawMessage(`"x"`)) {
		t.Fatal("non-empty string should have value")
	}
}
