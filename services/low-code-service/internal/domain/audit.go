package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	AuditEventKindCustomFieldValuesUpdated = "CUSTOM_FIELD_VALUES_UPDATED"
	AuditEventKindFormTemplateDraftCreated = "FORM_TEMPLATE_DRAFT_CREATED"
	AuditEventKindFormTemplateDraftUpdated = "FORM_TEMPLATE_DRAFT_UPDATED"
	AuditEventKindFormTemplateDraftPublished = "FORM_TEMPLATE_DRAFT_PUBLISHED"
	AuditEventKindFormTemplateClonedToDraft   = "FORM_TEMPLATE_CLONED_TO_DRAFT"
	AuditDBActionCreate                    = "CREATE"
	AuditDBActionUpdate                    = "UPDATE"
	AuditDBActionPublish                   = "PUBLISH"
)

type AuditContext struct {
	ChangedByUserID *uuid.UUID
	RequestID       string
	IPAddress       string
	UserAgent       string
}

type ConfigurationAuditEntry struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ConfigurationID *uuid.UUID
	EntityType      string
	EntityID        uuid.UUID
	Action          string
	OldValueJSON    json.RawMessage
	NewValueJSON    json.RawMessage
	ChangedByUserID *uuid.UUID
	RequestID       string
	IPAddress       string
	UserAgent       string
	ChangedAt       time.Time
}

type ListAuditEventsFilter struct {
	TenantID   uuid.UUID
	EntityType string
	EntityID   *uuid.UUID
	Action     string
	Limit      int
}

func BuildCustomFieldValuesAuditPayload(
	formTemplateID uuid.UUID,
	resolved []ResolvedCustomFieldValueForAudit,
	oldValues map[string][]byte,
) (oldJSON json.RawMessage, newJSON json.RawMessage, changedFields []string, err error) {
	newValues := make(map[string]json.RawMessage, len(resolved))
	oldMap := make(map[string]json.RawMessage, len(resolved))
	changedFields = make([]string, 0, len(resolved))

	for _, item := range resolved {
		changedFields = append(changedFields, item.FieldCode)
		if item.ValueJSON != nil {
			newValues[item.FieldCode] = json.RawMessage(append([]byte(nil), item.ValueJSON...))
		} else {
			newValues[item.FieldCode] = json.RawMessage("null")
		}
		if oldRaw, ok := oldValues[item.FieldCode]; ok {
			if oldRaw == nil {
				oldMap[item.FieldCode] = json.RawMessage("null")
			} else {
				oldMap[item.FieldCode] = json.RawMessage(append([]byte(nil), oldRaw...))
			}
		} else {
			oldMap[item.FieldCode] = json.RawMessage("null")
		}
	}

	oldPayload := map[string]any{"values": oldMap}
	newPayload := map[string]any{
		"event_kind":       AuditEventKindCustomFieldValuesUpdated,
		"form_template_id": formTemplateID.String(),
		"changed_fields":   changedFields,
		"values":           newValues,
	}

	oldJSON, err = json.Marshal(oldPayload)
	if err != nil {
		return nil, nil, nil, err
	}
	newJSON, err = json.Marshal(newPayload)
	if err != nil {
		return nil, nil, nil, err
	}
	return oldJSON, newJSON, changedFields, nil
}

type ResolvedCustomFieldValueForAudit struct {
	FieldCode string
	ValueJSON []byte
}

func ParseAuditEventAction(dbAction string, newValueJSON json.RawMessage) string {
	if len(newValueJSON) == 0 {
		return dbAction
	}
	var payload struct {
		EventKind string `json:"event_kind"`
	}
	if err := json.Unmarshal(newValueJSON, &payload); err == nil && payload.EventKind != "" {
		return payload.EventKind
	}
	return dbAction
}

func ParseAuditChangedFields(newValueJSON json.RawMessage) []string {
	if len(newValueJSON) == 0 {
		return nil
	}
	var payload struct {
		ChangedFields []string `json:"changed_fields"`
	}
	if err := json.Unmarshal(newValueJSON, &payload); err != nil {
		return nil
	}
	return payload.ChangedFields
}

func BuildFormTemplateClonedAuditPayload(
	sourceTemplateID uuid.UUID,
	draftTemplateID uuid.UUID,
	entityType string,
	sourceCode string,
	draftCode string,
	sourceVersion int,
	draftVersion int,
	sectionsCount int,
	fieldsCount int,
) (json.RawMessage, error) {
	payload := map[string]any{
		"event_kind":          AuditEventKindFormTemplateClonedToDraft,
		"source_template_id":  sourceTemplateID.String(),
		"draft_template_id":   draftTemplateID.String(),
		"entity_type":         entityType,
		"source_code":         sourceCode,
		"draft_code":          draftCode,
		"source_version":      sourceVersion,
		"draft_version":       draftVersion,
		"sections_count":      sectionsCount,
		"fields_count":        fieldsCount,
	}
	return json.Marshal(payload)
}

func BuildFormTemplateDraftAuditPayload(
	eventKind string,
	templateID uuid.UUID,
	entityType string,
	code string,
	sectionCodes []string,
	fieldCodes []string,
) (json.RawMessage, error) {
	payload := map[string]any{
		"event_kind":       eventKind,
		"template_id":      templateID.String(),
		"entity_type":      entityType,
		"code":             code,
		"section_codes":    sectionCodes,
		"field_codes":      fieldCodes,
		"sections_count":   len(sectionCodes),
		"fields_count":     len(fieldCodes),
	}
	return json.Marshal(payload)
}

func ParseAuditValuesMap(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return map[string]json.RawMessage{}
	}
	var payload struct {
		Values map[string]json.RawMessage `json:"values"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Values == nil {
		return map[string]json.RawMessage{}
	}
	return payload.Values
}
