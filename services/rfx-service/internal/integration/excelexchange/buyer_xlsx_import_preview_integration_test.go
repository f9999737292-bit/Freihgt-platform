//go:build integration

package excelexchange

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT21RichWorkbookPreviewSuccessUpdateDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if !preview.ReadyToCommit || preview.AnalysisID == nil || preview.ExpiresAt == nil {
		t.Fatalf("expected valid persisted preview ready=%v analysis=%v", preview.ReadyToCommit, preview.AnalysisID)
	}
	if preview.Mode != xlsxexchange.BuyerImportModeUpdateDraft {
		t.Fatalf("mode=%q", preview.Mode)
	}
}

func TestE7P2INT22NoEventGraphLotWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if before.eventVersion != after.eventVersion ||
		before.versionCount != after.versionCount ||
		before.sectionCount != after.sectionCount ||
		before.questionCount != after.questionCount ||
		before.optionCount != after.optionCount ||
		before.ruleCount != after.ruleCount ||
		before.lotCount != after.lotCount {
		t.Fatalf("preview mutated graph: before=%+v after=%+v", before, after)
	}
	if before.auditCount != after.auditCount || before.idempotencyCount != after.idempotencyCount {
		t.Fatal("preview created audit/idempotency writes")
	}
}

func TestE7P2INT23AnalysisRowPersistedOptionA(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := countImportAnalyses(t, env, fix.TenantID)
	fixed := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed })

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if countImportAnalyses(t, env, fix.TenantID) != before+1 {
		t.Fatal("expected exactly one analysis insert")
	}
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if row.Status != domain.ImportAnalysisStatusPreviewed {
		t.Fatalf("status=%q", row.Status)
	}
	if row.ActorID != fix.BuyerA.UserID || row.TenantID != fix.TenantID {
		t.Fatal("analysis actor/tenant binding mismatch")
	}
	if row.TargetID == nil || *row.TargetID != draft.Event.ID {
		t.Fatal("analysis event binding mismatch")
	}
	if !row.CreatedAt.Equal(fixed) {
		t.Fatalf("created_at=%s want=%s", row.CreatedAt.Format(time.RFC3339), fixed.Format(time.RFC3339))
	}
	if !row.ExpiresAt.Equal(fixed.Add(24 * time.Hour)) {
		t.Fatalf("expires_at=%s want=%s", row.ExpiresAt.Format(time.RFC3339), fixed.Add(24*time.Hour).Format(time.RFC3339))
	}
	if row.CanonicalHash != preview.CanonicalPayloadHash {
		t.Fatal("persisted hash mismatch")
	}
	if err := domain.VerifyImportAnalysisCanonicalHash(row.CanonicalPayloadJSON, row.CanonicalHash); err != nil {
		t.Fatalf("repository hash defense failed: %v", err)
	}
}

func TestE7P2INT24SchemaMismatch400ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := setMetadataValue(t, exportRichDraftWorkbook(t, env, fix, draft), "schema_name", "WRONG_SCHEMA")
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT25MissingUnexpectedSheet400ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	base := exportRichDraftWorkbook(t, env, fix, draft)

	t.Run("missing_sheet", func(t *testing.T) {
		before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
		workbook := deletePreviewSheet(t, base, previewSheetRules)
		rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
	})
	t.Run("unexpected_sheet", func(t *testing.T) {
		before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
		workbook := addUnexpectedPreviewSheet(t, base, "CarrierResponses")
		rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
	})
}

func TestE7P2INT26InvalidHeaders400ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := duplicatePreviewHeader(t, exportRichDraftWorkbook(t, env, fix, draft), previewSheetSections)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT27DuplicateStableCodes422ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithDuplicateSectionCode(t, exportRichDraftWorkbook(t, env, fix, draft))
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if preview.AnalysisID != nil || preview.ReadyToCommit {
		t.Fatal("domain invalid preview must not persist analysis")
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT28DanglingReferences422ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithDanglingQuestionSection(t, exportRichDraftWorkbook(t, env, fix, draft))
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT29RuleCycleSelfTarget422ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithSelfTargetRule(t, exportRichDraftWorkbook(t, env, fix, draft))
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT30MalformedJSONCells422ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithMalformedValidationJSON(t, exportRichDraftWorkbook(t, env, fix, draft))
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT31FormulaMacroExternalLink400ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	base := exportRichDraftWorkbook(t, env, fix, draft)

	t.Run("formula", func(t *testing.T) {
		before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
		rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbookWithFormulaCell(t, base), previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
	})
	t.Run("macro", func(t *testing.T) {
		before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
		rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbookWithMacroPackage(t, base), previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
	})
}

func TestE7P2INT32OversizedUpload413ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	oversized := bytes.Repeat([]byte("A"), int(xlsxsecurity.DefaultMaxUploadBytes)+1)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, oversized, previewHTTPOptions{})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d want 413 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT33BuyerRead403ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerRead, draft.Event.ID, workbook, previewHTTPOptions{})
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT34Carrier403ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, draft.Event.ID, workbook, previewHTTPOptions{})
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT35CrossTenant404ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CrossTenant, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT36CrossCompanyFailClosed404ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerB, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT37FeatureDisabled404NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT38GatewayIdentitySpoofDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	body, contentType := buildBuyerXlsxImportMultipartBody(t, workbook, previewHTTPOptions{})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, enabledExcelExchangeConfig(), env.rfxSvc, env.qSvc, nil, nil, nil, nil, nil, nil, env.excelExchangeSvc, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/"+draft.Event.ID.String()+"/xlsx-import/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	req.URL.RawQuery = "tenant_id=" + fix.OtherTenantID.String()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestE7P2INT39StaleMetadataRowVersionWarning(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	workbook = setMetadataValue(t, workbook, "event_row_version", "0")
	workbook = setMetadataValue(t, workbook, "version_row_version", "0")
	before := countImportAnalyses(t, env, fix.TenantID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	assertPreviewWarningCode(t, preview, xlsxexchange.MachineCodeMetadataMismatch)
	if preview.TargetEventRowVersion <= 1 || preview.TargetDraftRowVersion <= 1 {
		t.Fatalf("preview must bind current server baseline versions: %+v", preview)
	}
	if countImportAnalyses(t, env, fix.TenantID) != before+1 {
		t.Fatal("valid stale-metadata preview must still persist analysis")
	}
}

func TestE7P2INT40DeterministicRepeatedPreview(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	fixed := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed })

	first := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	second := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status first=%d second=%d", first.Code, second.Code)
	}
	p1 := decodePreviewResponse(t, first)
	p2 := decodePreviewResponse(t, second)
	if p1.AnalysisID == nil || p2.AnalysisID == nil || *p1.AnalysisID == *p2.AnalysisID {
		t.Fatal("repeated preview must create distinct analysis IDs")
	}
	if p1.CanonicalPayloadHash != p2.CanonicalPayloadHash {
		t.Fatal("repeated preview must preserve canonical hash")
	}
	if !reflect.DeepEqual(previewEnvelopeComparable(p1), previewEnvelopeComparable(p2)) {
		t.Fatal("repeated preview envelope must match except analysis_id/expires_at")
	}
	row1 := loadPersistedAnalysis(t, env, *p1.AnalysisID, fix.TenantID)
	row2 := loadPersistedAnalysis(t, env, *p2.AnalysisID, fix.TenantID)
	if row1.Status != domain.ImportAnalysisStatusPreviewed || row2.Status != domain.ImportAnalysisStatusPreviewed {
		t.Fatal("both analyses must remain PREVIEWED")
	}
	if row1.CanonicalHash != row2.CanonicalHash {
		t.Fatal("persisted hash must match across repeated previews")
	}
	if !row1.CreatedAt.Equal(fixed) || !row2.CreatedAt.Equal(fixed) {
		t.Fatal("created_at must come from injected service clock")
	}
	if !row1.ExpiresAt.Equal(fixed.Add(24*time.Hour)) || !row2.ExpiresAt.Equal(fixed.Add(24*time.Hour)) {
		t.Fatal("expires_at must be exactly 24h after created_at")
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if before.eventVersion != after.eventVersion || before.lotCount != after.lotCount {
		t.Fatal("repeated preview must not mutate event graph")
	}
	if after.importAnalysisCnt != before.importAnalysisCnt+2 {
		t.Fatalf("analysis count delta=%d want 2", after.importAnalysisCnt-before.importAnalysisCnt)
	}
}

func TestE7P2INT41CompetitorColumnRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	sent := seedCompetitorSentinels(t, env, fix, draft)
	workbook := workbookWithCompetitorColumn(t, exportRichDraftWorkbook(t, env, fix, draft), "carrier_id")
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
	assertResponseExcludesSentinels(t, rec.Body.Bytes(), sent)
	var errPayload struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errPayload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errPayload.Error.Details["machine_code"] != xlsxexchange.MachineCodeCompetitorColumnDenied {
		t.Fatalf("expected machine_code=%q details=%v body=%s", xlsxexchange.MachineCodeCompetitorColumnDenied, errPayload.Error.Details, rec.Body.String())
	}
}

func TestE7P2INT42RouteServiceGatewayOpenAPIParity(t *testing.T) {
	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	for _, route := range routes {
		switch route.Name {
		case "export_buyer_draft_xlsx":
			rec := getBuyerXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID)
			if rec.Code != route.SuccessStatus {
				t.Fatalf("export status=%d want=%d", rec.Code, route.SuccessStatus)
			}
		case "preview_buyer_draft_xlsx_import":
			rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
			if rec.Code != route.SuccessStatus {
				t.Fatalf("preview status=%d want=%d body=%s", rec.Code, route.SuccessStatus, rec.Body.String())
			}
			if route.OpenAPIOperationID != "post_preview_buyer_draft_rfx_event_xlsx_import" {
				t.Fatalf("unexpected operationId=%q", route.OpenAPIOperationID)
			}
		default:
			t.Fatalf("unknown route %s", route.Name)
		}
	}
}
