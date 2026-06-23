package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	TemplateExportSchemaVersion = "lowcode.template.export.v1"
	DefaultExportServiceName    = "low-code-service"
	DefaultExportEnvironment    = "local"
)

type TemplateExportSource struct {
	TemplateID      string `json:"template_id"`
	TenantID        string `json:"tenant_id"`
	Environment     string `json:"environment"`
	Service         string `json:"service"`
	TemplateCode    string `json:"template_code,omitempty"`
	TemplateVersion int    `json:"template_version,omitempty"`
	TemplateStatus  string `json:"template_status,omitempty"`
}

type ExportedFormField struct {
	Code               string          `json:"code"`
	Label              string          `json:"label"`
	FieldType          string          `json:"field_type"`
	Required           bool            `json:"required"`
	ReadOnly           bool            `json:"read_only"`
	SystemField        bool            `json:"system_field"`
	OptionsJSON        json.RawMessage `json:"options_json,omitempty"`
	ValidationRuleJSON json.RawMessage `json:"validation_rule_json,omitempty"`
	VisibilityRuleJSON json.RawMessage `json:"visibility_rule_json,omitempty"`
	SortOrder          int             `json:"sort_order"`
}

type ExportedFormSection struct {
	Code      string              `json:"code"`
	Title     string              `json:"title"`
	SortOrder int                 `json:"sort_order"`
	Fields    []ExportedFormField `json:"fields"`
}

type ExportedFormTemplate struct {
	EntityType  string                `json:"entity_type"`
	Code        string                `json:"code"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Version     int                   `json:"version"`
	Status      string                `json:"status"`
	Sections    []ExportedFormSection `json:"sections"`
}

type TemplateExportMetadata struct {
	Checksum   string  `json:"checksum,omitempty"`
	ExportedBy *string `json:"exported_by,omitempty"`
	RequestID  string  `json:"request_id,omitempty"`
}

type TemplateExportEnvelope struct {
	SchemaVersion string                 `json:"schema_version"`
	ExportedAt    string                 `json:"exported_at"`
	Source        TemplateExportSource   `json:"source"`
	Template      ExportedFormTemplate   `json:"template"`
	Metadata      TemplateExportMetadata `json:"metadata"`
}

func IsExportableTemplateStatus(status string) bool {
	switch status {
	case DraftStatus, PublishedStatus, ArchivedStatus:
		return true
	default:
		return false
	}
}

func BuildPortableTemplate(detail FormTemplateDetail) ExportedFormTemplate {
	sections := make([]ExportedFormSection, 0, len(detail.Sections))
	for _, section := range detail.Sections {
		fields := make([]ExportedFormField, 0, len(section.Fields))
		for _, field := range section.Fields {
			fields = append(fields, ExportedFormField{
				Code:               field.Code,
				Label:              field.Label,
				FieldType:          field.FieldType,
				Required:           field.Required,
				ReadOnly:           field.ReadOnly,
				SystemField:        field.SystemField,
				OptionsJSON:        normalizeExportJSON(field.OptionsJSON),
				ValidationRuleJSON: normalizeExportJSON(field.ValidationRuleJSON),
				VisibilityRuleJSON: normalizeExportJSON(field.VisibilityRuleJSON),
				SortOrder:          field.SortOrder,
			})
		}
		sections = append(sections, ExportedFormSection{
			Code:      section.Code,
			Title:     section.Title,
			SortOrder: section.SortOrder,
			Fields:    fields,
		})
	}

	return ExportedFormTemplate{
		EntityType:  detail.EntityType,
		Code:        detail.Code,
		Name:        detail.Name,
		Description: detail.Description,
		Version:     detail.Version,
		Status:      detail.Status,
		Sections:    sections,
	}
}

func BuildTemplateExportEnvelope(
	detail FormTemplateDetail,
	audit AuditContext,
	exportedAt time.Time,
	environment string,
	serviceName string,
) (TemplateExportEnvelope, error) {
	if environment == "" {
		environment = DefaultExportEnvironment
	}
	if serviceName == "" {
		serviceName = DefaultExportServiceName
	}

	portable := BuildPortableTemplate(detail)
	checksum, err := ComputeTemplateExportChecksum(portable)
	if err != nil {
		return TemplateExportEnvelope{}, err
	}

	metadata := TemplateExportMetadata{
		Checksum:  checksum,
		RequestID: audit.RequestID,
	}
	if audit.ChangedByUserID != nil {
		actor := audit.ChangedByUserID.String()
		metadata.ExportedBy = &actor
	}

	return TemplateExportEnvelope{
		SchemaVersion: TemplateExportSchemaVersion,
		ExportedAt:    exportedAt.UTC().Format(time.RFC3339),
		Source: TemplateExportSource{
			TemplateID:      detail.ID.String(),
			TenantID:        detail.TenantID.String(),
			Environment:     environment,
			Service:         serviceName,
			TemplateCode:    detail.Code,
			TemplateVersion: detail.Version,
			TemplateStatus:  detail.Status,
		},
		Template: portable,
		Metadata: metadata,
	}, nil
}

func ComputeTemplateExportChecksum(template ExportedFormTemplate) (string, error) {
	raw, err := json.Marshal(template)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func BuildFormTemplateExportedAuditPayload(
	templateID uuid.UUID,
	code string,
	version int,
	status string,
	schemaVersion string,
) (json.RawMessage, error) {
	payload := map[string]any{
		"event_kind":           AuditEventKindFormTemplateExported,
		"template_id":          templateID.String(),
		"code":                 code,
		"version":              version,
		"status":               status,
		"schema_version":       schemaVersion,
	}
	return json.Marshal(payload)
}

func normalizeExportJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}
