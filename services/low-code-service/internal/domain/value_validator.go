package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func ValidateFieldValue(field FieldDefinition, raw json.RawMessage) error {
	if field.SystemField {
		return apperrors.SystemFieldProtected(field.Code)
	}

	if isJSONNull(raw) {
		if field.Required {
			return apperrors.ValidationRuleFailed(field.Code, map[string]any{"reason": "required field"})
		}
		return nil
	}

	var err error
	switch field.FieldType {
	case "TEXT":
		err = validateText(raw)
	case "NUMBER":
		err = validateNumber(raw)
	case "DATE":
		err = validateDate(raw)
	case "DATETIME":
		err = validateDateTime(raw)
	case "SELECT":
		err = validateSelect(raw, field.OptionsJSON, false)
	case "MULTI_SELECT":
		err = validateSelect(raw, field.OptionsJSON, true)
	case "CHECKBOX":
		err = validateCheckbox(raw)
	case "MONEY":
		err = validateMoney(raw)
	case "CURRENCY":
		err = validateCurrency(raw)
	case "FILE":
		err = validateObjectOrNull(raw)
	case "COMPANY_REFERENCE", "DOCUMENT_REFERENCE":
		err = validateUUIDString(raw)
	case "ROUTE", "ADDRESS", "VEHICLE", "VAT_TAX":
		err = validateObject(raw)
	default:
		err = apperrors.FieldInvalidType(field.Code, field.FieldType, map[string]any{"reason": "unsupported field type"})
	}
	if err != nil {
		if _, ok := err.(*apperrors.AppError); ok {
			return err
		}
		return apperrors.FieldInvalidType(field.Code, field.FieldType, map[string]any{"reason": err.Error()})
	}

	return validateSimpleRules(field, raw)
}

func validateSimpleRules(field FieldDefinition, raw json.RawMessage) error {
	if len(field.ValidationRuleJSON) == 0 || isJSONNull(field.ValidationRuleJSON) {
		return nil
	}

	var rules map[string]any
	if err := json.Unmarshal(field.ValidationRuleJSON, &rules); err != nil {
		return nil
	}
	if _, hasIf := rules["if"]; hasIf {
		return nil
	}

	if field.FieldType == "TEXT" {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return apperrors.FieldInvalidType(field.Code, field.FieldType, nil)
		}
		if minLen, ok := rules["minLength"].(float64); ok && float64(len(value)) < minLen {
			return apperrors.ValidationRuleFailed(field.Code, map[string]any{"rule": "minLength", "minLength": int(minLen)})
		}
		if maxLen, ok := rules["maxLength"].(float64); ok && float64(len(value)) > maxLen {
			return apperrors.ValidationRuleFailed(field.Code, map[string]any{"rule": "maxLength", "maxLength": int(maxLen)})
		}
	}

	if field.FieldType == "NUMBER" {
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			return apperrors.FieldInvalidType(field.Code, field.FieldType, nil)
		}
		if minVal, ok := rules["min"].(float64); ok && value < minVal {
			return apperrors.ValidationRuleFailed(field.Code, map[string]any{"rule": "min", "min": minVal})
		}
		if maxVal, ok := rules["max"].(float64); ok && value > maxVal {
			return apperrors.ValidationRuleFailed(field.Code, map[string]any{"rule": "max", "max": maxVal})
		}
	}

	return nil
}

func validateText(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("text")
	}
	return nil
}

func validateNumber(raw json.RawMessage) error {
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("number")
	}
	return nil
}

func validateDate(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !datePattern.MatchString(value) {
		return fmt.Errorf("date")
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("date")
	}
	return nil
}

func validateDateTime(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("datetime")
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return fmt.Errorf("datetime")
	}
	return nil
}

func validateSelect(raw json.RawMessage, optionsJSON json.RawMessage, multi bool) error {
	allowed := extractOptionValues(optionsJSON)
	if multi {
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("multi_select")
		}
		for _, value := range values {
			if len(allowed) > 0 && !allowed[value] {
				return fmt.Errorf("multi_select option")
			}
		}
		return nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("select")
	}
	if len(allowed) > 0 && !allowed[value] {
		return fmt.Errorf("select option")
	}
	return nil
}

func validateCheckbox(raw json.RawMessage) error {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("checkbox")
	}
	return nil
}

func validateMoney(raw json.RawMessage) error {
	var payload struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("money")
	}
	if strings.TrimSpace(payload.Currency) == "" {
		return fmt.Errorf("money currency")
	}
	return nil
}

func validateCurrency(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return fmt.Errorf("currency")
	}
	return nil
}

func validateObject(raw json.RawMessage) error {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("object")
	}
	return nil
}

func validateObjectOrNull(raw json.RawMessage) error {
	if isJSONNull(raw) {
		return nil
	}
	return validateObject(raw)
}

func validateUUIDString(raw json.RawMessage) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("uuid")
	}
	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf("uuid")
	}
	return nil
}

func extractOptionValues(optionsJSON json.RawMessage) map[string]bool {
	result := map[string]bool{}
	if len(optionsJSON) == 0 || isJSONNull(optionsJSON) {
		return result
	}
	var payload struct {
		Options []struct {
			Value string `json:"value"`
		} `json:"options"`
	}
	if err := json.Unmarshal(optionsJSON, &payload); err != nil {
		return result
	}
	for _, option := range payload.Options {
		result[option.Value] = true
	}
	return result
}

func isJSONNull(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

func IsNullJSON(raw json.RawMessage) bool {
	return isJSONNull(raw)
}
