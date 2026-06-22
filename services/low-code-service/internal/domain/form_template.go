package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const PublishedStatus = "PUBLISHED"

type FormTemplateSummary struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	EntityType    string
	Code          string
	Name          string
	Status        string
	Version       int
	SectionsCount int
	FieldsCount   int
	PublishedAt   *time.Time
}

type FormField struct {
	ID                 uuid.UUID
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

type FormSection struct {
	ID        uuid.UUID
	Code      string
	Title     string
	SortOrder int
	Fields    []FormField
}

type FormTemplateDetail struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	EntityType  string
	Code        string
	Name        string
	Description string
	Status      string
	Version     int
	PublishedAt *time.Time
	Sections    []FormSection
}
