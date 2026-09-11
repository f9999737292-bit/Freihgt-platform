package xlsxexchange

import (
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

const (
	BuyerImportModeUpdateDraft = "UPDATE_DRAFT"

	IssueSeverityError   = "error"
	IssueSeverityWarning = "warning"

	MachineCodeInvalidMultipart        = "invalid_multipart"
	MachineCodeFileTooLarge            = "file_too_large"
	MachineCodeInvalidXLSXSignature    = "invalid_xlsx_signature"
	MachineCodeUnsafePackage           = "unsafe_package"
	MachineCodeUnsupportedSchema       = "unsupported_schema"
	MachineCodeMissingSheet            = "missing_sheet"
	MachineCodeUnexpectedSheet         = "unexpected_sheet"
	MachineCodeInvalidHeader           = "invalid_header"
	MachineCodeMissingRequiredValue    = "missing_required_value"
	MachineCodeInvalidType             = "invalid_type"
	MachineCodeDuplicateStableCode     = "duplicate_stable_code"
	MachineCodeDanglingReference       = "dangling_reference"
	MachineCodeCyclicRule              = "cyclic_rule"
	MachineCodeSelfTargetRule          = "self_target_rule"
	MachineCodeFormulaDenied           = "formula_denied"
	MachineCodeMergedCellDenied        = "merged_cell_denied"
	MachineCodeHiddenSheetDenied       = "hidden_sheet_denied"
	MachineCodeHiddenContentWarning    = "hidden_content_warning"
	MachineCodeCompetitorColumnDenied  = "competitor_column_denied"
	MachineCodeI18NMonolingualMismatch = "I18N_MONOLINGUAL_MISMATCH"
	MachineCodeMetadataMismatch        = "metadata_mismatch"
	MachineCodeTooManyRows             = "too_many_rows"
	MachineCodeTooManyCells            = "too_many_cells"
	MachineCodePreviewNotReady         = "preview_not_ready"
)

// BuyerImportLimits mirrors discovery §8.2.
type BuyerImportLimits struct {
	MaxRowsPerSheet int
	MaxTotalCells   int
	MaxStringLength int
	MaxLots         int
	MaxSections     int
	MaxQuestions    int
	MaxOptions      int
	MaxRules        int
}

func DefaultBuyerImportLimits() BuyerImportLimits {
	return BuyerImportLimits{
		MaxRowsPerSheet: 10_000,
		MaxTotalCells:   500_000,
		MaxStringLength: 8_192,
		MaxLots:         500,
		MaxSections:     500,
		MaxQuestions:    5_000,
		MaxOptions:      20_000,
		MaxRules:        5_000,
	}
}

var (
	lotsHeaders      = []string{"lot_number", "name", "description", "category", "estimated_value", "currency_code", "status"}
	sectionsHeaders  = []string{"section_code", "title_ru", "title_en", "title_zh", "description_ru", "description_en", "description_zh", "sort_order"}
	questionsHeaders = []string{
		"section_code", "question_code", "question_type",
		"title_ru", "title_en", "title_zh",
		"description_ru", "description_en", "description_zh",
		"required", "sort_order", "validation_json",
	}
	optionsHeaders = []string{"question_code", "option_code", "label_ru", "label_en", "label_zh", "sort_order"}
	rulesHeaders   = []string{"rule_code", "source_question_code", "condition", "target_question_code", "action", "sort_order"}
)

var metadataAllowedKeys = map[string]struct{}{
	"schema_name": {}, "schema_version": {}, "exported_at_utc": {},
	"tenant_id": {}, "rfx_event_id": {}, "rfx_version_id": {},
	"version_number": {}, "version_status": {},
	"event_row_version": {}, "version_row_version": {},
	"creation_channel": {}, "source_template_version_id": {}, "source_template_version_number": {},
}

// TargetDraftBaseline is supplied by the caller; parser never loads DB state.
type TargetDraftBaseline struct {
	TenantID           uuid.UUID
	EventID            uuid.UUID
	DraftVersionID     uuid.UUID
	DraftVersionNumber int
	EventRowVersion    int
	DraftRowVersion    int
	Questionnaire      domain.QuestionnaireDefinition
	Lots               []domain.RfxLot
}

// BuyerImportIssue is one validation finding for structured preview responses.
type BuyerImportIssue struct {
	Severity    string         `json:"severity"`
	MachineCode string         `json:"machine_code"`
	Sheet       string         `json:"sheet,omitempty"`
	Row         int            `json:"row,omitempty"`
	Column      string         `json:"column,omitempty"`
	StableCode  string         `json:"stable_code,omitempty"`
	MessageKey  string         `json:"message_key"`
	Params      map[string]any `json:"params,omitempty"`
}

// BuyerImportSummary aggregates diff counts for preview UI.
type BuyerImportSummary struct {
	Errors           int `json:"errors"`
	Warnings         int `json:"warnings"`
	SectionsAdded    int `json:"sections_added"`
	SectionsChanged  int `json:"sections_changed"`
	QuestionsAdded   int `json:"questions_added"`
	LotsAdded        int `json:"lots_added"`
	LotsChanged      int `json:"lots_changed"`
	LotsRemoved      int `json:"lots_removed"`
	QuestionsRemoved int `json:"questions_removed"`
	SectionsRemoved  int `json:"sections_removed"`
}

// BuyerImportProposal is the normalized graph parsed from workbook stable codes.
type BuyerImportProposal struct {
	Lots          []domain.RfxLot
	Questionnaire domain.QuestionnaireDefinition
}

// BuyerImportPreview is the pure parser output (P2 — no HTTP mapping, no DB writes).
type BuyerImportPreview struct {
	SchemaName            string
	SchemaVersion         string
	Mode                  string
	TargetEventID         uuid.UUID
	TargetDraftVersionID  uuid.UUID
	TargetVersionNumber   int
	TargetEventRowVersion int
	TargetDraftRowVersion int
	Proposal              BuyerImportProposal
	CanonicalPayloadHash  string
	ReadyToCommit         bool
	Summary               BuyerImportSummary
	Errors                []BuyerImportIssue
	Warnings              []BuyerImportIssue
	QuestionnaireDiff     domain.CompareVersionsResult
	LotsDiff              LotsCompareResult
}

// ParserOptions configures security limits and test hooks.
type ParserOptions struct {
	ContentType    string
	SecurityLimits xlsxsecurity.Limits
	ImportLimits   BuyerImportLimits
	// OpenWorkbook opens bytes after security PASS. Defaults to excelize.OpenReader.
	OpenWorkbook func(data []byte) (workbookReader, error)
}

type workbookReader interface {
	GetSheetList() []string
	GetSheetIndex(sheet string) (int, error)
	GetSheetVisible(sheet string) (excelizeSheetVisibility, bool, error)
	GetMergeCells(sheet string) ([]mergeCellRange, error)
	GetRows(sheet string) ([][]string, error)
	GetCellFormula(sheet, cell string) (string, error)
	GetCellValue(sheet, cell string) (string, error)
	GetRowVisible(sheet string, row int) (bool, error)
	GetColVisible(sheet, col string) (bool, error)
	Close() error
}

// LotsCompareResult is a stable-code diff between baseline and proposal lots.
type LotsCompareResult struct {
	Summary           domain.CompareSummary `json:"summary"`
	Differences       []LotCompareItemDiff  `json:"differences"`
	CanonicalDiffHash string                `json:"canonical_diff_hash"`
}

type LotCompareItemDiff struct {
	LotNumber  string                    `json:"lot_number"`
	Change     string                    `json:"change"`
	FieldDiffs []domain.CompareFieldDiff `json:"field_diffs,omitempty"`
}

type excelizeSheetVisibility int

const (
	sheetVisible excelizeSheetVisibility = iota
	sheetHidden
	sheetVeryHidden
)

type mergeCellRange struct {
	StartCell string
	EndCell   string
}
