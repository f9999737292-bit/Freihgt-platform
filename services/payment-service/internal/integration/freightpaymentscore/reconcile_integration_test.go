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
	var err1, err2 error
	var result1, result2 *domain.Payment
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		result1, err1 = env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	}()
	go func() {
		defer wg.Done()
		<-start
		result2, err2 = env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	}()
	close(start)
	wg.Wait()
	if err1 != nil || err2 != nil {
		t.Fatalf("CONCURRENT_RECONCILE_REQUEST: err1=%v err2=%v", err1, err2)
	}
	if result1 == nil || result2 == nil ||
		result1.Status != domain.PaymentStatusReconciled || result2.Status != domain.PaymentStatusReconciled {
		t.Fatal("CONCURRENT_RECONCILE_REQUEST: both must succeed with RECONCILED")
	}
	if result1.ReconciledAt == nil || result2.ReconciledAt == nil ||
		!result1.ReconciledAt.Equal(*result2.ReconciledAt) ||
		result1.ReconciledBy == nil || result2.ReconciledBy == nil ||
		*result1.ReconciledBy != *result2.ReconciledBy ||
		result1.Version != result2.Version {
		t.Fatal("CONCURRENT_DOUBLE_RECONCILE_SAFE=FAIL metadata mismatch between requests")
	}
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
	first, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil {
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
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Version != first.Version ||
		reloaded.ReconciledAt == nil || !reloaded.ReconciledAt.Equal(*first.ReconciledAt) ||
		reloaded.ReconciledBy == nil || *reloaded.ReconciledBy != *first.ReconciledBy {
		t.Fatal("CORRUPT_RECONCILED_REPEAT=FAIL metadata rewritten")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 1 {
		t.Fatal("CORRUPT_RECONCILED_REPEAT=FAIL duplicate audit")
	}
}

func TestCrossTenantObligationLinkReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligationA, "100.00")
	obligationB := seedCrossTenantObligation(t, env.pool, fix, "100.00")
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET obligation_id = $1
		WHERE payment_id = $2 AND voided_at IS NULL`, obligationB, payment.ID); err != nil {
		t.Fatalf("corrupt obligation link: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("CROSS_TENANT_OBLIGATION_LINK_RECONCILE=DENY")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Status != domain.PaymentStatusFullyAllocated ||
		reloaded.ReconciledAt != nil || reloaded.ReconciledBy != nil {
		t.Fatal("CROSS_TENANT_OBLIGATION_LINK_RECONCILE=FAIL payment state changed")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 0 {
		t.Fatal("CROSS_TENANT_OBLIGATION_LINK_RECONCILE=FAIL audit created")
	}
}

func TestCorruptReconciledCrossTenantRepeatDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligationA, "100.00")
	first, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	obligationB := seedCrossTenantObligation(t, env.pool, fix, "100.00")
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET obligation_id = $1
		WHERE payment_id = $2 AND voided_at IS NULL`, obligationB, payment.ID); err != nil {
		t.Fatalf("corrupt obligation link: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("CORRUPT_RECONCILED_CROSS_TENANT_REPEAT=DENY")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Version != first.Version ||
		reloaded.ReconciledAt == nil || !reloaded.ReconciledAt.Equal(*first.ReconciledAt) ||
		reloaded.ReconciledBy == nil || *reloaded.ReconciledBy != *first.ReconciledBy {
		t.Fatal("CORRUPT_RECONCILED_CROSS_TENANT_REPEAT=FAIL metadata rewritten")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 1 {
		t.Fatal("CORRUPT_RECONCILED_CROSS_TENANT_REPEAT=FAIL duplicate audit")
	}
}

func TestOrphanVoidedByReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	orphanBy := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payments
		SET voided_by = $1, voided_at = NULL, status = 'FULLY_ALLOCATED'
		WHERE id = $2`, orphanBy, payment.ID); err != nil {
		t.Fatalf("corrupt void metadata: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("ORPHAN_VOIDED_BY_RECONCILE=DENY")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.ReconciledAt != nil || reloaded.ReconciledBy != nil {
		t.Fatal("ORPHAN_VOIDED_BY_RECONCILE=FAIL reconciliation metadata set")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 0 {
		t.Fatal("ORPHAN_VOIDED_BY_RECONCILE=FAIL audit created")
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

func setupFullyAllocatedPayment(t *testing.T, env *env, fix fixture) (*domain.Payment, *domain.PaymentObligation) {
	t.Helper()
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	fullyAllocatePayment(t, env, fix, payment, obligation, "100.00")
	return payment, obligation
}

func assertReconcileDeniedUnreconciled(t *testing.T, env *env, fix fixture, paymentID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.payments.ReconcilePayment(ctx, paymentID, buyerActor(fix)); err == nil {
		t.Fatal("expected reconcile deny")
	}
	reloaded, err := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, paymentID)
	if err != nil {
		t.Fatalf("reload payment: %v", err)
	}
	if reloaded.Status != domain.PaymentStatusFullyAllocated ||
		reloaded.ReconciledAt != nil || reloaded.ReconciledBy != nil {
		t.Fatalf("payment state changed: status=%s reconciled_at=%v", reloaded.Status, reloaded.ReconciledAt)
	}
	if countReconcileAudit(t, env, fix.TenantID, paymentID) != 0 {
		t.Fatal("unexpected reconcile audit")
	}
}

func TestActiveOrphanVoidedByReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, _ := setupFullyAllocatedPayment(t, env, fix)
	orphanBy := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = $1
		WHERE payment_id = $2 AND voided_at IS NULL`, orphanBy, payment.ID); err != nil {
		t.Fatalf("corrupt voided_by: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestActiveOrphanVoidReasonReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, _ := setupFullyAllocatedPayment(t, env, fix)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET void_reason = 'corrupt metadata'
		WHERE payment_id = $1 AND voided_at IS NULL`, payment.ID); err != nil {
		t.Fatalf("corrupt void_reason: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestActiveOrphanBothVoidMetadataReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, _ := setupFullyAllocatedPayment(t, env, fix)
	orphanBy := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = $1, void_reason = 'corrupt metadata'
		WHERE payment_id = $2 AND voided_at IS NULL`, orphanBy, payment.ID); err != nil {
		t.Fatalf("corrupt void metadata: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestCorruptReconciledAllocationMetadataRepeatDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, _ := setupFullyAllocatedPayment(t, env, fix)
	first, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	orphanBy := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = $1
		WHERE payment_id = $2 AND voided_at IS NULL`, orphanBy, payment.ID); err != nil {
		t.Fatalf("corrupt allocation metadata: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("CORRUPT_RECONCILED_ALLOCATION_METADATA_REPEAT=DENY")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Version != first.Version ||
		reloaded.ReconciledAt == nil || !reloaded.ReconciledAt.Equal(*first.ReconciledAt) ||
		reloaded.ReconciledBy == nil || *reloaded.ReconciledBy != *first.ReconciledBy {
		t.Fatal("CORRUPT_RECONCILED_ALLOCATION_METADATA_REPEAT=FAIL metadata rewritten")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 1 {
		t.Fatal("CORRUPT_RECONCILED_ALLOCATION_METADATA_REPEAT=FAIL duplicate audit")
	}
}

func TestValidActiveAllocationReconcilePass(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, _ := setupFullyAllocatedPayment(t, env, fix)
	var voidedBy, voidReason *string
	if err := env.pool.QueryRow(ctx, `
		SELECT voided_by::text, void_reason
		FROM billing.payment_allocations
		WHERE payment_id = $1 AND voided_at IS NULL`, payment.ID).Scan(&voidedBy, &voidReason); err != nil {
		t.Fatalf("inspect active allocation metadata: %v", err)
	}
	if voidedBy != nil || voidReason != nil {
		t.Fatalf("expected clean active allocation metadata voided_by=%v void_reason=%v", voidedBy, voidReason)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("VALID_ACTIVE_ALLOCATION=PASS: %v", err)
	}
}

func TestValidVoidedAllocationDoesNotFalsePositive(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-VOID", "100.00")
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("60.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	if _, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "partial reversal", buyerActor(fix)); err != nil {
		t.Fatalf("void allocation: %v", err)
	}
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("second allocate: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("VALID_VOIDED_ALLOCATION=PASS: %v", err)
	}
}

func TestVoidedRowsExcludedFromActiveSum(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-SUM", "100.00")
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	if _, err := env.payments.VoidAllocation(ctx, outcome.Result.Allocation.ID, "reversal", buyerActor(fix)); err != nil {
		t.Fatalf("void allocation: %v", err)
	}
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("second allocate: %v", err)
	}
	reconciled, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil || reconciled.Status != domain.PaymentStatusReconciled {
		t.Fatalf("VOIDED_ROWS_EXCLUDED_FROM_SUM=PASS: %v status=%s", err, reconciled.Status)
	}
}

func setupFullyAllocatedPaymentWithVoidedHistory(t *testing.T, env *env, fix fixture) (*domain.Payment, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	registerB := uuid.New()
	seedBillingRegister(t, env.pool, fix, registerB, "REG-HIST", "100.00")
	obligationA, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	obligationB, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerB)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationA.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	voidedAllocID := outcome.Result.Allocation.ID
	if _, err := env.payments.VoidAllocation(ctx, voidedAllocID, "historical reversal", buyerActor(fix)); err != nil {
		t.Fatalf("void allocation: %v", err)
	}
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligationB.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix)); err != nil {
		t.Fatalf("second allocate: %v", err)
	}
	return payment, voidedAllocID
}

func TestVoidedMissingVoidedByReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, voidedAllocID := setupFullyAllocatedPaymentWithVoidedHistory(t, env, fix)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = NULL
		WHERE id = $1 AND voided_at IS NOT NULL`, voidedAllocID); err != nil {
		t.Fatalf("corrupt voided_by: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestVoidedMissingVoidReasonReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, voidedAllocID := setupFullyAllocatedPaymentWithVoidedHistory(t, env, fix)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET void_reason = NULL
		WHERE id = $1 AND voided_at IS NOT NULL`, voidedAllocID); err != nil {
		t.Fatalf("corrupt void_reason: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestVoidedMissingBothMetadataReconcileDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, voidedAllocID := setupFullyAllocatedPaymentWithVoidedHistory(t, env, fix)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = NULL, void_reason = NULL
		WHERE id = $1 AND voided_at IS NOT NULL`, voidedAllocID); err != nil {
		t.Fatalf("corrupt void metadata: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestFirstReconcileCorruptVoidedHistoryDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, voidedAllocID := setupFullyAllocatedPaymentWithVoidedHistory(t, env, fix)
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = NULL
		WHERE id = $1 AND voided_at IS NOT NULL`, voidedAllocID); err != nil {
		t.Fatalf("corrupt voided history: %v", err)
	}
	assertReconcileDeniedUnreconciled(t, env, fix, payment.ID)
}

func TestCorruptReconciledRepeatVoidedHistoryDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, voidedAllocID := setupFullyAllocatedPaymentWithVoidedHistory(t, env, fix)
	first, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.payment_allocations
		SET voided_by = NULL
		WHERE id = $1 AND voided_at IS NOT NULL`, voidedAllocID); err != nil {
		t.Fatalf("corrupt voided history: %v", err)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err == nil {
		t.Fatal("CORRUPT_RECONCILED_REPEAT_VOIDED_HISTORY=DENY")
	}
	reloaded, _ := env.paymentRepo.GetPaymentByID(ctx, fix.TenantID, payment.ID)
	if reloaded.Version != first.Version ||
		reloaded.ReconciledAt == nil || !reloaded.ReconciledAt.Equal(*first.ReconciledAt) ||
		reloaded.ReconciledBy == nil || *reloaded.ReconciledBy != *first.ReconciledBy {
		t.Fatal("CORRUPT_RECONCILED_REPEAT_VOIDED_HISTORY=FAIL metadata rewritten")
	}
	if countReconcileAudit(t, env, fix.TenantID, payment.ID) != 1 {
		t.Fatal("CORRUPT_RECONCILED_REPEAT_VOIDED_HISTORY=FAIL duplicate audit")
	}
}

func TestActiveCountExcludesVoided(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment, _ := setupFullyAllocatedPaymentWithVoidedHistory(t, env, fix)
	var totalRows, activeRows int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE voided_at IS NULL)
		FROM billing.payment_allocations
		WHERE payment_id = $1`, payment.ID).Scan(&totalRows, &activeRows); err != nil {
		t.Fatalf("count allocations: %v", err)
	}
	if totalRows != 2 || activeRows != 1 {
		t.Fatalf("ACTIVE_COUNT_EXCLUDES_VOIDED=FAIL total=%d active=%d", totalRows, activeRows)
	}
	if _, err := env.payments.ReconcilePayment(ctx, payment.ID, buyerActor(fix)); err != nil {
		t.Fatalf("reconcile with valid void history: %v", err)
	}
}
