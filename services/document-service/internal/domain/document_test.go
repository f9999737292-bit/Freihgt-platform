package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateDocumentInput(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if err := ValidateCreateDocumentInput(CreateDocumentInput{
		TenantID: tenantID, DocumentNumber: "DOC-1", DocumentType: "ETRN",
		OwnerCompanyID: uuid.New(), LegalLanguage: "ru-RU",
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if err := ValidateCreateDocumentInput(CreateDocumentInput{}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateDocumentType(t *testing.T) {
	t.Parallel()
	if err := ValidateDocumentType("ETRN"); err != nil {
		t.Fatalf("expected valid type")
	}
	if err := ValidateDocumentType("INVALID"); err == nil {
		t.Fatalf("expected invalid document_type error")
	}
}

func TestValidateCreateVersionStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateCreateVersionStatus(DocumentStatusDraft); err != nil {
		t.Fatalf("expected DRAFT allowed")
	}
	if err := ValidateCreateVersionStatus(DocumentStatusSigned); err == nil {
		t.Fatalf("expected error for SIGNED")
	}
}

func TestValidateReadyForSigningStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateReadyForSigningStatus(DocumentStatusDraft); err != nil {
		t.Fatalf("expected DRAFT allowed")
	}
	if err := ValidateReadyForSigningStatus(DocumentStatusSigned); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateCancelDocumentStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateCancelDocumentStatus(DocumentStatusSigned); err == nil {
		t.Fatalf("expected cannot cancel SIGNED")
	}
}

func TestValidateArchiveDocumentStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateArchiveDocumentStatus(DocumentStatusSigned); err != nil {
		t.Fatalf("expected SIGNED allowed")
	}
	if err := ValidateArchiveDocumentStatus(DocumentStatusDraft); err == nil {
		t.Fatalf("expected error for DRAFT")
	}
}

func TestResolveSigningAfterSignature(t *testing.T) {
	t.Parallel()
	sessionStatus, docStatus := ResolveSigningAfterSignature(2, 2)
	if sessionStatus != SigningSessionStatusCompleted || docStatus != DocumentStatusSigned {
		t.Fatalf("expected completed signing")
	}
	sessionStatus, docStatus = ResolveSigningAfterSignature(2, 1)
	if sessionStatus != SigningSessionStatusInProgress || docStatus != DocumentStatusSigningInProgress {
		t.Fatalf("expected in progress signing")
	}
}
