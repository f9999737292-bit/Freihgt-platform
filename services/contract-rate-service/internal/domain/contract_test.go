package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateActivateContract(t *testing.T) {
	today := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	validTo := today.AddDate(0, 1, 0)
	c := &TransportContract{Status: ContractStatusDraft, ValidFrom: today, ValidTo: &validTo}
	if err := ValidateActivateContract(c, today); err != nil {
		t.Fatalf("expected activate allowed: %v", err)
	}
}

func TestValidateReactivateExpiredDenied(t *testing.T) {
	c := &TransportContract{Status: ContractStatusExpired}
	if err := ValidateReactivateContract(c, time.Now().UTC()); err == nil {
		t.Fatal("expected expired reactivation deny")
	}
}

func TestShouldExpireContract(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	onDate := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if !ShouldExpireContract(ContractStatusActive, onDate, &past) {
		t.Fatal("expected expiration")
	}
}

func TestValidateCurrencyCode(t *testing.T) {
	if err := ValidateCurrencyCode("RUB"); err != nil {
		t.Fatalf("expected valid currency: %v", err)
	}
	if err := ValidateCurrencyCode("RU"); err == nil {
		t.Fatal("expected invalid currency deny")
	}
}

func TestValidateCreateContractInputParties(t *testing.T) {
	id := uuid.New()
	err := ValidateCreateContractInput(CreateContractInput{
		TenantID: id, BuyerCompanyID: id, CarrierCompanyID: id,
		ContractNumber: "C-1", Name: "Test", ValidFrom: time.Now().UTC(), CurrencyCode: "RUB",
	})
	if err == nil {
		t.Fatal("expected buyer=carrier deny")
	}
}
