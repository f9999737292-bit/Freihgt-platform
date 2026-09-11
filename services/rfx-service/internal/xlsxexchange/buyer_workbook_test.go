package xlsxexchange

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func TestGenerateBuyerDraftWorkbookSheetOrder(t *testing.T) {
	data, err := GenerateBuyerDraftWorkbook(minimalSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if !reflect.DeepEqual(sheets, buyerSheetOrder) {
		t.Fatalf("sheet order: got %v want %v", sheets, buyerSheetOrder)
	}
}

func TestGenerateBuyerDraftWorkbookMetadataSchema(t *testing.T) {
	snapshot := minimalSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	meta := readMetadataMap(t, data)
	if meta["schema_name"] != domain.SchemaVersionBuyerXLSXV1 {
		t.Fatalf("schema_name: got %q", meta["schema_name"])
	}
	if meta["schema_version"] != schemaVersionNumber {
		t.Fatalf("schema_version: got %q", meta["schema_version"])
	}
	if meta["version_status"] != domain.RfxVersionStatusDraft {
		t.Fatalf("version_status: got %q", meta["version_status"])
	}
}

func TestGenerateBuyerDraftWorkbookRichGraphExport(t *testing.T) {
	snapshot := richSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	assertRowCount(t, f, sheetSections, 1+len(snapshot.Sections))
	assertRowCount(t, f, sheetQuestions, 1+len(snapshot.Questions))
	assertRowCount(t, f, sheetOptions, 1+len(snapshot.Options))
	assertRowCount(t, f, sheetRules, 1+len(snapshot.Rules))
	assertRowCount(t, f, sheetLots, 1+len(snapshot.Lots))
}

func TestGenerateBuyerDraftWorkbookSectionsSortedDeterministically(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Sections = []domain.Section{
		{SectionCode: "B", Title: "Beta", SortOrder: 2},
		{SectionCode: "A", Title: "Alpha", SortOrder: 1},
		{SectionCode: "C", Title: "Charlie", SortOrder: 1},
	}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	codes := readColumnValues(t, data, sheetSections, "section_code")
	want := []string{"A", "C", "B"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("section order: got %v want %v", codes, want)
	}
}

func TestGenerateBuyerDraftWorkbookQuestionsSortedDeterministically(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Questions = []BuyerDraftQuestion{
		{SectionCode: "S1", Question: domain.Question{QuestionCode: "Q2", QuestionType: "TEXT", Label: "Two", SortOrder: 2}},
		{SectionCode: "S1", Question: domain.Question{QuestionCode: "Q1", QuestionType: "TEXT", Label: "One", SortOrder: 1}},
		{SectionCode: "S1", Question: domain.Question{QuestionCode: "Q0", QuestionType: "TEXT", Label: "Zero", SortOrder: 1}},
	}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	codes := readColumnValues(t, data, sheetQuestions, "question_code")
	want := []string{"Q0", "Q1", "Q2"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("question order: got %v want %v", codes, want)
	}
}

func TestGenerateBuyerDraftWorkbookOptionsSortedDeterministically(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Options = []BuyerDraftOption{
		{QuestionCode: "Q1", Option: domain.QuestionOption{OptionCode: "O2", Label: "Two", SortOrder: 2}},
		{QuestionCode: "Q1", Option: domain.QuestionOption{OptionCode: "O1", Label: "One", SortOrder: 1}},
		{QuestionCode: "Q2", Option: domain.QuestionOption{OptionCode: "O9", Label: "Nine", SortOrder: 1}},
	}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	codes := readColumnValues(t, data, sheetOptions, "option_code")
	want := []string{"O1", "O9", "O2"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("option order: got %v want %v", codes, want)
	}
}

func TestGenerateBuyerDraftWorkbookRulesPreserveSemantics(t *testing.T) {
	condition := json.RawMessage(`{"operator":"EQUALS","source_question_code":"SRC","value":"yes"}`)
	snapshot := richSnapshot()
	snapshot.Rules = []BuyerDraftRule{{
		RuleCode:           "RULE_SHOW",
		SourceQuestionCode: "SRC",
		ConditionJSON:      condition,
		TargetQuestionCode: "TGT",
		Action:             domain.RuleActionShow,
		SortOrder:          1,
	}}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	rows := readSheetRows(t, data, sheetRules)
	if len(rows) < 2 {
		t.Fatal("expected rule row")
	}
	row := mapRow(rows[0], rows[1])
	if row["rule_code"] != "RULE_SHOW" || row["source_question_code"] != "SRC" || row["target_question_code"] != "TGT" || row["action"] != domain.RuleActionShow {
		t.Fatalf("rule mapping: %#v", row)
	}
	if row["condition"] != `{"operator":"EQUALS","source_question_code":"SRC","value":"yes"}` {
		t.Fatalf("condition: got %q", row["condition"])
	}
}

func TestGenerateBuyerDraftWorkbookI18nTriplicated(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Sections = []domain.Section{{SectionCode: "SEC", Title: "Заголовок", SortOrder: 1}}
	desc := "Подсказка"
	snapshot.Questions = []BuyerDraftQuestion{{
		SectionCode: "SEC",
		Question: domain.Question{
			QuestionCode: "Q1", QuestionType: "TEXT", Label: "Метка", HelpText: &desc, SortOrder: 1,
		},
	}}
	snapshot.Options = []BuyerDraftOption{{
		QuestionCode: "Q1",
		Option:       domain.QuestionOption{OptionCode: "O1", Label: "Вариант", SortOrder: 1},
	}}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	sectionRow := mapRow(readSheetRows(t, data, sheetSections)[0], readSheetRows(t, data, sheetSections)[1])
	for _, col := range []string{"title_ru", "title_en", "title_zh"} {
		if sectionRow[col] != "Заголовок" {
			t.Fatalf("%s: got %q", col, sectionRow[col])
		}
	}
	questionRow := mapRow(readSheetRows(t, data, sheetQuestions)[0], readSheetRows(t, data, sheetQuestions)[1])
	for _, col := range []string{"title_ru", "title_en", "title_zh"} {
		if questionRow[col] != "Метка" {
			t.Fatalf("%s: got %q", col, questionRow[col])
		}
	}
	for _, col := range []string{"description_ru", "description_en", "description_zh"} {
		if questionRow[col] != "Подсказка" {
			t.Fatalf("%s: got %q", col, questionRow[col])
		}
	}
	optionRow := mapRow(readSheetRows(t, data, sheetOptions)[0], readSheetRows(t, data, sheetOptions)[1])
	for _, col := range []string{"label_ru", "label_en", "label_zh"} {
		if optionRow[col] != "Вариант" {
			t.Fatalf("%s: got %q", col, optionRow[col])
		}
	}
}

func TestGenerateBuyerDraftWorkbookValidationJSONCanonical(t *testing.T) {
	raw := json.RawMessage(`{"max_value":10,"min_value":1}`)
	snapshot := richSnapshot()
	snapshot.Questions = []BuyerDraftQuestion{{
		SectionCode: "SEC",
		Question: domain.Question{
			QuestionCode: "Q1", QuestionType: "NUMBER", Label: "N", ValidationRuleJSON: raw, SortOrder: 1,
		},
	}}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	row := mapRow(readSheetRows(t, data, sheetQuestions)[0], readSheetRows(t, data, sheetQuestions)[1])
	want := `{"max_value":10,"min_value":1}`
	if row["validation_json"] != want {
		t.Fatalf("validation_json: got %q want %q", row["validation_json"], want)
	}
}

func TestGenerateBuyerDraftWorkbookFormulaLikeValuesAsText(t *testing.T) {
	snapshot := richSnapshot()
	value := "=SUM(1,2)"
	snapshot.Lots = []domain.RfxLot{{LotNumber: "+001", Name: "@alias", Description: &value, Status: "DRAFT"}}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	lotNumber, _ := f.GetCellValue(sheetLots, "A2")
	name, _ := f.GetCellValue(sheetLots, "B2")
	description, _ := f.GetCellValue(sheetLots, "C2")
	if lotNumber != "+001" || name != "@alias" || description != value {
		t.Fatalf("original values must be preserved: lot=%q name=%q description=%q", lotNumber, name, description)
	}
	for _, cell := range []string{"A2", "B2", "C2"} {
		formula, _ := f.GetCellFormula(sheetLots, cell)
		if formula != "" {
			t.Fatalf("cell %s must not be formula", cell)
		}
	}
}

func TestGenerateBuyerDraftWorkbookNoFormulaElements(t *testing.T) {
	data, err := GenerateBuyerDraftWorkbook(richSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if containsWorksheetToken(t, data, "<f>") || containsWorksheetToken(t, data, "<f ") {
		t.Fatal("workbook contains formula elements")
	}
}

func TestGenerateBuyerDraftWorkbookNoForbiddenPackageContent(t *testing.T) {
	data, err := GenerateBuyerDraftWorkbook(richSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, err = xlsxsecurity.InspectUpload(
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		data,
		xlsxsecurity.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("generated workbook rejected by security inspect: %v", err)
	}
}

func TestGenerateBuyerDraftWorkbookSizeLimited(t *testing.T) {
	limits := xlsxsecurity.Limits{MaxUploadBytes: 1024}
	snapshot := richSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if int64(len(data)) <= limits.MaxUploadBytes {
		t.Fatalf("expected generated workbook to exceed test limit for assertion")
	}
	_, err = xlsxsecurity.InspectUpload(
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		data,
		limits,
	)
	if err == nil {
		t.Fatal("expected size limit rejection")
	}
}

func TestGenerateBuyerDraftWorkbookSemanticSnapshotDeterministic(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Metadata.ExportedAtUTC = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	first, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	snapshot.Metadata.ExportedAtUTC = time.Date(2026, 9, 11, 13, 0, 0, 0, time.UTC)
	second, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	canonicalFirst, err := CanonicalWorkbookSnapshot(first)
	if err != nil {
		t.Fatalf("canonical first: %v", err)
	}
	canonicalSecond, err := CanonicalWorkbookSnapshot(second)
	if err != nil {
		t.Fatalf("canonical second: %v", err)
	}
	if !reflect.DeepEqual(canonicalFirst, canonicalSecond) {
		t.Fatal("semantic snapshots differ after excluding exported_at_utc")
	}
}

func TestDependencyAnchorRemoved(t *testing.T) {
	path := filepath.Join("..", "xlsxsecurity", "excelize_dependency.go")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("excelize dependency anchor still present at %s", path)
	}
}

func TestGenerateBuyerDraftWorkbookNoDiskWrites(t *testing.T) {
	src, err := os.ReadFile("buyer_workbook.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	body := string(src)
	for _, forbidden := range []string{"os.CreateTemp", "ioutil.TempFile", "os.WriteFile", "os.OpenFile"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unexpected disk write pattern %q in buyer_workbook.go", forbidden)
		}
	}
	data, err := GenerateBuyerDraftWorkbook(minimalSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected in-memory workbook bytes")
	}
}

func TestExtractSourceQuestionCode(t *testing.T) {
	raw := json.RawMessage(`{"operator":"EQUALS","source_question_code":"SRC","value":"x"}`)
	if got := ExtractSourceQuestionCode(raw); got != "SRC" {
		t.Fatalf("got %q want SRC", got)
	}
}

func minimalSnapshot() BuyerDraftSnapshot {
	tenantID := uuid.New()
	eventID := uuid.New()
	versionID := uuid.New()
	return BuyerDraftSnapshot{
		Metadata: BuyerDraftMetadata{
			ExportedAtUTC:     time.Now().UTC(),
			TenantID:          tenantID,
			RfxEventID:        eventID,
			RfxVersionID:      versionID,
			VersionNumber:     1,
			VersionStatus:     domain.RfxVersionStatusDraft,
			EventRowVersion:   1,
			VersionRowVersion: 1,
			CreationChannel:   domain.CreationChannelManual,
		},
	}
}

func richSnapshot() BuyerDraftSnapshot {
	snapshot := minimalSnapshot()
	desc := "Lot description"
	category := "FREIGHT"
	value := 1000.0
	currency := "RUB"
	snapshot.Lots = []domain.RfxLot{{
		LotNumber: "L1", Name: "Lane bundle", Description: &desc, Category: &category,
		EstimatedValue: &value, CurrencyCode: &currency, Status: "DRAFT",
	}}
	snapshot.Sections = []domain.Section{{SectionCode: "SEC1", Title: "Section", SortOrder: 1}}
	help := "Help"
	snapshot.Questions = []BuyerDraftQuestion{{
		SectionCode: "SEC1",
		Question: domain.Question{
			QuestionCode: "Q1", QuestionType: "TEXT", Label: "Question", HelpText: &help, Required: true, SortOrder: 1,
		},
	}}
	snapshot.Options = []BuyerDraftOption{{
		QuestionCode: "Q1",
		Option:       domain.QuestionOption{OptionCode: "O1", Label: "Option", SortOrder: 1},
	}}
	snapshot.Rules = []BuyerDraftRule{{
		RuleCode: "R1", SourceQuestionCode: "Q1",
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q1","value":"yes"}`),
		TargetQuestionCode: "Q1", Action: domain.RuleActionShow, SortOrder: 1,
	}}
	return snapshot
}

func readMetadataMap(t *testing.T, data []byte) map[string]string {
	t.Helper()
	rows := readSheetRows(t, data, sheetMetadata)
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		out[row[0]] = row[1]
	}
	return out
}

func readSheetRows(t *testing.T, data []byte, sheet string) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("read sheet %s: %v", sheet, err)
	}
	return rows
}

func readColumnValues(t *testing.T, data []byte, sheet, column string) []string {
	t.Helper()
	rows := readSheetRows(t, data, sheet)
	if len(rows) == 0 {
		t.Fatalf("empty sheet %s", sheet)
	}
	colIdx := -1
	for idx, header := range rows[0] {
		if header == column {
			colIdx = idx
			break
		}
	}
	if colIdx < 0 {
		t.Fatalf("column %s not found in %s", column, sheet)
	}
	out := make([]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if colIdx >= len(row) {
			out = append(out, "")
			continue
		}
		out = append(out, row[colIdx])
	}
	return out
}

func mapRow(headers []string, values []string) map[string]string {
	out := make(map[string]string, len(headers))
	for idx, header := range headers {
		if idx < len(values) {
			out[header] = values[idx]
		}
	}
	return out
}

func assertRowCount(t *testing.T, f *excelize.File, sheet string, want int) {
	t.Helper()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("read %s: %v", sheet, err)
	}
	if len(rows) != want {
		t.Fatalf("%s row count: got %d want %d", sheet, len(rows), want)
	}
}

func containsWorksheetToken(t *testing.T, data []byte, token string) bool {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "xl/worksheets/") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open entry: %v", err)
		}
		body, err := io.ReadAll(io.LimitReader(rc, 1<<20))
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry: %v", err)
		}
		if strings.Contains(string(body), token) {
			return true
		}
		decoder := xml.NewDecoder(bytes.NewReader(body))
		for {
			tok, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("decode xml: %v", err)
			}
			if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "f" {
				return true
			}
		}
	}
	return false
}
