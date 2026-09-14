//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

func TestE7P2INT71CarrierDraftExport200Deterministic(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, service.BuyerDraftXLSXContentType) {
		t.Fatalf("content-type=%q", ct)
	}
	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	meta := readWorkbookMetadata(t, f)
	if meta["schema_name"] != domain.SchemaVersionCarrierXLSXV1 {
		t.Fatalf("schema_name=%q", meta["schema_name"])
	}
	if meta["export_mode"] != xlsxexchange.CarrierExportModeDraftEdit {
		t.Fatalf("export_mode=%q", meta["export_mode"])
	}
}

func TestE7P2INT72CarrierOwnerExportAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT73CrossTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CrossTenant, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT74CompetitorCannotAccessForeignResponse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	ctx := context.Background()
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, carrier.Event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: carrier.Event.ID, CompanyID: fix.CarrierBID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add carrier B participant: %v", err)
	}

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierBAct, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403/404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT75FeatureDisabled404NoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := getCarrierXlsxExportHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("feature disabled export must not mutate response state")
	}
}

func TestE7P2INT76ExportProducesNoDBWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	before := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	after := captureResponseWriteSnapshot(t, env, fix.TenantID, carrier.Response.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("export mutated response state: before=%+v after=%+v", before, after)
	}
}

func TestE7P2INT77SubmittedReadonlyExportMode(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	ctx := context.Background()
	if _, err := env.crSvc.Submit(ctx, fix.CarrierAct, carrier.Event.ID, fix.CarrierID, carrier.Response.SaveVersion, ""); err != nil {
		t.Fatalf("submit: %v", err)
	}
	submitted, err := env.rfxRepo.GetResponseByID(ctx, carrier.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload submitted: %v", err)
	}

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, submitted.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	meta := readWorkbookMetadata(t, f)
	if meta["export_mode"] != xlsxexchange.CarrierExportModeSubmittedReadonly {
		t.Fatalf("export_mode=%q", meta["export_mode"])
	}
}

func TestE7P2INT78DeterministicRepeatedExport(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)

	first := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	second := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status first=%d second=%d", first.Code, second.Code)
	}
	canonicalFirst, err := xlsxexchange.CanonicalCarrierWorkbookSnapshot(first.Body.Bytes())
	if err != nil {
		t.Fatalf("canonical first: %v", err)
	}
	canonicalSecond, err := xlsxexchange.CanonicalCarrierWorkbookSnapshot(second.Body.Bytes())
	if err != nil {
		t.Fatalf("canonical second: %v", err)
	}
	if !reflect.DeepEqual(canonicalFirst, canonicalSecond) {
		t.Fatal("repeated export must be semantically identical excluding exported_at_utc")
	}
}

func TestE7P2INT79ExportConfidentialitySentinel(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrier := seedCarrierDraftExportFixture(t, env, fix)
	draft := richDraftFixture{Event: carrier.Event, Lot: carrier.Lot, Question: carrier.Question}
	sent := seedCompetitorBSentinelsOnly(t, env, fix, draft)

	rec := getCarrierXlsxExportHTTP(t, env, enabledExcelExchangeConfig(), fix.CarrierAct, carrier.Event.ID, carrier.Response.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertCarrierWorkbookExcludesCompetitorSentinels(t, rec.Body.Bytes(), sent, carrier)
}

func readWorkbookMetadata(t *testing.T, f *excelize.File) map[string]string {
	t.Helper()
	rows, err := f.GetRows("Metadata")
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	meta := map[string]string{}
	for _, row := range rows {
		if len(row) >= 2 {
			meta[row[0]] = row[1]
		}
	}
	return meta
}
