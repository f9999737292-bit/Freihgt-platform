//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT21ValidRichPreview200(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if !preview.ReadyToCommit || len(preview.Errors) != 0 {
		t.Fatalf("expected valid preview, ready=%v errors=%d", preview.ReadyToCommit, len(preview.Errors))
	}
	if preview.AnalysisID == nil || preview.ExpiresAt == nil {
		t.Fatal("expected analysis_id and expires_at on valid preview")
	}
	if preview.Mode != xlsxexchange.BuyerImportModeUpdateDraft {
		t.Fatalf("mode=%q", preview.Mode)
	}
}

func TestE7P2INT22ExactlyOneImmutableAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := countImportAnalyses(t, env, fix.TenantID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := countImportAnalyses(t, env, fix.TenantID)
	if after-before != 1 {
		t.Fatalf("analysis count delta=%d want 1", after-before)
	}
}

func TestE7P2INT23NormalizedPayloadHashMatchesParser(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	previewHTTP := decodePreviewResponse(t, rec)

	ctx := context.Background()
	event, err := env.rfxRepo.GetEventByID(ctx, draft.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	version, err := env.qRepo.GetActiveDraftVersion(ctx, fix.TenantID, draft.Event.ID)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	sections, err := env.qRepo.LoadQuestionnaireTree(ctx, version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("sections: %v", err)
	}
	rules, err := env.qRepo.ListRulesByVersion(ctx, version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	lots, err := env.rfxRepo.ListLotsByEvent(ctx, draft.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("lots: %v", err)
	}
	target := xlsxexchange.TargetDraftBaseline{
		TenantID: fix.TenantID, EventID: event.ID, DraftVersionID: version.ID,
		DraftVersionNumber: version.VersionNumber, EventRowVersion: event.Version, DraftRowVersion: version.Version,
		Questionnaire: domain.QuestionnaireDefinition{
			EventID: event.ID, RfxVersionID: version.ID, VersionNumber: version.VersionNumber,
			QuestionnaireEnabled: true, VersionStatus: version.Status, Sections: sections, Rules: rules,
		},
		Lots: lots,
	}
	parserPreview, err := xlsxexchange.ParseBuyerImportPreview(ctx, workbook, target)
	if err != nil {
		t.Fatalf("parser: %v", err)
	}
	if previewHTTP.CanonicalPayloadHash != parserPreview.CanonicalPayloadHash {
		t.Fatalf("hash mismatch http=%q parser=%q", previewHTTP.CanonicalPayloadHash, parserPreview.CanonicalPayloadHash)
	}
}

func TestE7P2INT24InvalidDomain422ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithDuplicateQuestionCode(t, exportRichDraftWorkbook(t, env, fix, draft))
	before := countImportAnalyses(t, env, fix.TenantID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if preview.ReadyToCommit || preview.AnalysisID != nil {
		t.Fatal("invalid domain preview must not be commit-ready or persisted")
	}
	if len(preview.Errors) == 0 {
		t.Fatal("expected domain errors")
	}
	if countImportAnalyses(t, env, fix.TenantID) != before {
		t.Fatal("invalid preview must not create analysis")
	}
}

func TestE7P2INT25UnsafeMalformed400ZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := countImportAnalyses(t, env, fix.TenantID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, []byte("not-a-zip"), previewHTTPOptions{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
	assertHTTPErrorCode(t, rec, http.StatusBadRequest, apperrors.CodeValidation)
	if countImportAnalyses(t, env, fix.TenantID) != before {
		t.Fatal("malformed upload must not create analysis")
	}
}

func TestE7P2INT26FileRequestOversized413(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	oversized := bytes.Repeat([]byte("A"), int(xlsxsecurity.DefaultMaxUploadBytes)+1)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, oversized, previewHTTPOptions{})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d want 413 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT27MultipartPartValidation400(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	cases := []struct {
		name string
		opts previewHTTPOptions
	}{
		{name: "missing_file", opts: previewHTTPOptions{skipFile: true}},
		{name: "duplicate_file", opts: previewHTTPOptions{duplicateFile: true}},
		{name: "empty_file", opts: previewHTTPOptions{emptyFile: true}},
		{name: "unexpected_part", opts: previewHTTPOptions{extraPart: true}},
		{name: "bad_content_type", opts: previewHTTPOptions{contentType: "application/json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := countImportAnalyses(t, env, fix.TenantID)
			rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, tc.opts)
			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if countImportAnalyses(t, env, fix.TenantID) != before {
				t.Fatal("multipart validation must not create analysis")
			}
		})
	}
}

func TestE7P2INT28NoActiveDraft409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_events SET draft_version_id = NULL WHERE id = $1`, draft.Event.ID); err != nil {
		t.Fatalf("clear draft: %v", err)
	}

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT29BuyerRead403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerRead, draft.Event.ID, workbook, previewHTTPOptions{})
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
}

func TestE7P2INT30Carrier403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, draft.Event.ID, workbook, previewHTTPOptions{})
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
}

func TestE7P2INT31CrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CrossTenant, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT32CrossCompanyFailClosed404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerB, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT33FeatureDisabled404NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("feature disabled route must not mutate persisted state")
	}
}

func TestE7P2INT34IdentitySpoofDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

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
}

func TestE7P2INT35RepeatedValidPreviewDistinctIDsSameHash(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)

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
}

func TestE7P2INT36TTL24hFixedClock(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	fixed := time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed })

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if preview.ExpiresAt == nil {
		t.Fatal("missing expires_at")
	}
	want := fixed.Add(24 * time.Hour)
	if !preview.ExpiresAt.Equal(want) {
		t.Fatalf("expires_at=%s want=%s", preview.ExpiresAt.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestE7P2INT38EventGraphAuditIdempotencyUnchanged(t *testing.T) {
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
		t.Fatalf("preview mutated event graph: before=%+v after=%+v", before, after)
	}
	if before.auditCount != after.auditCount || before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("preview created audit/idempotency writes: before=%+v after=%+v", before, after)
	}
}

func TestE7P2INT39IssueCapStructured422AtMost2000(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithManyDuplicateSections(t, exportRichDraftWorkbook(t, env, fix, draft), 2100)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusUnprocessableEntity {
		preview := decodePreviewResponse(t, rec)
		if len(preview.Errors) > xlsxexchange.MaxPreviewIssues {
			t.Fatalf("errors=%d max=%d", len(preview.Errors), xlsxexchange.MaxPreviewIssues)
		}
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

func workbookWithDuplicateQuestionCode(t *testing.T, data []byte) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	if err := f.SetCellStr("Questions", "A3", "SEC1"); err != nil {
		t.Fatalf("set section: %v", err)
	}
	if err := f.SetCellStr("Questions", "B3", "NOTES"); err != nil {
		t.Fatalf("set duplicate question code: %v", err)
	}
	if err := f.SetCellStr("Questions", "C3", "TEXT"); err != nil {
		t.Fatalf("set type: %v", err)
	}
	for col, value := range map[string]string{"D3": "Dup", "G3": "Dup help", "J3": "true", "K3": "9"} {
		if err := f.SetCellStr("Questions", col, value); err != nil {
			t.Fatalf("set %s: %v", col, err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}

func workbookWithManyDuplicateSections(t *testing.T, data []byte, rows int) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	for i := 0; i < rows; i++ {
		row := i + 3
		code := fmt.Sprintf("SECX%d", i)
		if err := f.SetCellStr("Sections", fmt.Sprintf("A%d", row), code); err != nil {
			t.Fatalf("set section code: %v", err)
		}
		if err := f.SetCellStr("Sections", fmt.Sprintf("B%d", row), "Title "+code); err != nil {
			t.Fatalf("set section title: %v", err)
		}
		if err := f.SetCellStr("Sections", fmt.Sprintf("H%d", row), fmt.Sprintf("%d", row)); err != nil {
			t.Fatalf("set sort order: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}
