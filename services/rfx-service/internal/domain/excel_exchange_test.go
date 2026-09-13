package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateImportAnalysisPreviewInput(t *testing.T) {
	payload := []byte(`{"event":{"title":"T"}}`)
	sum := sha256.Sum256(payload)
	valid := ImportAnalysis{
		TenantID:             uuid.New(),
		ActorID:              uuid.New(),
		ActorCompanyID:       uuid.New(),
		WorkbookType:         WorkbookTypeBuyerTender,
		SchemaVersion:        SchemaVersionBuyerXLSXV1,
		TargetType:           ImportTargetTypeNewEvent,
		CanonicalPayloadJSON: payload,
		CanonicalHash:        hex.EncodeToString(sum[:]),
		CreatedAt:            time.Now().UTC(),
		ExpiresAt:            time.Now().UTC().Add(time.Hour),
	}
	if err := ValidateImportAnalysisPreviewInput(valid); err != nil {
		t.Fatalf("valid preview input rejected: %v", err)
	}
}

func TestVerifyImportAnalysisCanonicalHashMatch(t *testing.T) {
	payload := []byte(`{"schema_name":"BINTRANS_RFX_BUYER_XLSX_V1"}`)
	valid := ImportAnalysis{
		TenantID: uuid.New(), ActorID: uuid.New(), ActorCompanyID: uuid.New(),
		WorkbookType: WorkbookTypeBuyerTender, SchemaVersion: SchemaVersionBuyerXLSXV1,
		TargetType: ImportTargetTypeDraftEvent, CanonicalPayloadJSON: payload,
		CanonicalHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CreatedAt:     time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := ValidateImportAnalysisPreviewInput(valid); err == nil {
		t.Fatal("expected hash mismatch rejection")
	}
}

func TestValidateSchemaVersionMismatch(t *testing.T) {
	in := ImportAnalysis{
		TenantID:             uuid.New(),
		ActorID:              uuid.New(),
		ActorCompanyID:       uuid.New(),
		WorkbookType:         WorkbookTypeCarrierOffer,
		SchemaVersion:        SchemaVersionBuyerXLSXV1,
		TargetType:           ImportTargetTypeCarrierResponse,
		CanonicalPayloadJSON: []byte(`{}`),
		CanonicalHash:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ExpiresAt:            time.Now().UTC().Add(time.Hour),
	}
	if err := ValidateImportAnalysisPreviewInput(in); err == nil {
		t.Fatal("expected carrier/buyer schema mismatch rejection")
	}
}
