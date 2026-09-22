//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT91CommitSuccess200StatusRemainsDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-91")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	commit := decodeCarrierCommitResponse(t, rec)
	if commit.AnalysisID != *preview.AnalysisID || commit.ResponseID != carrier.Response.ID {
		t.Fatalf("unexpected commit envelope: %+v", commit)
	}
	row := loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID)
	if row.Status != domain.ImportAnalysisStatusConsumed {
		t.Fatalf("status=%q want CONSUMED", row.Status)
	}
	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	if response.Status != domain.RfxResponseStatusDraft {
		t.Fatalf("status=%q want DRAFT", response.Status)
	}
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if after.auditCount <= before.auditCount || after.idempotencyCount <= before.idempotencyCount {
		t.Fatal("successful commit must write audit and idempotency records")
	}
}

func TestE7P2INT92CommitReconcilesAnswersVsProposal(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	detailQuestionID := carrierQuestionIDByCode(t, env, fix, carrier, "DETAIL")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: detailQuestionID, Value: json.RawMessage(`"before-proposal"`)}},
	}); err != nil {
		t.Fatalf("seed detail answer: %v", err)
	}
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithCarrierAnswerValue(t, data, "DETAIL", "updated-proposal")
	})
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-92")

	stored := parseStoredCarrierProposal(t, loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID))
	value := carrierAnswerText(t, env, fix, carrier, "DETAIL")
	var want string
	for _, row := range stored.Answers {
		if row.QuestionCode == "DETAIL" {
			want = row.AnswerValue
			break
		}
	}
	if want == "" {
		t.Fatal("expected stored DETAIL answer row")
	}
	if value != want {
		t.Fatalf("answer=%q want proposal %q", value, want)
	}
}

func TestE7P2INT93CommitReconcilesOfferLinesVsProposal(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithCarrierOfferAmount(t, data, "54321.00", "xlsx-offer")
	})
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-93")

	stored := parseStoredCarrierProposal(t, loadPersistedAnalysis(t, env, *preview.AnalysisID, fix.TenantID))
	amount, comment := carrierOfferLine(t, env, fix, carrier)
	if len(stored.OfferLines) == 0 {
		t.Fatal("expected stored offer lines")
	}
	if amount != stored.OfferLines[0].Amount {
		t.Fatalf("amount=%q want %q", amount, stored.OfferLines[0].Amount)
	}
	if comment != stored.OfferLines[0].Comment {
		t.Fatalf("comment=%q want %q", comment, stored.OfferLines[0].Comment)
	}
}

func TestE7P2INT94RemovedAnswerRowsDeleted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithRemovedCarrierAnswerRow(t, data, carrier.Question.QuestionCode)
	})
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-94")
	if count := countAnswersForQuestion(t, env, fix, carrier, carrier.Question.ID); count != 0 {
		t.Fatalf("answer row count=%d want 0", count)
	}
}

func TestE7P2INT95RemovedOfferLinesDeleted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithRemovedCarrierOfferLine(t, data)
	})
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-95")
	if count := countOfferLinesForResponse(t, env, fix, carrier.Response.ID); count != 0 {
		t.Fatalf("offer line count=%d want 0", count)
	}
}

func TestE7P2INT96StaleResponseSaveVersion409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_responses SET save_version = save_version + 1 WHERE id = $1 AND tenant_id = $2`,
		carrier.Response.ID, fix.TenantID); err != nil {
		t.Fatalf("bump save_version: %v", err)
	}
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-96")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT97StaleQuestionnaireVersion409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_versions SET version_number = version_number + 1 WHERE id = $1 AND tenant_id = $2`,
		*carrier.Response.RfxVersionID, fix.TenantID); err != nil {
		t.Fatalf("bump questionnaire version number: %v", err)
	}

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-97")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT98StaleLotsFingerprint409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if _, err := env.rfxSvc.CreateLot(context.Background(), fix.BuyerA, carrier.Event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: carrier.Event.ID, LotNumber: "L2", Name: "Inserted lot",
	}); err != nil {
		t.Fatalf("insert lot after preview: %v", err)
	}

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-98")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT99ExpiredAnalysis409NoConsume(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	fixed := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed })
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return fixed.Add(24 * time.Hour) })

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-99")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT100WrongActor403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	altCarrier := seedAlternateCarrierDispatcherOnCompanyA(t, env, fix)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), altCarrier, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-100")
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT101WrongCarrierCompany403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	analysisID := insertCarrierAnalysisWithWrongCompany(t, env, fix, preview)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, analysisID, "e7p2-int-101")
	assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, analysisID, before)
}

func TestE7P2INT102HashTampering422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	analysisID := insertCarrierAnalysisWithHashMismatch(t, env, fix, preview)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, analysisID, "e7p2-int-102")
	assertHTTPErrorCode(t, rec, http.StatusUnprocessableEntity, apperrors.CodeUnprocessable)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, analysisID, before)
}

func TestE7P2INT103IdempotentReplay200(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	key := "e7p2-int-103"
	first := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, key)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	mid := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	second := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, key)
	if second.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", second.Code, second.Body.String())
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("idempotent replay must return byte-stable response")
	}
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if !reflect.DeepEqual(mid, after) {
		t.Fatalf("replay mutated response state: mid=%+v after=%+v", mid, after)
	}
}

func TestE7P2INT104SameKeyDifferentAnalysis409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview1 := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	key := "e7p2-int-104"
	carrierCommitSuccessful(t, env, fix, carrier, preview1, key)
	preview2 := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithCarrierOfferAmount(t, data, "11111.11", "other-analysis")
	})
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview2.AnalysisID, key)
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview2.AnalysisID, before)
}

func TestE7P2INT105ConcurrentCommitsOneMutation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview1 := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	preview2 := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithCarrierOfferAmount(t, data, "22222.22", "concurrent")
	})
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	var wg sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0] = postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview1.AnalysisID, "e7p2-int-105-a")
	}()
	go func() {
		defer wg.Done()
		results[1] = postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview2.AnalysisID, "e7p2-int-105-b")
	}()
	wg.Wait()

	successes, conflicts := 0, 0
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
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
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

func TestE7P2INT106MidAnswerFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	detailQuestionID := carrierQuestionIDByCode(t, env, fix, carrier, "DETAIL")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: detailQuestionID, Value: json.RawMessage(`"before-rollback"`)}},
	}); err != nil {
		t.Fatalf("seed detail answer: %v", err)
	}
	beforeValue := carrierAnswerText(t, env, fix, carrier, "DETAIL")
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithCarrierAnswerValue(t, data, "DETAIL", "ROLLBACK-ANSWER")
	})
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-106")
	if rec.Code == http.StatusOK {
		t.Fatal("expected commit failure due to audit rollback")
	}
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
	if got := carrierAnswerText(t, env, fix, carrier, "DETAIL"); got != beforeValue {
		t.Fatalf("answer changed despite rollback: %q -> %q", beforeValue, got)
	}
}

func TestE7P2INT107MidOfferFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	beforeAmount, _ := carrierOfferLine(t, env, fix, carrier)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, func(data []byte) []byte {
		return workbookWithCarrierOfferAmount(t, data, "33333.33", "rollback-offer")
	})
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-107")
	if rec.Code == http.StatusOK {
		t.Fatal("expected commit failure due to audit rollback")
	}
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
	amount, _ := carrierOfferLine(t, env, fix, carrier)
	if amount != beforeAmount {
		t.Fatalf("offer amount changed despite rollback: %q -> %q", beforeAmount, amount)
	}
}

func TestE7P2INT108SubmitRaceCommit409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)

	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	if _, err := env.crSvc.Submit(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, response.SaveVersion, "e7p2-int-108-submit"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-108")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT109DeadlineWithoutLatePermission409Or422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier, now := seedCarrierPastDeadlineFixture(t, env, fix)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return now })
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-109")
	if rec.Code != http.StatusConflict && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 409 or 422 body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT110PostCommitNoSubmittedAtStatusDraft(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-110")

	var status string
	var submittedAt *time.Time
	if err := env.pool.QueryRow(context.Background(), `
		SELECT status, submitted_at FROM rfx.rfx_responses WHERE id = $1 AND tenant_id = $2`,
		carrier.Response.ID, fix.TenantID).Scan(&status, &submittedAt); err != nil {
		t.Fatalf("load response: %v", err)
	}
	if status != domain.RfxResponseStatusDraft {
		t.Fatalf("status=%q want DRAFT", status)
	}
	if submittedAt != nil {
		t.Fatalf("submitted_at=%v want nil", submittedAt)
	}
}

func TestE7P2INT111PostCommitNoResponseSubmittedAudit(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	before := countAuditEventsByAction(t, env, fix, "response.submitted")
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-111")
	if countAuditEventsByAction(t, env, fix, "response.submitted") != before {
		t.Fatal("commit must not emit response.submitted audit")
	}
}

func TestE7P2INT112LatePermissionNotConsumed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier, now := seedCarrierPastDeadlineWithApprovedLate(t, env, fix)
	env.excelExchangeSvc.SetNowFunc(func() time.Time { return now })
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	carrierCommitSuccessful(t, env, fix, carrier, preview, "e7p2-int-112")

	var status string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT status FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3
		ORDER BY created_at DESC LIMIT 1`,
		fix.TenantID, carrier.Event.ID, fix.CarrierID).Scan(&status); err != nil {
		t.Fatalf("load late permission status: %v", err)
	}
	if domain.LateSubmissionStatus(status) == domain.LateSubmissionStatusConsumed {
		t.Fatalf("permission status=%q must not be consumed by XLSX commit", status)
	}
}

func TestE7P2INT113SubmittedBlocksCommit409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)

	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	if _, err := env.crSvc.Submit(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, response.SaveVersion, "e7p2-int-113-submit"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	workbook := exportCarrierDraftWorkbook(t, env, fix, carrier)
	recPreview := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
	if recPreview.Code != http.StatusConflict {
		t.Fatalf("preview status=%d want 409 body=%s", recPreview.Code, recPreview.Body.String())
	}

	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-113")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCarrierCommitFailureNoWrites(t, env, fix, carrier, *preview.AnalysisID, before)
}

func TestE7P2INT114RouteOpenAPIParity6RoutesStatic(t *testing.T) {
	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 9 {
		t.Fatalf("expected 9 routes, got %d", len(routes))
	}
	var commitRoute sharedrfx.ExcelExchangeRoute
	for _, route := range routes {
		if route.Name == "commit_carrier_response_xlsx_import" {
			commitRoute = route
			break
		}
	}
	if commitRoute.Name == "" {
		t.Fatal("carrier commit route missing from shared manifest")
	}
	if commitRoute.Method != http.MethodPost || !commitRoute.IdempotencyRequired {
		t.Fatalf("unexpected commit route contract: %+v", commitRoute)
	}
	if commitRoute.OpenAPIOperationID != "post_commit_carrier_rfx_response_xlsx_import" {
		t.Fatalf("operationId=%q", commitRoute.OpenAPIOperationID)
	}
	if commitRoute.RBACPolicy != "PolicyCarrierRespond" {
		t.Fatalf("RBAC=%q", commitRoute.RBACPolicy)
	}

	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	preview := previewReadyCarrierAnalysis(t, env, fix, carrier, nil)
	rec := postCarrierXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, *preview.AnalysisID, "e7p2-int-114")
	if rec.Code != commitRoute.SuccessStatus {
		t.Fatalf("commit status=%d want=%d body=%s", rec.Code, commitRoute.SuccessStatus, rec.Body.String())
	}
}

func TestE7P2INT115BuyerRegressionSmoke(t *testing.T) {
	TestP3PreviewRegressionExtra(t)
}

func TestE7P2INT116SaveAnswersDirectPathStillWorks(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	detailQuestionID := carrierQuestionIDByCode(t, env, fix, carrier, "DETAIL")
	saved, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: detailQuestionID, Value: json.RawMessage(`"updated via direct save"`)}},
	})
	if err != nil {
		t.Fatalf("save answers: %v", err)
	}
	if saved.SaveVersion <= response.SaveVersion {
		t.Fatalf("save_version=%d want bump from %d", saved.SaveVersion, response.SaveVersion)
	}
	if got := carrierAnswerText(t, env, fix, carrier, "DETAIL"); got != "updated via direct save" {
		t.Fatalf("answer=%q want updated via direct save", got)
	}
}

func TestE7P2INT117SubmitDirectPathStillWorksSeparateFromXLSX(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	response, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	submitted, err := env.crSvc.Submit(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, response.SaveVersion, "e7p2-int-117")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != domain.RfxResponseStatusSubmitted {
		t.Fatalf("status=%q want SUBMITTED", submitted.Status)
	}
	if submitted.SubmittedAt.IsZero() {
		t.Fatal("expected submitted_at on direct submit")
	}
}

func TestE7P2INT118ConfidentialityDefinedNamesRejectedOnPreview(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	base := exportCarrierDraftWorkbook(t, env, fix, carrier)

	t.Run("external_defined_name", func(t *testing.T) {
		before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
		workbook := workbookWithExternalDefinedName(t, base, "[OtherBook]Sheet1!$A$1")
		rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
		}
		assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
	})
	t.Run("formula_defined_name", func(t *testing.T) {
		before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
		workbook := workbookWithExternalDefinedName(t, base, "=SUM(A1:A2)")
		rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbook, previewHTTPOptions{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
		}
		assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
	})
}

func TestE7P2INT119FormulaCellRejectionOnPreview(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	base := exportCarrierDraftWorkbook(t, env, fix, carrier)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := postCarrierXlsxImportPreviewHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID, workbookWithCarrierFormulaCell(t, base), previewHTTPOptions{})
	switch rec.Code {
	case http.StatusUnprocessableEntity:
		preview := decodeCarrierPreviewResponse(t, rec)
		if preview.AnalysisID != nil || preview.ReadyToCommit {
			t.Fatal("formula workbook must not persist analysis")
		}
		assertPreviewIssueCodeCarrier(t, preview, xlsxexchange.MachineCodeFormulaDenied)
	case http.StatusBadRequest:
		// Excelize formula cells may be rejected as unsafe package before domain preview mapping.
	default:
		t.Fatalf("status=%d want 400 or 422 body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierPreviewFailureNoWrites(t, env, fix.TenantID, carrier.Response.ID, before)
}

func assertPreviewIssueCodeCarrier(t *testing.T, preview service.CarrierImportPreviewResponse, code string) {
	t.Helper()
	for _, issue := range preview.Errors {
		if issue.MachineCode == code {
			return
		}
	}
	t.Fatalf("issue code %q not found in preview errors", code)
}

func carrierQuestionIDByCode(t *testing.T, env *testEnv, fix buyerFixture, carrier carrierExportFixture, questionCode string) uuid.UUID {
	t.Helper()
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), *carrier.Response.RfxVersionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load questionnaire: %v", err)
	}
	for _, swq := range graph.Sections {
		for _, q := range swq.Questions {
			if q.QuestionCode == questionCode {
				return q.ID
			}
		}
	}
	t.Fatalf("question code %q not found", questionCode)
	return uuid.Nil
}

func carrierAnswerText(t *testing.T, env *testEnv, fix buyerFixture, carrier carrierExportFixture, questionCode string) string {
	t.Helper()
	answers, err := env.answerRepo.ListByResponse(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("list answers: %v", err)
	}
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), *carrier.Response.RfxVersionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load questionnaire: %v", err)
	}
	codeByID := map[uuid.UUID]string{}
	for _, swq := range graph.Sections {
		for _, q := range swq.Questions {
			codeByID[q.ID] = q.QuestionCode
		}
	}
	for _, answer := range answers {
		if codeByID[answer.QuestionID] == questionCode {
			return xlsxexchange.FormatCarrierAnswerValue(answer.AnswerValueJSON)
		}
	}
	return ""
}

func carrierOfferLine(t *testing.T, env *testEnv, fix buyerFixture, carrier carrierExportFixture) (amount, comment string) {
	t.Helper()
	lines, err := env.rfxRepo.ListOfferLinesByResponse(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("list offer lines: %v", err)
	}
	if len(lines) == 0 {
		return "", ""
	}
	return xlsxexchange.FormatOfferAmountForExport(lines[0].Amount), optionalStringPtr(lines[0].Comment)
}

func optionalStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func countAnswersForQuestion(t *testing.T, env *testEnv, fix buyerFixture, carrier carrierExportFixture, questionID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_answers
		WHERE tenant_id = $1 AND rfx_response_id = $2 AND question_id = $3`,
		fix.TenantID, carrier.Response.ID, questionID).Scan(&count); err != nil {
		t.Fatalf("count answers: %v", err)
	}
	return count
}

func countOfferLinesForResponse(t *testing.T, env *testEnv, fix buyerFixture, responseID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_response_offer_lines
		WHERE tenant_id = $1 AND rfx_response_id = $2`,
		fix.TenantID, responseID).Scan(&count); err != nil {
		t.Fatalf("count offer lines: %v", err)
	}
	return count
}

func seedCarrierPastDeadlineFixture(t *testing.T, env *testEnv, fix buyerFixture) (carrierExportFixture, time.Time) {
	t.Helper()
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	past := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	now := past.Add(2 * time.Hour)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET response_deadline = $2 WHERE id = $1 AND tenant_id = $3`,
		carrier.Event.ID, past, fix.TenantID); err != nil {
		t.Fatalf("set past deadline: %v", err)
	}
	reloaded, err := env.rfxRepo.GetResponseByID(context.Background(), carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	carrier.Response = reloaded
	return carrier, now
}

func seedCarrierPastDeadlineWithApprovedLate(t *testing.T, env *testEnv, fix buyerFixture) (carrierExportFixture, time.Time) {
	t.Helper()
	carrier, now := seedCarrierPastDeadlineFixture(t, env, fix)
	req, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, carrier.Event.ID, fix.CarrierID, "e7p2-int-112-req", domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonTechnicalFailure, ReasonText: "outage",
		RequestedUntil: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create late request: %v", err)
	}
	from := now.Add(-time.Hour)
	until := now.Add(24 * time.Hour)
	if _, err := env.lateSvc.Approve(context.Background(), fix.BuyerA, carrier.Event.ID, req.ID, "e7p2-int-112-approve", domain.ApproveLateSubmissionInput{
		ExpectedVersion: req.Version, ApprovedValidFrom: from, ApprovedValidUntil: until, DecisionComment: "approved",
	}); err != nil {
		t.Fatalf("approve late request: %v", err)
	}
	return carrier, now
}
