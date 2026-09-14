package xlsxexchange

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestParseCarrierImportPreviewRejectsCompetitorColumn(t *testing.T) {
	snapshot := richCarrierSnapshot()
	data, err := GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	data = mutateCarrierImportWorkbook(t, data, func(f *excelize.File) {
		_ = f.SetCellStr(sheetAnswers, "C1", "competitor_rate")
	})
	target := carrierImportTargetFromSnapshot(snapshot)
	preview, err := ParseCarrierImportPreview(context.Background(), data, target)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.ReadyToCommit {
		t.Fatal("expected invalid preview")
	}
	found := false
	for _, issue := range preview.Errors {
		if issue.MachineCode == MachineCodeCompetitorColumnDenied {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected competitor column issue, got %+v", preview.Errors)
	}
}

func TestClassifyCarrierPreviewErrorsSchemaMismatchIsDomain(t *testing.T) {
	class := ClassifyCarrierPreviewErrors([]BuyerImportIssue{{
		MachineCode: MachineCodeUnsupportedSchema,
		Severity:    IssueSeverityError,
	}})
	if class != PreviewErrorClassDomain {
		t.Fatalf("class=%v want domain", class)
	}
}

func TestClassifyCarrierPreviewErrorsCompetitorColumnIsDomain(t *testing.T) {
	class := ClassifyCarrierPreviewErrors([]BuyerImportIssue{{
		MachineCode: MachineCodeCompetitorColumnDenied,
		Severity:    IssueSeverityError,
	}})
	if class != PreviewErrorClassDomain {
		t.Fatalf("class=%v want domain", class)
	}
}

func TestClassifyCarrierPreviewErrorsOfferValidationIsDomain(t *testing.T) {
	class := ClassifyCarrierPreviewErrors([]BuyerImportIssue{{
		MachineCode: MachineCodeInvalidType,
		MessageKey:  "rfx.carrier_xlsx_import.invalid_offer_line",
		Severity:    IssueSeverityError,
	}})
	if class != PreviewErrorClassDomain {
		t.Fatalf("class=%v want domain", class)
	}
}

func carrierImportTargetFromSnapshot(snapshot CarrierResponseSnapshot) TargetCarrierBaseline {
	lotID := uuid.New()
	lots := snapshot.Lots
	if len(lots) > 0 {
		lots[0].ID = lotID
	}
	question := snapshot.Questions[0].Question
	return TargetCarrierBaseline{
		TenantID:                   snapshot.Metadata.TenantID,
		EventID:                    snapshot.Metadata.RfxEventID,
		ResponseID:                 snapshot.Metadata.RfxResponseID,
		CarrierCompanyID:           snapshot.Metadata.CarrierCompanyID,
		RfxVersionID:               snapshot.Metadata.RfxVersionID,
		QuestionnaireVersionNumber: snapshot.Metadata.QuestionnaireVersionNumber,
		ResponseSaveVersion:        snapshot.Metadata.ResponseSaveVersion,
		ResponseStatus:             snapshot.Metadata.ResponseStatus,
		EventRowVersion:            snapshot.Metadata.EventRowVersion,
		EventCurrency:              "RUB",
		LotCount:                   len(lots),
		AvailableLotsFingerprint:   snapshot.Metadata.AvailableLotsFingerprint,
		AnswersFingerprint:         snapshot.Metadata.AnswersFingerprint,
		OfferLinesFingerprint:      snapshot.Metadata.OfferLinesFingerprint,
		Questionnaire: domain.QuestionnaireDefinition{
			Sections: []domain.SectionWithQuestions{{
				Section:   domain.Section{SectionCode: snapshot.Questions[0].SectionCode, Title: "S"},
				Questions: []domain.Question{question},
			}},
		},
		Lots:               lots,
		BaselineAnswers:    snapshot.Answers,
		BaselineOfferLines: snapshot.OfferLines,
	}
}

func mutateCarrierImportWorkbook(t *testing.T, data []byte, fn func(*excelize.File)) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	fn(f)
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	return buf.Bytes()
}
