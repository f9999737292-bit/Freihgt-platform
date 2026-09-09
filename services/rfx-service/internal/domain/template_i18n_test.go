package domain

import (
	"encoding/json"
	"testing"
)

func TestValidateTemplateI18nMapRequiresNonEmptyName(t *testing.T) {
	raw := json.RawMessage(`{"ru-RU":"","en-US":""}`)
	if _, err := ValidateTemplateI18nMap(raw, "name_i18n", true); err == nil {
		t.Fatal("expected validation error for empty-only locales")
	}
}

func TestValidateTemplateI18nMapRejectsUnsupportedLocale(t *testing.T) {
	raw := json.RawMessage(`{"de-DE":"Name"}`)
	if _, err := ValidateTemplateI18nMap(raw, "name_i18n", true); err == nil {
		t.Fatal("expected unsupported locale error")
	}
}

func TestValidateTemplateI18nMapAcceptsRUENZH(t *testing.T) {
	raw := json.RawMessage(`{"ru-RU":"Шаблон","en-US":"Template","zh-CN":"模板"}`)
	out, err := ValidateTemplateI18nMap(raw, "name_i18n", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected normalized output")
	}
}
