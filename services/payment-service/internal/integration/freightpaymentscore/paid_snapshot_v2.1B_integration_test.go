//go:build integration

package freightpaymentscore

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
)

func TestFC_B_PAY_OUTBOX_001_SameObligationRevisionTwoOneSnapshotEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	payment := createManualPayment(t, env, fix, "40.00")
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("partial allocate: %v", err)
	}
	count, err := env.outboxRepo.CountOutboxByAggregateVersion(ctx, fix.TenantID,
		domain.PaymentEventObligationPaidSnapshot, obligation.ID, int64(obligation.Version))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one paid_snapshot row at obligation version %d, got %d", obligation.Version, count)
	}
}

func TestFC_B_PAY_OUTBOX_002_ReplayAllocationDoesNotDuplicateSnapshot(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	payment := createManualPayment(t, env, fix, "100.00")
	input := domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}
	if _, err := env.payments.Allocate(ctx, input, buyerActor(fix)); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	reloaded, err := env.paymentRepo.GetObligationByID(ctx, fix.TenantID, obligation.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	count, err := env.outboxRepo.CountOutboxByAggregateVersion(ctx, fix.TenantID,
		domain.PaymentEventObligationPaidSnapshot, obligation.ID, int64(reloaded.Version))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one snapshot event, got %d", count)
	}
}

func TestFC_B_PAY_OUTBOX_003_RevisionIncrementCreatesSecondSnapshotEvent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	payment := createManualPayment(t, env, fix, "100.00")
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("50.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	firstVersion := obligation.Version
	countV1, _ := env.outboxRepo.CountOutboxByAggregateVersion(ctx, fix.TenantID,
		domain.PaymentEventObligationPaidSnapshot, obligation.ID, int64(firstVersion))
	if countV1 != 1 {
		t.Fatalf("expected snapshot at version %d", firstVersion)
	}
	payment2 := createManualPayment(t, env, fix, "50.00")
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment2.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("50.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("second allocate: %v", err)
	}
	reloaded, err := env.paymentRepo.GetObligationByID(ctx, fix.TenantID, obligation.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Version <= firstVersion {
		t.Fatalf("expected obligation version increment, got %d -> %d", firstVersion, reloaded.Version)
	}
	countV2, err := env.outboxRepo.CountOutboxByAggregateVersion(ctx, fix.TenantID,
		domain.PaymentEventObligationPaidSnapshot, obligation.ID, int64(reloaded.Version))
	if err != nil {
		t.Fatalf("count v2: %v", err)
	}
	if countV2 != 1 {
		t.Fatalf("expected second snapshot at version %d, got %d", reloaded.Version, countV2)
	}
	total, err := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaidSnapshot, obligation.ID)
	if err != nil {
		t.Fatalf("total count: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected at least two versioned snapshot events, got %d", total)
	}
}
