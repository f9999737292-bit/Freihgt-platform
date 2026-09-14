package xlsxexchange

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

const (
	CarrierImportModeUpdateDraft = "UPDATE_CARRIER_DRAFT"

	MachineCodeStaleBaseline            = "stale_baseline"
	MachineCodeResponseNotEditable      = "response_not_editable"
	MachineCodeDuplicateLotNumber       = "duplicate_lot_number"
	MachineCodeUnknownLot               = "unknown_lot"
	MachineCodeDuplicateEventLevelOffer = "duplicate_event_level_offer"
	MachineCodeRfxLotIDRequired         = "rfx_lot_id_required"
)

// CarrierImportLimits bounds carrier workbook parsing.
type CarrierImportLimits struct {
	MaxRowsPerSheet int
	MaxTotalCells   int
	MaxStringLength int
	MaxAnswers      int
	MaxOfferLines   int
}

func DefaultCarrierImportLimits() CarrierImportLimits {
	return CarrierImportLimits{
		MaxRowsPerSheet: 10_000,
		MaxTotalCells:   500_000,
		MaxStringLength: 8_192,
		MaxAnswers:      5_000,
		MaxOfferLines:   500,
	}
}

var carrierDataSheetHeaders = map[string][]string{
	sheetLots:       carrierLotsHeaders,
	sheetQuestions:  carrierQuestionsHeaders,
	sheetOptions:    carrierOptionsHeaders,
	sheetRules:      carrierRulesHeaders,
	sheetAnswers:    carrierAnswersHeaders,
	sheetOfferLines: carrierOfferLinesHeaders,
}

var (
	carrierLotsHeaders      = []string{"lot_number", "name", "description", "category", "currency_code", "status"}
	carrierQuestionsHeaders = []string{
		"section_code", "question_code", "question_type",
		"title_ru", "title_en", "title_zh",
		"required", "sort_order", "validation_json",
	}
	carrierOptionsHeaders    = []string{"question_code", "option_code", "label_ru", "label_en", "label_zh", "sort_order"}
	carrierRulesHeaders      = []string{"rule_code", "source_question_code", "condition", "target_question_code", "action", "sort_order"}
	carrierAnswersHeaders    = []string{"question_code", "answer_value"}
	carrierOfferLinesHeaders = []string{"lot_number", "amount", "currency_code", "comment"}
)

var carrierMetadataAllowedKeys = map[string]struct{}{
	"schema_name": {}, "schema_version": {}, "exported_at_utc": {},
	"tenant_id": {}, "rfx_event_id": {}, "rfx_response_id": {}, "carrier_company_id": {},
	"rfx_version_id": {}, "questionnaire_version_number": {}, "response_save_version": {},
	"response_status": {}, "event_row_version": {}, "available_lots_fingerprint": {},
	"answers_fingerprint": {}, "offer_lines_fingerprint": {}, "export_mode": {},
}

var carrierForbiddenColumnPrefixes = []string{
	"competitor_", "other_carrier_", "benchmark_",
}

var carrierForbiddenColumnNames = map[string]struct{}{
	"rank": {}, "score": {}, "participant_id": {}, "response_id": {},
}

// TargetCarrierBaseline is supplied by the caller; parser never loads DB state.
type TargetCarrierBaseline struct {
	TenantID                   uuid.UUID
	EventID                    uuid.UUID
	ResponseID                 uuid.UUID
	CarrierCompanyID           uuid.UUID
	RfxVersionID               uuid.UUID
	QuestionnaireVersionNumber int
	ResponseSaveVersion        int64
	ResponseStatus             string
	EventRowVersion            int
	EventCurrency              string
	LotCount                   int
	AvailableLotsFingerprint   string
	AnswersFingerprint         string
	OfferLinesFingerprint      string
	Questionnaire              domain.QuestionnaireDefinition
	Lots                       []domain.RfxLot
	BaselineAnswers            []CarrierAnswerRow
	BaselineOfferLines         []CarrierOfferLineRow
}

// CarrierImportProposal is the normalized carrier mutation parsed from workbook.
type CarrierImportProposal struct {
	Answers    []CarrierAnswerPatch
	OfferLines []CarrierOfferLinePatch
}

// CarrierAnswerPatch is one parsed answer keyed by question_code.
type CarrierAnswerPatch struct {
	QuestionCode string
	QuestionID   uuid.UUID
	Value        json.RawMessage
	Delete       bool
}

// CarrierOfferLinePatch is one parsed commercial offer line.
type CarrierOfferLinePatch struct {
	LotNumber    string
	RfxLotID     uuid.UUID
	Amount       float64
	CurrencyCode string
	Comment      string
	Delete       bool
}

// CarrierImportSummary aggregates diff counts for preview UI.
type CarrierImportSummary struct {
	Errors            int `json:"errors"`
	Warnings          int `json:"warnings"`
	AnswersAdded      int `json:"answers_added"`
	AnswersChanged    int `json:"answers_changed"`
	AnswersRemoved    int `json:"answers_removed"`
	OfferLinesAdded   int `json:"offer_lines_added"`
	OfferLinesChanged int `json:"offer_lines_changed"`
	OfferLinesRemoved int `json:"offer_lines_removed"`
}

// CarrierAnswersDiff compares baseline vs proposed answer maps.
type CarrierAnswersDiff struct {
	Added    []string `json:"added,omitempty"`
	Changed  []string `json:"changed,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	DiffHash string   `json:"diff_hash,omitempty"`
}

// CarrierOfferLinesDiff compares baseline vs proposed offer lines.
type CarrierOfferLinesDiff struct {
	Added    []string `json:"added,omitempty"`
	Changed  []string `json:"changed,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	DiffHash string   `json:"diff_hash,omitempty"`
}

// CarrierImportPreview is the pure parser output (no HTTP mapping, no DB writes).
type CarrierImportPreview struct {
	SchemaName                string
	SchemaVersion             string
	Mode                      string
	TargetEventID             uuid.UUID
	TargetResponseID          uuid.UUID
	TargetRfxVersionID        uuid.UUID
	TargetVersionNumber       int
	TargetEventRowVersion     int
	TargetResponseSaveVersion int64
	Proposal                  CarrierImportProposal
	CanonicalPayloadHash      string
	ReadyToCommit             bool
	StaleBaseline             bool
	Summary                   CarrierImportSummary
	Errors                    []BuyerImportIssue
	Warnings                  []BuyerImportIssue
	AnswersDiff               CarrierAnswersDiff
	OfferLinesDiff            CarrierOfferLinesDiff
}
