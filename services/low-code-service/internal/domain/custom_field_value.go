package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FieldDefinition struct {
	ID                 uuid.UUID
	Code               string
	FieldType          string
	Required           bool
	SystemField        bool
	OptionsJSON        json.RawMessage
	ValidationRuleJSON json.RawMessage
}

type PublishedTemplateContext struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	EntityType string
	Status     string
	Fields     map[string]FieldDefinition
}

type CustomFieldValue struct {
	FieldID    uuid.UUID
	FieldCode  string
	ValueJSON  json.RawMessage
	UpdatedAt  time.Time
}

type CustomFieldValueInput struct {
	FieldCode string
	ValueJSON json.RawMessage
}

type UpsertCustomFieldValuesInput struct {
	TenantID           uuid.UUID
	EntityType         string
	EntityID           uuid.UUID
	FormTemplateID     uuid.UUID
	Values             []CustomFieldValueInput
	ValidationContext  ValidationContext
	Audit              AuditContext
}

type UpsertCustomFieldValuesResult struct {
	TenantID   uuid.UUID
	EntityType string
	EntityID   uuid.UUID
	SavedCount int
}
