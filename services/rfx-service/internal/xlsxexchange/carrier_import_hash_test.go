package xlsxexchange

import (
	"testing"

	"github.com/google/uuid"
)

func TestStableStoredCarrierPayloadRoundTrip(t *testing.T) {
	target := TargetCarrierBaseline{
		TenantID:                   uuid.New(),
		EventID:                    uuid.New(),
		ResponseID:                 uuid.New(),
		CarrierCompanyID:           uuid.New(),
		RfxVersionID:               uuid.New(),
		QuestionnaireVersionNumber: 1,
		ResponseSaveVersion:        2,
		EventRowVersion:            3,
		AvailableLotsFingerprint:   "abc",
		AnswersFingerprint:         "def",
		OfferLinesFingerprint:      "ghi",
	}
	preview := CarrierImportPreview{
		SchemaName:    "BINTRANS_RFX_CARRIER_XLSX_V1",
		SchemaVersion: "1",
		Mode:          CarrierImportModeUpdateDraft,
		ReadyToCommit: true,
	}
	proposal := CarrierImportProposal{
		Answers: []CarrierAnswerPatch{{QuestionCode: "Q1", Value: []byte(`"YES"`)}},
		OfferLines: []CarrierOfferLinePatch{{
			LotNumber: "1", Amount: 10, CurrencyCode: "RUB",
		}},
	}
	raw, err := CanonicalCarrierImportPayloadJSON(preview, target, proposal)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	hash, err := StableStoredCarrierPayloadHash(raw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := VerifyStoredCarrierCanonicalPayloadHash(raw, hash); err != nil {
		t.Fatalf("verify: %v", err)
	}
	_, rehash, err := StableStoredCarrierPayload(raw)
	if err != nil {
		t.Fatalf("stable payload: %v", err)
	}
	if rehash != hash {
		t.Fatalf("hash mismatch after round trip")
	}
}
