//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT43SuccessfulCommit200AnalysisConsumed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-43")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	commit := decodeCommitResponse(t, rec)
	if commit.AnalysisID != *preview.AnalysisID || commit.EventID != draft.Event.ID {
		t.Fatalf("unexpected commit envelope: %+v", commit)
	}
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if row.Status != domain.ImportAnalysisStatusConsumed {
		t.Fatalf("status=%q want CONSUMED", row.Status)
	}
	if row.ConsumedAt == nil || row.ResultReferenceID == nil {
		t.Fatal("consumed analysis must record consume metadata")
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if after.auditCount <= before.auditCount || after.idempotencyCount <= before.idempotencyCount {
		t.Fatal("successful commit must write audit and idempotency records")
	}
}

func TestE7P2INT44GraphLotParityWithStoredProposal(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithUpdatedSectionTitle(t, exportRichDraftWorkbook(t, env, fix, draft), "Updated Main")
	preview := previewWorkbook(t, env, fix, draft, workbook)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-44")

	stored := parseStoredProposal(t, loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID))
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), draft.Version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	lots, err := env.rfxRepo.ListLotsByEvent(context.Background(), draft.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("list lots: %v", err)
	}
	assertGraphMatchesProposal(t, graph, stored)
	assertLotsMatchProposal(t, lots, stored)
}

func TestE7P2INT45ExistingUUIDsPreserved(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	beforeIDs := captureStableCodeIDs(t, env, fix, draft)
	preview := previewReadyAnalysis(t, env, fix, draft)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-45")
	afterIDs := captureStableCodeIDs(t, env, fix, draft)
	for code, id := range beforeIDs.sections {
		if afterIDs.sections[code] != id {
			t.Fatalf("section %s uuid changed %s -> %s", code, id, afterIDs.sections[code])
		}
	}
	for code, id := range beforeIDs.questions {
		if afterIDs.questions[code] != id {
			t.Fatalf("question %s uuid changed %s -> %s", code, id, afterIDs.questions[code])
		}
	}
	for key, id := range beforeIDs.options {
		if afterIDs.options[key] != id {
			t.Fatalf("option %s uuid changed %s -> %s", key, id, afterIDs.options[key])
		}
	}
	for code, id := range beforeIDs.rules {
		if afterIDs.rules[code] != id {
			t.Fatalf("rule %s uuid changed %s -> %s", code, id, afterIDs.rules[code])
		}
	}
	for number, id := range beforeIDs.lots {
		if afterIDs.lots[number] != id {
			t.Fatalf("lot %s uuid changed %s -> %s", number, id, afterIDs.lots[number])
		}
	}
}

func TestE7P2INT46NewStableCodesNewUUIDs(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	beforeIDs := captureStableCodeIDs(t, env, fix, draft)
	workbook := workbookWithAddedLotAndSection(t, exportRichDraftWorkbook(t, env, fix, draft))
	preview := previewWorkbook(t, env, fix, draft, workbook)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-46")
	afterIDs := captureStableCodeIDs(t, env, fix, draft)

	if id, ok := afterIDs.sections["EXTRA"]; !ok || id == uuid.Nil {
		t.Fatal("expected new section EXTRA")
	} else if _, existed := beforeIDs.sections["EXTRA"]; existed {
		t.Fatal("section EXTRA should be new")
	}
	if id, ok := afterIDs.questions["QEXTRA"]; !ok || id == uuid.Nil {
		t.Fatal("expected new question QEXTRA")
	} else if _, existed := beforeIDs.questions["QEXTRA"]; existed {
		t.Fatal("question QEXTRA should be new")
	}
	if id, ok := afterIDs.lots["L2"]; !ok || id == uuid.Nil {
		t.Fatal("expected new lot L2")
	}
	if _, existed := beforeIDs.lots["L2"]; existed {
		t.Fatal("lot L2 should be new")
	}
}

func TestE7P2INT47RemovedEntitiesDeleted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithRemovedDetailQuestion(t, exportRichDraftWorkbook(t, env, fix, draft))
	preview := previewWorkbook(t, env, fix, draft, workbook)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-47")

	if countActiveQuestionByCode(t, env, fix, draft.Version.ID, "DETAIL") != 0 {
		t.Fatal("DETAIL question must be soft-deleted")
	}
	if countActiveRuleByCode(t, env, fix, draft.Version.ID, "SHOW_DETAIL") != 0 {
		t.Fatal("SHOW_DETAIL rule must be soft-deleted")
	}
}

func TestE7P2INT48RuleTargetsResolvedLocally(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-48")

	var targetQuestionID uuid.UUID
	var ruleCode string
	err := env.pool.QueryRow(context.Background(), `
		SELECT r.rule_code, r.target_question_id
		FROM rfx.rfx_question_rules r
		INNER JOIN rfx.rfx_questions q ON q.id = r.target_question_id
		WHERE r.tenant_id = $1 AND r.rfx_version_id = $2 AND r.deleted_at IS NULL AND r.rule_code = 'SHOW_DETAIL'
	`, fix.TenantID, draft.Version.ID).Scan(&ruleCode, &targetQuestionID)
	if err != nil {
		t.Fatalf("load rule target: %v", err)
	}
	var questionCode string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT question_code FROM rfx.rfx_questions WHERE id = $1 AND tenant_id = $2`,
		targetQuestionID, fix.TenantID).Scan(&questionCode); err != nil {
		t.Fatalf("load target question: %v", err)
	}
	if questionCode != "DETAIL" {
		t.Fatalf("target question=%q want DETAIL", questionCode)
	}
}

func TestE7P2INT49PublishedSupersededUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	published := publishRichDraft(t, env, fix, draft, "e7p2-int-49-pub")
	publishedSnap := captureQuestionnaireSnapshot(t, env, fix, published.ID)
	activeDraft, err := forkDraftFromPublished(t, env, fix, draft.Event.ID, "e7p2-int-49-fork")
	if err != nil {
		t.Fatalf("fork draft: %v", err)
	}
	draft.Version = activeDraft
	workbook := workbookWithUpdatedSectionTitle(t, exportRichDraftWorkbook(t, env, fix, draft), "Published-safe edit")
	preview := previewWorkbook(t, env, fix, draft, workbook)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-49-commit")

	afterPublishedSnap := captureQuestionnaireSnapshot(t, env, fix, published.ID)
	if !reflect.DeepEqual(publishedSnap, afterPublishedSnap) {
		t.Fatalf("published graph changed: before=%+v after=%+v", publishedSnap, afterPublishedSnap)
	}
}

func TestE7P2INT50StaleEventRowVersion409NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET version = version + 1 WHERE id = $1 AND tenant_id = $2`,
		draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("bump event version: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-50")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT51StaleDraftRowVersion409NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_versions SET version = version + 1 WHERE id = $1 AND tenant_id = $2`,
		draft.Version.ID, fix.TenantID); err != nil {
		t.Fatalf("bump draft version: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-51")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT52ReplacedActiveDraftID409NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	publishRichDraft(t, env, fix, draft, "e7p2-int-52-pub")
	if _, err := forkDraftFromPublished(t, env, fix, draft.Event.ID, "e7p2-int-52-fork"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-52")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT53ExpiredAnalysis409NoConsume(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	fixed := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed })
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed.Add(24 * time.Hour) })

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-53")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT54ConsumedNewIdempotencyKey409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-54-a")
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-54-b")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoGraphWrites(t, env, fix.TenantID, draft.Event.ID, before)
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if row.Status != domain.ImportAnalysisStatusConsumed {
		t.Fatalf("status=%q want CONSUMED after first successful commit", row.Status)
	}
}

func TestE7P2INT55SameKeyReplay200NoDuplicateWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	key := "e7p2-int-55"
	first := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, key)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	mid := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	second := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, key)
	if second.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", second.Code, second.Body.String())
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("idempotent replay must return byte-stable response")
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if !reflect.DeepEqual(mid, after) {
		t.Fatalf("replay mutated graph: mid=%+v after=%+v", mid, after)
	}
}

func TestE7P2INT56SameKeyDifferentAnalysisID409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview1 := previewReadyAnalysis(t, env, fix, draft)
	key := "e7p2-int-56"
	commitSuccessful(t, env, fix, draft, preview1, key)
	workbook := workbookWithUpdatedSectionTitle(t, exportRichDraftWorkbook(t, env, fix, draft), "Different analysis")
	preview2 := previewWorkbook(t, env, fix, draft, workbook)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview2.AnalysisID, key)
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview2.AnalysisID, before)
}

func TestE7P2INT57ConcurrentCommitsOneMutationOnly(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview1 := previewReadyAnalysis(t, env, fix, draft)
	workbook := workbookWithUpdatedSectionTitle(t, exportRichDraftWorkbook(t, env, fix, draft), "Concurrent edit")
	preview2 := previewWorkbook(t, env, fix, draft, workbook)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	var wg sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0] = postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview1.AnalysisID, "e7p2-int-57-a")
	}()
	go func() {
		defer wg.Done()
		results[1] = postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview2.AnalysisID, "e7p2-int-57-b")
	}()
	wg.Wait()

	successes := 0
	conflicts := 0
	for _, rec := range results {
		switch rec.Code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one success and one conflict, got success=%d conflict=%d", successes, conflicts)
	}
	after := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	if after.auditCount != before.auditCount+1 {
		t.Fatalf("expected exactly one audit write, before=%d after=%d", before.auditCount, after.auditCount)
	}
	if after.idempotencyCount != before.idempotencyCount+1 {
		t.Fatalf("expected exactly one idempotency write, before=%d after=%d", before.idempotencyCount, after.idempotencyCount)
	}
	consumed := 0
	for _, id := range []*uuid.UUID{preview1.AnalysisID, preview2.AnalysisID} {
		row := loadPersistedAnalysis(t, env, *id, fix.TenantID)
		if row.Status == domain.ImportAnalysisStatusConsumed {
			consumed++
		}
	}
	if consumed != 1 {
		t.Fatalf("expected exactly one consumed analysis, got %d", consumed)
	}
}

func TestE7P2INT58AuditAtomicWithMutation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	beforeAudit := countAuditEventsByAction(t, env, fix, "rfx.buyer_xlsx_import.committed.v1")
	commitSuccessful(t, env, fix, draft, preview, "e7p2-int-58-ok")
	if countAuditEventsByAction(t, env, fix, "rfx.buyer_xlsx_import.committed.v1") != beforeAudit+1 {
		t.Fatal("successful commit must append exactly one audit event")
	}

	previewFail := previewReadyAnalysis(t, env, fix, draft)
	beforeAudit = countAuditEventsByAction(t, env, fix, "rfx.buyer_xlsx_import.committed.v1")
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET version = version + 1 WHERE id = $1 AND tenant_id = $2`,
		draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("bump event version: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *previewFail.AnalysisID, "e7p2-int-58-fail")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *previewFail.AnalysisID, before)
	if countAuditEventsByAction(t, env, fix, "rfx.buyer_xlsx_import.committed.v1") != beforeAudit {
		t.Fatal("failed commit must not append audit event")
	}
}

func TestE7P2INT59IdempotencyAtomicNoSuccessRowOnRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	key := "e7p2-int-59"
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_versions SET version = version + 1 WHERE id = $1 AND tenant_id = $2`,
		draft.Version.ID, fix.TenantID); err != nil {
		t.Fatalf("bump draft version: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, key)
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
	if countIdempotencyRecords(t, env, fix, draft.Event.ID, key) != 0 {
		t.Fatal("failed commit must not store idempotency success row")
	}
}

func TestE7P2INT60AnalysisConsumeAtomicNoConsumedOnFailure(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET version = version + 1 WHERE id = $1 AND tenant_id = $2`,
		draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("bump event version: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-60")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if row.Status != domain.ImportAnalysisStatusPreviewed {
		t.Fatalf("status=%q want PREVIEWED", row.Status)
	}
}

func TestE7P2INT61MidGraphFailureRollbackNoPartialGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := workbookWithUpdatedSectionTitle(t, exportRichDraftWorkbook(t, env, fix, draft), "Rollback probe")
	preview := previewWorkbook(t, env, fix, draft, workbook)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-61")
	if rec.Code == http.StatusOK {
		t.Fatal("expected commit failure due to OCC conflict")
	}
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT62LotFailureRollbackNoPartialLots(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	base := exportRichDraftWorkbook(t, env, fix, draft)
	workbook := workbookWithUpdatedSectionTitle(t, workbookWithUpdatedLotName(t, base, "Renamed lot"), "Rollback lot probe")
	preview := previewWorkbook(t, env, fix, draft, workbook)
	beforeName := lotNameByNumber(t, env, fix, draft.Event.ID, "L1")
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-62")
	if rec.Code == http.StatusOK {
		t.Fatal("expected commit failure due to downstream graph conflict")
	}
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
	if got := lotNameByNumber(t, env, fix, draft.Event.ID, "L1"); got != beforeName {
		t.Fatalf("lot name changed despite rollback: %q -> %q", beforeName, got)
	}
}

func TestE7P2INT63HashMismatchDenied422NoConsume(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	analysisID := insertAnalysisWithHashMismatch(t, env, fix, preview)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, analysisID, "e7p2-int-63")
	assertHTTPErrorCode(t, rec, http.StatusUnprocessableEntity, apperrors.CodeUnprocessable)
	assertCommitFailureNoWrites(t, env, fix, draft, analysisID, before)
}

func TestE7P2INT64ActorBindingDenied403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)
	altBuyer := seedAlternateBuyerManageOnCompanyA(t, env, fix)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), altBuyer, draft.Event.ID, *preview.AnalysisID, "e7p2-int-64")
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT65BuyerRead403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerRead, draft.Event.ID, *preview.AnalysisID, "e7p2-int-65")
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT66Carrier403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, draft.Event.ID, *preview.AnalysisID, "e7p2-int-66")
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT67CrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CrossTenant, draft.Event.ID, *preview.AnalysisID, "e7p2-int-67")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT68GatewayIdentitySpoofDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	body, err := json.Marshal(domain.BuyerImportCommitInput{AnalysisID: *preview.AnalysisID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, enabledExcelExchangeConfig(), env.rfxSvc, env.qSvc, nil, nil, nil, nil, nil, nil, env.excelExchangeSvc, nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/"+draft.Event.ID.String()+"/xlsx-import/commit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "e7p2-int-68")
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	req.URL.RawQuery = "tenant_id=" + fix.OtherTenantID.String()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT69FeatureDisabled404NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-69")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestE7P2INT70RouteParityCommitRouteExists(t *testing.T) {
	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 9 {
		t.Fatalf("expected 9 routes, got %d", len(routes))
	}
	var commitRoute sharedrfx.ExcelExchangeRoute
	for _, route := range routes {
		if route.Name == "commit_buyer_draft_xlsx_import" {
			commitRoute = route
			break
		}
	}
	if commitRoute.Name == "" {
		t.Fatal("commit route missing from shared manifest")
	}
	if commitRoute.Method != http.MethodPost || !commitRoute.IdempotencyRequired {
		t.Fatalf("unexpected commit route contract: %+v", commitRoute)
	}
	if commitRoute.OpenAPIOperationID != "post_commit_buyer_draft_rfx_event_xlsx_import" {
		t.Fatalf("operationId=%q", commitRoute.OpenAPIOperationID)
	}

	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "e7p2-int-70")
	if rec.Code != commitRoute.SuccessStatus {
		t.Fatalf("commit status=%d want=%d body=%s", rec.Code, commitRoute.SuccessStatus, rec.Body.String())
	}
}

func TestCommitLotFingerprintStaleDetectionExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	if _, err := env.rfxSvc.CreateLot(context.Background(), fix.BuyerA, draft.Event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: draft.Event.ID, LotNumber: "L2", Name: "Inserted lot",
	}); err != nil {
		t.Fatalf("insert lot after preview: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "commit-extra-lot-fp")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestCommitTransactionLeakExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	beforeOpen := countOtherActiveTransactions(t, env)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET version = version + 1 WHERE id = $1 AND tenant_id = $2`,
		draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("bump event version: %v", err)
	}
	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "commit-extra-tx-leak")
	if rec.Code == http.StatusOK {
		t.Fatal("expected stale commit failure")
	}
	afterOpen := countOtherActiveTransactions(t, env)
	if afterOpen > beforeOpen {
		t.Fatalf("open transactions increased: before=%d after=%d", beforeOpen, afterOpen)
	}
}

func TestCommitDeterministicReplayExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	key := "commit-extra-replay"
	first := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, key)
	second := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, key)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status first=%d second=%d", first.Code, second.Code)
	}
	c1 := decodeCommitResponse(t, first)
	c2 := decodeCommitResponse(t, second)
	if !reflect.DeepEqual(c1, c2) {
		t.Fatalf("decoded replay mismatch: %+v vs %+v", c1, c2)
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("raw replay body must be byte-identical")
	}
}

func TestCommitCompetitorConfidentialityExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	sent := seedCompetitorSentinels(t, env, fix, draft)
	preview := previewReadyAnalysis(t, env, fix, draft)
	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "commit-extra-conf")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertResponseExcludesSentinels(t, rec.Body.Bytes(), sent)
	auditBody := latestAuditPayload(t, env, fix, "rfx.buyer_xlsx_import.committed.v1")
	assertResponseExcludesSentinels(t, auditBody, sent)
}

func TestMigration073RegressionExtra(t *testing.T) {
	TestE7P2INT03Migration073UpDownUp(t)
}

func TestP3PreviewRegressionExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	workbook := exportRichDraftWorkbook(t, env, fix, draft)
	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("preview regression status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if !preview.ReadyToCommit || preview.AnalysisID == nil {
		t.Fatal("preview regression lost ready_to_commit path")
	}
}

type stableCodeIDs struct {
	sections  map[string]uuid.UUID
	questions map[string]uuid.UUID
	options   map[string]uuid.UUID
	rules     map[string]uuid.UUID
	lots      map[string]uuid.UUID
}

type questionnaireSnapshot struct {
	SectionTitles map[string]string
	QuestionCount int
	RuleCount     int
}

func previewWorkbook(t *testing.T, env *testEnv, fix buyerFixture, draft richDraftFixture, workbook []byte) service.BuyerImportPreviewResponse {
	t.Helper()
	rec := postBuyerXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, workbook, previewHTTPOptions{})
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	preview := decodePreviewResponse(t, rec)
	if !preview.ReadyToCommit || preview.AnalysisID == nil {
		t.Fatalf("preview not ready: %+v", preview)
	}
	return preview
}

func commitSuccessful(t *testing.T, env *testEnv, fix buyerFixture, draft richDraftFixture, preview service.BuyerImportPreviewResponse, key string) service.BuyerImportCommitResponse {
	t.Helper()
	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit status=%d body=%s", rec.Code, rec.Body.String())
	}
	return decodeCommitResponse(t, rec)
}

func assertCommitFailureNoGraphWrites(t *testing.T, env *testEnv, tenantID, eventID uuid.UUID, before graphWriteSnapshot) {
	t.Helper()
	after := captureGraphWriteSnapshot(t, env, tenantID, eventID)
	if before.eventVersion != after.eventVersion ||
		before.versionCount != after.versionCount ||
		before.sectionCount != after.sectionCount ||
		before.questionCount != after.questionCount ||
		before.optionCount != after.optionCount ||
		before.ruleCount != after.ruleCount ||
		before.lotCount != after.lotCount {
		t.Fatalf("commit failure mutated graph: before=%+v after=%+v", before, after)
	}
	if before.auditCount != after.auditCount || before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("commit failure created audit/idempotency writes")
	}
}

func assertCommitFailureNoWrites(t *testing.T, env *testEnv, fix buyerFixture, draft richDraftFixture, analysisID uuid.UUID, before graphWriteSnapshot) {
	t.Helper()
	assertCommitFailureNoGraphWrites(t, env, fix.TenantID, draft.Event.ID, before)
	row := loadPersistedAnalysis(t, env, analysisID, fix.TenantID)
	if row.Status == domain.ImportAnalysisStatusConsumed {
		t.Fatal("failed commit must not consume analysis")
	}
}

func seedAlternateBuyerManageOnCompanyA(t *testing.T, env *testEnv, fix buyerFixture) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	altBuyer := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`,
		altBuyer.UserID, fix.TenantID, "buyer-alt@test.local", "buyer-alt@test.local"); err != nil {
		t.Fatalf("seed alt buyer user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`,
		fix.TenantID, fix.CompanyA, altBuyer.UserID); err != nil {
		t.Fatalf("seed alt buyer membership: %v", err)
	}
	var buyerManageRole uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = $1 LIMIT 1`, "PROCUREMENT_MANAGER").Scan(&buyerManageRole); err != nil {
		t.Fatalf("lookup buyer manage role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.TenantID, altBuyer.UserID, fix.CompanyA, buyerManageRole); err != nil {
		t.Fatalf("seed alt buyer role: %v", err)
	}
	return altBuyer
}

func parseStoredProposal(t *testing.T, analysis domain.ImportAnalysis) xlsxexchange.StoredImportPayload {
	t.Helper()
	stored, err := xlsxexchange.ParseStoredImportPayload(analysis.CanonicalPayloadJSON)
	if err != nil {
		t.Fatalf("parse stored proposal: %v", err)
	}
	return stored
}

func captureStableCodeIDs(t *testing.T, env *testEnv, fix buyerFixture, draft richDraftFixture) stableCodeIDs {
	t.Helper()
	ctx := context.Background()
	out := stableCodeIDs{
		sections:  map[string]uuid.UUID{},
		questions: map[string]uuid.UUID{},
		options:   map[string]uuid.UUID{},
		rules:     map[string]uuid.UUID{},
		lots:      map[string]uuid.UUID{},
	}
	graph, err := env.qRepo.LoadQuestionnaire(ctx, draft.Version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	for _, swq := range graph.Sections {
		out.sections[swq.Section.SectionCode] = swq.Section.ID
		for _, q := range swq.Questions {
			out.questions[q.QuestionCode] = q.ID
			for _, opt := range q.Options {
				out.options[q.QuestionCode+"\x00"+opt.OptionCode] = opt.ID
			}
		}
	}
	for _, rule := range graph.Rules {
		out.rules[rule.RuleCode] = rule.ID
	}
	lots, err := env.rfxRepo.ListLotsByEvent(ctx, draft.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("list lots: %v", err)
	}
	for _, lot := range lots {
		out.lots[lot.LotNumber] = lot.ID
	}
	return out
}

func assertGraphMatchesProposal(t *testing.T, graph *domain.QuestionnaireDefinition, stored xlsxexchange.StoredImportPayload) {
	t.Helper()
	sectionCodes := map[string]struct{}{}
	for _, sec := range stored.Questionnaire.Sections {
		sectionCodes[sec.SectionCode] = struct{}{}
	}
	for _, swq := range graph.Sections {
		if _, ok := sectionCodes[swq.Section.SectionCode]; !ok {
			t.Fatalf("unexpected section %s in graph", swq.Section.SectionCode)
		}
	}
	if len(graph.Sections) != len(stored.Questionnaire.Sections) {
		t.Fatalf("section count=%d proposal=%d", len(graph.Sections), len(stored.Questionnaire.Sections))
	}
	questionCount := 0
	for _, swq := range graph.Sections {
		questionCount += len(swq.Questions)
	}
	if questionCount != len(stored.Questionnaire.Questions) {
		t.Fatalf("question count=%d proposal=%d", questionCount, len(stored.Questionnaire.Questions))
	}
	if len(graph.Rules) != len(stored.Questionnaire.Rules) {
		t.Fatalf("rule count=%d proposal=%d", len(graph.Rules), len(stored.Questionnaire.Rules))
	}
}

func assertLotsMatchProposal(t *testing.T, lots []domain.RfxLot, stored xlsxexchange.StoredImportPayload) {
	t.Helper()
	if len(lots) != len(stored.Lots) {
		t.Fatalf("lot count=%d proposal=%d", len(lots), len(stored.Lots))
	}
	byNumber := map[string]domain.RfxLot{}
	for _, lot := range lots {
		byNumber[lot.LotNumber] = lot
	}
	for _, want := range stored.Lots {
		got, ok := byNumber[strings.TrimSpace(want.LotNumber)]
		if !ok {
			t.Fatalf("missing lot %s", want.LotNumber)
		}
		if got.Name != want.Name {
			t.Fatalf("lot %s name=%q want=%q", want.LotNumber, got.Name, want.Name)
		}
	}
}

func captureQuestionnaireSnapshot(t *testing.T, env *testEnv, fix buyerFixture, versionID uuid.UUID) questionnaireSnapshot {
	t.Helper()
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), versionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	out := questionnaireSnapshot{SectionTitles: map[string]string{}}
	for _, swq := range graph.Sections {
		out.SectionTitles[swq.Section.SectionCode] = swq.Section.Title
		out.QuestionCount += len(swq.Questions)
	}
	out.RuleCount = len(graph.Rules)
	return out
}

func versionLifecycleSvc(env *testEnv) *service.VersionLifecycleService {
	return service.NewVersionLifecycleService(
		env.pool,
		env.rfxRepo,
		env.qRepo,
		repository.NewScoreRepository(env.pool),
		env.idemRepo,
		env.auditRepo,
		repository.NewChangeImpactRepository(env.pool),
		env.rfxSvc,
	)
}

func publishRichDraft(t *testing.T, env *testEnv, fix buyerFixture, draft richDraftFixture, key string) *domain.RfxVersion {
	t.Helper()
	event, err := env.rfxRepo.GetEventByID(context.Background(), draft.Event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	version, err := env.qRepo.GetActiveDraftVersion(context.Background(), fix.TenantID, draft.Event.ID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	published, err := versionLifecycleSvc(env).PublishQuestionnaire(context.Background(), fix.BuyerA, draft.Event.ID, key, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: version.Version,
		ChangeSummary:        "Commit integration publish",
	})
	if err != nil {
		t.Fatalf("publish questionnaire: %v", err)
	}
	return published
}

func forkDraftFromPublished(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, key string) (*domain.RfxVersion, error) {
	t.Helper()
	return versionLifecycleSvc(env).ForkDraftFromPublished(context.Background(), fix.BuyerA, eventID, key)
}

func countAuditEventsByAction(t *testing.T, env *testEnv, fix buyerFixture, action string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1 AND action = $2`,
		fix.TenantID, action).Scan(&count); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return count
}

func countIdempotencyRecords(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, key string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, eventID, key).Scan(&count); err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	return count
}

func countActiveQuestionByCode(t *testing.T, env *testEnv, fix buyerFixture, versionID uuid.UUID, code string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_questions q
		INNER JOIN rfx.rfx_sections s ON s.id = q.section_id
		WHERE q.tenant_id = $1 AND s.rfx_version_id = $2 AND q.question_code = $3 AND q.deleted_at IS NULL`,
		fix.TenantID, versionID, code).Scan(&count); err != nil {
		t.Fatalf("count question: %v", err)
	}
	return count
}

func countActiveRuleByCode(t *testing.T, env *testEnv, fix buyerFixture, versionID uuid.UUID, code string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_question_rules
		WHERE tenant_id = $1 AND rfx_version_id = $2 AND rule_code = $3 AND deleted_at IS NULL`,
		fix.TenantID, versionID, code).Scan(&count); err != nil {
		t.Fatalf("count rule: %v", err)
	}
	return count
}

func lotNameByNumber(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, lotNumber string) string {
	t.Helper()
	var name string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT name FROM rfx.rfx_lots
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND lot_number = $3 AND deleted_at IS NULL`,
		fix.TenantID, eventID, lotNumber).Scan(&name); err != nil {
		t.Fatalf("load lot name: %v", err)
	}
	return name
}

func insertAnalysisWithHashMismatch(t *testing.T, env *testEnv, fix buyerFixture, preview service.BuyerImportPreviewResponse) uuid.UUID {
	t.Helper()
	source := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	id := uuid.New()
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_import_analyses (
			id, tenant_id, actor_id, actor_company_id, workbook_type, schema_version,
			target_type, target_id, target_version, canonical_payload_json, canonical_hash,
			status, validation_summary, created_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)`,
		id, source.TenantID, source.ActorID, source.ActorCompanyID, source.WorkbookType, source.SchemaVersion,
		source.TargetType, source.TargetID, source.TargetVersion, source.CanonicalPayloadJSON,
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		domain.ImportAnalysisStatusPreviewed, source.ValidationSummary, source.CreatedAt, source.ExpiresAt,
	)
	if err != nil {
		t.Fatalf("insert mismatched analysis: %v", err)
	}
	return id
}

func latestAuditPayload(t *testing.T, env *testEnv, fix buyerFixture, action string) []byte {
	t.Helper()
	var payload []byte
	if err := env.pool.QueryRow(context.Background(), `
		SELECT metadata FROM rfx.audit_events
		WHERE tenant_id = $1 AND action = $2
		ORDER BY occurred_at DESC LIMIT 1`, fix.TenantID, action).Scan(&payload); err != nil {
		t.Fatalf("load audit payload: %v", err)
	}
	return payload
}

func countOtherActiveTransactions(t *testing.T, env *testEnv) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM pg_stat_activity
		WHERE datname = current_database()
		  AND state = 'active'
		  AND xact_start IS NOT NULL
		  AND pid <> pg_backend_pid()`).Scan(&count); err != nil {
		t.Fatalf("count active transactions: %v", err)
	}
	return count
}

func workbookWithUpdatedSectionTitle(t *testing.T, data []byte, title string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.SetCellStr(previewSheetSections, "B2", title); err != nil {
			t.Fatalf("set section title: %v", err)
		}
	})
}

func workbookWithUpdatedLotName(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.SetCellStr(previewSheetLots, "B2", name); err != nil {
			t.Fatalf("set lot name: %v", err)
		}
	})
}

func workbookWithAddedLotAndSection(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		if err := f.SetCellStr(previewSheetLots, "A3", "L2"); err != nil {
			t.Fatalf("set lot number: %v", err)
		}
		_ = f.SetCellStr(previewSheetLots, "B3", "Second lot")
		_ = f.SetCellStr(previewSheetLots, "G3", "ACTIVE")
		if err := f.SetCellStr(previewSheetSections, "A3", "EXTRA"); err != nil {
			t.Fatalf("set section code: %v", err)
		}
		_ = f.SetCellStr(previewSheetSections, "B3", "Extra Section")
		_ = f.SetCellStr(previewSheetSections, "H3", "20")
		if err := f.SetCellStr(previewSheetQuestions, "A4", "EXTRA"); err != nil {
			t.Fatalf("set section code: %v", err)
		}
		_ = f.SetCellStr(previewSheetQuestions, "B4", "QEXTRA")
		_ = f.SetCellStr(previewSheetQuestions, "C4", "TEXT")
		_ = f.SetCellStr(previewSheetQuestions, "D4", "Extra question")
		_ = f.SetCellStr(previewSheetQuestions, "J4", "false")
		_ = f.SetCellStr(previewSheetQuestions, "K4", "10")
	})
}

func workbookWithRemovedDetailQuestion(t *testing.T, data []byte) []byte {
	t.Helper()
	return mutatePreviewWorkbook(t, data, func(f *excelize.File) {
		rows, err := f.GetRows(previewSheetQuestions)
		if err != nil {
			t.Fatalf("read questions: %v", err)
		}
		for i := len(rows) - 1; i >= 1; i-- {
			row := rows[i]
			if len(row) < 2 || strings.TrimSpace(row[1]) != "DETAIL" {
				continue
			}
			if err := f.RemoveRow(previewSheetQuestions, i+1); err != nil {
				t.Fatalf("remove DETAIL question row: %v", err)
			}
			break
		}
		rows, err = f.GetRows(previewSheetRules)
		if err != nil {
			t.Fatalf("read rules: %v", err)
		}
		for i := len(rows) - 1; i >= 1; i-- {
			row := rows[i]
			if len(row) == 0 || strings.TrimSpace(row[0]) != "SHOW_DETAIL" {
				continue
			}
			if err := f.RemoveRow(previewSheetRules, i+1); err != nil {
				t.Fatalf("remove SHOW_DETAIL rule row: %v", err)
			}
			break
		}
	})
}
