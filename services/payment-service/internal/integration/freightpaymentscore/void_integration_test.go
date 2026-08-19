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
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
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
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-B", "100.00")
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "100.00")
	a, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	b, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("60.00"),
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

func TestClosedRegisterPaidObligationReversalDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if _, err := env.pool.Exec(ctx, `UPDATE billing.billing_registers SET status='CLOSED' WHERE id=$1`, fix.RegisterID); err != nil {
		t.Fatalf("close register: %v", err)
	}
	_, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "attempt", buyerActor(fix))
	if err == nil {
		t.Fatal("CLOSED_REGISTER_REVERSAL=FAIL must deny PAID obligation reversal")
	}
}

func TestReconciledPaymentAllocationVoidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-REC", "100.00")
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("60.00"),
	}, buyerActor(fix))
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	_, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "attempt", buyerActor(fix))
	if err == nil {
		t.Fatal("RECONCILED_ALLOCATION_REVERSAL=FAIL must deny")
	}
}

func TestReconciledPaymentVoidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	_, err := env.payments.VoidPayment(ctx, payment.ID, "attempt", buyerActor(fix))
	if err == nil {
		t.Fatal("RECONCILED_PAYMENT_VOID=FAIL must deny")
	}
}

func TestCrossTenantVoidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	actor := buyerActor(fix)
	actor.TenantID = uuid.New()
	_, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "attempt", actor)
	if err == nil {
		t.Fatal("CROSS_TENANT_VOID=FAIL must deny")
	}
	actor = buyerActor(fix)
	actor.TenantID = uuid.New()
	_, err = env.payments.VoidPayment(ctx, payment.ID, "attempt", actor)
	if err == nil {
		t.Fatal("CROSS_TENANT_VOID=FAIL payment void must deny")
	}
}

func TestCrossCompanyVoidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	wrongActor := domain.PaymentActorInput{
		TenantID: fix.TenantID, ActorCompanyID: uuid.New(),
		ActorKind: domain.PaymentActorBuyer, ActorUserID: fix.BuyerUserID,
	}
	_, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "attempt", wrongActor)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("CROSS_COMPANY_VOID=FAIL expected forbidden, got %v", err)
	}
	_, err = env.payments.VoidPayment(ctx, payment.ID, "attempt", wrongActor)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("CROSS_COMPANY_VOID=FAIL payment void expected forbidden, got %v", err)
	}
}

func TestDoubleConcurrentAllocationVoid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	allocID := outcome.Result.Allocation.ID
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = env.payments.VoidAllocation(ctx, allocID, "concurrent", buyerActor(fix))
		}()
	}
	close(start)
	wg.Wait()
	if count := countPaymentAudit(t, env, fix.TenantID, "PAYMENT_ALLOCATION", allocID, domain.AuditAllocationVoided); count != 1 {
		t.Fatalf("DOUBLE_ALLOCATION_VOID_SAFE=FAIL audit count=%d", count)
	}
	alloc, _ := env.paymentRepo.GetAllocationByID(ctx, fix.TenantID, allocID)
	if alloc.VoidedAt == nil {
		t.Fatal("DOUBLE_ALLOCATION_VOID_SAFE=FAIL allocation must be voided")
	}
}

func TestProviderExternalIDReuseAfterVoidDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	externalID := "import-ext-" + uuid.NewString()[:8]
	paymentID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.payments (
			id, tenant_id, payment_number, payer_company_id, payee_company_id,
			amount, currency_code, payment_date, source, external_id, status,
			allocated_amount, unallocated_amount, created_by
		) VALUES ($1,$2,$3,$4,$5,'100.00','RUB',$6,'IMPORT',$7,'RECEIVED','0.00','100.00',$8)`,
		paymentID, fix.TenantID, "PAY-IMP-"+paymentID.String()[:8], fix.BuyerID, fix.CarrierID,
		time.Now().UTC(), externalID, fix.BuyerUserID); err != nil {
		t.Fatalf("seed import payment: %v", err)
	}
	if _, err := env.payments.VoidPayment(ctx, paymentID, "duplicate import", buyerActor(fix)); err != nil {
		t.Fatalf("void import payment: %v", err)
	}
	dupID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO billing.payments (
			id, tenant_id, payment_number, payer_company_id, payee_company_id,
			amount, currency_code, payment_date, source, external_id, status,
			allocated_amount, unallocated_amount, created_by
		) VALUES ($1,$2,$3,$4,$5,'50.00','RUB',$6,'IMPORT',$7,'RECEIVED','0.00','50.00',$8)`,
		dupID, fix.TenantID, "PAY-IMP2-"+dupID.String()[:8], fix.BuyerID, fix.CarrierID,
		time.Now().UTC(), externalID, fix.BuyerUserID)
	if err == nil {
		t.Fatal("PROVIDER_EXTERNAL_ID_REUSE_AFTER_VOID=DENY expected unique violation")
	}
}

func TestManualExternalIDReuseAfterVoidAllowed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	externalID := "manual-ext-" + uuid.NewString()[:8]
	first, err := env.payments.CreateManualPayment(ctx, domain.CreateManualPaymentInput{
		Amount: decimal.RequireFromString("100.00"), CurrencyCode: "RUB", PaymentDate: time.Now().UTC(),
		PayerCompanyID: fix.BuyerID, PayeeCompanyID: fix.CarrierID, ExternalID: &externalID,
		TenantID: fix.TenantID, CreatedBy: fix.BuyerUserID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create manual with external id: %v", err)
	}
	if _, err := env.payments.VoidPayment(ctx, first.ID, "duplicate manual", buyerActor(fix)); err != nil {
		t.Fatalf("void manual payment: %v", err)
	}
	second, err := env.payments.CreateManualPayment(ctx, domain.CreateManualPaymentInput{
		Amount: decimal.RequireFromString("50.00"), CurrencyCode: "RUB", PaymentDate: time.Now().UTC(),
		PayerCompanyID: fix.BuyerID, PayeeCompanyID: fix.CarrierID, ExternalID: &externalID,
		TenantID: fix.TenantID, CreatedBy: fix.BuyerUserID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("MANUAL_EXTERNAL_ID_POLICY=PRESERVED_ACTIVE_ONLY expected reuse allowed, got %v", err)
	}
	if second.ExternalID == nil || *second.ExternalID != externalID {
		t.Fatal("MANUAL_EXTERNAL_ID_POLICY=FAIL external id not preserved on new active payment")
	}
}
