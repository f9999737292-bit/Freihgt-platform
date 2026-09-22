//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT196PreviewReadyNewEventNullTarget(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	beforeEvents := countTenantEvents(t, env, fix.TenantID)
	beforeAudit := countCreateAudits(t, env, fix.TenantID)

	rec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-196"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodeCreatePreviewResponse(t, rec)
	if preview.Mode != xlsxexchange.BuyerImportModeCreateNewDraft || !preview.ReadyToCommit || preview.AnalysisID == nil || preview.ExpiresAt == nil {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if raw := rec.Body.String(); strings.Contains(raw, "target_event_id") || strings.Contains(raw, "canonical_payload_hash") {
		t.Fatalf("preview leaked reserved identity: %s", raw)
	}
	var targetType string
	var targetID *uuid.UUID
	var targetVersion *int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT target_type, target_id, target_version FROM rfx.rfx_import_analyses WHERE id = $1 AND tenant_id = $2`,
		*preview.AnalysisID, fix.TenantID).Scan(&targetType, &targetID, &targetVersion); err != nil {
		t.Fatalf("load analysis: %v", err)
	}
	if targetType != domain.ImportTargetTypeNewEvent || targetID != nil || targetVersion != nil {
		t.Fatalf("target_type=%s target_id=%v target_version=%v", targetType, targetID, targetVersion)
	}
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if err := xlsxexchange.VerifyStoredCanonicalPayloadHash(row.CanonicalPayloadJSON, row.CanonicalHash); err != nil {
		t.Fatalf("persisted canonical hash: %v", err)
	}
	if row.CanonicalHash == "" || len(row.CanonicalHash) != 64 {
		t.Fatalf("canonical_hash=%q", row.CanonicalHash)
	}
	if countTenantEvents(t, env, fix.TenantID) != beforeEvents {
		t.Fatal("preview must not create an event")
	}
	if countCreateAudits(t, env, fix.TenantID) != beforeAudit {
		t.Fatal("preview must not write create audit")
	}
}

func TestE7P2INT197DomainInvalidPreviewZeroAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := countImportAnalyses(t, env, fix.TenantID)
	fields := defaultCreatePreviewFields(fix, "RFX-F5-197")
	fields.RfxType = "NOT_A_TYPE"
	rec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, fields)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodeCreatePreviewResponse(t, rec)
	if preview.ReadyToCommit || preview.AnalysisID != nil {
		t.Fatalf("domain-invalid preview persisted analysis: %+v", preview)
	}
	if countImportAnalyses(t, env, fix.TenantID) != before {
		t.Fatal("422 preview must not persist analysis")
	}
}

func TestE7P2INT198StructuralAndOversized(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	t.Run("structural", func(t *testing.T) {
		rec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, []byte("not-a-zip"), defaultCreatePreviewFields(fix, "RFX-F5-198A"))
		assertHTTPErrorCode(t, rec, http.StatusBadRequest, apperrors.CodeValidation)
	})
	t.Run("oversized", func(t *testing.T) {
		oversized := bytes.Repeat([]byte("A"), int(xlsxsecurity.DefaultMaxUploadBytes)+1)
		rec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, oversized, defaultCreatePreviewFields(fix, "RFX-F5-198B"))
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status=%d want 413 body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("unsafe_formula", func(t *testing.T) {
		draft := seedRichDraftEvent(t, env, fix)
		workbook := injectFormulaIntoWorkbook(t, exportRichDraftWorkbook(t, env, fix, draft))
		rec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-198C"))
		assertHTTPErrorCode(t, rec, http.StatusBadRequest, apperrors.CodeValidation)
	})
}

func TestE7P2INT199BuyerReadAndCarrierForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	readRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerRead, workbook, defaultCreatePreviewFields(fix, "RFX-F5-199R"))
	assertHTTPErrorCode(t, readRec, http.StatusForbidden, apperrors.CodeForbidden)
	carrierRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, workbook, defaultCreatePreviewFields(fix, "RFX-F5-199C"))
	assertHTTPErrorCode(t, carrierRec, http.StatusForbidden, apperrors.CodeForbidden)
	noAuth := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), domain.ActorContext{}, workbook, defaultCreatePreviewFields(fix, "RFX-F5-199U"))
	assertHTTPErrorCode(t, noAuth, http.StatusUnauthorized, apperrors.CodeUnauthorized)
}

func TestE7P2INT200OwnerMembershipAndTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	unknown := defaultCreatePreviewFields(fix, "RFX-F5-200U")
	unknown.OwnerCompanyID = uuid.New()
	rec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, unknown)
	assertHTTPErrorCode(t, rec, http.StatusNotFound, apperrors.CodeNotFound)
	foreign := defaultCreatePreviewFields(fix, "RFX-F5-200M")
	foreign.OwnerCompanyID = fix.CompanyB
	rec = postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, foreign)
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
}

func TestE7P2INT201AnalysisActorCompanyIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	previewRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-201"))
	preview := decodeCreatePreviewResponse(t, previewRec)
	otherActor := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerB, *preview.AnalysisID, "e7p2-int-201-b")
	assertHTTPErrorCode(t, otherActor, http.StatusForbidden, apperrors.CodeForbidden)
	otherTenant := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CrossTenant, *preview.AnalysisID, "e7p2-int-201-t")
	assertHTTPErrorCode(t, otherTenant, http.StatusNotFound, apperrors.CodeNotFound)

	tampered := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-201H")))
	mismatchID := insertCreateAnalysisWithHashMismatch(t, env, *tampered.AnalysisID, fix.TenantID)
	mismatch := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, mismatchID, "e7p2-int-201-hash")
	assertHTTPErrorCode(t, mismatch, http.StatusUnprocessableEntity, apperrors.CodeUnprocessable)
	assertMachineCode(t, mismatch, domain.MachineCodeCanonicalHashMismatch)
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-201H") != 0 {
		t.Fatal("hash mismatch created an event")
	}

	updateRec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update preview=%d body=%s", updateRec.Code, updateRec.Body.String())
	}
	updatePreview := decodePreviewResponse(t, updateRec)
	if updatePreview.AnalysisID == nil {
		t.Fatal("update preview missing analysis")
	}
	stale := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *updatePreview.AnalysisID, "e7p2-int-201-stale")
	assertHTTPErrorCode(t, stale, http.StatusConflict, apperrors.CodeConflict)
	assertMachineCode(t, stale, domain.MachineCodeStaleTarget)
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-201") != 0 {
		t.Fatal("stale UPDATE analysis created a CREATE event")
	}
}

func TestE7P2INT202ExpiredAndConsumedAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	preview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-202E")))
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_import_analyses SET expires_at = now() - interval '1 minute' WHERE id = $1`, *preview.AnalysisID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	expired := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-202-exp")
	assertHTTPErrorCode(t, expired, http.StatusConflict, apperrors.CodeConflict)
	assertMachineCode(t, expired, domain.MachineCodeAnalysisExpired)

	ready := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-202C")))
	first := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *ready.AnalysisID, "e7p2-int-202-first")
	if first.Code != http.StatusCreated {
		t.Fatalf("first commit status=%d body=%s", first.Code, first.Body.String())
	}
	consumed := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *ready.AnalysisID, "e7p2-int-202-newkey")
	assertHTTPErrorCode(t, consumed, http.StatusConflict, apperrors.CodeConflict)
	assertMachineCode(t, consumed, domain.MachineCodeAnalysisAlreadyConsumed)
}

func TestE7P2INT203IdempotentReplaySameEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	preview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-203")))
	first := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-203")
	if first.Code != http.StatusCreated {
		t.Fatalf("first=%d body=%s", first.Code, first.Body.String())
	}
	replay := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-203")
	if replay.Code != http.StatusCreated {
		t.Fatalf("replay=%d body=%s", replay.Code, replay.Body.String())
	}
	a := decodeCreateCommitResponse(t, first)
	b := decodeCreateCommitResponse(t, replay)
	if a.EventID != b.EventID || first.Body.String() != replay.Body.String() {
		t.Fatalf("replay mismatch first=%s replay=%s", first.Body.String(), replay.Body.String())
	}
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-203") != 1 {
		t.Fatal("replay must not create a second event")
	}
}

func TestE7P2INT204SameKeyDifferentAnalysisConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	one := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-204A")))
	two := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-204B")))
	first := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *one.AnalysisID, "e7p2-int-204")
	if first.Code != http.StatusCreated {
		t.Fatalf("first=%d body=%s", first.Code, first.Body.String())
	}
	conflict := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *two.AnalysisID, "e7p2-int-204")
	assertHTTPErrorCode(t, conflict, http.StatusConflict, apperrors.CodeConflict)
	assertMachineCode(t, conflict, domain.MachineCodeIdempotencyConflict)
}

func TestE7P2INT205ConcurrentSameKeyOneEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	preview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-205")))
	var wg sync.WaitGroup
	bodies := make([]string, 2)
	codes := make([]int, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-205")
			codes[i] = rec.Code
			bodies[i] = rec.Body.String()
		}()
	}
	wg.Wait()
	if codes[0] != http.StatusCreated || codes[1] != http.StatusCreated {
		t.Fatalf("concurrent codes=%v bodies=%v", codes, bodies)
	}
	if bodies[0] != bodies[1] {
		t.Fatalf("concurrent bodies differ: %s vs %s", bodies[0], bodies[1])
	}
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-205") != 1 {
		t.Fatal("concurrent commits created more than one event")
	}
}

func TestE7P2INT206InducedFailureZeroOrphans(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	preview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-206")))
	beforeEvents := countEventsByNumber(t, env, fix.TenantID, "RFX-F5-206")
	beforeAudit := countCreateAudits(t, env, fix.TenantID)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-206")
	if rec.Code == http.StatusCreated {
		t.Fatal("injected audit failure must not commit")
	}
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-206") != beforeEvents {
		t.Fatal("failed commit left an event orphan")
	}
	if countCreateAudits(t, env, fix.TenantID) != beforeAudit {
		t.Fatal("failed commit left an audit orphan")
	}
	var status string
	if err := env.pool.QueryRow(context.Background(), `SELECT status FROM rfx.rfx_import_analyses WHERE id = $1`, *preview.AnalysisID).Scan(&status); err != nil {
		t.Fatalf("analysis: %v", err)
	}
	if status != domain.ImportAnalysisStatusPreviewed {
		t.Fatalf("analysis status=%s", status)
	}
}

func TestE7P2INT207CreationChannelExcelDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	preview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-207")))
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-207")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	result := decodeCreateCommitResponse(t, rec)
	if result.CreationChannel != domain.CreationChannelExcel || result.Status != domain.RfxStatusDraft {
		t.Fatalf("result=%+v", result)
	}
	meta, err := env.rfxRepo.GetEventExchangeMetadata(context.Background(), result.EventID, fix.TenantID)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if meta.CreationChannel != domain.CreationChannelExcel {
		t.Fatalf("persisted channel=%q", meta.CreationChannel)
	}
	var participants int
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_participants WHERE rfx_event_id = $1 AND tenant_id = $2`, result.EventID, fix.TenantID).Scan(&participants); err != nil {
		t.Fatalf("participants: %v", err)
	}
	if participants != 0 {
		t.Fatalf("participants=%d", participants)
	}
}

func TestE7P2INT208ZeroLotDraftAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := headerOnlyLotsWorkbook(t, exportRichDraftWorkbook(t, env, fix, draft))
	previewRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-208"))
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	preview := decodeCreatePreviewResponse(t, previewRec)
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-208")
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit=%d body=%s", rec.Code, rec.Body.String())
	}
	result := decodeCreateCommitResponse(t, rec)
	var lots int
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_lots WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, result.EventID, fix.TenantID).Scan(&lots); err != nil {
		t.Fatalf("lots: %v", err)
	}
	if lots != 0 {
		t.Fatalf("lots=%d", lots)
	}
}

func TestE7P2INT209QuestionnaireImportedDisabled(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	preview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-209")))
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-209")
	result := decodeCreateCommitResponse(t, rec)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if result.QuestionnaireEnabled {
		t.Fatal("questionnaire must stay disabled")
	}
	var enabled bool
	var sections int
	if err := env.pool.QueryRow(context.Background(), `SELECT questionnaire_enabled FROM rfx.rfx_versions WHERE id = $1 AND tenant_id = $2`, result.DraftVersionID, fix.TenantID).Scan(&enabled); err != nil {
		t.Fatalf("version: %v", err)
	}
	if enabled {
		t.Fatal("persisted questionnaire_enabled must be false")
	}
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_sections WHERE rfx_version_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, result.DraftVersionID, fix.TenantID).Scan(&sections); err != nil {
		t.Fatalf("sections: %v", err)
	}
	if sections == 0 {
		t.Fatal("expected imported questionnaire graph")
	}
}

func TestE7P2INT210EmptyQuestionnaireAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := emptyQuestionnaireWorkbook(t, exportRichDraftWorkbook(t, env, fix, draft))
	previewRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-210"))
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	preview := decodeCreatePreviewResponse(t, previewRec)
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-210")
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT211DuplicateRfxNumberConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	firstPreview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-211")))
	if rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *firstPreview.AnalysisID, "e7p2-int-211-a"); rec.Code != http.StatusCreated {
		t.Fatalf("first=%d body=%s", rec.Code, rec.Body.String())
	}
	secondPreview := decodeCreatePreviewResponse(t, postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-211")))
	dup := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *secondPreview.AnalysisID, "e7p2-int-211-b")
	assertHTTPErrorCode(t, dup, http.StatusConflict, apperrors.CodeConflict)
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-211") != 1 {
		t.Fatal("duplicate rfx_number leaked a second event")
	}
}

func TestE7P2INT212DeadlineExpiresAfterPreview(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	// Preview HTTP validation uses wall-clock time; keep the shell deadline in the real future
	// while the service clock still expires it before commit.
	now := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return now })
	fields := defaultCreatePreviewFields(fix, "RFX-F5-212")
	fields.ResponseDeadline = now.Add(time.Hour).Format(time.RFC3339)
	previewRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, fields)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	preview := decodeCreatePreviewResponse(t, previewRec)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return now.Add(2 * time.Hour) })
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-212")
	assertHTTPErrorCode(t, rec, http.StatusUnprocessableEntity, apperrors.CodeUnprocessable)
	assertMachineCode(t, rec, domain.MachineCodeProposalRevalidation)
	if countEventsByNumber(t, env, fix.TenantID, "RFX-F5-212") != 0 {
		t.Fatal("expired deadline created an event")
	}
}

func TestE7P2INT213AuditPresentPreviewAbsent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	before := countCreateAudits(t, env, fix.TenantID)
	previewRec := postBuyerXlsxCreatePreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-213"))
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview=%d", previewRec.Code)
	}
	if countCreateAudits(t, env, fix.TenantID) != before {
		t.Fatal("preview wrote create audit")
	}
	preview := decodeCreatePreviewResponse(t, previewRec)
	rec := postBuyerXlsxCreateCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, *preview.AnalysisID, "e7p2-int-213")
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit=%d body=%s", rec.Code, rec.Body.String())
	}
	if countCreateAudits(t, env, fix.TenantID) != before+1 {
		t.Fatal("commit must write exactly one create audit")
	}
}

func TestE7P2INT214FlagOffAndRateLimitContract(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	rec := postBuyerXlsxCreatePreviewHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.BuyerA, workbook, defaultCreatePreviewFields(fix, "RFX-F5-214"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("flag-off status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	routes := sharedrfx.E7ExcelExchangeRoutes()
	var commit sharedrfx.ExcelExchangeRoute
	for _, route := range routes {
		if route.Name == "commit_buyer_new_rfx_event_xlsx_create" {
			commit = route
		}
	}
	if !commit.FeatureFlagProtected {
		t.Fatal("create commit must be feature-flag protected")
	}
}

func TestE7P2INT215ManifestOpenAPIClassifierParity(t *testing.T) {
	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 8 {
		t.Fatalf("expected 8 routes, got %d", len(routes))
	}
	if sharedrfx.IsIntegrationProtectedRoute(http.MethodPost, "/api/v1/rfx-events/xlsx-create/preview") ||
		sharedrfx.IsIntegrationProtectedRoute(http.MethodPost, "/api/v1/rfx-events/xlsx-create/commit") {
		t.Fatal("create routes must not be ERP/integration protected")
	}
	if !sharedrfx.RequiresHumanAuth(http.MethodPost, "/api/v1/rfx-events/xlsx-create/preview") ||
		!sharedrfx.RequiresHumanAuth(http.MethodPost, "/api/v1/rfx-events/xlsx-create/commit") {
		t.Fatal("create routes must be human JWT classified")
	}
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.Name] = true
		if route.Name == "preview_buyer_new_rfx_event_xlsx_create" && route.OpenAPIOperationID != "post_preview_buyer_new_rfx_event_xlsx_create" {
			t.Fatalf("preview operationId=%q", route.OpenAPIOperationID)
		}
		if route.Name == "commit_buyer_new_rfx_event_xlsx_create" && route.OpenAPIOperationID != "post_commit_buyer_new_rfx_event_xlsx_create" {
			t.Fatalf("commit operationId=%q", route.OpenAPIOperationID)
		}
	}
	if !seen["preview_buyer_new_rfx_event_xlsx_create"] || !seen["commit_buyer_new_rfx_event_xlsx_create"] {
		t.Fatal("create routes missing from manifest")
	}
}

func assertMachineCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var payload struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v body=%s", err, rec.Body.String())
	}
	if payload.Error.Details["machine_code"] != want {
		t.Fatalf("machine_code=%v want=%s body=%s", payload.Error.Details["machine_code"], want, rec.Body.String())
	}
}
