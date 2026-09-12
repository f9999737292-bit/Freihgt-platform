//go:build integration

package excelexchange

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/service"
)

const (
	previewSheetMetadata  = "Metadata"
	previewSheetLots      = "Lots"
	previewSheetSections  = "Sections"
	previewSheetQuestions = "Questions"
	previewSheetOptions   = "Options"
	previewSheetRules     = "Rules"
)

func mutatePreviewWorkbook(t *testing.T, data []byte, fn func(*excelize.File)) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	fn(f)
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func setMetadataValue(t *testing.T, data []byte, key, value string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		rows, err := f.GetRows(previewSheetMetadata)
		if err != nil {
			t.Fatalf("read metadata: %v", err)
		}
		for i, row := range rows {
			if len(row) == 0 || strings.TrimSpace(row[0]) != key {
				continue
			}
			cell := fmt.Sprintf("B%d", i+1)
			if err := f.SetCellStr(previewSheetMetadata, cell, value); err != nil {
				t.Fatalf("set metadata %s: %v", key, err)
			}
			return
		}
		t.Fatalf("metadata key %q not found", key)
	})
}

func deletePreviewSheet(t *testing.T, data []byte, sheet string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.DeleteSheet(sheet); err != nil {
			t.Fatalf("delete sheet %s: %v", sheet, err)
		}
	})
}

func addUnexpectedPreviewSheet(t *testing.T, data []byte, sheet string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if _, err := f.NewSheet(sheet); err != nil {
			t.Fatalf("new sheet %s: %v", sheet, err)
		}
	})
}

func duplicatePreviewHeader(t *testing.T, data []byte, sheet string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.SetCellStr(sheet, "B1", "section_code"); err != nil {
			t.Fatalf("duplicate header: %v", err)
		}
	})
}

func workbookWithDuplicateSectionCode(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		_ = f.SetCellStr(previewSheetSections, "A3", "SEC1")
		_ = f.SetCellStr(previewSheetSections, "B3", "Dup")
		_ = f.SetCellStr(previewSheetSections, "H3", "2")
	})
}

func workbookWithDanglingQuestionSection(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		_ = f.SetCellStr(previewSheetQuestions, "A2", "MISSING")
	})
}

func workbookWithSelfTargetRule(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		_ = f.SetCellStr(previewSheetRules, "D2", "NOTES")
	})
}

func workbookWithMalformedValidationJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		_ = f.SetCellStr(previewSheetQuestions, "L2", "{not-json")
	})
}

func workbookWithFormulaCell(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.SetCellFormula(previewSheetSections, "A2", "=1+1"); err != nil {
			t.Fatalf("set formula: %v", err)
		}
	})
}

func workbookWithCompetitorColumn(t *testing.T, data []byte, columnName string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		headers, err := f.GetRows(previewSheetQuestions)
		if err != nil || len(headers) == 0 {
			t.Fatalf("read questions header: %v", err)
		}
		col := len(headers[0]) + 1
		cell, _ := excelize.CoordinatesToCellName(col, 1)
		if err := f.SetCellStr(previewSheetQuestions, cell, columnName); err != nil {
			t.Fatalf("set competitor header: %v", err)
		}
		valueCell, _ := excelize.CoordinatesToCellName(col, 2)
		_ = f.SetCellStr(previewSheetQuestions, valueCell, "SENTINEL-COMPETITOR-VALUE")
	})
}

func workbookWithMacroPackage(t *testing.T, base []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	reader, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("open base zip: %v", err)
	}
	for _, file := range reader.File {
		w, err := zw.Create(file.Name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry: %v", err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry: %v", err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	macroWriter, err := zw.Create("xl/vbaProject.bin")
	if err != nil {
		t.Fatalf("create macro entry: %v", err)
	}
	if _, err := macroWriter.Write([]byte("macro")); err != nil {
		t.Fatalf("write macro entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func assertPreviewFailureNoWrites(t *testing.T, env *testEnv, tenantID, eventID uuid.UUID, before graphWriteSnapshot) {
	t.Helper()
	after := captureGraphWriteSnapshot(t, env, tenantID, eventID)
	if before.eventVersion != after.eventVersion ||
		before.versionCount != after.versionCount ||
		before.sectionCount != after.sectionCount ||
		before.questionCount != after.questionCount ||
		before.optionCount != after.optionCount ||
		before.ruleCount != after.ruleCount ||
		before.lotCount != after.lotCount {
		t.Fatalf("preview failure mutated graph: before=%+v after=%+v", before, after)
	}
	if before.auditCount != after.auditCount || before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("preview failure created audit/idempotency writes")
	}
	if after.importAnalysisCnt != before.importAnalysisCnt {
		t.Fatalf("preview failure changed analysis count: %d -> %d", before.importAnalysisCnt, after.importAnalysisCnt)
	}
}

func assertResponseExcludesSentinels(t *testing.T, body []byte, sent competitorSentinels) {
	t.Helper()
	text := strings.ToLower(string(body))
	needles := []string{
		sent.ParticipantAID.String(), sent.ResponseAID.String(),
		sent.CarrierACompany.String(), sent.OfferRateA,
		sent.NameTokenA, sent.EmailTokenA, "SENTINEL-COMPETITOR-VALUE",
	}
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			t.Fatalf("response leaked sentinel %q body=%s", needle, string(body))
		}
	}
}

func previewEnvelopeComparable(p service.BuyerImportPreviewResponse) service.BuyerImportPreviewResponse {
	p.AnalysisID = nil
	p.ExpiresAt = nil
	return p
}

func assertPreviewIssueCode(t *testing.T, preview service.BuyerImportPreviewResponse, code string) {
	t.Helper()
	for _, issue := range preview.Errors {
		if issue.MachineCode == code {
			return
		}
	}
	for _, issue := range preview.Warnings {
		if issue.MachineCode == code {
			return
		}
	}
	encoded, _ := json.Marshal(preview)
	t.Fatalf("issue code %q not found in %s", code, string(encoded))
}

func assertPreviewWarningCode(t *testing.T, preview service.BuyerImportPreviewResponse, code string) {
	t.Helper()
	for _, issue := range preview.Warnings {
		if issue.MachineCode == code {
			return
		}
	}
	t.Fatalf("warning code %q not found", code)
}

func loadPersistedAnalysis(t *testing.T, env *testEnv, id, tenantID uuid.UUID) domain.ImportAnalysis {
	t.Helper()
	analysis, err := env.importAnalysisRepo.GetByID(context.Background(), id, tenantID)
	if err != nil {
		t.Fatalf("load analysis: %v", err)
	}
	return *analysis
}
