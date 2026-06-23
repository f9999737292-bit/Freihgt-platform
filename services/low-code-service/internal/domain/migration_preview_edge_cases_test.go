package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildMigrationPreviewItemSafeNoLegacyMissingIncompatible(t *testing.T) {
	targetID := uuid.New()
	target := PublishedTemplateContext{
		ID: targetID,
		Fields: map[string]FieldDefinition{
			"note": {Code: "note", FieldType: "TEXT"},
		},
	}

	item := BuildMigrationPreviewItem(
		uuid.New(),
		targetID,
		target,
		[]CustomFieldValue{{FieldCode: "note", ValueJSON: json.RawMessage(`"hello"`), FormTemplateID: targetID}},
		map[string]FieldDefinition{"note": {FieldType: "TEXT"}},
	)

	if item.Status != MigrationPreviewStatusSafe {
		t.Fatalf("expected SAFE, got %s: %+v", item.Status, item)
	}
	if len(item.LegacyFields) != 0 || len(item.MissingRequiredFields) != 0 || len(item.IncompatibleFields) != 0 {
		t.Fatalf("expected clean SAFE item, got %+v", item)
	}
}

func TestBuildMigrationPreviewItemLegacyWithCompatibleField(t *testing.T) {
	target := PublishedTemplateContext{
		ID: uuid.New(),
		Fields: map[string]FieldDefinition{
			"active_field": {Code: "active_field", FieldType: "TEXT"},
		},
	}

	item := BuildMigrationPreviewItem(
		uuid.New(),
		uuid.New(),
		target,
		[]CustomFieldValue{
			{FieldCode: "active_field", ValueJSON: json.RawMessage(`"ok"`)},
			{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
		},
		nil,
	)

	if item.Status != MigrationPreviewStatusWarning {
		t.Fatalf("expected WARNING, got %s", item.Status)
	}
	if len(item.CopiedFields) != 1 || item.CopiedFields[0] != "active_field" {
		t.Fatalf("expected compatible field copied, got %+v", item.CopiedFields)
	}
	if len(item.LegacyFields) != 1 || item.LegacyFields[0] != "deprecated_field" {
		t.Fatalf("expected legacy field preserved in preview, got %+v", item.LegacyFields)
	}
}

func TestBuildMigrationPreviewItemEmptyValuesWithRequiredTargetField(t *testing.T) {
	target := PublishedTemplateContext{
		ID: uuid.New(),
		Fields: map[string]FieldDefinition{
			"required_note": {Code: "required_note", FieldType: "TEXT", Required: true},
		},
	}

	item := BuildMigrationPreviewItem(uuid.New(), uuid.Nil, target, nil, nil)
	if item.Status != MigrationPreviewStatusWarning {
		t.Fatalf("expected WARNING, got %s", item.Status)
	}
	if len(item.CopiedFields) != 0 {
		t.Fatalf("expected no copied fields, got %+v", item.CopiedFields)
	}
	if len(item.MissingRequiredFields) != 1 {
		t.Fatalf("expected missing required, got %+v", item.MissingRequiredFields)
	}
}

func TestBuildMigrationPreviewItemIncompatibleMoneyNumberDateTextCheckbox(t *testing.T) {
	cases := []struct {
		name       string
		sourceType string
		targetType string
		value      json.RawMessage
	}{
		{"money_number", "NUMBER", "MONEY", json.RawMessage(`42`)},
		{"number_money", "MONEY", "NUMBER", json.RawMessage(`{"amount":1,"currency":"USD"}`)},
		{"date_text", "TEXT", "DATE", json.RawMessage(`"2026-01-01"`)},
		{"checkbox_text", "TEXT", "CHECKBOX", json.RawMessage(`"true"`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := PublishedTemplateContext{
				ID: uuid.New(),
				Fields: map[string]FieldDefinition{
					"field_a": {Code: "field_a", FieldType: tc.targetType},
				},
			}
			item := BuildMigrationPreviewItem(
				uuid.New(),
				uuid.New(),
				target,
				[]CustomFieldValue{{FieldCode: "field_a", ValueJSON: tc.value}},
				map[string]FieldDefinition{"field_a": {FieldType: tc.sourceType}},
			)
			if item.Status != MigrationPreviewStatusBlocked {
				t.Fatalf("expected BLOCKED, got %s: %+v", item.Status, item)
			}
			if len(item.IncompatibleFields) != 1 {
				t.Fatalf("expected incompatible field, got %+v", item.IncompatibleFields)
			}
		})
	}
}

func TestEvaluateTypeCompatibilitySelectToTextCompatible(t *testing.T) {
	result := evaluateTypeCompatibility(
		"SELECT",
		FieldDefinition{FieldType: "TEXT"},
		json.RawMessage(`"GENERAL"`),
	)
	if result.Outcome != typeCompatibilityCompatible {
		t.Fatalf("expected compatible SELECT to TEXT, got %d", result.Outcome)
	}
}

func TestEvaluateTypeCompatibilityTextToSelectInOptionsWarning(t *testing.T) {
	result := evaluateTypeCompatibility(
		"TEXT",
		FieldDefinition{
			FieldType:   "SELECT",
			OptionsJSON: json.RawMessage(`{"options":[{"value":"A","label":"A"}]}`),
		},
		json.RawMessage(`"A"`),
	)
	if result.Outcome != typeCompatibilityWarning {
		t.Fatalf("expected warning for TEXT to SELECT conversion, got %d", result.Outcome)
	}
}

func TestEvaluateTypeCompatibilityTextToSelectOutOfOptionsWarning(t *testing.T) {
	result := evaluateTypeCompatibility(
		"TEXT",
		FieldDefinition{
			FieldType:   "SELECT",
			OptionsJSON: json.RawMessage(`{"options":[{"value":"A","label":"A"}]}`),
		},
		json.RawMessage(`"B"`),
	)
	if result.Outcome != typeCompatibilityWarning {
		t.Fatalf("expected warning for out-of-options TEXT to SELECT, got %d", result.Outcome)
	}
	if result.Reason == "" {
		t.Fatal("expected reason for out-of-options warning")
	}
}

func TestBuildMigrationPreviewItemSkipsProtectedFieldsWithWarning(t *testing.T) {
	target := PublishedTemplateContext{
		ID: uuid.New(),
		Fields: map[string]FieldDefinition{
			"system_field": {Code: "system_field", FieldType: "TEXT", SystemField: true},
			"read_only":    {Code: "read_only", FieldType: "TEXT", ReadOnly: true},
			"editable":     {Code: "editable", FieldType: "TEXT"},
		},
	}

	item := BuildMigrationPreviewItem(
		uuid.New(),
		uuid.New(),
		target,
		[]CustomFieldValue{
			{FieldCode: "system_field", ValueJSON: json.RawMessage(`"x"`)},
			{FieldCode: "read_only", ValueJSON: json.RawMessage(`"y"`)},
			{FieldCode: "editable", ValueJSON: json.RawMessage(`"z"`)},
		},
		nil,
	)

	if len(item.CopiedFields) != 1 || item.CopiedFields[0] != "editable" {
		t.Fatalf("expected only editable copied, got %+v", item.CopiedFields)
	}
	if len(item.Warnings) < 2 {
		t.Fatalf("expected warnings for protected fields, got %+v", item.Warnings)
	}
}
