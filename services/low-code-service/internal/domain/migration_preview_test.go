package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildMigrationPreviewItemCopiesCompatibleFields(t *testing.T) {
	targetID := uuid.New()
	sourceID := uuid.New()
	target := PublishedTemplateContext{
		ID: targetID,
		Fields: map[string]FieldDefinition{
			"cargo_class": {
				Code:      "cargo_class",
				FieldType: "SELECT",
				OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
			},
			"internal_cost_center": {
				Code:      "internal_cost_center",
				FieldType: "TEXT",
			},
		},
	}
	sourceFields := map[string]FieldDefinition{
		"cargo_class": {FieldType: "SELECT"},
	}

	item := BuildMigrationPreviewItem(
		uuid.New(),
		sourceID,
		target,
		[]CustomFieldValue{
			{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`), FormTemplateID: sourceID},
			{FieldCode: "internal_cost_center", ValueJSON: json.RawMessage(`"CC-001"`), FormTemplateID: sourceID},
		},
		sourceFields,
	)

	if item.Status != MigrationPreviewStatusSafe {
		t.Fatalf("expected SAFE, got %s: %+v", item.Status, item)
	}
	if len(item.CopiedFields) != 2 {
		t.Fatalf("expected 2 copied fields, got %+v", item.CopiedFields)
	}
	if item.SourceTemplateID != sourceID {
		t.Fatalf("expected source template id, got %s", item.SourceTemplateID)
	}
}

func TestBuildMigrationPreviewItemDetectsLegacyFields(t *testing.T) {
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
			{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
		},
		nil,
	)

	if item.Status != MigrationPreviewStatusWarning {
		t.Fatalf("expected WARNING, got %s", item.Status)
	}
	if len(item.LegacyFields) != 1 || item.LegacyFields[0] != "deprecated_field" {
		t.Fatalf("unexpected legacy fields: %+v", item.LegacyFields)
	}
}

func TestBuildMigrationPreviewItemDetectsMissingRequired(t *testing.T) {
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
	if len(item.MissingRequiredFields) != 1 {
		t.Fatalf("expected missing required field, got %+v", item.MissingRequiredFields)
	}
}

func TestBuildMigrationPreviewItemDetectsIncompatibleTypes(t *testing.T) {
	target := PublishedTemplateContext{
		ID: uuid.New(),
		Fields: map[string]FieldDefinition{
			"amount": {Code: "amount", FieldType: "MONEY"},
		},
	}
	sourceFields := map[string]FieldDefinition{
		"amount": {FieldType: "NUMBER"},
	}

	item := BuildMigrationPreviewItem(
		uuid.New(),
		uuid.New(),
		target,
		[]CustomFieldValue{{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
		sourceFields,
	)

	if item.Status != MigrationPreviewStatusBlocked {
		t.Fatalf("expected BLOCKED, got %s", item.Status)
	}
	if len(item.IncompatibleFields) != 1 {
		t.Fatalf("expected incompatible field, got %+v", item.IncompatibleFields)
	}
}

func TestInferSourceTemplateIDUsesMostFrequent(t *testing.T) {
	templateA := uuid.New()
	templateB := uuid.New()
	result := InferSourceTemplateID([]CustomFieldValue{
		{FormTemplateID: templateA},
		{FormTemplateID: templateB},
		{FormTemplateID: templateA},
	})
	if result != templateA {
		t.Fatalf("expected %s, got %s", templateA, result)
	}
}

func TestEvaluateTypeCompatibilityTextToSelectWarning(t *testing.T) {
	targetField := FieldDefinition{
		FieldType:   "SELECT",
		OptionsJSON: json.RawMessage(`{"options":[{"value":"A","label":"A"}]}`),
	}
	result := evaluateTypeCompatibility("TEXT", targetField, json.RawMessage(`"B"`))
	if result.Outcome != typeCompatibilityWarning {
		t.Fatalf("expected warning, got %d", result.Outcome)
	}
}
