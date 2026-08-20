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

	voidedAt := time.Now().UTC()
	for _, corrupt := range []struct {
		name string
		mut  func(*Payment)
	}{
		{"voided_at only", func(p *Payment) { p.VoidedAt = &voidedAt }},
		{"voided_by only", func(p *Payment) { by := uuid.New(); p.VoidedBy = &by }},
		{"void_reason only", func(p *Payment) { reason := "orphan"; p.VoidReason = &reason }},
	} {
		bad := *payment
		corrupt.mut(&bad)
		if err := ValidateFirstReconcileInvariants(&bad, snapshot); err == nil {
			t.Fatalf("expected deny for %s", corrupt.name)
		}
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

	voidedAt := time.Now().UTC()
	for _, corrupt := range []struct {
		name string
		mut  func(*Payment)
	}{
		{"voided_at only", func(p *Payment) { p.VoidedAt = &voidedAt }},
		{"voided_by only", func(p *Payment) { by := uuid.New(); p.VoidedBy = &by }},
		{"void_reason only", func(p *Payment) { reason := "orphan"; p.VoidReason = &reason }},
	} {
		bad := *payment
		corrupt.mut(&bad)
		if err := ValidateReconciledIntegrity(&bad, snapshot); err == nil {
			t.Fatalf("expected deny for reconciled %s", corrupt.name)
		}
	}
}

func TestValidateReconciliationSnapshotIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		snapshot ReconciliationSnapshot
	}{
		{"invalid currency", ReconciliationSnapshot{InvalidCurrencyCount: 1}},
		{"invalid obligation tenant", ReconciliationSnapshot{InvalidObligationTenantCount: 1}},
		{"active orphan void metadata", ReconciliationSnapshot{InvalidActiveAllocationVoidMetadataCount: 1}},
		{"voided orphan void metadata", ReconciliationSnapshot{InvalidVoidedAllocationMetadataCount: 1}},
		{"both void metadata violations", ReconciliationSnapshot{
			InvalidActiveAllocationVoidMetadataCount: 1,
			InvalidVoidedAllocationMetadataCount:     1,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReconciliationSnapshotIntegrity(tc.snapshot); err == nil {
				t.Fatal("expected relational violation deny")
			}
		})
	}
}
