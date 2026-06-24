package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

func TestParseImportPreviewRequestDefaults(t *testing.T) {
	raw := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Cargo class","field_type":"SELECT"}]}]
		}
	}`)
	input, err := ParseImportPreviewRequest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if input.ConflictStrategy != ConflictStrategyNewVersion {
		t.Fatalf("conflict_strategy = %q", input.ConflictStrategy)
	}
	if input.Mode != ImportModeCreateDraft {
		t.Fatalf("mode = %q", input.Mode)
	}
}

func TestParseImportPreviewRequestUnsupportedSchema(t *testing.T) {
	_, err := ParseImportPreviewRequest([]byte(`{"schema_version":"v2","template":{"entity_type":"TRANSPORT_ORDER","code":"x","name":"X","sections":[{"code":"s","title":"S","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]}}`))
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeUnsupportedSchemaVersion {
		t.Fatalf("expected unsupported schema, got %v", err)
	}
}

func TestBuildTemplateImportPreviewNewVersionReady(t *testing.T) {
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
	existing := []FormTemplateSummary{{
		ID: uuid.New(), EntityType: "TRANSPORT_ORDER", Code: "transport_order_default",
		Status: PublishedStatus, Version: 1,
	}}

	result, err := BuildTemplateImportPreview(input, draftInput, existing, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Status != ImportPreviewStatusReady {
		t.Fatalf("status = %q", result.Status)
	}
	if result.ProposedDraftVersionOnPublish != 2 {
		t.Fatalf("proposed version = %d", result.ProposedDraftVersionOnPublish)
	}
	if len(result.ExistingPublishedVersions) != 1 || result.ExistingPublishedVersions[0] != 1 {
		t.Fatalf("published versions = %+v", result.ExistingPublishedVersions)
	}
}

func TestBuildTemplateImportPreviewFailIfExistsConflict(t *testing.T) {
	input := TemplateImportPreviewInput{
		SchemaVersion:    TemplateExportSchemaVersion,
		Mode:             ImportModeCreateDraft,
		ConflictStrategy: ConflictStrategyFailIfExists,
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
		t.Fatal("expected conflict")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeFormTemplateConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestBuildImportPreviewSummaryTypeChanges(t *testing.T) {
	draftInput := DraftFormTemplateInput{
		Sections: []DraftFormSectionInput{{
			Fields: []DraftFormFieldInput{{Code: "cargo_class", FieldType: "TEXT"}},
		}},
	}
	existing := &FormTemplateDetail{
		Sections: []FormSection{{
			Fields: []FormField{{Code: "cargo_class", FieldType: "SELECT", SystemField: false}},
		}},
	}
	summary := buildImportPreviewSummary(draftInput, existing)
	if len(summary.TypeChanges) != 1 || summary.TypeChanges[0].FieldCode != "cargo_class" {
		t.Fatalf("type changes = %+v", summary.TypeChanges)
	}
}

func TestBuildFormTemplateImportPreviewedAuditPayload(t *testing.T) {
	raw, err := BuildFormTemplateImportPreviewedAuditPayload(
		TemplateImportPreviewInput{SchemaVersion: TemplateExportSchemaVersion, SourceMetadata: TemplateImportSourceMetadata{SourceTemplateID: "b1111111-1111-4111-8111-111111111102"}},
		TemplateImportPreviewResult{Status: ImportPreviewStatusReady, TargetCode: "transport_order_default", TargetEntityType: "TRANSPORT_ORDER"},
	)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["event_kind"] != AuditEventKindFormTemplateImportPreviewed {
		t.Fatalf("event_kind = %v", payload["event_kind"])
	}
}

func TestResolveImportExecutionPlanCreateNewDraft(t *testing.T) {
	input := TemplateImportPreviewInput{
		Mode:             ImportModeCreateDraft,
		ConflictStrategy: ConflictStrategyNewVersion,
	}
	plan, err := ResolveImportExecutionPlan(input, []FormTemplateSummary{{Status: PublishedStatus, Version: 1}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.CreateNewDraft || plan.ReplaceDraftID != nil {
		t.Fatalf("expected create new draft: %+v", plan)
	}
}

func TestResolveImportExecutionPlanReplaceExistingDraft(t *testing.T) {
	draftID := uuid.New()
	input := TemplateImportPreviewInput{
		Mode:             ImportModeReplaceExistingDraft,
		ConflictStrategy: ConflictStrategyNewVersion,
	}
	plan, err := ResolveImportExecutionPlan(input, []FormTemplateSummary{{ID: draftID, Status: DraftStatus}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.ReplaceDraftID == nil || *plan.ReplaceDraftID != draftID {
		t.Fatalf("expected replace draft id")
	}
}

func TestBuildFormTemplateImportedAuditPayload(t *testing.T) {
	templateID := uuid.New()
	raw, err := BuildFormTemplateImportedAuditPayload(
		templateID,
		2,
		TemplateImportPreviewInput{SchemaVersion: TemplateExportSchemaVersion},
		TemplateImportPreviewResult{
			TargetCode:       "transport_order_default",
			TargetEntityType: "TRANSPORT_ORDER",
			ConflictStrategy: ConflictStrategyNewVersion,
			ImportMode:       ImportModeCreateDraft,
			Summary:          TemplateImportPreviewSummary{SectionsCount: 1, FieldsCount: 2},
		},
		false,
	)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["event_kind"] != AuditEventKindFormTemplateImportedAsDraft {
		t.Fatalf("event_kind = %v", payload["event_kind"])
	}
	if payload["dry_run"] != false {
		t.Fatal("expected dry_run false")
	}
}
