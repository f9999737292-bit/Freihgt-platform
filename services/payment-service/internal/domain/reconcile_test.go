package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestValidateFirstReconcileInvariants(t *testing.T) {
	amount := decimal.RequireFromString("100.00")
	snapshot := ReconciliationSnapshot{
		ActiveAllocationCount: 1,
		ActiveAllocationSum:   amount,
	}
	payment := &Payment{
		Amount:            amount,
		AllocatedAmount:   amount,
		UnallocatedAmount: decimal.Zero,
		Status:            PaymentStatusFullyAllocated,
	}

	if err := ValidateFirstReconcileInvariants(payment, snapshot); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	for _, status := range []string{PaymentStatusReceived, PaymentStatusPartiallyAllocated, PaymentStatusVoided} {
		bad := *payment
		bad.Status = status
		if err := ValidateFirstReconcileInvariants(&bad, snapshot); err == nil {
			t.Fatalf("expected deny for status %s", status)
		}
	}

	mismatch := *payment
	mismatch.AllocatedAmount = decimal.RequireFromString("90.00")
	if err := ValidateFirstReconcileInvariants(&mismatch, snapshot); err == nil {
		t.Fatal("expected stored allocated mismatch deny")
	}

	unallocated := *payment
	unallocated.UnallocatedAmount = decimal.RequireFromString("10.00")
	if err := ValidateFirstReconcileInvariants(&unallocated, snapshot); err == nil {
		t.Fatal("expected unallocated mismatch deny")
	}
}

func TestValidateReconciledIntegrity(t *testing.T) {
	amount := decimal.RequireFromString("100.00")
	now := time.Now().UTC()
	actor := uuid.New()
	reconciledBy := actor
	payment := &Payment{
		Amount:            amount,
		AllocatedAmount:   amount,
		UnallocatedAmount: decimal.Zero,
		Status:            PaymentStatusReconciled,
		ReconciledAt:      &now,
		ReconciledBy:      &reconciledBy,
	}
	snapshot := ReconciliationSnapshot{
		ActiveAllocationCount: 1,
		ActiveAllocationSum:   amount,
	}
	if err := ValidateReconciledIntegrity(payment, snapshot); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	corrupt := *payment
	corrupt.ReconciledAt = nil
	if err := ValidateReconciledIntegrity(&corrupt, snapshot); err == nil {
		t.Fatal("expected corrupt reconciled metadata deny")
	}

	sumMismatch := ReconciliationSnapshot{
		ActiveAllocationCount: 1,
		ActiveAllocationSum:   decimal.RequireFromString("50.00"),
	}
	if err := ValidateReconciledIntegrity(payment, sumMismatch); err == nil {
		t.Fatal("expected corrupt reconciled active sum deny")
	}
}

func TestValidateReconciliationSnapshotIntegrity(t *testing.T) {
	snapshot := ReconciliationSnapshot{InvalidCurrencyCount: 1}
	if err := ValidateReconciliationSnapshotIntegrity(snapshot); err == nil {
		t.Fatal("expected relational violation deny")
	}
}
