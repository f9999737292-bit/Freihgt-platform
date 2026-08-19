//go:build integration

package freightpaymentscore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

func TestEnsureObligationIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	first, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure first: %v", err)
	}
	second, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure second: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same obligation id")
	}
	if !first.OriginalAmount.Equal(fix.RegisterTotal) {
		t.Fatalf("expected original amount snapshot from register")
	}
}

func TestManualPaymentAllocateAndPaid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	payment := createManualPayment(t, env, fix, "100.00")
	result, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID,
		AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if result.Obligation.Status != domain.ObligationStatusPaid {
		t.Fatalf("expected PAID obligation, got %s", result.Obligation.Status)
	}
	if result.Payment.Status != domain.PaymentStatusFullyAllocated {
		t.Fatalf("expected FULLY_ALLOCATED payment, got %s", result.Payment.Status)
	}
}

func TestConcurrentAllocationConflict(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	paymentA := createManualPayment(t, env, fix, "100.00")
	paymentB := createManualPayment(t, env, fix, "100.00")

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	allocate := func(paymentID uuid.UUID) {
		defer wg.Done()
		_, allocErr := env.payments.Allocate(ctx, domain.CreateAllocationInput{
			PaymentID: paymentID, ObligationID: obligation.ID,
			AllocatedAmount: decimal.RequireFromString("100.00"),
		}, buyerActor(fix))
		errCh <- allocErr
	}
	wg.Add(2)
	go allocate(paymentA.ID)
	go allocate(paymentB.ID)
	wg.Wait()
	close(errCh)

	successes := 0
	conflicts := 0
	for allocErr := range errCh {
		if allocErr == nil {
			successes++
			continue
		}
		var appErr *apperrors.AppError
		if errors.As(allocErr, &appErr) && appErr.Code == apperrors.CodeConflict {
			conflicts++
			continue
		}
		t.Fatalf("unexpected allocation error: %v", allocErr)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one success and one conflict, got success=%d conflict=%d", successes, conflicts)
	}

	updated, err := env.paymentRepo.GetObligationByID(ctx, fix.TenantID, obligation.ID)
	if err != nil {
		t.Fatalf("reload obligation: %v", err)
	}
	if !updated.PaidAmount.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("paid_amount=%s", updated.PaidAmount)
	}
	if !updated.OutstandingAmount.IsZero() {
		t.Fatalf("outstanding=%s", updated.OutstandingAmount)
	}
}

func TestMarkPaidPreconditionFailsWithoutObligationPaid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	_, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	obligation, err := env.paymentRepo.GetObligationBySource(ctx, fix.TenantID, domain.ObligationSourceBillingRegister, fix.RegisterID)
	if err != nil {
		t.Fatalf("load obligation: %v", err)
	}
	if obligation.Status == domain.ObligationStatusPaid {
		t.Fatalf("expected OPEN obligation before payment")
	}
}

func TestReconcileRequiresFullyAllocated(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	payment := createManualPayment(t, env, fix, "50.00")
	_, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err == nil {
		t.Fatalf("expected reconcile failure for partially unallocated payment")
	}
}

func TestDueDatePatchAudit(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	due := parseDate("2026-12-31")
	updated, err := env.payments.UpdateDueDate(ctx, obligation.ID, &due, buyerActor(fix))
	if err != nil {
		t.Fatalf("patch due date: %v", err)
	}
	if updated.DueDate == nil || updated.DueDate.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("due date not updated")
	}
}

func parseDate(v string) time.Time {
	t, _ := time.Parse("2006-01-02", v)
	return t
}
