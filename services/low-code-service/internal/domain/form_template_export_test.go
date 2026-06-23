package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildPortableTemplateOmitsDBIDs(t *testing.T) {
	sectionID := uuid.New()
	fieldID := uuid.New()
	detail := FormTemplateDetail{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Status:     PublishedStatus,
		Version:    1,
		Sections: []FormSection{
			{
				ID:        sectionID,
				Code:      "general",
				Title:     "General",
				SortOrder: 100,
				Fields: []FormField{
					{
						ID:        fieldID,
						Code:      "cargo_class",
						Label:     "Cargo class",
						FieldType: "SELECT",
						SortOrder: 100,
						OptionsJSON: json.RawMessage(`{"options":["GENERAL"]}`),
					},
				},
			},
		},
	}

	portable := BuildPortableTemplate(detail)
	raw, err := json.Marshal(portable)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, sectionID.String()) || strings.Contains(body, fieldID.String()) {
		t.Fatalf("portable template must not contain DB ids: %s", body)
	}
	if portable.Sections[0].Code != "general" || portable.Sections[0].Fields[0].Code != "cargo_class" {
		t.Fatalf("expected logical codes preserved: %+v", portable)
	}
}

func TestBuildTemplateExportEnvelopeSchemaVersion(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.New()
	detail := FormTemplateDetail{
		ID:         templateID,
		TenantID:   tenantID,
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Default",
		Status:     PublishedStatus,
		Version:    1,
		Sections:   []FormSection{},
	}

	envelope, err := BuildTemplateExportEnvelope(
		detail,
		AuditContext{RequestID: "req-1"},
		mustParseTime(t, "2026-06-24T12:00:00Z"),
		"local",
		"low-code-service",
	)
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	if envelope.SchemaVersion != TemplateExportSchemaVersion {
		t.Fatalf("schema_version = %q", envelope.SchemaVersion)
	}
	if envelope.Source.TemplateID != templateID.String() {
		t.Fatalf("source template_id mismatch")
	}
	if envelope.Metadata.Checksum == "" {
		t.Fatal("expected checksum")
	}
}

func TestIsExportableTemplateStatus(t *testing.T) {
	for _, status := range []string{DraftStatus, PublishedStatus, ArchivedStatus} {
		if !IsExportableTemplateStatus(status) {
			t.Fatalf("expected exportable: %s", status)
		}
	}
	if IsExportableTemplateStatus("REVIEW") {
		t.Fatal("REVIEW should not be exportable")
	}
}

func mustParseTime(t *testing.T, raw string) (result time.Time) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}
