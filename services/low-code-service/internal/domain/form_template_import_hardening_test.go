package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateImportTopLevelKeysRejectsUnknown(t *testing.T) {
	raw := map[string][]byte{
		"schema_version": []byte(`"lowcode.template.export.v1"`),
		"template":       []byte(`{}`),
		"unexpected_key": []byte(`true`),
	}
	if err := validateImportTopLevelKeys(mustRawMap(t, raw)); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestValidateImportTopLevelKeysRejectsForbidden(t *testing.T) {
	raw := map[string][]byte{
		"schema_version": []byte(`"lowcode.template.export.v1"`),
		"template":       []byte(`{}`),
		"custom_values":  []byte(`[]`),
	}
	if err := validateImportTopLevelKeys(mustRawMap(t, raw)); err == nil {
		t.Fatal("expected forbidden key error")
	}
}

func TestValidateImportTopLevelKeysAllowsExportEnvelopeFields(t *testing.T) {
	raw := map[string][]byte{
		"schema_version": []byte(`"lowcode.template.export.v1"`),
		"exported_at":    []byte(`"2026-06-24T12:00:00Z"`),
		"metadata":       []byte(`{"checksum":"abc"}`),
		"template":       []byte(`{}`),
	}
	if err := validateImportTopLevelKeys(mustRawMap(t, raw)); err != nil {
		t.Fatalf("expected allowed keys, got %v", err)
	}
}

func TestChecksumImportWarningMissingAllowed(t *testing.T) {
	template := ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{{Code: "f", Label: "F", FieldType: "TEXT"}},
		}},
	}
	warning, err := ChecksumImportWarning(template, "")
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
}

func TestChecksumImportWarningMismatch(t *testing.T) {
	template := ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{{Code: "f", Label: "F", FieldType: "TEXT"}},
		}},
	}
	warning, err := ChecksumImportWarning(template, "deadbeef")
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	if !strings.Contains(warning, "checksum mismatch") {
		t.Fatalf("expected mismatch warning, got %q", warning)
	}
}

func TestBuildTemplateImportPreviewChecksumMismatchWarning(t *testing.T) {
	input := TemplateImportPreviewInput{
		SchemaVersion:    TemplateExportSchemaVersion,
		Mode:             ImportModeCreateDraft,
		ConflictStrategy: ConflictStrategyNewVersion,
		ExportChecksum:   "not-a-valid-checksum",
		Template: ExportedFormTemplate{
			EntityType: "TRANSPORT_ORDER",
			Code:       "transport_order_default",
			Name:       "Default",
			Sections: []ExportedFormSection{{
				Code: "cargo", Title: "Cargo",
				Fields: []ExportedFormField{{Code: "cargo_class", Label: "Class", FieldType: "SELECT"}},
			}},
		},
	}
	draftInput := ExportedTemplateToDraftInput(input.Template, "")
	result, err := BuildTemplateImportPreview(input, draftInput, nil, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Status != ImportPreviewStatusWarning {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "checksum mismatch") {
		t.Fatalf("warnings = %+v", result.Warnings)
	}
}

func TestParseImportRequestRejectsUnknownTopLevelKey(t *testing.T) {
	_, err := ParseImportRequest([]byte(`{
		"schema_version":"lowcode.template.export.v1",
		"audit_events":[],
		"template":{"entity_type":"TRANSPORT_ORDER","code":"x","name":"X","sections":[{"code":"s","title":"S","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]}
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func mustRawMap(t *testing.T, raw map[string][]byte) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	return out
}
