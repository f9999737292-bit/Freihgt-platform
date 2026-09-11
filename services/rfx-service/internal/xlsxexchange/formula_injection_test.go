package xlsxexchange

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func TestGenerateBuyerDraftWorkbookFormulaInjectionCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
	}{
		{name: "equals_formula", value: "=SUM(1,2)"},
		{name: "plus_prefix", value: "+CMD"},
		{name: "minus_prefix", value: "-1+2"},
		{name: "at_prefix", value: "@IMPORT"},
		{name: "leading_space_equals", value: " =CMD"},
		{name: "leading_spaces_equals", value: "  =SUM(1,2)"},
		{name: "leading_tab_equals", value: "\t=SUM(1,2)"},
		{name: "leading_cr_equals", value: "\r=CMD"},
		{name: "leading_lf_equals", value: "\n=CMD"},
		{name: "leading_mixed_whitespace_at", value: " \t\r\n@CMD"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := generateWorkbookWithLotDescription(t, tc.value)
			assertFormulaSafeWorkbookCell(t, data, sheetLots, "C2", tc.value)
		})
	}

	t.Run("safe_plain_text", func(t *testing.T) {
		t.Parallel()
		value := "Lane bundle description"
		data := generateWorkbookWithLotDescription(t, value)
		assertFormulaSafeWorkbookCell(t, data, sheetLots, "C2", value)
	})

	t.Run("empty_string", func(t *testing.T) {
		t.Parallel()
		data := generateWorkbookWithLotDescription(t, "")
		assertFormulaSafeWorkbookCell(t, data, sheetLots, "C2", "")
	})

	t.Run("equals_not_at_significant_prefix", func(t *testing.T) {
		t.Parallel()
		value := "price=100"
		data := generateWorkbookWithLotDescription(t, value)
		assertFormulaSafeWorkbookCell(t, data, sheetLots, "C2", value)
	})
}

func generateWorkbookWithLotDescription(t *testing.T, description string) []byte {
	t.Helper()
	snapshot := richSnapshot()
	desc := description
	snapshot.Lots = []domain.RfxLot{{
		LotNumber: "L1", Name: "Lane bundle", Description: &desc, Status: "DRAFT",
	}}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return data
}

func assertFormulaSafeWorkbookCell(t *testing.T, data []byte, sheet, cell, wantValue string) {
	t.Helper()

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()

	got, err := f.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatalf("read cell %s!%s: %v", sheet, cell, err)
	}
	if got != wantValue {
		t.Fatalf("cell value: got %q want %q", got, wantValue)
	}
	if isFormulaLikeCellValue(wantValue) {
		formula, err := f.GetCellFormula(sheet, cell)
		if err != nil {
			t.Fatalf("read formula %s!%s: %v", sheet, cell, err)
		}
		if formula != "" {
			t.Fatalf("cell %s!%s must not be formula, got %q", sheet, cell, formula)
		}
	}
	if containsWorksheetToken(t, data, "<f>") || containsWorksheetToken(t, data, "<f ") {
		t.Fatalf("worksheet XML must not contain formula elements for %s!%s", sheet, cell)
	}
	if _, err := xlsxsecurity.InspectUpload(
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		data,
		xlsxsecurity.DefaultLimits(),
	); err != nil {
		t.Fatalf("security inspect rejected workbook: %v", err)
	}
}

func TestGenerateBuyerDraftWorkbookFormulaInjectionAcrossSheets(t *testing.T) {
	t.Parallel()

	payload := " \t=SUM(1,2)"
	snapshot := richSnapshot()
	snapshot.Sections = []domain.Section{{
		SectionCode: "SEC1", Title: payload, Description: &payload, SortOrder: 1,
	}}
	snapshot.Questions = []BuyerDraftQuestion{{
		SectionCode: "SEC1",
		Question: domain.Question{
			QuestionCode:       "Q1",
			QuestionType:       "TEXT",
			Label:              payload,
			HelpText:           &payload,
			ValidationRuleJSON: json.RawMessage(`{"prefix":"leading-space-formula"}`),
			SortOrder:          1,
		},
	}}
	snapshot.Options = []BuyerDraftOption{{
		QuestionCode: "Q1",
		Option:       domain.QuestionOption{OptionCode: "O1", Label: payload, SortOrder: 1},
	}}
	snapshot.Rules = []BuyerDraftRule{{
		RuleCode:           "R1",
		SourceQuestionCode: "Q1",
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q1","value":"yes"}`),
		TargetQuestionCode: "Q1",
		Action:             domain.RuleActionShow,
		SortOrder:          1,
	}}
	desc := payload
	snapshot.Lots = []domain.RfxLot{{
		LotNumber: payload, Name: payload, Description: &desc, Category: &payload, Status: "DRAFT",
	}}

	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	foundPayload := false
	for _, sheet := range buyerSheetOrder {
		rows, err := f.GetRows(sheet)
		if err != nil {
			t.Fatalf("read sheet %s: %v", sheet, err)
		}
		for rowIdx, row := range rows {
			for colIdx, value := range row {
				if value != payload {
					continue
				}
				foundPayload = true
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				formula, err := f.GetCellFormula(sheet, cell)
				if err != nil {
					t.Fatalf("read formula %s!%s: %v", sheet, cell, err)
				}
				if formula != "" {
					t.Fatalf("payload cell %s!%s must not be formula", sheet, cell)
				}
			}
		}
	}
	if !foundPayload {
		t.Fatalf("expected payload %q to be exported verbatim on at least one sheet", payload)
	}
	if containsWorksheetToken(t, data, "<f>") || containsWorksheetToken(t, data, "<f ") {
		t.Fatal("worksheet XML contains formula elements")
	}
	if _, err := xlsxsecurity.InspectUpload(
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		data,
		xlsxsecurity.DefaultLimits(),
	); err != nil {
		t.Fatalf("security inspect rejected workbook: %v", err)
	}
}
