package domain

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

const (
	MaxImportPayloadBytes = 512 * 1024

	ImportModeCreateDraft            = "CREATE_DRAFT"
	ImportModeCreateNewCode          = "CREATE_NEW_CODE"
	ImportModeReplaceExistingDraft   = "REPLACE_EXISTING_DRAFT"
	ImportModeNewVersionFromExport   = "NEW_VERSION_FROM_EXPORT"

	ConflictStrategyFailIfExists           = "FAIL_IF_EXISTS"
	ConflictStrategyNewVersion             = "NEW_VERSION"
	ConflictStrategyNewCode                = "NEW_CODE"
	ConflictStrategyReplaceExistingDraft   = "REPLACE_EXISTING_DRAFT"

	ImportPreviewStatusReady    = "READY"
	ImportPreviewStatusWarning  = "WARNING"
	ImportPreviewStatusBlocked  = "BLOCKED"
)

type TemplateImportSourceMetadata struct {
	SourceTenantID   string `json:"source_tenant_id,omitempty"`
	SourceTemplateID string `json:"source_template_id,omitempty"`
	SourceVersion    int    `json:"source_version,omitempty"`
	SourceStatus     string `json:"source_status,omitempty"`
	ExportedAt       string `json:"exported_at,omitempty"`
}

type TemplateImportPreviewInput struct {
	SchemaVersion     string
	Mode              string
	ConflictStrategy  string
	TargetCode        string
	AllowSystemFields bool
	Template          ExportedFormTemplate
	SourceMetadata    TemplateImportSourceMetadata
}

type TemplateImportFieldTypeChange struct {
	FieldCode string `json:"field_code"`
	FromType  string `json:"from_type"`
	ToType    string `json:"to_type"`
}

type TemplateImportPreviewSummary struct {
	SectionsCount     int                             `json:"sections_count"`
	FieldsCount       int                             `json:"fields_count"`
	NewFieldCodes     []string                        `json:"new_field_codes"`
	RemovedFieldCodes []string                        `json:"removed_field_codes"`
	TypeChanges       []TemplateImportFieldTypeChange `json:"type_changes"`
}

type TemplateImportPreviewResult struct {
	Status                         string                         `json:"status"`
	ConflictStrategy               string                         `json:"conflict_strategy"`
	ImportMode                     string                         `json:"import_mode"`
	TargetEntityType               string                         `json:"target_entity_type"`
	TargetCode                     string                         `json:"target_code"`
	ExistingDraftID                *string                        `json:"existing_draft_id,omitempty"`
	ExistingPublishedVersions      []int                          `json:"existing_published_versions"`
	ProposedDraftVersionOnPublish  int                            `json:"proposed_draft_version_on_publish"`
	Warnings                       []string                       `json:"warnings"`
	ValidationErrors               []string                       `json:"validation_errors"`
	Summary                        TemplateImportPreviewSummary   `json:"summary"`
	SchemaVersion                  string                         `json:"schema_version"`
}

type importPreviewHTTPBody struct {
	SchemaVersion     string                          `json:"schema_version"`
	Mode              string                          `json:"mode"`
	ConflictStrategy  string                          `json:"conflict_strategy"`
	TargetCode        *string                         `json:"target_code"`
	AllowSystemFields bool                            `json:"allow_system_fields"`
	Template          ExportedFormTemplate            `json:"template"`
	SourceMetadata    *TemplateImportSourceMetadata   `json:"source_metadata"`
	Source            *TemplateExportSource           `json:"source"`
}

func ParseImportPreviewRequest(raw []byte) (TemplateImportPreviewInput, error) {
	if len(raw) > MaxImportPayloadBytes {
		return TemplateImportPreviewInput{}, apperrors.ImportPayloadTooLarge()
	}

	var body importPreviewHTTPBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return TemplateImportPreviewInput{}, apperrors.Validation("invalid json body", map[string]any{"error": err.Error()})
	}

	schemaVersion := strings.TrimSpace(body.SchemaVersion)
	if schemaVersion == "" {
		return TemplateImportPreviewInput{}, apperrors.Validation("schema_version is required", map[string]any{"field": "schema_version"})
	}
	if schemaVersion != TemplateExportSchemaVersion {
		return TemplateImportPreviewInput{}, apperrors.UnsupportedSchemaVersion(schemaVersion)
	}

	mode := normalizeImportMode(body.Mode)
	if err := validateImportMode(mode); err != nil {
		return TemplateImportPreviewInput{}, err
	}

	conflictStrategy := normalizeConflictStrategy(body.ConflictStrategy)
	if err := validateConflictStrategy(conflictStrategy); err != nil {
		return TemplateImportPreviewInput{}, err
	}

	if mode == ImportModeNewVersionFromExport && conflictStrategy == "" {
		conflictStrategy = ConflictStrategyNewVersion
	}
	if conflictStrategy == "" {
		conflictStrategy = ConflictStrategyNewVersion
	}

	targetCode := ""
	if body.TargetCode != nil {
		targetCode = strings.TrimSpace(*body.TargetCode)
	}

	sourceMetadata := TemplateImportSourceMetadata{}
	if body.SourceMetadata != nil {
		sourceMetadata = *body.SourceMetadata
	} else if body.Source != nil {
		sourceMetadata = TemplateImportSourceMetadata{
			SourceTenantID:   body.Source.TenantID,
			SourceTemplateID: body.Source.TemplateID,
			SourceVersion:    body.Source.TemplateVersion,
			SourceStatus:     body.Source.TemplateStatus,
		}
	}

	return TemplateImportPreviewInput{
		SchemaVersion:     schemaVersion,
		Mode:              mode,
		ConflictStrategy:  conflictStrategy,
		TargetCode:        targetCode,
		AllowSystemFields: body.AllowSystemFields,
		Template:          body.Template,
		SourceMetadata:    sourceMetadata,
	}, nil
}

func ExportedTemplateToDraftInput(exported ExportedFormTemplate, targetCode string) DraftFormTemplateInput {
	code := strings.TrimSpace(targetCode)
	if code == "" {
		code = strings.TrimSpace(exported.Code)
	}

	sections := make([]DraftFormSectionInput, 0, len(exported.Sections))
	for _, section := range exported.Sections {
		fields := make([]DraftFormFieldInput, 0, len(section.Fields))
		for _, field := range section.Fields {
			fields = append(fields, DraftFormFieldInput{
				Code:               field.Code,
				Label:              field.Label,
				FieldType:          field.FieldType,
				Required:           field.Required,
				ReadOnly:           field.ReadOnly,
				SystemField:        field.SystemField,
				OptionsJSON:        field.OptionsJSON,
				ValidationRuleJSON: field.ValidationRuleJSON,
				VisibilityRuleJSON: field.VisibilityRuleJSON,
				SortOrder:          field.SortOrder,
			})
		}
		sections = append(sections, DraftFormSectionInput{
			Code:      section.Code,
			Title:     section.Title,
			SortOrder: section.SortOrder,
			Fields:    fields,
		})
	}

	return DraftFormTemplateInput{
		EntityType:  strings.TrimSpace(exported.EntityType),
		Code:        code,
		Name:        strings.TrimSpace(exported.Name),
		Description: strings.TrimSpace(exported.Description),
		Sections:    sections,
	}
}

func ValidateImportSystemFields(input DraftFormTemplateInput, allowSystemFields bool) error {
	if allowSystemFields {
		return nil
	}
	for _, section := range input.Sections {
		for _, field := range section.Fields {
			if field.SystemField {
				return apperrors.Validation(
					"system_field import requires allow_system_fields=true",
					map[string]any{"field_code": field.Code},
				)
			}
		}
	}
	return nil
}

func BuildTemplateImportPreview(
	input TemplateImportPreviewInput,
	draftInput DraftFormTemplateInput,
	existing []FormTemplateSummary,
	comparisonTemplate *FormTemplateDetail,
) (TemplateImportPreviewResult, error) {
	summary := buildImportPreviewSummary(draftInput, comparisonTemplate)
	warnings := make([]string, 0)
	validationErrors := make([]string, 0)

	var existingDraft *FormTemplateSummary
	publishedVersions := make([]int, 0)
	maxVersion := 0
	for i := range existing {
		item := existing[i]
		if item.Version > maxVersion {
			maxVersion = item.Version
		}
		switch item.Status {
		case DraftStatus:
			copyItem := item
			existingDraft = &copyItem
		case PublishedStatus:
			publishedVersions = append(publishedVersions, item.Version)
		}
	}

	result := TemplateImportPreviewResult{
		Status:                        ImportPreviewStatusReady,
		ConflictStrategy:              input.ConflictStrategy,
		ImportMode:                    input.Mode,
		TargetEntityType:              draftInput.EntityType,
		TargetCode:                    draftInput.Code,
		ExistingPublishedVersions:     publishedVersions,
		ProposedDraftVersionOnPublish: maxVersion + 1,
		Warnings:                      warnings,
		ValidationErrors:              validationErrors,
		Summary:                       summary,
		SchemaVersion:                 input.SchemaVersion,
	}
	if existingDraft != nil {
		id := existingDraft.ID.String()
		result.ExistingDraftID = &id
	}

	switch input.Mode {
	case ImportModeCreateNewCode:
		if strings.TrimSpace(input.TargetCode) == "" {
			return TemplateImportPreviewResult{}, apperrors.Validation("target_code is required for CREATE_NEW_CODE mode", map[string]any{"field": "target_code"})
		}
		if len(existing) > 0 {
			return TemplateImportPreviewResult{}, apperrors.FormTemplateConflict(map[string]any{
				"target_code": draftInput.Code,
				"reason":      "target_code already exists",
			})
		}
	case ImportModeReplaceExistingDraft:
		if existingDraft == nil {
			return TemplateImportPreviewResult{}, apperrors.FormTemplateNotDraft("missing")
		}
	default:
		switch input.ConflictStrategy {
		case ConflictStrategyFailIfExists:
			if len(existing) > 0 {
				return TemplateImportPreviewResult{}, apperrors.FormTemplateConflict(map[string]any{
					"target_code": draftInput.Code,
					"reason":      "template with target code already exists",
				})
			}
		case ConflictStrategyNewCode:
			if strings.TrimSpace(input.TargetCode) == "" {
				return TemplateImportPreviewResult{}, apperrors.Validation("target_code is required for NEW_CODE strategy", map[string]any{"field": "target_code"})
			}
			if len(existing) > 0 {
				return TemplateImportPreviewResult{}, apperrors.FormTemplateConflict(map[string]any{
					"target_code": draftInput.Code,
					"reason":      "target_code already exists",
				})
			}
		case ConflictStrategyReplaceExistingDraft:
			if existingDraft == nil {
				return TemplateImportPreviewResult{}, apperrors.FormTemplateNotDraft("missing")
			}
		case ConflictStrategyNewVersion:
			if existingDraft != nil {
				result.Status = ImportPreviewStatusWarning
				result.Warnings = append(result.Warnings, "existing draft will be replaced on import execute when using REPLACE_EXISTING_DRAFT; NEW_VERSION creates an additional draft row")
			}
		}
	}

	if len(summary.RemovedFieldCodes) > 0 {
		result.Status = ImportPreviewStatusWarning
		result.Warnings = append(result.Warnings, "import removes field codes present in active published template")
	}
	if len(summary.TypeChanges) > 0 {
		result.Status = ImportPreviewStatusWarning
		result.Warnings = append(result.Warnings, "import changes field types compared to active published template")
	}

	return result, nil
}

func buildImportPreviewSummary(draftInput DraftFormTemplateInput, comparisonTemplate *FormTemplateDetail) TemplateImportPreviewSummary {
	sectionsCount := len(draftInput.Sections)
	fieldsCount := 0
	importFields := make(map[string]string)
	for _, section := range draftInput.Sections {
		fieldsCount += len(section.Fields)
		for _, field := range section.Fields {
			importFields[strings.TrimSpace(field.Code)] = strings.TrimSpace(field.FieldType)
		}
	}

	summary := TemplateImportPreviewSummary{
		SectionsCount:     sectionsCount,
		FieldsCount:       fieldsCount,
		NewFieldCodes:     []string{},
		RemovedFieldCodes: []string{},
		TypeChanges:       []TemplateImportFieldTypeChange{},
	}

	if comparisonTemplate == nil {
		for code := range importFields {
			summary.NewFieldCodes = append(summary.NewFieldCodes, code)
		}
		return summary
	}

	existingFields := make(map[string]string)
	for _, section := range comparisonTemplate.Sections {
		for _, field := range section.Fields {
			if field.SystemField {
				continue
			}
			code := strings.TrimSpace(field.Code)
			existingFields[code] = strings.TrimSpace(field.FieldType)
		}
	}

	for code, fieldType := range importFields {
		existingType, ok := existingFields[code]
		if !ok {
			summary.NewFieldCodes = append(summary.NewFieldCodes, code)
			continue
		}
		if existingType != fieldType {
			summary.TypeChanges = append(summary.TypeChanges, TemplateImportFieldTypeChange{
				FieldCode: code,
				FromType:  existingType,
				ToType:    fieldType,
			})
		}
	}

	for code := range existingFields {
		if _, ok := importFields[code]; !ok {
			summary.RemovedFieldCodes = append(summary.RemovedFieldCodes, code)
		}
	}

	return summary
}

func SelectComparisonPublishedTemplate(existing []FormTemplateSummary) *FormTemplateSummary {
	var best *FormTemplateSummary
	for i := range existing {
		item := existing[i]
		if item.Status != PublishedStatus {
			continue
		}
		if best == nil || item.Version > best.Version {
			copyItem := item
			best = &copyItem
		}
	}
	return best
}

func BuildFormTemplateImportPreviewedAuditPayload(
	input TemplateImportPreviewInput,
	result TemplateImportPreviewResult,
) (json.RawMessage, error) {
	payload := map[string]any{
		"event_kind":          AuditEventKindFormTemplateImportPreviewed,
		"schema_version":      input.SchemaVersion,
		"conflict_strategy":   result.ConflictStrategy,
		"import_mode":         result.ImportMode,
		"target_code":         result.TargetCode,
		"target_entity_type":  result.TargetEntityType,
		"preview_status":      result.Status,
		"dry_run":             true,
	}
	if input.SourceMetadata.SourceTemplateID != "" {
		payload["source_template_id"] = input.SourceMetadata.SourceTemplateID
	}
	if input.SourceMetadata.SourceTenantID != "" {
		payload["source_tenant_id"] = input.SourceMetadata.SourceTenantID
	}
	if input.SourceMetadata.SourceVersion > 0 {
		payload["source_version"] = input.SourceMetadata.SourceVersion
	}
	return json.Marshal(payload)
}

func ResolveImportPreviewAuditEntityID(input TemplateImportPreviewInput, existing []FormTemplateSummary) *uuid.UUID {
	if draft := findDraftTemplate(existing); draft != nil {
		id := draft.ID
		return &id
	}
	if raw := strings.TrimSpace(input.SourceMetadata.SourceTemplateID); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			return &id
		}
	}
	if published := SelectComparisonPublishedTemplate(existing); published != nil {
		id := published.ID
		return &id
	}
	return nil
}

func findDraftTemplate(existing []FormTemplateSummary) *FormTemplateSummary {
	for i := range existing {
		if existing[i].Status == DraftStatus {
			copyItem := existing[i]
			return &copyItem
		}
	}
	return nil
}

func normalizeImportMode(mode string) string {
	mode = strings.TrimSpace(strings.ToUpper(mode))
	if mode == "" {
		return ImportModeCreateDraft
	}
	return mode
}

func normalizeConflictStrategy(strategy string) string {
	return strings.TrimSpace(strings.ToUpper(strategy))
}

func validateImportMode(mode string) error {
	switch mode {
	case ImportModeCreateDraft, ImportModeCreateNewCode, ImportModeReplaceExistingDraft, ImportModeNewVersionFromExport:
		return nil
	default:
		return apperrors.Validation("invalid import mode", map[string]any{"field": "mode", "value": mode})
	}
}

func validateConflictStrategy(strategy string) error {
	if strategy == "" {
		return nil
	}
	switch strategy {
	case ConflictStrategyFailIfExists, ConflictStrategyNewVersion, ConflictStrategyNewCode, ConflictStrategyReplaceExistingDraft:
		return nil
	default:
		return apperrors.Validation("invalid conflict_strategy", map[string]any{"field": "conflict_strategy", "value": strategy})
	}
}
