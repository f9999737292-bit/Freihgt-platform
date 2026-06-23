package domain

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type typeCompatibilityOutcome int

const (
	typeCompatibilityCompatible typeCompatibilityOutcome = iota
	typeCompatibilityWarning
	typeCompatibilityBlocked
)

type typeCompatibilityResult struct {
	Outcome          typeCompatibilityOutcome
	Reason           string
	PreviewValueJSON json.RawMessage
}

func BuildMigrationPreviewItem(
	entityID uuid.UUID,
	sourceTemplateID uuid.UUID,
	targetTemplate PublishedTemplateContext,
	existing []CustomFieldValue,
	sourceFields map[string]FieldDefinition,
) MigrationPreviewItem {
	item := MigrationPreviewItem{
		EntityID:         entityID,
		SourceTemplateID: sourceTemplateID,
		TargetTemplateID: targetTemplate.ID,
		CopiedFields:     []string{},
		LegacyFields:     []string{},
		MissingRequiredFields: []string{},
		IncompatibleFields:    []MigrationPreviewIncompatibleField{},
		Warnings:              []string{},
	}

	valueByCode := make(map[string]CustomFieldValue, len(existing))
	for _, value := range existing {
		valueByCode[value.FieldCode] = value
	}

	merged := ValueSnapshot{}

	for _, value := range existing {
		targetField, ok := targetTemplate.Fields[value.FieldCode]
		if !ok {
			item.LegacyFields = append(item.LegacyFields, value.FieldCode)
			continue
		}
		if targetField.SystemField || targetField.ReadOnly {
			item.Warnings = append(item.Warnings, fmt.Sprintf("%s is protected and will be skipped", value.FieldCode))
			continue
		}

		sourceType := resolveSourceFieldType(value, sourceFields, targetTemplate.Fields)
		compat := evaluateTypeCompatibility(sourceType, targetField, value.ValueJSON)
		switch compat.Outcome {
		case typeCompatibilityBlocked:
			item.IncompatibleFields = append(item.IncompatibleFields, MigrationPreviewIncompatibleField{
				FieldCode: value.FieldCode,
				Reason:    compat.Reason,
			})
			continue
		case typeCompatibilityWarning:
			item.Warnings = append(item.Warnings, compat.Reason)
		}

		previewValue := value.ValueJSON
		if len(compat.PreviewValueJSON) > 0 {
			previewValue = compat.PreviewValueJSON
		}
		if err := ValidateFieldValue(targetField, previewValue); err != nil {
			item.IncompatibleFields = append(item.IncompatibleFields, MigrationPreviewIncompatibleField{
				FieldCode: value.FieldCode,
				Reason:    validationFailureReason(err),
			})
			continue
		}

		item.CopiedFields = append(item.CopiedFields, value.FieldCode)
		merged[value.FieldCode] = previewValue
	}

	for code, field := range targetTemplate.Fields {
		if !field.Required || field.SystemField || field.ReadOnly {
			continue
		}
		raw, ok := merged[code]
		if !ok || IsNullJSON(raw) {
			item.MissingRequiredFields = append(item.MissingRequiredFields, code)
		}
	}

	item.Status = calculateMigrationPreviewStatus(item)
	return item
}

func calculateMigrationPreviewStatus(item MigrationPreviewItem) string {
	if len(item.IncompatibleFields) > 0 {
		return MigrationPreviewStatusBlocked
	}
	if len(item.LegacyFields) > 0 || len(item.MissingRequiredFields) > 0 || len(item.Warnings) > 0 {
		return MigrationPreviewStatusWarning
	}
	return MigrationPreviewStatusSafe
}

func InferSourceTemplateID(values []CustomFieldValue) uuid.UUID {
	counts := map[uuid.UUID]int{}
	for _, value := range values {
		if value.FormTemplateID == uuid.Nil {
			continue
		}
		counts[value.FormTemplateID]++
	}
	var best uuid.UUID
	bestCount := 0
	for templateID, count := range counts {
		if count > bestCount {
			best = templateID
			bestCount = count
		}
	}
	return best
}

func resolveSourceFieldType(value CustomFieldValue, sourceFields map[string]FieldDefinition, targetFields map[string]FieldDefinition) string {
	if sourceFields != nil {
		if field, ok := sourceFields[value.FieldCode]; ok {
			return field.FieldType
		}
	}
	if targetFields != nil {
		if field, ok := targetFields[value.FieldCode]; ok {
			return field.FieldType
		}
	}
	return inferFieldTypeFromJSON(value.ValueJSON)
}

func inferFieldTypeFromJSON(raw json.RawMessage) string {
	if IsNullJSON(raw) {
		return "TEXT"
	}
	var asBool bool
	if err := json.Unmarshal(raw, &asBool); err == nil {
		return "CHECKBOX"
	}
	var asNumber float64
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return "NUMBER"
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return "TEXT"
	}
	var asArray []string
	if err := json.Unmarshal(raw, &asArray); err == nil {
		return "MULTI_SELECT"
	}
	var asObject map[string]any
	if err := json.Unmarshal(raw, &asObject); err == nil {
		if _, ok := asObject["amount"]; ok {
			return "MONEY"
		}
		return "TEXT"
	}
	return "TEXT"
}

func evaluateTypeCompatibility(sourceType string, targetField FieldDefinition, raw json.RawMessage) typeCompatibilityResult {
	sourceType = strings.ToUpper(strings.TrimSpace(sourceType))
	targetType := strings.ToUpper(strings.TrimSpace(targetField.FieldType))

	if sourceType == targetType {
		return typeCompatibilityResult{Outcome: typeCompatibilityCompatible, PreviewValueJSON: raw}
	}

	switch {
	case sourceType == "TEXT" && targetType == "CURRENCY":
		return typeCompatibilityResult{Outcome: typeCompatibilityCompatible, PreviewValueJSON: raw}
	case sourceType == "CURRENCY" && targetType == "TEXT":
		return typeCompatibilityResult{Outcome: typeCompatibilityCompatible, PreviewValueJSON: raw}
	case sourceType == "SELECT" && targetType == "TEXT":
		return typeCompatibilityResult{Outcome: typeCompatibilityCompatible, PreviewValueJSON: raw}
	case sourceType == "TEXT" && targetType == "SELECT":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return typeCompatibilityResult{
				Outcome: typeCompatibilityBlocked,
				Reason:  "TEXT value is not a string",
			}
		}
		allowed := extractOptionValues(targetField.OptionsJSON)
		if len(allowed) > 0 && !allowed[value] {
			return typeCompatibilityResult{
				Outcome: typeCompatibilityWarning,
				Reason:  fmt.Sprintf("TEXT value %q is not in target SELECT options", value),
				PreviewValueJSON: raw,
			}
		}
		return typeCompatibilityResult{
			Outcome:          typeCompatibilityWarning,
			Reason:           "TEXT to SELECT conversion",
			PreviewValueJSON: raw,
		}
	case sourceType == "SELECT" && targetType == "MULTI_SELECT":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return typeCompatibilityResult{
				Outcome: typeCompatibilityBlocked,
				Reason:  "SELECT value is not a string",
			}
		}
		wrapped, err := json.Marshal([]string{value})
		if err != nil {
			return typeCompatibilityResult{
				Outcome: typeCompatibilityBlocked,
				Reason:  "failed to wrap SELECT value for MULTI_SELECT preview",
			}
		}
		return typeCompatibilityResult{
			Outcome:          typeCompatibilityWarning,
			Reason:           "SELECT to MULTI_SELECT conversion",
			PreviewValueJSON: wrapped,
		}
	case sourceType == "NUMBER" && targetType == "MONEY":
		return typeCompatibilityResult{
			Outcome: typeCompatibilityBlocked,
			Reason:  "NUMBER to MONEY is incompatible",
		}
	case sourceType == "MONEY" && targetType == "NUMBER":
		return typeCompatibilityResult{
			Outcome: typeCompatibilityBlocked,
			Reason:  "MONEY to NUMBER is incompatible",
		}
	case sourceType == "CHECKBOX" || targetType == "CHECKBOX":
		return typeCompatibilityResult{
			Outcome: typeCompatibilityBlocked,
			Reason:  fmt.Sprintf("%s to %s is incompatible for CHECKBOX", sourceType, targetType),
		}
	case sourceType == "DATE" || targetType == "DATE":
		if sourceType != targetType {
			return typeCompatibilityResult{
				Outcome: typeCompatibilityBlocked,
				Reason:  fmt.Sprintf("%s to %s is incompatible for DATE", sourceType, targetType),
			}
		}
	case sourceType == "DATETIME" || targetType == "DATETIME":
		if sourceType != targetType {
			return typeCompatibilityResult{
				Outcome: typeCompatibilityBlocked,
				Reason:  fmt.Sprintf("%s to %s is incompatible for DATETIME", sourceType, targetType),
			}
		}
	default:
		return typeCompatibilityResult{
			Outcome:          typeCompatibilityWarning,
			Reason:           fmt.Sprintf("%s to %s requires validation", sourceType, targetType),
			PreviewValueJSON: raw,
		}
	}

	return typeCompatibilityResult{Outcome: typeCompatibilityCompatible, PreviewValueJSON: raw}
}

func validationFailureReason(err error) string {
	if err == nil {
		return "validation failed"
	}
	return err.Error()
}

func BuildResolvedMigrationValues(
	preview MigrationPreviewItem,
	existing []CustomFieldValue,
	target PublishedTemplateContext,
	sourceFields map[string]FieldDefinition,
) []ResolvedMigrationValue {
	valueByCode := make(map[string]CustomFieldValue, len(existing))
	for _, value := range existing {
		valueByCode[value.FieldCode] = value
	}

	resolved := make([]ResolvedMigrationValue, 0, len(preview.CopiedFields))
	for _, fieldCode := range preview.CopiedFields {
		value, ok := valueByCode[fieldCode]
		if !ok {
			continue
		}
		targetField, ok := target.Fields[fieldCode]
		if !ok {
			continue
		}
		sourceType := resolveSourceFieldType(value, sourceFields, target.Fields)
		compat := evaluateTypeCompatibility(sourceType, targetField, value.ValueJSON)
		previewValue := value.ValueJSON
		if len(compat.PreviewValueJSON) > 0 {
			previewValue = compat.PreviewValueJSON
		}
		var valueBytes []byte
		if IsNullJSON(previewValue) {
			valueBytes = nil
		} else {
			valueBytes = append([]byte(nil), previewValue...)
		}
		resolved = append(resolved, ResolvedMigrationValue{
			FieldID:   targetField.ID,
			FieldCode: fieldCode,
			ValueJSON: valueBytes,
		})
	}
	return resolved
}

func MigrationPreviewItemToMap(item MigrationPreviewItem, targetTemplate MigrationPreviewTargetTemplate, entityType string, tenantID uuid.UUID) map[string]any {
	incompatible := make([]map[string]string, 0, len(item.IncompatibleFields))
	for _, field := range item.IncompatibleFields {
		incompatible = append(incompatible, map[string]string{
			"field_code": field.FieldCode,
			"reason":     field.Reason,
		})
	}
	sourceTemplateID := ""
	if item.SourceTemplateID != uuid.Nil {
		sourceTemplateID = item.SourceTemplateID.String()
	}
	return map[string]any{
		"tenant_id":   tenantID.String(),
		"entity_type": entityType,
		"target_template": map[string]any{
			"id":      targetTemplate.ID.String(),
			"code":    targetTemplate.Code,
			"version": targetTemplate.Version,
		},
		"summary": map[string]any{
			"entities_checked": 1,
			"safe_to_migrate":  boolItemCount(item.Status, MigrationPreviewStatusSafe),
			"warnings":         boolItemCount(item.Status, MigrationPreviewStatusWarning),
			"blocked":          boolItemCount(item.Status, MigrationPreviewStatusBlocked),
		},
		"items": []map[string]any{{
			"entity_id":               item.EntityID.String(),
			"source_template_id":      sourceTemplateID,
			"target_template_id":      item.TargetTemplateID.String(),
			"status":                  item.Status,
			"copied_fields":           item.CopiedFields,
			"legacy_fields":           item.LegacyFields,
			"missing_required_fields": item.MissingRequiredFields,
			"incompatible_fields":     incompatible,
			"warnings":                item.Warnings,
		}},
	}
}

func boolItemCount(status string, expected string) int {
	if status == expected {
		return 1
	}
	return 0
}
