package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateImportAnalysisPreviewInput(t *testing.T) {
	valid := ImportAnalysis{
		TenantID:             uuid.New(),
		ActorID:              uuid.New(),
		ActorCompanyID:       uuid.New(),
		WorkbookType:         WorkbookTypeBuyerTender,
		SchemaVersion:        SchemaVersionBuyerXLSXV1,
		TargetType:           ImportTargetTypeNewEvent,
		CanonicalPayloadJSON: []byte(`{"event":{"title":"T"}}`),
		CanonicalHash:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ExpiresAt:            time.Now().UTC().Add(time.Hour),
	}
	if err := ValidateImportAnalysisPreviewInput(valid); err != nil {
		t.Fatalf("valid preview input rejected: %v", err)
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
