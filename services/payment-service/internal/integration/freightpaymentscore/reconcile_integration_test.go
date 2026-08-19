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

func countReconcileAudit(t *testing.T, env *env, tenantID, paymentID uuid.UUID) int {
	t.Helper()
	count, err := env.paymentRepo.CountAuditEvents(context.Background(), tenantID, "PAYMENT", paymentID, domain.AuditPaymentReconciled)
	if err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return count
}

func fullyAllocatePayment(t *testing.T, env *env, fix fixture, payment *domain.Payment, obligation *domain.PaymentObligation, amount string) {
	t.Helper()
	_, err := env.payments.Allocate(context.Background(), domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString(amount),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
}

func TestReconcileSingleAllocation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	reconciled, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil || reconciled.Status != domain.PaymentStatusReconciled {
		t.Fatalf("RECONCILE_SINGLE_ALLOCATION=FAIL: %v status=%s", err, reconciled.Status)
	}
	if reconciled.ReconciledAt == nil || reconciled.ReconciledBy == nil || *reconciled.ReconciledBy != fix.BuyerUserID {
		t.Fatal("RECONCILIATION_METADATA_RESPONSE=FAIL")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 1 {
		t.Fatal("RECONCILE_AUDIT_EXACTLY_ONCE=FAIL")
	}
}

func TestReconcileMultiAllocation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-B", "200.00")
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "300.00")
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("200.00"),
	}, buyerActor(fix))
	reconciled, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil || reconciled.Status != domain.PaymentStatusReconciled {
		t.Fatalf("MULTI_ALLOCATION_RECONCILE=FAIL: %v", err)
	}
}

func TestPartialPaymentReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("PARTIAL_PAYMENT_RECONCILE=DENY")
	}
}

func TestStaleFullyAllocatedStatusRejected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	_, _ = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payments
		SET status='FULLY_ALLOCATED', allocated_amount='100.00', unallocated_amount='0.00'
		WHERE id=$1`, payment.ID); err != nil {
		t.Fatalf("corrupt payment: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("STALE_FULLY_ALLOCATED_STATUS_REJECTED=FAIL")
	}
}

func TestStoredAllocatedMismatchRejected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payments
		SET allocated_amount='90.00', unallocated_amount='10.00', status='FULLY_ALLOCATED'
		WHERE id=$1`, payment.ID); err != nil {
		t.Fatalf("corrupt allocated: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("STORED_ALLOCATED_MISMATCH_REJECTED=FAIL")
	}
}

func TestStoredUnallocatedMismatchRejected(t *testing.T) {
	t.Skip("DB chk_payment_amounts enforces unallocated_amount = amount - allocated_amount; covered by domain unit test")
}

func TestAllocationCurrencyIntegrity(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	if _, err := env.pool.Exec(ctx, `UPDATE billing.payment_allocations SET currency_code='USD' WHERE payment_id=$1 AND voided_at IS NULL`, payment.ID); err != nil {
		t.Fatalf("corrupt currency: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("ALLOCATION_CURRENCY_INTEGRITY=FAIL")
	}
}

func TestAllocationPartyIntegrity(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	wrongPayee := uuid.New()
	if _, err := env.pool.Exec(ctx, `UPDATE billing.payment_obligations SET payee_company_id=$1 WHERE id=$2`, wrongPayee, obligation.ID); err != nil {
		t.Fatalf("corrupt party: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("ALLOCATION_PARTY_INTEGRITY=FAIL")
	}
}

func TestInvalidObligationStateReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	if _, err := env.pool.Exec(ctx, `UPDATE billing.payment_obligations SET status='CANCELLED' WHERE id=$1`, obligation.ID); err != nil {
		t.Fatalf("cancel obligation: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("INVALID_OBLIGATION_STATE_RECONCILE=DENY")
	}
}

func TestRepeatReconcileIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	first, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	auditAfterFirst := countReconcileAudit(t, env, fix.TenantID, payment.ID)
	second, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("repeat reconcile: %v", err)
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != auditAfterFirst {
		t.Fatal("DUPLICATE_RECONCILE_AUDIT=YES")
	}
	if second.Version != first.Version || !second.ReconciledAt.Equal(*first.ReconciledAt) || *second.ReconciledBy != *first.ReconciledBy {
		t.Fatal("REPEAT_RECONCILE_IDEMPOTENT=FAIL metadata changed")
	}
}

func TestReconcileAuditFailureRollback(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	err := env.paymentRepo.SimulateReconcileAuditFailureForTest(ctx, fix.TenantID, payment.ID, buyerActor(fix))
	if err == nil {
		t.Fatal("RECONCILE_AUDIT_FAILURE_ROLLBACK=FAIL expected rollback")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Status == domain.PaymentStatusReconciled || reloaded.ReconciledAt != nil {
		t.Fatal("payment must remain unreconciled after audit failure")
	}
}

func TestConcurrentDoubleReconcile(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
		}()
	}
	close(start)
	wg.Wait()
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Status != domain.PaymentStatusReconciled || countReconcileAudit(t, env, fix.TenantID, payment.ID) != 1 {
		t.Fatal("CONCURRENT_DOUBLE_RECONCILE_SAFE=FAIL")
	}
}

func TestReconcileVsAllocationVoidRace(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-RACE", "200.00")
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, _ := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	start := make(chan struct{})
	var wg sync.WaitGroup
	var reconcileErr, voidErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, reconcileErr = env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	}()
	go func() {
		defer wg.Done()
		<-start
		_, voidErr = env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "race", buyerActor(fix))
	}()
	close(start)
	wg.Wait()
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	alloc, _ := env.paymentRepo.GetAllocationByID(ctx, fix.TenantID, outcome.Result.Allocation.ID)
	if reloaded.Status == domain.PaymentStatusReconciled && alloc.VoidedAt != nil {
		t.Fatal("RECONCILE_VS_ALLOCATION_VOID_RACE_SAFE=FAIL inconsistent final state")
	}
	if reloaded.Status == domain.PaymentStatusReconciled && reconcileErr != nil {
		t.Fatal("reconcile should succeed when it wins")
	}
	if reloaded.Status != domain.PaymentStatusReconciled && voidErr != nil && reconcileErr == nil {
		t.Fatal("void should succeed when it wins")
	}
}

func TestReconcileVsAllocateRace(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-R2", "50.00")
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var reconcileErr, allocErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, reconcileErr = env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	}()
	go func() {
		defer wg.Done()
		<-start
		_, allocErr = env.payments.Allocate(ctx, domain.CreateAllocationInput{
			PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("10.00"),
		}, buyerActor(fix))
	}()
	close(start)
	wg.Wait()
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Status == domain.PaymentStatusReconciled && allocErr == nil {
		t.Fatal("RECONCILE_VS_ALLOCATE_RACE_SAFE=FAIL allocation after reconcile")
	}
	if reloaded.Status != domain.PaymentStatusReconciled && reconcileErr == nil {
		t.Fatal("RECONCILE_VS_ALLOCATE_RACE_SAFE=FAIL reconcile lost unexpectedly")
	}
}

func TestAllocateAfterReconciledDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-POST", "50.00")
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("10.00"),
	}, buyerActor(fix)); err == nil {
		t.Fatal("ALLOCATE_AFTER_RECONCILED=DENY")
	}
}

func TestCrossTenantReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	actor := buyerActor(fix)
	actor.TenantID = uuid.New()
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, actor); err == nil {
		t.Fatal("CROSS_TENANT_RECONCILE=DENY")
	}
}

func TestCrossCompanyReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	actor := domain.PaymentActorInput{
		TenantID: fix.TenantID, ActorCompanyID: uuid.New(),
		ActorKind: domain.PaymentActorBuyer, ActorUserID: fix.BuyerUserID,
	}
	_, err := env.payments.ReconcilePayment(ctx, payment.ID, actor)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("CROSS_COMPANY_RECONCILE=DENY expected forbidden, got %v", err)
	}
}

func TestCorruptReconciledRepeatRejected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_at = now(), void_reason = 'test corruption'
		WHERE payment_id = $1 AND voided_at IS NULL`, payment.ID); err != nil {
		t.Fatalf("corrupt reconciled active allocations: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("CORRUPT_RECONCILED_REPEAT_REJECTED=FAIL")
	}
}

func TestVoidedPaymentReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment := createManualPayment(t, env, fix, "100.00")
	if _, err := env.payments.VoidPayment(ctx, payment.ID, "duplicate", buyerActor(fix)); err != nil {
		t.Fatalf("void payment: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("VOIDED_PAYMENT_RECONCILE=DENY")
	}
}

func TestPaymentExternalIDUnchangedByReconcile(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	ext := "manual-ext-" + uuid.NewString()[:8]
	first, err := env.payments.CreateManualPayment(ctx, domain.CreateManualPaymentInput{
		Amount: decimal.RequireFromString("100.00"), CurrencyCode: "RUB", PaymentDate: time.Now().UTC(),
		PayerCompanyID: fix.BuyerID, PayeeCompanyID: fix.CarrierID, ExternalID: &ext,
		TenantID: fix.TenantID, CreatedBy: fix.BuyerUserID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	fullyAllocatePayment(t, env, fix, first, obligation, "100.00")
	reconciled, err := env.payments.ReconcilePayment(ctx, first.ID, buyerActor(fix))
	if err != nil || reconciled.ExternalID == nil || *reconciled.ExternalID != ext {
		t.Fatal("PAYMENT_EXTERNAL_ID_UNCHANGED_BY_RECONCILE=FAIL")
	}
}
