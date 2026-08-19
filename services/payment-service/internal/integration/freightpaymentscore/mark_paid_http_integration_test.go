//go:build integration

package freightpaymentscore

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	billingconfig "github.com/freight-platform/billing-register-service/internal/config"
	billingdomain "github.com/freight-platform/billing-register-service/internal/domain"
	billinghttp "github.com/freight-platform/billing-register-service/internal/http"
	billinghandlers "github.com/freight-platform/billing-register-service/internal/http/handlers"
	billingrepository "github.com/freight-platform/billing-register-service/internal/repository"
	billingservice "github.com/freight-platform/billing-register-service/internal/service"
	"github.com/freight-platform/payment-service/internal/domain"
)

type markPaidFixture struct {
	fixture
	ForeignBuyerID uuid.UUID
	OtherTenantID  uuid.UUID
	OtherUserID    uuid.UUID
}

func seedMarkPaidFixture(t *testing.T, pool *pgxpool.Pool) markPaidFixture {
	t.Helper()
	base := seedFixture(t, pool)
	ctx := context.Background()
	mp := markPaidFixture{fixture: base, ForeignBuyerID: uuid.New(), OtherTenantID: uuid.New(), OtherUserID: uuid.New()}
	if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
		VALUES ($1,$2,$3,'SHIPPER','ACTIVE')`, mp.ForeignBuyerID, mp.TenantID, "Foreign Buyer Co"); err != nil {
		t.Fatalf("foreign buyer company: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1,$2,$3,'ACTIVE')`, mp.TenantID, mp.ForeignBuyerID, mp.BuyerUserID); err != nil {
		t.Fatalf("foreign buyer membership: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		mp.OtherTenantID, "T-"+mp.OtherTenantID.String()[:8], "Other Tenant"); err != nil {
		t.Fatalf("other tenant: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1,$2,$3,$4,'ACTIVE')`, mp.OtherUserID, mp.OtherTenantID, "other@test.local", "Other User"); err != nil {
		t.Fatalf("other tenant user: %v", err)
	}
	return mp
}

func setupBillingRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	registerRepo := billingrepository.NewBillingRegisterRepository(pool)
	obligationLookup := billingrepository.NewPaymentObligationLookupRepository(pool)
	membershipRepo := billingrepository.NewMembershipRepository(pool)
	registerSvc := billingservice.NewBillingRegisterServiceWithPayments(registerRepo, obligationLookup, nil)
	cfg := billingconfig.Config{
		InternalServiceToken: "test-token",
		Environment:          "test",
		ServiceName:          "billing-register-service",
	}
	return billinghttp.NewRouter(slog.Default(), pool, cfg, registerSvc, nil, nil, billinghandlers.NewSettlementActorResolver(membershipRepo))
}

func postMarkPaidHTTP(router http.Handler, registerID, tenantID, userID, companyID uuid.UUID, actorKind string, bodyTenant uuid.UUID) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"tenant_id": bodyTenant.String()})
	path := "/v1/billing-registers/" + registerID.String() + "/mark-paid"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("X-Company-ID", companyID.String())
	req.Header.Set("X-Actor-Kind", actorKind)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func loadRegisterStatus(t *testing.T, pool *pgxpool.Pool, registerID, tenantID uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM billing.billing_registers WHERE id=$1 AND tenant_id=$2`, registerID, tenantID).Scan(&status); err != nil {
		t.Fatalf("load register status: %v", err)
	}
	return status
}

func countMarkPaidAuditEvents(t *testing.T, pool *pgxpool.Pool, registerID uuid.UUID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM billing.billing_register_audit_events WHERE register_id=$1 AND event_type='MARKED_PAID'`, registerID).Scan(&count); err != nil {
		t.Fatalf("count mark-paid audit: %v", err)
	}
	return count
}

func TestLegacyMarkPaidHTTPOpenObligationDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedMarkPaidFixture(t, env.pool)
	ctx := context.Background()
	router := setupBillingRouter(t, env.pool)

	if _, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID); err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	rec := postMarkPaidHTTP(router, fix.RegisterID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, billingdomain.SettlementActorBuyer, fix.TenantID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_OPEN_DENY expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	if status := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID); status != billingdomain.RegisterStatusSignedByCounterparty {
		t.Fatalf("register must remain SIGNED, got %s", status)
	}
}

func TestLegacyMarkPaidHTTPPaidObligationAllows(t *testing.T) {
	env := setupEnv(t)
	fix := seedMarkPaidFixture(t, env.pool)
	ctx := context.Background()
	router := setupBillingRouter(t, env.pool)

	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	payment := createManualPayment(t, env, fix.fixture, fix.RegisterTotal.StringFixed(2))
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: fix.RegisterTotal,
	}, buyerActor(fix.fixture)); err != nil {
		t.Fatalf("allocate: %v", err)
	}

	rec := postMarkPaidHTTP(router, fix.RegisterID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, billingdomain.SettlementActorBuyer, fix.TenantID)
	if rec.Code != http.StatusOK {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_PAID_ALLOW expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if status := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID); status != billingdomain.RegisterStatusPaid {
		t.Fatalf("register must become PAID, got %s", status)
	}
}

func TestLegacyMarkPaidHTTPCrossCompanyDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedMarkPaidFixture(t, env.pool)
	ctx := context.Background()
	router := setupBillingRouter(t, env.pool)
	if _, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID); err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	before := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID)
	rec := postMarkPaidHTTP(router, fix.RegisterID, fix.TenantID, fix.BuyerUserID, fix.ForeignBuyerID, billingdomain.SettlementActorBuyer, fix.TenantID)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_CROSS_COMPANY expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if after := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID); after != before {
		t.Fatalf("register must remain unchanged, before=%s after=%s", before, after)
	}
}

func TestLegacyMarkPaidHTTPCrossTenantDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedMarkPaidFixture(t, env.pool)
	ctx := context.Background()
	router := setupBillingRouter(t, env.pool)
	if _, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID); err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	before := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID)
	rec := postMarkPaidHTTP(router, fix.RegisterID, fix.OtherTenantID, fix.OtherUserID, fix.BuyerID, billingdomain.SettlementActorBuyer, fix.OtherTenantID)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_CROSS_TENANT expected deny, got %d body=%s", rec.Code, rec.Body.String())
	}
	if after := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID); after != before {
		t.Fatalf("register must remain unchanged")
	}
}

func TestLegacyMarkPaidHTTPRepeatIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedMarkPaidFixture(t, env.pool)
	ctx := context.Background()
	router := setupBillingRouter(t, env.pool)

	obligation, err := env.payments.EnsurePaymentObligationForBillingRegister(ctx, fix.TenantID, fix.RegisterID)
	if err != nil {
		t.Fatalf("ensure obligation: %v", err)
	}
	payment := createManualPayment(t, env, fix.fixture, fix.RegisterTotal.StringFixed(2))
	if _, err := env.payments.Allocate(ctx, domain.CreateAllocationInput{
		PaymentID: payment.ID, ObligationID: obligation.ID, AllocatedAmount: fix.RegisterTotal,
	}, buyerActor(fix.fixture)); err != nil {
		t.Fatalf("allocate: %v", err)
	}

	first := postMarkPaidHTTP(router, fix.RegisterID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, billingdomain.SettlementActorBuyer, fix.TenantID)
	if first.Code != http.StatusOK {
		t.Fatalf("first mark-paid: %d body=%s", first.Code, first.Body.String())
	}
	auditAfterFirst := countMarkPaidAuditEvents(t, env.pool, fix.RegisterID)

	second := postMarkPaidHTTP(router, fix.RegisterID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, billingdomain.SettlementActorBuyer, fix.TenantID)
	if second.Code != http.StatusOK {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_REPEAT_IDEMPOTENT expected 200, got %d body=%s", second.Code, second.Body.String())
	}
	if auditAfterSecond := countMarkPaidAuditEvents(t, env.pool, fix.RegisterID); auditAfterSecond != auditAfterFirst {
		t.Fatalf("expected no duplicate MARKED_PAID audit, before=%d after=%d", auditAfterFirst, auditAfterSecond)
	}
	if status := loadRegisterStatus(t, env.pool, fix.RegisterID, fix.TenantID); status != billingdomain.RegisterStatusPaid {
		t.Fatalf("register must remain PAID, got %s", status)
	}
}
