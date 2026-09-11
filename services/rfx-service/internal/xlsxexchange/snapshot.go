package xlsxexchange

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

const schemaVersionNumber = "1"

// BuyerDraftMetadata holds export metadata written to the Metadata sheet.
type BuyerDraftMetadata struct {
	ExportedAtUTC               time.Time
	TenantID                    uuid.UUID
	RfxEventID                  uuid.UUID
	RfxVersionID                uuid.UUID
	VersionNumber               int
	VersionStatus               string
	EventRowVersion             int
	VersionRowVersion           int
	CreationChannel             string
	SourceTemplateVersionID     *uuid.UUID
	SourceTemplateVersionNumber *int
}

// BuyerDraftQuestion binds a questionnaire question to its section_code for export.
type BuyerDraftQuestion struct {
	SectionCode string
	Question    domain.Question
}

// BuyerDraftOption binds an option to its parent question_code for export.
type BuyerDraftOption struct {
	QuestionCode string
	Option       domain.QuestionOption
}

// BuyerDraftRule binds a visibility rule to resolved question codes for export.
type BuyerDraftRule struct {
	RuleCode           string
	SourceQuestionCode string
	ConditionJSON      json.RawMessage
	TargetQuestionCode string
	Action             string
	SortOrder          int
}

// BuyerDraftSnapshot is the canonical in-memory graph exported to BUYER XLSX V1.
type BuyerDraftSnapshot struct {
	Metadata  BuyerDraftMetadata
	Lots      []domain.RfxLot
	Sections  []domain.Section
	Questions []BuyerDraftQuestion
	Options   []BuyerDraftOption
	Rules     []BuyerDraftRule
}
