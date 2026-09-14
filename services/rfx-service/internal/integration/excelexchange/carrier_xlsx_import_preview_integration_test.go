//go:build integration

package excelexchange

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func TestE7P2INT80CarrierPreviewValid200AnalysisPersisted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := exportCarrierDraftWorkbook(t, env, fix, carrier)
	before := countImportAnalyses(t, env, fix.TenantID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodeCarrierPreviewResponse(t, rec)
	if !preview.ReadyToCommit || preview.AnalysisID == nil || preview.ExpiresAt == nil {
		t.Fatalf("ready=%v analysis=%v", preview.ReadyToCommit, preview.AnalysisID)
	}
	if countImportAnalyses(t, env, fix.TenantID) != before+1 {
		t.Fatal("expected one analysis row")
	}
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if row.WorkbookType != domain.WorkbookTypeCarrierOffer || row.TargetType != domain.ImportTargetTypeCarrierResponse {
		t.Fatalf("workbook_type=%q target_type=%q", row.WorkbookType, row.TargetType)
	}
	if err := xlsxexchange.VerifyStoredCarrierCanonicalPayloadHash(row.CanonicalPayloadJSON, row.CanonicalHash); err != nil {
		t.Fatalf("hash verify: %v", err)
	}
}

func TestE7P2INT81InvalidValidation422NoAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := workbookWithNegativeOfferAmount(t, exportCarrierDraftWorkbook(t, env, fix, carrier))
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodeCarrierPreviewResponse(t, rec)
	if preview.AnalysisID != nil || preview.ReadyToCommit {
		t.Fatal("invalid preview must not persist analysis")
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT82MalformedXlsx400(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, []byte("not-a-xlsx"), previewHTTPOptions{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT83OversizedUpload413(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	oversized := bytes.Repeat([]byte("A"), int(xlsxsecurity.DefaultMaxUploadBytes)+1)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, oversized, previewHTTPOptions{})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT84SecurityRejectsUnsafePackage(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	base := exportCarrierDraftWorkbook(t, env, fix, carrier)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbookWithMacroPackage(t, base), previewHTTPOptions{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT85HiddenOrUnknownSheet422Or400(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	base := exportCarrierDraftWorkbook(t, env, fix, carrier)

	t.Run("missing_sheet", func(t *testing.T) {
		before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
		rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, deletePreviewSheet(t, base, "Rules"), previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
	})
	t.Run("unexpected_sheet", func(t *testing.T) {
		before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
		rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, addUnexpectedPreviewSheet(t, base, "CompetitorBids"), previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
	})
}

func TestE7P2INT86CompetitorColumnHeader422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := workbookWithCompetitorColumn(t, exportCarrierDraftWorkbook(t, env, fix, carrier), "competitor_rate")
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT87MetadataSchemaMismatch422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := setMetadataValue(t, exportCarrierDraftWorkbook(t, env, fix, carrier), "schema_name", "WRONG_SCHEMA")
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT88StaleResponseSaveVersion409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := setMetadataValue(t, exportCarrierDraftWorkbook(t, env, fix, carrier), "response_save_version", "0")
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func TestE7P2INT89PreviewDoesNotWriteAnswers(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := mutatePreviewWorkbook(t, exportCarrierDraftWorkbook(t, env, fix, carrier), func(f *excelize.File) {
		_ = f.SetCellStr("Answers", "B2", "CHANGED-ANSWER")
	})
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if before.answerCount != after.answerCount || before.saveVersion != after.saveVersion {
		t.Fatalf("preview mutated answers: before=%+v after=%+v", before, after)
	}
}

func TestE7P2INT90PreviewDoesNotWriteOfferLines(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	workbook := mutatePreviewWorkbook(t, exportCarrierDraftWorkbook(t, env, fix, carrier), func(f *excelize.File) {
		_ = f.SetCellStr("OfferLines", "B2", "99999.99")
	})
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if before.offerLineCount != after.offerLineCount || before.saveVersion != after.saveVersion {
		t.Fatalf("preview mutated offer lines: before=%+v after=%+v", before, after)
	}
}

func workbookWithNegativeOfferAmount(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.SetCellStr("OfferLines", "B2", "-1"); err != nil {
			t.Fatalf("set negative amount: %v", err)
		}
	})
}

func assertCarrierPreviewFailureNoWrites(t *testing.T, env *testEnv, tenantID, responseID uuid.UUID, before responseWriteSnapshot) {
	t.Helper()
	after := captureResponseWriteSnapshot(t, env, tenantID, responseID)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("preview failure mutated response state: before=%+v after=%+v", before, after)
	}
}
