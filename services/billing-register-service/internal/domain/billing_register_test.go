package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateCreateBillingRegisterInput(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	if err := ValidateCreateBillingRegisterInput(CreateBillingRegisterInput{
		TenantID: uuid.New(), RegisterNumber: "BR-1", CustomerCompanyID: uuid.New(),
		ContractorCompanyID: uuid.New(), PeriodFrom: from, PeriodTo: to,
	}); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestValidatePeriodToBeforeFrom(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := ValidateCreateBillingRegisterInput(CreateBillingRegisterInput{
		TenantID: uuid.New(), RegisterNumber: "BR-1", CustomerCompanyID: uuid.New(),
		ContractorCompanyID: uuid.New(), PeriodFrom: from, PeriodTo: to,
	}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestCalculateItemAmounts(t *testing.T) {
	t.Parallel()
	vat := 20.0
	amounts := CalculateItemAmounts(100000, 5000, 0, &vat)
	if amounts.AmountWithoutVAT != 105000 {
		t.Fatalf("expected 105000, got %v", amounts.AmountWithoutVAT)
	}
	if amounts.VATAmount != 21000 {
		t.Fatalf("expected 21000, got %v", amounts.VATAmount)
	}
	if amounts.AmountWithVAT != 126000 {
		t.Fatalf("expected 126000, got %v", amounts.AmountWithVAT)
	}
}

func TestValidateApproveRegisterStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateApproveRegisterStatus(RegisterStatusCalculated); err != nil {
		t.Fatalf("expected CALCULATED allowed")
	}
	if err := ValidateApproveRegisterStatus(RegisterStatusDraft); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateMarkPaidStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateMarkPaidStatus(RegisterStatusSignedByCounterparty); err != nil {
		t.Fatalf("expected SIGNED_BY_COUNTERPARTY allowed")
	}
	if err := ValidateMarkPaidStatus(RegisterStatusApproved); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateCloseRegisterStatus(t *testing.T) {
	t.Parallel()
	if err := ValidateCloseRegisterStatus(RegisterStatusPaid); err != nil {
		t.Fatalf("expected PAID allowed")
	}
	if err := ValidateCloseRegisterStatus(RegisterStatusApproved); err == nil {
		t.Fatalf("expected validation error")
	}
}
