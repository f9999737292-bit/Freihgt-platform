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
	"github.com/freight-platform/payment-service/internal/repository"
	"github.com/freight-platform/payment-service/internal/service"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

func countPaymentAudit(t *testing.T, env *env, tenantID uuid.UUID, entityType string, entityID uuid.UUID, eventType string) int {
	t.Helper()
	count, err := env.paymentRepo.CountAuditEvents(context.Background(), tenantID, entityType, entityID, eventType)
	if err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return count
}

func TestPartialObligationReversal(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	result, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "duplicate allocation", buyerActor(fix))
	if err != nil {
		t.Fatalf("PARTIAL_OBLIGATION_REVERSAL=FAIL: %v", err)
	}
	if result.Obligation.Status != domain.ObligationStatusOpen {
		t.Fatalf("expected OPEN obligation, got %s", result.Obligation.Status)
	}
	if !result.Payment.UnallocatedAmount.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("PAYMENT_BALANCE_RECOMPUTE=FAIL unallocated=%s", result.Payment.UnallocatedAmount)
	}
}

func TestRepeatAllocationVoidIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	allocID := outcome.Result.Allocation.ID
	first, err := env.payments.VoidAllocation(ctx, allocID, "mistake", buyerActor(fix))
	if err != nil {
		t.Fatalf("first void: %v", err)
	}
	auditAfterFirst := countPaymentAudit(t, env, fix.TenantID, "PAYMENT_ALLOCATION", allocID, domain.AuditAllocationVoided)
	second, err := env.payments.VoidAllocation(ctx, allocID, "other reason", buyerActor(fix))
	if err != nil {
		t.Fatalf("repeat void: %v", err)
	}
	if auditAfterSecond := countPaymentAudit(t, env, fix.TenantID, "PAYMENT_ALLOCATION", allocID, domain.AuditAllocationVoided); auditAfterSecond != auditAfterFirst {
		t.Fatal("DUPLICATE_VOID_AUDIT=YES")
	}
	if second.Allocation.VoidReason == nil || *second.Allocation.VoidReason != *first.Allocation.VoidReason {
		t.Fatal("REPEAT_ALLOCATION_VOID_IDEMPOTENT=FAIL reason must not change")
	}
}

func TestPaidObligationReversalDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	_, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "attempt", buyerActor(fix))
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("PAID_OBLIGATION_REVERSAL=FAIL expected conflict, got %v", err)
	}
}

func TestPaidObligationPendingOutboxReversalDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	env.payments = service.NewPaymentService(env.paymentRepo, repository.NewBillingRegisterLookupRepository(env.pool), repository.NewMembershipRepository(env.pool), failingBillingSync{}, env.outboxRepo)
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if outcome.Result.Obligation.Status != domain.ObligationStatusPaid {
		t.Fatal("obligation must be PAID")
	}
	event, _ := env.outboxRepo.GetOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if event == nil || event.Status != domain.PaymentOutboxStatusPending {
		t.Fatal("outbox must remain pending")
	}
	_, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "attempt", buyerActor(fix))
	if err == nil {
		t.Fatal("PAID_PENDING_PROJECTION_REVERSAL=FAIL")
	}
}

func TestReceivedZeroAllocationPaymentVoid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment := createManualPayment(t, env, fix, "100.00")
	voided, err := env.payments.VoidPayment(ctx, payment.ID, "duplicate manual payment", buyerActor(fix))
	if err != nil || voided.Status != domain.PaymentStatusVoided {
		t.Fatalf("RECEIVED_ZERO_ALLOCATION_VOID=FAIL: %v status=%s", err, voided.Status)
	}
}

func TestRepeatPaymentVoidIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment := createManualPayment(t, env, fix, "50.00")
	first, _ := env.payments.VoidPayment(ctx, payment.ID, "duplicate", buyerActor(fix))
	auditAfterFirst := countPaymentAudit(t, env, fix.TenantID, "PAYMENT", payment.ID, domain.AuditPaymentVoided)
	second, err := env.payments.VoidPayment(ctx, payment.ID, "other", buyerActor(fix))
	if err != nil {
		t.Fatalf("repeat payment void: %v", err)
	}
	if auditAfterSecond := countPaymentAudit(t, env, fix.TenantID, "PAYMENT", payment.ID, domain.AuditPaymentVoided); auditAfterSecond != auditAfterFirst {
		t.Fatal("DUPLICATE_PAYMENT_VOID_AUDIT=YES")
	}
	if second.VoidReason == nil || first.VoidReason == nil || *second.VoidReason != *first.VoidReason {
		t.Fatal("REPEAT_PAYMENT_VOID_IDEMPOTENT=FAIL")
	}
}

func TestPartialPaymentVoidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	_, err := env.payments.VoidPayment(ctx, payment.ID, "attempt", buyerActor(fix))
	if err == nil {
		t.Fatal("PARTIAL_PAYMENT_VOID=FAIL must deny")
	}
}

func TestAllocationVoidAuditFailureRollback(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	err := env.paymentRepo.SimulateAllocationVoidAuditFailureForTest(ctx, domain.VoidAllocationInput{
		TenantID: fix.TenantID, AllocationID: outcome.Result.Allocation.ID, Reason: "test",
		ActorUserID: fix.BuyerUserID, ActorCompanyID: fix.BuyerID, ActorKind: domain.PaymentActorBuyer,
	})
	if err == nil {
		t.Fatal("ALLOCATION_VOID_AUDIT_FAILURE_ROLLBACK=FAIL expected rollback")
	}
	alloc, _ := env.paymentRepo.GetAllocationByID(ctx, fix.TenantID, outcome.Result.Allocation.ID)
	if alloc.VoidedAt != nil {
		t.Fatal("allocation must not be voided after audit failure")
	}
}

func TestPaymentVoidAuditFailureRollback(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment := createManualPayment(t, env, fix, "100.00")
	err := env.paymentRepo.SimulatePaymentVoidAuditFailureForTest(ctx, domain.VoidPaymentInput{
		TenantID: fix.TenantID, PaymentID: payment.ID, Reason: "test",
		ActorUserID: fix.BuyerUserID, ActorCompanyID: fix.BuyerID, ActorKind: domain.PaymentActorBuyer,
	})
	if err == nil {
		t.Fatal("PAYMENT_VOID_AUDIT_FAILURE_ROLLBACK=FAIL expected rollback")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Status == domain.PaymentStatusVoided {
		t.Fatal("payment must not be voided after audit failure")
	}
}

func TestMigration047Postgres(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	var exists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'billing' AND table_name = 'payment_allocations' AND column_name = 'voided_by'
		)`).Scan(&exists); err != nil || !exists {
		t.Fatalf("POSTGRES_MIGRATION_TEST=FAIL voided_by missing: exists=%v err=%v", exists, err)
	}
}

func TestOutboxUnchangedAfterPartialVoidAttempt(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	countBefore, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	_, _ = env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "fix", buyerActor(fix))
	countAfter, _ := env.outboxRepo.CountOutboxByAggregate(ctx, fix.TenantID, domain.PaymentEventObligationPaid, obligation.ID)
	if countBefore != countAfter {
		t.Fatal("PAYMENT_OUTBOX_MODIFIED_BY_REVERSAL=YES")
	}
}

func TestConcurrentMultiAllocationVoid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	a, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	b, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("60.00"),
	}, buyerActor(fix))
	var wg sync.WaitGroup
	for _, allocID := range []uuid.UUID{a.Result.Allocation.ID, b.Result.Allocation.ID} {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			_, _ = env.payments.VoidAllocation(ctx, id, "concurrent", buyerActor(fix))
		}(allocID)
	}
	wg.Wait()
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if !reloaded.UnallocatedAmount.Equal(decimal.RequireFromString("100.00")) || reloaded.Status != domain.PaymentStatusReceived {
		t.Fatalf("CONCURRENT_MULTI_ALLOCATION_VOID_SAFE=FAIL status=%s unallocated=%s", reloaded.Status, reloaded.UnallocatedAmount)
	}
}

func TestVoidVsAllocateRace(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	start := make(chan struct{})
	var wg sync.WaitGroup
	var allocErr, voidErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, allocErr = env.payments.Allocate(ctx, domain.CreateAllocationInput{
			PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
		}, buyerActor(fix))
	}()
	go func() {
		defer wg.Done()
		<-start
		_, voidErr = env.payments.VoidPayment(ctx, payment.ID, "race", buyerActor(fix))
	}()
	close(start)
	wg.Wait()
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Status == domain.PaymentStatusVoided && allocErr == nil {
		t.Fatal("VOID_VS_ALLOCATE_RACE_SAFE=FAIL voided with active allocation")
	}
	if reloaded.Status != domain.PaymentStatusVoided && voidErr == nil {
		t.Fatal("VOID_VS_ALLOCATE_RACE_SAFE=FAIL allocated with voided payment")
	}
	_ = voidErr
}
