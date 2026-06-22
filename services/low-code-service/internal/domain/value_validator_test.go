package domain

import (
	"encoding/json"
	"testing"
)

func TestValidateFieldValueText(t *testing.T) {
	field := FieldDefinition{Code: "note", FieldType: "TEXT"}
	if err := ValidateFieldValue(field, json.RawMessage(`"hello"`)); err != nil {
		t.Fatalf("expected valid text, got %v", err)
	}
}

func TestValidateFieldValueTextInvalid(t *testing.T) {
	field := FieldDefinition{Code: "note", FieldType: "TEXT"}
	if err := ValidateFieldValue(field, json.RawMessage(`123`)); err == nil {
		t.Fatal("expected invalid text")
	}
}

func TestValidateFieldValueSelectOption(t *testing.T) {
	field := FieldDefinition{
		Code:      "cargo_class",
		FieldType: "SELECT",
		OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
	}
	if err := ValidateFieldValue(field, json.RawMessage(`"GENERAL"`)); err != nil {
		t.Fatalf("expected valid select, got %v", err)
	}
	if err := ValidateFieldValue(field, json.RawMessage(`"INVALID"`)); err == nil {
		t.Fatal("expected invalid select option")
	}
}

func TestValidateFieldValueSystemFieldProtected(t *testing.T) {
	field := FieldDefinition{Code: "sys", FieldType: "TEXT", SystemField: true}
	if err := ValidateFieldValue(field, json.RawMessage(`"x"`)); err == nil {
		t.Fatal("expected system field protected error")
	}
}

func TestValidateFieldValueMoney(t *testing.T) {
	field := FieldDefinition{Code: "price", FieldType: "MONEY"}
	if err := ValidateFieldValue(field, json.RawMessage(`{"amount":1000,"currency":"RUB"}`)); err != nil {
		t.Fatalf("expected valid money, got %v", err)
	}
}

func TestValidateFieldValueRequiredNull(t *testing.T) {
	field := FieldDefinition{Code: "note", FieldType: "TEXT", Required: true}
	if err := ValidateFieldValue(field, json.RawMessage(`null`)); err == nil {
		t.Fatal("expected required validation failure")
	}
}
