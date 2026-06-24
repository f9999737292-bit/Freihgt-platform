package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

func TestParseImportRequestInvalidJSON(t *testing.T) {
	_, err := ParseImportRequest([]byte(`{`))
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestExportedTemplateMissingEntityTypeFailsValidation(t *testing.T) {
	draftInput := ExportedTemplateToDraftInput(ExportedFormTemplate{
		Code: "transport_order_default",
		Name: "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{{Code: "f", Label: "F", FieldType: "TEXT"}},
		}},
	}, "")
	if err := ValidateDraftFormTemplateInput(draftInput); err == nil {
		t.Fatal("expected validation error for missing entity_type")
	}
}

func TestExportedTemplateMissingCodeFailsValidation(t *testing.T) {
	draftInput := ExportedTemplateToDraftInput(ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Name:       "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{{Code: "f", Label: "F", FieldType: "TEXT"}},
		}},
	}, "")
	if err := ValidateDraftFormTemplateInput(draftInput); err == nil {
		t.Fatal("expected validation error for missing code")
	}
}

func TestParseImportRequestMissingSchemaVersionRejected(t *testing.T) {
	_, err := ParseImportRequest([]byte(`{"template":{"entity_type":"TRANSPORT_ORDER","code":"x","name":"X","sections":[]}}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseImportRequestEmptyTemplateAllowedAtParseStage(t *testing.T) {
	input, err := ParseImportRequest([]byte(`{"schema_version":"lowcode.template.export.v1"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := ValidateDraftFormTemplateInput(ExportedTemplateToDraftInput(input.Template, input.TargetCode)); err == nil {
		t.Fatal("expected validation error after parse for empty template")
	}
}

func TestParseImportRequestPayloadTooLarge(t *testing.T) {
	raw := make([]byte, MaxImportPayloadBytes+1)
	for i := range raw {
		raw[i] = 'a'
	}
	_, err := ParseImportRequest(raw)
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeImportPayloadTooLarge {
		t.Fatalf("expected payload too large, got %v", err)
	}
}

func TestParseImportRequestMapsExportSourceMetadata(t *testing.T) {
	raw := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"source":{
			"tenant_id":"00000000-0000-4000-8000-000000000099",
			"template_id":"b1111111-1111-4111-8111-111111111102",
			"template_version":3,
			"template_status":"PUBLISHED"
		},
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]
		}
	}`)
	input, err := ParseImportRequest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if input.SourceMetadata.SourceTenantID != "00000000-0000-4000-8000-000000000099" {
		t.Fatalf("source tenant = %q", input.SourceMetadata.SourceTenantID)
	}
	if input.SourceMetadata.SourceTemplateID != "b1111111-1111-4111-8111-111111111102" {
		t.Fatalf("source template id = %q", input.SourceMetadata.SourceTemplateID)
	}
}

func TestValidateDraftDuplicateSectionCodeOnImportPath(t *testing.T) {
	draftInput := ExportedTemplateToDraftInput(ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Sections: []ExportedFormSection{
			{Code: "dup", Title: "A", Fields: []ExportedFormField{{Code: "f1", Label: "F1", FieldType: "TEXT"}}},
			{Code: "dup", Title: "B", Fields: []ExportedFormField{{Code: "f2", Label: "F2", FieldType: "TEXT"}}},
		},
	}, "")
	if err := ValidateDraftFormTemplateInput(draftInput); err == nil {
		t.Fatal("expected duplicate section error")
	}
}

func TestValidateDraftDuplicateFieldCodeOnImportPath(t *testing.T) {
	draftInput := ExportedTemplateToDraftInput(ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{
				{Code: "dup", Label: "A", FieldType: "TEXT"},
				{Code: "dup", Label: "B", FieldType: "TEXT"},
			},
		}},
	}, "")
	if err := ValidateDraftFormTemplateInput(draftInput); err == nil {
		t.Fatal("expected duplicate field error")
	}
}

func TestBuildTemplateImportPreviewReplaceDraftWithoutDraft(t *testing.T) {
	input := TemplateImportPreviewInput{
		SchemaVersion:    TemplateExportSchemaVersion,
		Mode:             ImportModeCreateDraft,
		ConflictStrategy: ConflictStrategyReplaceExistingDraft,
	}
	draftInput := ExportedTemplateToDraftInput(ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{{Code: "cargo_class", Label: "Class", FieldType: "SELECT"}},
		}},
	}, "")
	existing := []FormTemplateSummary{{Status: PublishedStatus, Version: 1, Code: "transport_order_default"}}

	_, err := BuildTemplateImportPreview(input, draftInput, existing, nil)
	if err == nil {
		t.Fatal("expected error when no draft exists for REPLACE_EXISTING_DRAFT")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeFormTemplateNotDraft {
		t.Fatalf("expected FORM_TEMPLATE_NOT_DRAFT, got %v", err)
	}
}

func TestBuildTemplateImportPreviewNewVersionWithExistingDraftWarning(t *testing.T) {
	draftID := uuid.New()
	input := TemplateImportPreviewInput{
		SchemaVersion:    TemplateExportSchemaVersion,
		Mode:             ImportModeCreateDraft,
		ConflictStrategy: ConflictStrategyNewVersion,
	}
	draftInput := ExportedTemplateToDraftInput(ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []ExportedFormField{{Code: "cargo_class", Label: "Class", FieldType: "SELECT"}},
		}},
	}, "")
	existing := []FormTemplateSummary{
		{ID: draftID, Status: DraftStatus, Version: 2, Code: "transport_order_default"},
		{Status: PublishedStatus, Version: 1, Code: "transport_order_default"},
	}

	result, err := BuildTemplateImportPreview(input, draftInput, existing, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Status != ImportPreviewStatusWarning {
		t.Fatalf("status = %q", result.Status)
	}
	if result.ProposedDraftVersionOnPublish != 3 {
		t.Fatalf("proposed version = %d", result.ProposedDraftVersionOnPublish)
	}
}

func TestComputeTemplateExportChecksumStable(t *testing.T) {
	template := ExportedFormTemplate{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Status:     PublishedStatus,
		Version:    1,
		Sections: []ExportedFormSection{{
			Code: "cargo", Title: "Cargo", SortOrder: 100,
			Fields: []ExportedFormField{{
				Code: "cargo_class", Label: "Class", FieldType: "SELECT", SortOrder: 100,
				OptionsJSON: json.RawMessage(`{"options":["GENERAL"]}`),
			}},
		}},
	}
	first, err := ComputeTemplateExportChecksum(template)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	second, err := ComputeTemplateExportChecksum(template)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	if first != second || first == "" {
		t.Fatalf("expected stable non-empty checksum, first=%q second=%q", first, second)
	}
}

func TestBuildTemplateExportEnvelopeExcludesCustomValuesAndAudit(t *testing.T) {
	detail := FormTemplateDetail{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Status:     PublishedStatus,
		Version:    1,
		Sections: []FormSection{{
			Code: "cargo", Title: "Cargo",
			Fields: []FormField{{Code: "cargo_class", Label: "Class", FieldType: "SELECT"}},
		}},
	}
	envelope, err := BuildTemplateExportEnvelope(detail, AuditContext{RequestID: "req-edge"}, time.Now().UTC(), "local", "low-code-service")
	if err != nil {
		t.Fatalf("envelope: %v", err)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, `"values"`) || strings.Contains(body, "audit_events") {
		t.Fatalf("export envelope must not include custom values or audit: %s", body)
	}
}

func TestBuildFormTemplateExportedAuditPayload(t *testing.T) {
	templateID := uuid.New()
	raw, err := BuildFormTemplateExportedAuditPayload(templateID, "transport_order_default", 1, PublishedStatus, TemplateExportSchemaVersion)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["event_kind"] != AuditEventKindFormTemplateExported {
		t.Fatalf("event_kind = %v", payload["event_kind"])
	}
}
