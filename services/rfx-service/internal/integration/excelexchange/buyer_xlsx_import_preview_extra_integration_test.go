//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

func TestPreviewUnauthenticated401Extra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), domain.ActorContext{}, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
	if stringsContainsPreviewPayload(rec.Body.Bytes()) {
		t.Fatalf("401 must not expose preview payload body=%s", rec.Body.String())
	}
}

func TestPreviewNoActiveDraft409Extra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_events SET draft_version_id = NULL WHERE id = $1`, draft.Event.ID); err != nil {
		t.Fatalf("clear draft: %v", err)
	}

	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestPreviewImportAnalysisTransactionRollbackExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	before := countImportAnalyses(t, env, fix.TenantID)
	createdAt := time.Date(2026, 9, 12, 16, 0, 0, 0, time.UTC)
	payload := []byte(`{"schema_name":"BINTRANS_RFX_BUYER_XLSX_V1","mode":"UPDATE_DRAFT"}`)
	hash := sha256Hex(payload)
	targetVersion := draft.Version.VersionNumber
	input := domain.ImportAnalysis{
		TenantID: fix.TenantID, ActorID: fix.BuyerA.UserID, ActorCompanyID: fix.CompanyA,
		WorkbookType: domain.WorkbookTypeBuyerTender, SchemaVersion: domain.SchemaVersionBuyerXLSXV1,
		TargetType: domain.ImportTargetTypeDraftEvent, TargetID: &draft.Event.ID, TargetVersion: &targetVersion,
		CanonicalPayloadJSON: payload, CanonicalHash: hash,
		ValidationSummary: []byte(`{}`), CreatedAt: createdAt, ExpiresAt: createdAt.Add(24 * time.Hour),
	}
	ctx := context.Background()
	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	repo := env.importAnalysisRepo.WithTx(tx)
	if _, err := repo.CreatePreview(ctx, input); err != nil {
		t.Fatalf("create preview in tx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if countImportAnalyses(t, env, fix.TenantID) != before {
		t.Fatal("rolled back analysis must not persist")
	}
}

func TestPreviewRepositoryHashMismatchDeniedExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	before := countImportAnalyses(t, env, fix.TenantID)
	payload := []byte(`{"schema_name":"BINTRANS_RFX_BUYER_XLSX_V1"}`)
	createdAt := time.Now().UTC()
	input := domain.ImportAnalysis{
		TenantID: fix.TenantID, ActorID: fix.BuyerA.UserID, ActorCompanyID: fix.CompanyA,
		WorkbookType: domain.WorkbookTypeBuyerTender, SchemaVersion: domain.SchemaVersionBuyerXLSXV1,
		TargetType: domain.ImportTargetTypeDraftEvent,
		CanonicalPayloadJSON: payload,
		CanonicalHash:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ValidationSummary:    []byte(`{}`), CreatedAt: createdAt, ExpiresAt: createdAt.Add(time.Hour),
	}
	if _, err := env.importAnalysisRepo.CreatePreview(context.Background(), input); err == nil {
		t.Fatal("expected hash mismatch rejection")
	}
	if countImportAnalyses(t, env, fix.TenantID) != before {
		t.Fatal("hash mismatch must not insert analysis")
	}
}

func TestPreviewIssueCapStructured422Extra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithManyDuplicateSectionsExtra(t, exportRichDraftWorkbook(t, env, fix, draft), 2100)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

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
	assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
}

func TestPreviewMultipartValidation400Extra(t *testing.T) {
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
			rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, tc.opts)
			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			assertPreviewFailureNoWrites(t, env, fix.TenantID, draft.Event.ID, before)
		})
	}
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func stringsContainsPreviewPayload(body []byte) bool {
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	_, ok := probe["ready_to_commit"]
	return ok
}

func workbookWithManyDuplicateSectionsExtra(t *testing.T, data []byte, rows int) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	for i := 0; i < rows; i++ {
		row := i + 3
		code := fmt.Sprintf("SECX%d", i)
		if err := f.SetCellStr(previewSheetSections, fmt.Sprintf("A%d", row), code); err != nil {
			t.Fatalf("set section code: %v", err)
		}
		if err := f.SetCellStr(previewSheetSections, fmt.Sprintf("B%d", row), "Title "+code); err != nil {
			t.Fatalf("set section title: %v", err)
		}
		if err := f.SetCellStr(previewSheetSections, fmt.Sprintf("H%d", row), fmt.Sprintf("%d", row)); err != nil {
			t.Fatalf("set sort order: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return buf.Bytes()
}
