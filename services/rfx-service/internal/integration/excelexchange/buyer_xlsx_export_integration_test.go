//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT06BuyerManageRichDraftExport200(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, service.BuyerDraftXLSXContentType) {
		t.Fatalf("content-type=%q", ct)
	}
	data := rec.Body.Bytes()
	if len(data) == 0 {
		t.Fatal("expected non-empty workbook")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("excelize reopen: %v", err)
	}
	defer f.Close()

	assertWorkbookGraphParity(t, f, draft)
}

func TestE7P2INT07NoActiveDraft409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	ctx := context.Background()

	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_events SET draft_version_id = NULL WHERE id = $1`, draft.Event.ID); err != nil {
		t.Fatalf("clear draft: %v", err)
	}

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	_, _, err := env.excelExchangeSvc.ExportBuyerDraftWorkbook(ctx, fix.BuyerA, draft.Event.ID)
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE7P2INT08CrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CrossTenant, draft.Event.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT09CrossCompanyFailClosed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerB, draft.Event.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT10Carrier403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, draft.Event.ID)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT11Unauthenticated401(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), domain.ActorContext{}, draft.Event.ID)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT12FeatureDisabled404NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := getBuyerXlsxExportHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("feature disabled route must not mutate persisted state")
	}
}

func TestE7P2INT13UnknownEvent404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_ = seedRichDraftEvent(t, env, fix)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, uuid.New())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT14DeterministicRepeatedExport(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	first := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	second := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status first=%d second=%d", first.Code, second.Code)
	}

	canonicalFirst, err := xlsxexchange.CanonicalWorkbookSnapshot(first.Body.Bytes())
	if err != nil {
		t.Fatalf("canonical first: %v", err)
	}
	canonicalSecond, err := xlsxexchange.CanonicalWorkbookSnapshot(second.Body.Bytes())
	if err != nil {
		t.Fatalf("canonical second: %v", err)
	}
	if !reflect.DeepEqual(canonicalFirst, canonicalSecond) {
		t.Fatal("repeated export must be semantically identical excluding exported_at_utc")
	}
}

func TestE7P2INT15EventVersionGraphUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("export mutated graph: before=%+v after=%+v", before, after)
	}
}

func TestE7P2INT16ImportAnalysisCountUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if before.importAnalysisCnt != after.importAnalysisCnt {
		t.Fatalf("import analysis count changed: %d -> %d", before.importAnalysisCnt, after.importAnalysisCnt)
	}
}

func TestE7P2INT17IdempotencyUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("idempotency count changed: %d -> %d", before.idempotencyCount, after.idempotencyCount)
	}
}

func TestE7P2INT18NoCompetitorDataInWorkbook(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	sent := seedCompetitorSentinels(t, env, fix, draft)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertWorkbookExcludesCompetitorSentinels(t, rec.Body.Bytes(), sent, draft)

	for _, sheet := range []string{"Participants", "Responses", "Bids", "CarrierResponses"} {
		f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("open workbook: %v", err)
		}
		if idx, _ := f.GetSheetIndex(sheet); idx >= 0 {
			_ = f.Close()
			t.Fatalf("unexpected competitor sheet %q", sheet)
		}
		_ = f.Close()
	}
}

func TestE7P2INT19RouteParitySmoke(t *testing.T) {
	const canonicalOperationID = "get_export_buyer_draft_rfx_event_as_xlsx_workbook"

	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	route := routes[0]
	if route.Method != http.MethodGet || route.SuccessStatus != http.StatusOK {
		t.Fatalf("unexpected route contract: %+v", route)
	}
	if route.OpenAPIOperationID != canonicalOperationID {
		t.Fatalf("manifest operationId mismatch: got=%q want=%q method=%s path=%s gateway=%s service=%s",
			route.OpenAPIOperationID, canonicalOperationID, route.Method, route.OpenAPIPath, route.GatewayPath, route.ServicePath)
	}
	if route.GatewayPath != "/api/v1/rfx-events/{id}/xlsx-export" {
		t.Fatalf("gateway path mismatch: got=%q want=%q operationId=%s",
			route.GatewayPath, "/api/v1/rfx-events/{id}/xlsx-export", route.OpenAPIOperationID)
	}

	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
	if rec.Code != route.SuccessStatus {
		t.Fatalf("service path smoke status=%d want=%d method=%s path=%s operationId=%s",
			rec.Code, route.SuccessStatus, route.Method, route.ServicePath, route.OpenAPIOperationID)
	}
}

func TestE7P2INT20BuyerReadOnlyHTTP403Forbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerRead, draft.Event.ID)
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("forbidden response must not return XLSX content-type: %q", ct)
	}
	body := rec.Body.Bytes()
	if len(body) >= 2 && body[0] == 'P' && body[1] == 'K' {
		t.Fatal("forbidden response must not contain XLSX bytes")
	}

	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("buyer read-only denial must not mutate persisted graph or side-effect tables")
	}
}

func assertWorkbookGraphParity(t *testing.T, f *excelize.File, draft richDraftFixture) {
	t.Helper()
	metaRows, err := f.GetRows("Metadata")
	if err != nil {
		t.Fatalf("metadata sheet: %v", err)
	}
	meta := map[string]string{}
	for _, row := range metaRows {
		if len(row) >= 2 {
			meta[row[0]] = row[1]
		}
	}
	if meta["schema_name"] != domain.SchemaVersionBuyerXLSXV1 {
		t.Fatalf("schema_name=%q", meta["schema_name"])
	}
	if meta["rfx_event_id"] != draft.Event.ID.String() {
		t.Fatalf("event id mismatch")
	}
	if meta["version_status"] != domain.RfxVersionStatusDraft {
		t.Fatalf("version_status=%q", meta["version_status"])
	}

	assertSheetDataRows(t, f, "Sections", 1)
	assertSheetDataRows(t, f, "Questions", 2)
	assertSheetDataRows(t, f, "Options", 1)
	assertSheetDataRows(t, f, "Rules", 1)
	assertSheetDataRows(t, f, "Lots", 1)
}

func assertSheetDataRows(t *testing.T, f *excelize.File, sheet string, want int) {
	t.Helper()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("read %s: %v", sheet, err)
	}
	if len(rows) != want+1 {
		t.Fatalf("%s rows=%d want header+%d data", sheet, len(rows), want)
	}
}
