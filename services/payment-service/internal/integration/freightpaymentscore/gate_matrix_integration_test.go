//go:build integration

package freightpaymentscore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	"github.com/freight-platform/payment-service/internal/repository"
	"github.com/freight-platform/payment-service/internal/service"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

func TestPrecisionValidationGate(t *testing.T) {
	if _, err := domain.ParseMoney("1.234", "amount"); err == nil {
		t.Fatal("PRECISION_VALIDATION=FAIL over-precision accepted")
	}
	if _, err := domain.ParseMoney("1.23", "amount"); err != nil {
		t.Fatalf("valid precision rejected: %v", err)
	}
}

func TestObligationAuditFailureRollback(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	snap := &repository.BillingRegisterSnapshot{
		RegisterNumber: "REG-TEST", Status: "SIGNED_BY_COUNTERPARTY",
		CustomerCompanyID: fix.BuyerID, ContractorCompanyID: fix.CarrierID,
		CurrencyCode: "RUB", TotalWithVAT: fix.RegisterTotal,
	}
	if err := env.paymentRepo.SimulateObligationAuditFailureForTest(ctx, fix.TenantID, fix.RegisterID, snap); err == nil {
		t.Fatal("OBLIGATION_AUDIT_FAILURE_ROLLBACK=FAIL expected simulated audit failure")
	}
	if _, err := env.paymentRepo.GetObligationBySource(ctx, fix.TenantID, domain.ObligationSourceBillingRegister, fix.RegisterID); err == nil {
		t.Fatal("obligation must not survive audit failure")
	}
}

func TestPaymentAuditFailureRollback(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	in := domain.CreateManualPaymentInput{
		TenantID: fix.TenantID, Amount: decimal.RequireFromString("10.00"), CurrencyCode: "RUB",
		PaymentDate: time.Now().UTC(), PayerCompanyID: fix.BuyerID, PayeeCompanyID: fix.CarrierID,
		CreatedBy: fix.BuyerUserID, Source: domain.PaymentSourceManual,
	}
	if err := env.paymentRepo.SimulatePaymentAuditFailureForTest(ctx, in); err == nil {
		t.Fatal("PAYMENT_AUDIT_FAILURE_ROLLBACK=FAIL expected simulated audit failure")
	}
	var count int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM billing.payments WHERE tenant_id = $1`, fix.TenantID).Scan(&count); err != nil {
		t.Fatalf("count payments: %v", err)
	}
	if count != 0 {
		t.Fatal("payment must not survive audit failure")
	}
}

func TestPartialAndFullAllocation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	payment := createManualPayment(t, env, fix, "100.00")
	partial, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("40.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("partial allocate: %v", err)
	}
	if partial.Result.Obligation.Status != domain.ObligationStatusPartiallyPaid {
		t.Fatalf("expected PARTIALLY_PAID, got %s", partial.Result.Obligation.Status)
	}
	payment2 := createManualPayment(t, env, fix, "60.00")
	full, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment2.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("60.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("full allocate: %v", err)
	}
	if full.Result.Obligation.Status != domain.ObligationStatusPaid {
		t.Fatalf("expected PAID, got %s", full.Result.Obligation.Status)
	}
}

func TestOverAllocationRejected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "50.00")
	_, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err == nil {
		t.Fatal("expected over-allocation rejection")
	}
}

func TestCurrencyMismatchRejected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment, err := env.payments.CreateManualPayment(ctx, domain.CreateManualPaymentInput{
		Amount: decimal.RequireFromString("100.00"), CurrencyCode: "USD", PaymentDate: time.Now().UTC(),
		PayerCompanyID: fix.BuyerID, PayeeCompanyID: fix.CarrierID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	_, err = env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("10.00"),
	}, buyerActor(fix))
	if err == nil {
		t.Fatal("expected currency mismatch rejection")
	}
}

func TestCrossTenantObligationDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	otherTenant := uuid.New()
	actor := buyerActor(fix)
	actor.TenantID = otherTenant
	_, err := env.payments.GetObligation(ctx, obligation.ID, actor)
	if err == nil {
		t.Fatal("CROSS_TENANT_OBLIGATION=DENY expected")
	}
}

func TestCrossCompanyObligationDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	wrongCompany := uuid.New()
	actor := domain.PaymentActorInput{
		TenantID: fix.TenantID, ActorCompanyID: wrongCompany,
		ActorKind: domain.PaymentActorBuyer, ActorUserID: fix.BuyerUserID,
	}
	_, err := env.payments.GetObligation(ctx, obligation.ID, actor)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("CROSS_COMPANY_OBLIGATION=DENY expected forbidden, got %v", err)
	}
}

type failingBillingSync struct{}

func (failingBillingSync) SyncRegisterPaid(context.Context, uuid.UUID, uuid.UUID) error {
	return errors.New("billing unavailable")
}

func TestPaidSyncFailureVisibleAndRetry(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	env.payments = service.NewPaymentService(env.paymentRepo, repository.NewBillingRegisterLookupRepository(env.pool), repository.NewMembershipRepository(env.pool), failingBillingSync{})

	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	payment := createManualPayment(t, env, fix, "100.00")
	outcome, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("100.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if outcome.RegisterPaidProjection == nil || outcome.RegisterPaidProjection.Status != service.RegisterPaidProjectionFailed {
		t.Fatal("PAID_SYNC_FAILURE_VISIBLE=FAIL")
	}

	billingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer billingServer.Close()
	env.payments = service.NewPaymentService(env.paymentRepo, repository.NewBillingRegisterLookupRepository(env.pool), repository.NewMembershipRepository(env.pool), service.NewBillingRegisterHTTPClient(billingServer.URL, "token"))
	if err := env.payments.EnsureBillingRegisterPaidProjection(ctx, fix.TenantID, fix.RegisterID); err != nil {
		t.Fatalf("PAID_SYNC_RETRY=FAIL: %v", err)
	}
	if err := env.payments.EnsureBillingRegisterPaidProjection(ctx, fix.TenantID, fix.RegisterID); err != nil {
		t.Fatalf("PAID_SYNC_IDEMPOTENT=FAIL: %v", err)
	}
}

func TestSignedEnsureRetrySafe(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `UPDATE billing.billing_registers SET status = 'SENT_TO_EDO' WHERE id = $1`, fix.RegisterID); err != nil {
		t.Fatalf("set sent_to_edo: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE billing.billing_registers SET status = 'SIGNED_BY_COUNTERPARTY' WHERE id = $1`, fix.RegisterID); err != nil {
		t.Fatalf("set signed: %v", err)
	}
	first, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure first: %v", err)
	}
	second, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure retry: %v", err)
	}
	if first.ID != second.ID {
		t.Fatal("FAILED_HOOK_RETRY_SAFE=FAIL duplicate obligation")
	}
	var count int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM billing.payment_obligations WHERE tenant_id = $1 AND source_id = $2`, fix.TenantID, fix.RegisterID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one obligation, got %d", count)
	}
}
