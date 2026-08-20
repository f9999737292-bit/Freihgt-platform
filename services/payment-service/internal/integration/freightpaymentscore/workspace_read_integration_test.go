//go:build integration

package freightpaymentscore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

type tenantBActor struct {
	TenantID       uuid.UUID
	BuyerCompanyID uuid.UUID
	UserID         uuid.UUID
}

func seedTenantBActor(t *testing.T, pool *pgxpool.Pool, payerID, payeeID uuid.UUID) tenantBActor {
	t.Helper()
	ctx := context.Background()
	actor := tenantBActor{
		TenantID:       uuid.New(),
		BuyerCompanyID: uuid.New(),
		UserID:         uuid.New(),
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		actor.TenantID, "T-"+actor.TenantID.String()[:8], "Tenant B"); err != nil {
		t.Fatalf("tenant B: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
		VALUES ($1,$2,$3,'SHIPPER','ACTIVE')`, actor.BuyerCompanyID, actor.TenantID, "Tenant B Buyer"); err != nil {
		t.Fatalf("tenant B company: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1,$2,$3,$4,'ACTIVE')`, actor.UserID, actor.TenantID, "b@test.local", "Tenant B User"); err != nil {
		t.Fatalf("tenant B user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1,$2,$3,'ACTIVE')`, actor.TenantID, actor.BuyerCompanyID, actor.UserID); err != nil {
		t.Fatalf("tenant B membership: %v", err)
	}
	_ = payerID
	_ = payeeID
	return actor
}

func (a tenantBActor) buyer() domain.PaymentActorInput {
	return domain.PaymentActorInput{
		TenantID: a.TenantID, ActorCompanyID: a.BuyerCompanyID,
		ActorKind: domain.PaymentActorBuyer, ActorUserID: a.UserID,
	}
}

func wrongCompanyBuyer(fix fixture) domain.PaymentActorInput {
	return domain.PaymentActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.CarrierID,
		ActorKind: domain.PaymentActorBuyer, ActorUserID: fix.BuyerUserID,
	}
}

func carrierActor(fix fixture) domain.PaymentActorInput {
	return domain.PaymentActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.CarrierID,
		ActorKind: domain.PaymentActorCarrier, ActorUserID: fix.BuyerUserID,
	}
}

func expectAccessDenied(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected access denied")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	if appErr.Code != apperrors.CodeNotFound && appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected NOT_FOUND or FORBIDDEN, got %s: %v", appErr.Code, err)
	}
}

func TestCrossTenantAllocationsReadDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	payment := createManualPayment(t, env, fix, "100.00")
	ctx := context.Background()
	obligation, _ := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	_, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("10.00"),
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	tenantB := seedTenantBActor(t, env.pool, fix.BuyerID, fix.CarrierID)
	_, err = env.payments.ListPaymentAllocations(ctx, payment.ID, tenantB.buyer(), 20, 0)
	expectAccessDenied(t, err)
}

func TestCrossTenantAuditReadDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	payment := createManualPayment(t, env, fix, "50.00")
	tenantB := seedTenantBActor(t, env.pool, fix.BuyerID, fix.CarrierID)
	_, err := env.payments.ListPaymentAuditEvents(context.Background(), payment.ID, tenantB.buyer(), 20, 0)
	expectAccessDenied(t, err)
}

func TestCrossTenantEligibleObligationsDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	payment := createManualPayment(t, env, fix, "50.00")
	tenantB := seedTenantBActor(t, env.pool, fix.BuyerID, fix.CarrierID)
	_, err := env.payments.ListEligibleObligationsForPayment(context.Background(), payment.ID, tenantB.buyer(), 20, 0)
	expectAccessDenied(t, err)
}

func TestWrongCompanyEligibleObligationsDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	payment := createManualPayment(t, env, fix, "50.00")
	_, err := env.payments.ListEligibleObligationsForPayment(context.Background(), payment.ID, wrongCompanyBuyer(fix), 20, 0)
	expectAccessDenied(t, err)
}

func TestPaymentListTenantIsolation(t *testing.T) {
	env := setupEnv(t)
	fixA := seedFixture(t, env.pool)
	fixB := seedFixture(t, env.pool)
	createManualPayment(t, env, fixA, "100.00")
	createManualPayment(t, env, fixB, "200.00")
	result, err := env.payments.ListPaymentsFiltered(context.Background(), buyerActor(fixA), domain.PaymentListQuery{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	for _, p := range result.Items {
		if p.TenantID != fixA.TenantID {
			t.Fatalf("PAYMENT_LIST_TENANT_ISOLATION=FAIL leaked tenant %s", p.TenantID)
		}
	}
}

func TestBuyerPaymentListCompanyScope(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	createManualPayment(t, env, fix, "100.00")
	result, err := env.payments.ListPaymentsFiltered(context.Background(), buyerActor(fix), domain.PaymentListQuery{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range result.Items {
		if p.PayerCompanyID != fix.BuyerID {
			t.Fatalf("BUYER_PAYMENT_LIST_COMPANY_SCOPE=FAIL payer=%s", p.PayerCompanyID)
		}
	}
}

func TestCarrierPaymentListCompanyScope(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	createManualPayment(t, env, fix, "100.00")
	result, err := env.payments.ListPaymentsFiltered(context.Background(), carrierActor(fix), domain.PaymentListQuery{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range result.Items {
		if p.PayeeCompanyID != fix.CarrierID {
			t.Fatalf("CARRIER_PAYMENT_LIST_COMPANY_SCOPE=FAIL payee=%s", p.PayeeCompanyID)
		}
	}
}

func seedManyAllocations(t *testing.T, env *env, fix fixture, paymentID uuid.UUID, count int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		registerID := uuid.New()
		seedBillingRegister(t, env.pool, fix, registerID, fmt.Sprintf("REG-PAG-%d", i), "10.00")
		obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, registerID)
		if err != nil {
			t.Fatalf("obligation %d: %v", i, err)
		}
		_, err = env.payments.Allocate(ctx, domain.CreateAllocationInput{
			PaymentID: paymentID, ObligationID: obligation.ID, AllocatedAmount: decimal.RequireFromString("1.00"),
		}, buyerActor(fix))
		if err != nil {
			t.Fatalf("allocate %d: %v", i, err)
		}
	}
}

func seedManyAuditEvents(t *testing.T, pool *pgxpool.Pool, tenantID, paymentID uuid.UUID, count int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := pool.Exec(ctx, `INSERT INTO billing.payment_audit_events (
			id, tenant_id, entity_type, entity_id, event_type, payload, created_at
		) VALUES ($1,$2,'PAYMENT',$3,'payment.created','{}',$4)`,
			uuid.New(), tenantID, paymentID, time.Now().UTC().Add(-time.Duration(i)*time.Second))
		if err != nil {
			t.Fatalf("audit %d: %v", i, err)
		}
	}
}

func seedManyEligibleObligations(t *testing.T, env *env, fix fixture, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		registerID := uuid.New()
		seedBillingRegister(t, env.pool, fix, registerID, fmt.Sprintf("REG-ELIG-%d", i), "5.00")
		if _, err := env.payments.EnsurePaymentObligationForBillingRegister(context.Background(), fix.TenantID, registerID); err != nil {
			t.Fatalf("eligible obligation %d: %v", i, err)
		}
	}
}

func TestReadPaginationPostgres(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	payment := createManualPayment(t, env, fix, "100.00")
	seedManyAllocations(t, env, fix, payment.ID, 25)
	seedManyAuditEvents(t, env.pool, fix.TenantID, payment.ID, 25)
	seedManyEligibleObligations(t, env, fix, 25)

	allocPage1, err := env.payments.ListPaymentAllocations(ctx, payment.ID, buyerActor(fix), 20, 0)
	if err != nil || allocPage1.Total != 25 || len(allocPage1.Items) != 20 {
		t.Fatalf("alloc page1: err=%v total=%d len=%d", err, allocPage1.Total, len(allocPage1.Items))
	}
	allocPage2, err := env.payments.ListPaymentAllocations(ctx, payment.ID, buyerActor(fix), 20, 20)
	if err != nil || len(allocPage2.Items) != 5 {
		t.Fatalf("alloc page2: err=%v len=%d", err, len(allocPage2.Items))
	}
	seen := map[uuid.UUID]bool{}
	for _, page := range [][]domain.PaymentAllocationRead{allocPage1.Items, allocPage2.Items} {
		for _, item := range page {
			if seen[item.ID] {
				t.Fatal("duplicate allocation id in pagination")
			}
			seen[item.ID] = true
			if item.ObligationNumber == nil || *item.ObligationNumber == "" {
				t.Fatal("allocation read model missing obligation_number enrichment")
			}
		}
	}

	auditPage1, err := env.payments.ListPaymentAuditEvents(ctx, payment.ID, buyerActor(fix), 20, 0)
	if err != nil || auditPage1.Total < 25 || len(auditPage1.Items) != 20 {
		t.Fatalf("audit page1: err=%v total=%d len=%d", err, auditPage1.Total, len(auditPage1.Items))
	}
	auditPage2, err := env.payments.ListPaymentAuditEvents(ctx, payment.ID, buyerActor(fix), 20, 20)
	if err != nil || len(auditPage2.Items) < 5 {
		t.Fatalf("audit page2: err=%v len=%d", err, len(auditPage2.Items))
	}

	eligiblePage1, err := env.payments.ListEligibleObligationsForPayment(ctx, payment.ID, buyerActor(fix), 20, 0)
	if err != nil || eligiblePage1.Total < 25 || len(eligiblePage1.Items) != 20 {
		t.Fatalf("eligible page1: err=%v total=%d len=%d", err, eligiblePage1.Total, len(eligiblePage1.Items))
	}
	eligiblePage2, err := env.payments.ListEligibleObligationsForPayment(ctx, payment.ID, buyerActor(fix), 20, 20)
	if err != nil || len(eligiblePage2.Items) < 5 {
		t.Fatalf("eligible page2: err=%v len=%d", err, len(eligiblePage2.Items))
	}
}
