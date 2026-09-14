package xlsxexchange

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func minimalCarrierSnapshot() CarrierResponseSnapshot {
	tenantID := uuid.New()
	eventID := uuid.New()
	responseID := uuid.New()
	versionID := uuid.New()
	carrierID := uuid.New()
	return CarrierResponseSnapshot{
		Metadata: CarrierResponseMetadata{
			ExportedAtUTC:              time.Now().UTC(),
			TenantID:                   tenantID,
			RfxEventID:                 eventID,
			RfxResponseID:              responseID,
			CarrierCompanyID:           carrierID,
			RfxVersionID:               versionID,
			QuestionnaireVersionNumber: 1,
			ResponseSaveVersion:        1,
			ResponseStatus:             domain.RfxResponseStatusDraft,
			EventRowVersion:            1,
			AvailableLotsFingerprint:   "abc",
			AnswersFingerprint:         "def",
			OfferLinesFingerprint:      "ghi",
			ExportMode:                 CarrierExportModeDraftEdit,
		},
	}
}

func richCarrierSnapshot() CarrierResponseSnapshot {
	snapshot := minimalCarrierSnapshot()
	snapshot.Lots = []domain.RfxLot{{LotNumber: "L1", Name: "Lane", Status: "ACTIVE"}}
	snapshot.Questions = []CarrierQuestion{{
		SectionCode: "SEC1",
		Question: domain.Question{
			QuestionCode: "Q1", QuestionType: "TEXT", Label: "Question", Required: true, SortOrder: 1,
		},
	}}
	snapshot.Options = []CarrierOption{{
		QuestionCode: "Q1",
		Option:       domain.QuestionOption{OptionCode: "O1", Label: "Option", SortOrder: 1},
	}}
	snapshot.Rules = []CarrierRule{{
		RuleCode: "R1", SourceQuestionCode: "Q1",
		ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q1","value":"yes"}`),
		TargetQuestionCode: "Q1", Action: domain.RuleActionShow, SortOrder: 1,
	}}
	snapshot.Answers = []CarrierAnswerRow{{QuestionCode: "Q1", AnswerValue: "YES"}}
	snapshot.OfferLines = []CarrierOfferLineRow{{LotNumber: "L1", Amount: "100", CurrencyCode: "RUB", Comment: "note"}}
	return snapshot
}

func TestGenerateCarrierResponseWorkbookSheetOrder(t *testing.T) {
	data, err := GenerateCarrierResponseWorkbook(minimalCarrierSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if !reflect.DeepEqual(f.GetSheetList(), carrierSheetOrder) {
		t.Fatalf("sheet order: got %v want %v", f.GetSheetList(), carrierSheetOrder)
	}
}

func TestGenerateCarrierResponseWorkbookMetadataSchema(t *testing.T) {
	data, err := GenerateCarrierResponseWorkbook(minimalCarrierSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	meta := readMetadataMap(t, data)
	if meta["schema_name"] != domain.SchemaVersionCarrierXLSXV1 {
		t.Fatalf("schema_name=%q", meta["schema_name"])
	}
	if meta["export_mode"] != CarrierExportModeDraftEdit {
		t.Fatalf("export_mode=%q", meta["export_mode"])
	}
}

func TestGenerateCarrierResponseWorkbookSubmittedReadonlyBanner(t *testing.T) {
	snapshot := minimalCarrierSnapshot()
	snapshot.Metadata.ExportMode = CarrierExportModeSubmittedReadonly
	snapshot.Metadata.ResponseStatus = domain.RfxResponseStatusSubmitted
	data, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	rows := readSheetRows(t, data, sheetInstructions)
	found := false
	for _, row := range rows {
		for _, cell := range row {
			if cell == submittedReadonlyBannerRows[0][1] {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected SUBMITTED_READONLY banner in Instructions")
	}
}

func TestGenerateCarrierResponseWorkbookZeroLotOfferLine(t *testing.T) {
	snapshot := minimalCarrierSnapshot()
	snapshot.Lots = nil
	snapshot.OfferLines = []CarrierOfferLineRow{{LotNumber: "", Amount: "500", CurrencyCode: "RUB"}}
	data, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	lotNumbers := readColumnValues(t, data, sheetOfferLines, "lot_number")
	if len(lotNumbers) != 1 || lotNumbers[0] != "" {
		t.Fatalf("zero-lot offer line: got %v", lotNumbers)
	}
}

func TestGenerateCarrierResponseWorkbookFormulaInjectionCases(t *testing.T) {
	t.Parallel()
	value := "=SUM(1,2)"
	snapshot := richCarrierSnapshot()
	snapshot.Answers = []CarrierAnswerRow{{QuestionCode: "Q1", AnswerValue: value}}
	data, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	assertFormulaSafeWorkbookCell(t, data, sheetAnswers, "B2", value)
}

func TestGenerateCarrierResponseWorkbookDeterministicExcludingExportedAt(t *testing.T) {
	snapshot := richCarrierSnapshot()
	first, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	snapshot.Metadata.ExportedAtUTC = snapshot.Metadata.ExportedAtUTC.Add(time.Hour)
	second, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	canonicalFirst, err := CanonicalCarrierWorkbookSnapshot(first)
	if err != nil {
		t.Fatalf("canonical first: %v", err)
	}
	canonicalSecond, err := CanonicalCarrierWorkbookSnapshot(second)
	if err != nil {
		t.Fatalf("canonical second: %v", err)
	}
	if !reflect.DeepEqual(canonicalFirst, canonicalSecond) {
		t.Fatal("semantic snapshots differ after excluding exported_at_utc")
	}
}

func TestGenerateCarrierResponseWorkbookLotsExcludeEstimatedValue(t *testing.T) {
	snapshot := richCarrierSnapshot()
	value := 999.0
	snapshot.Lots[0].EstimatedValue = &value
	data, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	headers := readSheetRows(t, data, sheetLots)[0]
	for _, header := range headers {
		if header == "estimated_value" {
			t.Fatal("carrier lots sheet must not include estimated_value")
		}
	}
}
