package domain

import (
	"encoding/json"
	"regexp"
	"strings"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

const (
	DraftStatus    = "DRAFT"
	ArchivedStatus = "ARCHIVED"

	MaxDraftSections = 50
	MaxDraftFields   = 200
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var allowedFieldTypes = map[string]struct{}{
	"TEXT":               {},
	"NUMBER":             {},
	"DATE":               {},
	"DATETIME":           {},
	"SELECT":             {},
	"MULTI_SELECT":       {},
	"CHECKBOX":           {},
	"MONEY":              {},
	"CURRENCY":           {},
	"FILE":               {},
	"COMPANY_REFERENCE":  {},
	"DOCUMENT_REFERENCE": {},
	"ROUTE":              {},
	"ADDRESS":            {},
	"VEHICLE":            {},
	"VAT_TAX":            {},
}

var sqlFragmentPattern = regexp.MustCompile(`(?i)\b(select|insert|update|delete|drop|alter|truncate|exec|union)\b`)

type DraftFormFieldInput struct {
	Code               string
	Label              string
	FieldType          string
	Required           bool
	ReadOnly           bool
	SystemField        bool
	OptionsJSON        json.RawMessage
	ValidationRuleJSON json.RawMessage
	VisibilityRuleJSON json.RawMessage
	SortOrder          int
}

type DraftFormSectionInput struct {
	Code      string
	Title     string
	SortOrder int
	Fields    []DraftFormFieldInput
}

type DraftFormTemplateInput struct {
	EntityType  string
	Code        string
	Name        string
	Description string
	Sections    []DraftFormSectionInput
}

func ValidateDraftFormTemplateInput(input DraftFormTemplateInput) error {
	entityType := strings.TrimSpace(input.EntityType)
	if entityType == "" {
		return apperrors.Validation("entity_type is required", map[string]any{"field": "entity_type"})
	}
	if err := ValidateEntityType(entityType); err != nil {
		return err
	}

	code := strings.TrimSpace(input.Code)
	if code == "" {
		return apperrors.Validation("code is required", map[string]any{"field": "code"})
	}
	if !codePattern.MatchString(code) {
		return apperrors.Validation("code must be lowercase snake_case", map[string]any{"field": "code", "value": code})
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return apperrors.Validation("name is required", map[string]any{"field": "name"})
	}

	if len(input.Sections) == 0 {
		return apperrors.Validation("at least one section is required", map[string]any{"field": "sections"})
	}
	if len(input.Sections) > MaxDraftSections {
		return apperrors.Validation("too many sections", map[string]any{"max": MaxDraftSections})
	}

	sectionCodes := make(map[string]struct{}, len(input.Sections))
	fieldCodes := make(map[string]struct{})
	totalFields := 0

	for i, section := range input.Sections {
		if err := validateDraftSection(section, i, sectionCodes, fieldCodes, &totalFields); err != nil {
			return err
		}
	}
	if totalFields > MaxDraftFields {
		return apperrors.Validation("too many fields", map[string]any{"max": MaxDraftFields})
	}

	return nil
}

func validateDraftSection(
	section DraftFormSectionInput,
	index int,
	sectionCodes map[string]struct{},
	fieldCodes map[string]struct{},
	totalFields *int,
) error {
	code := strings.TrimSpace(section.Code)
	if code == "" {
		return apperrors.Validation("section code is required", map[string]any{"section_index": index, "field": "code"})
	}
	if !codePattern.MatchString(code) {
		return apperrors.Validation("section code must be lowercase snake_case", map[string]any{
			"section_index": index,
			"field":         "code",
			"value":         code,
		})
	}
	if _, exists := sectionCodes[code]; exists {
		return apperrors.Validation("duplicate section code", map[string]any{"section_code": code})
	}
	sectionCodes[code] = struct{}{}

	title := strings.TrimSpace(section.Title)
	if title == "" {
		return apperrors.Validation("section title is required", map[string]any{"section_index": index, "field": "title"})
	}

	for j, field := range section.Fields {
		if err := validateDraftField(field, index, j, fieldCodes); err != nil {
			return err
		}
		*totalFields++
	}
	return nil
}

func validateDraftField(
	field DraftFormFieldInput,
	sectionIndex int,
	fieldIndex int,
	fieldCodes map[string]struct{},
) error {
	code := strings.TrimSpace(field.Code)
	if code == "" {
		return apperrors.Validation("field code is required", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         "code",
		})
	}
	if !codePattern.MatchString(code) {
		return apperrors.Validation("field code must be lowercase snake_case", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         "code",
			"value":         code,
		})
	}
	if _, exists := fieldCodes[code]; exists {
		return apperrors.Validation("duplicate field code", map[string]any{"field_code": code})
	}
	fieldCodes[code] = struct{}{}

	label := strings.TrimSpace(field.Label)
	if label == "" {
		return apperrors.Validation("field label is required", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         "label",
		})
	}

	fieldType := strings.TrimSpace(field.FieldType)
	if fieldType == "" {
		return apperrors.Validation("field_type is required", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         "field_type",
		})
	}
	if _, ok := allowedFieldTypes[fieldType]; !ok {
		return apperrors.Validation("invalid field_type", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field_type":    fieldType,
		})
	}

	if err := validateRuleJSON(field.ValidationRuleJSON, "validation_rule_json", sectionIndex, fieldIndex); err != nil {
		return err
	}
	if err := validateRuleJSON(field.VisibilityRuleJSON, "visibility_rule_json", sectionIndex, fieldIndex); err != nil {
		return err
	}
	if err := validateOptionsJSON(fieldType, field.OptionsJSON, sectionIndex, fieldIndex); err != nil {
		return err
	}

	return nil
}

func validateRuleJSON(raw json.RawMessage, fieldName string, sectionIndex, fieldIndex int) error {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return apperrors.Validation("invalid json", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         fieldName,
		})
	}
	if containsSQLFragment(string(raw)) {
		return apperrors.Validation("rule json must not contain SQL fragments", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         fieldName,
		})
	}
	return nil
}

func validateOptionsJSON(fieldType string, raw json.RawMessage, sectionIndex, fieldIndex int) error {
	if fieldType != "SELECT" && fieldType != "MULTI_SELECT" {
		return nil
	}
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return apperrors.Validation("invalid options_json", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
			"field":         "options_json",
		})
	}
	var payload struct {
		Options json.RawMessage `json:"options"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return apperrors.Validation("options_json must be an object with options array", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
		})
	}
	if len(payload.Options) == 0 {
		return apperrors.Validation("options_json.options is required for SELECT fields", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
		})
	}
	if !json.Valid(payload.Options) {
		return apperrors.Validation("options_json.options must be valid json array", map[string]any{
			"section_index": sectionIndex,
			"field_index":   fieldIndex,
		})
	}
	return nil
}

func containsSQLFragment(value string) bool {
	return sqlFragmentPattern.MatchString(value)
}

func AllowedFieldTypes() []string {
	return []string{
		"TEXT", "NUMBER", "DATE", "DATETIME",
		"SELECT", "MULTI_SELECT", "CHECKBOX", "MONEY", "CURRENCY", "FILE",
		"COMPANY_REFERENCE", "DOCUMENT_REFERENCE", "ROUTE", "ADDRESS", "VEHICLE", "VAT_TAX",
	}
}

func ValidateTemplateStatusFilter(status string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		return nil
	}
	switch status {
	case DraftStatus, PublishedStatus, ArchivedStatus, "REVIEW":
		return nil
	default:
		return apperrors.Validation("invalid status filter", map[string]any{
			"status":  status,
			"allowed": []string{DraftStatus, PublishedStatus, ArchivedStatus},
		})
	}
}

func CollectSectionCodes(sections []DraftFormSectionInput) []string {
	codes := make([]string, 0, len(sections))
	for _, section := range sections {
		codes = append(codes, section.Code)
	}
	return codes
}

func CollectFieldCodes(sections []DraftFormSectionInput) []string {
	codes := make([]string, 0)
	for _, section := range sections {
		for _, field := range section.Fields {
			codes = append(codes, field.Code)
		}
	}
	return codes
}
