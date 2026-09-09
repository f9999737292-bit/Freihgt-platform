package domain

import (
	"encoding/json"
	"strings"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

var SupportedTemplateLocales = []string{"ru-RU", "en-US", "zh-CN"}

func ValidateTemplateI18nMap(raw json.RawMessage, field string, required bool) (json.RawMessage, error) {
	if len(raw) == 0 {
		if required {
			return nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
		}
		return nil, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, apperrors.Validation(field+" must be a JSON object", map[string]any{"field": field})
	}
	if decoded == nil {
		return nil, apperrors.Validation(field+" must be a JSON object", map[string]any{"field": field})
	}
	hasNonEmpty := false
	for key, value := range decoded {
		if !isSupportedTemplateLocale(key) {
			return nil, apperrors.Validation("unsupported locale key", map[string]any{"field": field, "locale": key})
		}
		strValue, ok := value.(string)
		if !ok {
			return nil, apperrors.Validation("locale values must be strings", map[string]any{"field": field, "locale": key})
		}
		strValue = strings.TrimSpace(strValue)
		if strValue != "" {
			hasNonEmpty = true
		}
	}
	if required && !hasNonEmpty {
		return nil, apperrors.Validation("at least one non-empty locale value is required", map[string]any{"field": field})
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, apperrors.Internal("failed to normalize i18n map", err)
	}
	return normalized, nil
}

func isSupportedTemplateLocale(locale string) bool {
	locale = strings.TrimSpace(locale)
	for _, supported := range SupportedTemplateLocales {
		if locale == supported {
			return true
		}
	}
	return false
}
