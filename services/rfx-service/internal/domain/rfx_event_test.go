package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateCreateRfxEventInput(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	deadline := time.Now().UTC().Add(24 * time.Hour)

	if err := ValidateCreateRfxEventInput(CreateRfxEventInput{
		TenantID: tenantID, RfxNumber: "RFX-1", RfxType: "RFQ", Category: "FREIGHT",
		Title: "Test", OwnerCompanyID: companyID, ResponseDeadline: &deadline,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if err := ValidateCreateRfxEventInput(CreateRfxEventInput{
		RfxNumber: "RFX-1", RfxType: "RFQ", Category: "FREIGHT", Title: "Test", OwnerCompanyID: companyID,
	}); err == nil {
		t.Fatalf("expected tenant_id validation error")
	}
}

func TestValidatePublishRfxEvent(t *testing.T) {
	t.Parallel()
	if err := ValidatePublishRfxEvent(RfxStatusDraft); err != nil {
		t.Fatalf("expected publish from DRAFT")
	}
	if err := ValidatePublishRfxEvent(RfxStatusPublished); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateCancelRfxEvent(t *testing.T) {
	t.Parallel()
	if err := ValidateCancelRfxEvent(RfxStatusDraft); err != nil {
		t.Fatalf("expected cancel from DRAFT")
	}
	if err := ValidateCancelRfxEvent(RfxStatusPublished); err != nil {
		t.Fatalf("expected cancel from PUBLISHED")
	}
	if err := ValidateCancelRfxEvent("ASSIGNED"); err == nil {
		t.Fatalf("expected validation error")
	}
}
