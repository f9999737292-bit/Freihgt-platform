package xlsxexchange

import (
	"bytes"
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

var forbiddenCreateIdentityKeys = []string{
	"tenant_id",
	"rfx_event_id",
	"rfx_version_id",
	"version_number",
	"version_status",
	"event_row_version",
	"version_row_version",
	"creation_channel",
	"source_template_version_id",
	"source_template_version_number",
	"exported_at_utc",
	"owner_company_id",
	"participant_id",
	"carrier_id",
}

var uuidLikeRE = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)

func TestGenerateBuyerCreateBlankWorkbookSheetOrderAndHeaders(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	if !reflect.DeepEqual(f.GetSheetList(), buyerSheetOrder) {
		t.Fatalf("sheet order: got %v want %v", f.GetSheetList(), buyerSheetOrder)
	}
	assertHeaderOnlySheet(t, f, sheetLots, lotsHeaders)
	assertHeaderOnlySheet(t, f, sheetSections, sectionsHeaders)
	assertHeaderOnlySheet(t, f, sheetQuestions, questionsHeaders)
	assertHeaderOnlySheet(t, f, sheetOptions, optionsHeaders)
	assertHeaderOnlySheet(t, f, sheetRules, rulesHeaders)
	for _, name := range buyerSheetOrder {
		visible, visErr := f.GetSheetVisible(name)
		if visErr != nil {
			t.Fatalf("visible %s: %v", name, visErr)
		}
		if !visible {
			t.Fatalf("sheet %s must not be hidden", name)
		}
	}
}

func TestGenerateBuyerCreateBlankWorkbookMetadataOnlySchema(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	meta := readMetadataMap(t, data)
	if len(meta) != 2 {
		t.Fatalf("metadata keys=%v want exactly schema_name and schema_version", meta)
	}
	if meta["schema_name"] != domain.SchemaVersionBuyerXLSXV1 {
		t.Fatalf("schema_name=%q", meta["schema_name"])
	}
	if meta["schema_version"] != schemaVersionNumber {
		t.Fatalf("schema_version=%q", meta["schema_version"])
	}
	for _, key := range forbiddenCreateIdentityKeys {
		if _, ok := meta[key]; ok {
			t.Fatalf("blank workbook must not write metadata key %q", key)
		}
	}
}

func TestGenerateBuyerCreateBlankWorkbookFilenameConstant(t *testing.T) {
	if BuyerCreateBlankWorkbookFilename != "bintrans-rfx-buyer-xlsx-v1-create-template.xlsx" {
		t.Fatalf("filename=%q", BuyerCreateBlankWorkbookFilename)
	}
}

func TestGenerateBuyerCreateBlankWorkbookCreateInstructions(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	text := strings.ToLower(strings.Join(flattenRows(readSheetRows(t, data, sheetInstructions)), " "))
	for _, needle := range []string{"create", "new", "draft", "upload"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("instructions must explain CREATE flow, missing %q", needle)
		}
	}
	if !strings.Contains(text, "does not publish") || !strings.Contains(text, "does not create participants") {
		t.Fatal("instructions must explicitly deny publish and participant creation")
	}
	for _, forbidden := range []string{"auto-publish", "autopublish", "auto publish", "automatically publish"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("instructions must not promise %q", forbidden)
		}
	}
}

func TestGenerateBuyerCreateBlankWorkbookHasNoForbiddenIdentity(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	canonical, err := CanonicalWorkbookSnapshot(data)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	for sheet, rows := range canonical.Sheets {
		for _, row := range rows {
			for _, cell := range row {
				lower := strings.ToLower(strings.TrimSpace(cell))
				for _, key := range forbiddenCreateIdentityKeys {
					if lower == key {
						t.Fatalf("sheet %s contains forbidden identity key %q", sheet, key)
					}
				}
				if uuidLikeRE.MatchString(cell) {
					t.Fatalf("sheet %s contains UUID-like identity %q", sheet, cell)
				}
			}
		}
	}
}

func TestGenerateBuyerCreateBlankWorkbookParserParity(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.Mode != BuyerImportModeCreateNewDraft {
		t.Fatalf("mode=%q want %s", preview.Mode, BuyerImportModeCreateNewDraft)
	}
	if preview.SchemaName != domain.SchemaVersionBuyerXLSXV1 || preview.SchemaVersion != schemaVersionNumber {
		t.Fatalf("schema=%s/%s", preview.SchemaName, preview.SchemaVersion)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("blank CREATE workbook must be ready, errors=%v warnings=%v", preview.Errors, preview.Warnings)
	}
	if len(preview.Errors) != 0 {
		t.Fatalf("errors=%v", preview.Errors)
	}
	for _, warning := range preview.Warnings {
		if warning.MachineCode == MachineCodeMetadataMismatch {
			t.Fatalf("blank workbook must not emit identity warnings: %+v", preview.Warnings)
		}
	}
	if preview.ChangeCounts.Lots.Added != 0 || preview.ChangeCounts.Sections.Added != 0 ||
		preview.ChangeCounts.Questions.Added != 0 || preview.ChangeCounts.Options.Added != 0 ||
		preview.ChangeCounts.Rules.Added != 0 {
		t.Fatalf("optional graph must stay empty: %+v", preview.ChangeCounts)
	}
}

func TestGenerateBuyerCreateBlankWorkbookFormulaAndPackageSafe(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	for _, sheet := range buyerSheetOrder {
		rows, rowErr := f.GetRows(sheet)
		if rowErr != nil {
			t.Fatalf("rows %s: %v", sheet, rowErr)
		}
		for r := 1; r <= len(rows)+2; r++ {
			for c := 1; c <= 16; c++ {
				cell, _ := excelize.CoordinatesToCellName(c, r)
				formula, _ := f.GetCellFormula(sheet, cell)
				if formula != "" {
					t.Fatalf("sheet %s cell %s has formula %q", sheet, cell, formula)
				}
			}
		}
	}
	if containsWorksheetToken(t, data, "<f>") || containsWorksheetToken(t, data, "<f ") {
		t.Fatal("workbook contains formula elements")
	}
	if _, err := xlsxsecurity.InspectUpload(
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		data,
		xlsxsecurity.DefaultLimits(),
	); err != nil {
		t.Fatalf("security inspect: %v", err)
	}
}

func TestGenerateBuyerCreateBlankWorkbookDoesNotUseDraftSnapshotExport(t *testing.T) {
	data, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	meta := readMetadataMap(t, data)
	if _, ok := meta["rfx_event_id"]; ok {
		t.Fatal("blank generator must not reuse draft metadata writer")
	}
}

func TestGenerateBuyerCreateBlankWorkbookSemanticRepeatable(t *testing.T) {
	first, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := GenerateBuyerCreateBlankWorkbook()
	if err != nil {
		t.Fatalf("second: %v", err)
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
		t.Fatal("repeated blank generation must be semantically equivalent")
	}
}

func assertHeaderOnlySheet(t *testing.T, f *excelize.File, sheet string, want []string) {
	t.Helper()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("read %s: %v", sheet, err)
	}
	if len(rows) != 1 {
		t.Fatalf("%s must be header-only, rows=%d", sheet, len(rows))
	}
	if !reflect.DeepEqual(rows[0], want) {
		t.Fatalf("%s headers=%v want %v", sheet, rows[0], want)
	}
}

func flattenRows(rows [][]string) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row...)
	}
	return out
}
