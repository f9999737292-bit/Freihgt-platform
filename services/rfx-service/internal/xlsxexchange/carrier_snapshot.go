package xlsxexchange

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

const (
	CarrierExportModeDraftEdit         = "DRAFT_EDIT"
	CarrierExportModeSubmittedReadonly = "SUBMITTED_READONLY"

	carrierEventLevelOfferKey = "__EVENT_LEVEL__"
)

// CarrierResponseMetadata holds export metadata written to the Metadata sheet.
type CarrierResponseMetadata struct {
	ExportedAtUTC              time.Time
	TenantID                   uuid.UUID
	RfxEventID                 uuid.UUID
	RfxResponseID              uuid.UUID
	CarrierCompanyID           uuid.UUID
	RfxVersionID               uuid.UUID
	QuestionnaireVersionNumber int
	ResponseSaveVersion        int64
	ResponseStatus             string
	EventRowVersion            int
	AvailableLotsFingerprint   string
	AnswersFingerprint         string
	OfferLinesFingerprint      string
	ExportMode                 string
}

// CarrierQuestion binds a questionnaire question to its section_code for export.
type CarrierQuestion struct {
	SectionCode string
	Question    domain.Question
}

// CarrierOption binds an option to its parent question_code for export.
type CarrierOption struct {
	QuestionCode string
	Option       domain.QuestionOption
}

// CarrierRule binds a visibility rule to resolved question codes for export.
type CarrierRule struct {
	RuleCode           string
	SourceQuestionCode string
	ConditionJSON      json.RawMessage
	TargetQuestionCode string
	Action             string
	SortOrder          int
}

// CarrierAnswerRow is an exported answer keyed by question_code.
type CarrierAnswerRow struct {
	QuestionCode string
	AnswerValue  string
}

// CarrierOfferLineRow is an exported commercial offer line.
type CarrierOfferLineRow struct {
	LotNumber    string
	Amount       string
	CurrencyCode string
	Comment      string
}

// CarrierResponseSnapshot is the canonical in-memory graph exported to CARRIER XLSX V1.
type CarrierResponseSnapshot struct {
	Metadata   CarrierResponseMetadata
	Lots       []domain.RfxLot
	Questions  []CarrierQuestion
	Options    []CarrierOption
	Rules      []CarrierRule
	Answers    []CarrierAnswerRow
	OfferLines []CarrierOfferLineRow
}
