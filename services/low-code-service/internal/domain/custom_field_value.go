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
	ReadOnly           bool
	SystemField        bool
	OptionsJSON        json.RawMessage
	ValidationRuleJSON json.RawMessage
}

type PublishedTemplateContext struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	EntityType string
	Code       string
	Version    int
	Status     string
	Fields     map[string]FieldDefinition
}

type CustomFieldValue struct {
	FieldID        uuid.UUID
	FieldCode      string
	ValueJSON      json.RawMessage
	FormTemplateID uuid.UUID
	UpdatedAt      time.Time
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
	MigrationAudit     *MigrateToActiveMigrationAudit
}

type MigrateToActiveMigrationAudit struct {
	SourceTemplateID uuid.UUID
	AllowWarnings    bool
	SkipBlocked      bool
	BatchID          uuid.UUID
	TemplateCode     string
	PreviewItem      MigrationPreviewItem
}

type UpsertCustomFieldValuesResult struct {
	TenantID   uuid.UUID
	EntityType string
	EntityID   uuid.UUID
	SavedCount int
}

type MigrateCustomFieldValuesToActiveInput struct {
	TenantID          uuid.UUID
	EntityType        string
	EntityID          uuid.UUID
	Code              string
	TemplateCode      string
	TargetTemplateID  uuid.UUID
	AllowWarnings     bool
	SkipBlocked       bool
	BatchID           uuid.UUID
	ValidationContext ValidationContext
	Audit             AuditContext
}

type MigrateCustomFieldValuesToActiveResult struct {
	Status                string
	ActiveTemplateID      uuid.UUID
	TargetTemplateID      uuid.UUID
	SourceTemplateID      uuid.UUID
	MigratedCount         int
	SkippedCount          int
	SkippedFields         []string
	CopiedFields          []string
	LegacyFields          []string
	MissingRequiredFields []string
	IncompatibleFields    []MigrationPreviewIncompatibleField
	Warnings              []string
}

const (
	MigrationPreviewStatusSafe     = "SAFE"
	MigrationPreviewStatusWarning  = "WARNING"
	MigrationPreviewStatusBlocked  = "BLOCKED"
	MaxMigrationPreviewEntityCount = 100
)

type MigrationPreviewInput struct {
	TenantID         uuid.UUID
	EntityType       string
	EntityIDs        []uuid.UUID
	TemplateCode     string
	TargetTemplateID uuid.UUID
}

type MigrationPreviewTargetTemplate struct {
	ID      uuid.UUID
	Code    string
	Version int
}

type MigrationPreviewSummary struct {
	EntitiesChecked int
	SafeToMigrate   int
	Warnings        int
	Blocked         int
}

type MigrationPreviewIncompatibleField struct {
	FieldCode string
	Reason    string
}

type MigrationPreviewItem struct {
	EntityID              uuid.UUID
	SourceTemplateID      uuid.UUID
	TargetTemplateID      uuid.UUID
	Status                string
	CopiedFields          []string
	LegacyFields          []string
	MissingRequiredFields []string
	IncompatibleFields    []MigrationPreviewIncompatibleField
	Warnings              []string
}

type ResolvedMigrationValue struct {
	FieldID   uuid.UUID
	FieldCode string
	ValueJSON []byte
}

type MigrationPreviewResult struct {
	TenantID       uuid.UUID
	EntityType     string
	TargetTemplate MigrationPreviewTargetTemplate
	Summary        MigrationPreviewSummary
	Items          []MigrationPreviewItem
}

const (
	BatchMigrateStatusCompleted          = "completed"
	BatchMigrateStatusPartiallyCompleted = "partially_completed"
	BatchMigrateStatusFailed             = "failed"
	BatchMigrateStatusBlocked            = "blocked"

	BatchMigrateItemStatusMigrated             = "migrated"
	BatchMigrateItemStatusMigratedWithWarnings = "migrated_with_warnings"
	BatchMigrateItemStatusSkipped              = "skipped"
	BatchMigrateItemStatusFailed               = "failed"

	BatchMigrateSkipReasonBlocked                     = "BLOCKED"
	BatchMigrateSkipReasonWarningsRequireConfirmation = "WARNINGS_REQUIRE_CONFIRMATION"
)

type BatchMigrateCustomFieldValuesToActiveInput struct {
	TenantID          uuid.UUID
	EntityType        string
	EntityIDs         []uuid.UUID
	TemplateCode      string
	TargetTemplateID  uuid.UUID
	AllowWarnings     bool
	SkipBlocked       bool
	BatchID           uuid.UUID
	ValidationContext ValidationContext
	Audit             AuditContext
}

type BatchMigrateSummary struct {
	Total    int
	Migrated int
	Skipped  int
	Blocked  int
	Failed   int
	Warnings int
}

type BatchMigrateItemResult struct {
	EntityID              uuid.UUID
	Status                string
	PreviewStatus         string
	Reason                string
	MigratedCount         int
	CopiedFields          []string
	LegacyFields          []string
	MissingRequiredFields []string
	IncompatibleFields    []MigrationPreviewIncompatibleField
	Warnings              []string
}

type BatchMigrateCustomFieldValuesToActiveResult struct {
	BatchID        uuid.UUID
	Status         string
	TenantID       uuid.UUID
	EntityType     string
	TemplateCode   string
	TargetTemplate MigrationPreviewTargetTemplate
	Summary        BatchMigrateSummary
	Items          []BatchMigrateItemResult
}
