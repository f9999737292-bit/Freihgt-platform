package xlsxexchange

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func TestParseBuyerCreatePreviewReadyWithoutBaseline(t *testing.T) {
	snapshot := validImportSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.Mode != BuyerImportModeCreateNewDraft {
		t.Fatalf("mode=%q", preview.Mode)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("expected ready create preview, errors=%v", preview.Errors)
	}
	if preview.ChangeCounts.Lots.Added == 0 {
		t.Fatal("expected lot additions versus empty event")
	}
	assertIssueCode(t, preview.Warnings, MachineCodeMetadataMismatch)
}

func TestParseBuyerCreatePreviewHeaderOnlyLotsReady(t *testing.T) {
	data := mutateWorkbook(t, validImportSnapshot(), func(f *excelize.File) {
		rows, err := f.GetRows(sheetLots)
		if err != nil {
			t.Fatalf("rows: %v", err)
		}
		for i := len(rows); i > 1; i-- {
			_ = f.RemoveRow(sheetLots, i)
		}
	})
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("zero-lot create preview must be ready, errors=%v", preview.Errors)
	}
	if preview.ChangeCounts.Lots.Added != 0 {
		t.Fatalf("lot additions=%d", preview.ChangeCounts.Lots.Added)
	}
}

func TestParseBuyerCreatePreviewEmptyQuestionnaireReady(t *testing.T) {
	snapshot := validImportSnapshot()
	snapshot.Sections = nil
	snapshot.Questions = nil
	snapshot.Options = nil
	snapshot.Rules = nil
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("empty questionnaire must be ready, errors=%v", preview.Errors)
	}
}

func TestParseBuyerCreatePreviewDoesNotCallUpdateEntryPoint(t *testing.T) {
	data, err := GenerateBuyerDraftWorkbook(validImportSnapshot())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	create, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("create parse: %v", err)
	}
	update, err := ParseBuyerImportPreview(context.Background(), data, targetFromSnapshot(validImportSnapshot()))
	if err != nil {
		t.Fatalf("update parse: %v", err)
	}
	if create.Mode == update.Mode {
		t.Fatal("CREATE parser must not reuse UPDATE_DRAFT mode")
	}
}

func TestParseBuyerCreatePreviewUnsupportedSchemaIsError(t *testing.T) {
	data := mutateWorkbook(t, validImportSnapshot(), func(f *excelize.File) {
		_ = f.SetCellStr(sheetMetadata, "B1", "WRONG_SCHEMA")
	})
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.ReadyToCommit {
		t.Fatal("unsupported schema must not be ready")
	}
	assertIssueCode(t, preview.Errors, MachineCodeUnsupportedSchema)
	if ClassifyCreatePreviewErrors(preview.Errors) != PreviewErrorClassStructural {
		t.Fatal("unsupported_schema must stay structural 400")
	}
}

func TestParseBuyerCreatePreviewOversized(t *testing.T) {
	preview, err := ParseBuyerCreatePreview(context.Background(), bytesRepeat('A', int(xlsxsecurity.DefaultMaxUploadBytes)+1))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertIssueCode(t, preview.Errors, MachineCodeFileTooLarge)
}

func TestParseBuyerCreatePreviewFormulaDenied(t *testing.T) {
	data, err := injectWorksheetFormula(t, validImportSnapshot())
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.ReadyToCommit {
		t.Fatal("formula workbook must not be ready")
	}
	if ClassifyCreatePreviewErrors(preview.Errors) != PreviewErrorClassStructural {
		t.Fatalf("formula/unsafe workbook must be structural, errors=%v", preview.Errors)
	}
}

func TestCanonicalCreatePayloadOmitsTrustedWorkbookIDs(t *testing.T) {
	snapshot := validImportSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err := CanonicalCreatePayloadJSON(BuyerCreateEventShell{
		RfxNumber:      "RFX-CREATE-1",
		Title:          "Create",
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		OwnerCompanyID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}, preview)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	payload := string(raw)
	for _, banned := range []string{"target_event_id", "event_row_version", "creation_channel", "exported_at_utc", "source_template"} {
		if strings.Contains(payload, banned) {
			t.Fatalf("trusted field leaked into CREATE payload: %s", banned)
		}
	}
	if !strings.Contains(payload, `"mode":"CREATE_NEW_DRAFT"`) {
		t.Fatal("CREATE payload must include mode")
	}
}

func TestCanonicalCreatePayloadHashIgnoresWorkbookIdentity(t *testing.T) {
	snapshot := validImportSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerCreatePreview(context.Background(), data)
	if err != nil || !preview.ReadyToCommit {
		t.Fatalf("parse: err=%v errors=%v", err, preview.Errors)
	}
	shell := BuyerCreateEventShell{
		RfxNumber:      "RFX-CREATE-HASH",
		Title:          "Create hash",
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		OwnerCompanyID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}
	first, err := CanonicalCreatePayloadJSON(shell, preview)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	firstHash, err := StableStoredPayloadHash(first)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	snapshot.Metadata.RfxEventID = uuid.New()
	snapshot.Metadata.EventRowVersion++
	altData, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate alt: %v", err)
	}
	altPreview, err := ParseBuyerCreatePreview(context.Background(), altData)
	if err != nil || !altPreview.ReadyToCommit {
		t.Fatalf("parse alt: err=%v errors=%v", err, altPreview.Errors)
	}
	second, err := CanonicalCreatePayloadJSON(shell, altPreview)
	if err != nil {
		t.Fatalf("canonical alt: %v", err)
	}
	secondHash, err := StableStoredPayloadHash(second)
	if err != nil {
		t.Fatalf("hash alt: %v", err)
	}
	if firstHash != secondHash {
		t.Fatalf("CREATE hash must ignore workbook identity first=%s second=%s", firstHash, secondHash)
	}
	update, err := ParseBuyerImportPreview(context.Background(), data, targetFromSnapshot(validImportSnapshot()))
	if err != nil || !update.ReadyToCommit {
		t.Fatalf("update parse: err=%v errors=%v", err, update.Errors)
	}
	if update.CanonicalPayloadHash == firstHash {
		t.Fatal("CREATE hash must diverge from UPDATE_EXISTING_DRAFT hash")
	}
}
