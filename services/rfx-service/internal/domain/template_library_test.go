package domain

import (
	"encoding/json"
	"testing"
)

func TestValidateCreateTemplateInput(t *testing.T) {
	in := CreateTemplateInput{
		TemplateCode: "lane-tender",
		NameI18n:     json.RawMessage(`{"en-US":"Lane tender"}`),
	}
	if err := ValidateCreateTemplateInput(in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePublishTemplateVersionInputRequiresSummary(t *testing.T) {
	in := PublishTemplateVersionInput{ExpectedTemplateVersion: 1, ExpectedDraftVersion: 1}
	if err := ValidatePublishTemplateVersionInput(in); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEnsureTemplateActiveRejectsArchived(t *testing.T) {
	if err := EnsureTemplateActive(RfxTemplateStatusArchived); err == nil {
		t.Fatal("expected conflict")
	}
}
