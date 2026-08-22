//go:build integration

package freightbillingclosing

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/config"
	"github.com/freight-platform/billing-register-service/internal/domain"
	billinghttp "github.com/freight-platform/billing-register-service/internal/http"
	"github.com/freight-platform/billing-register-service/internal/http/handlers"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
)

const obligationAmount = "100.00"

func setupMarkPaidHTTPRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	registerRepo := repository.NewBillingRegisterRepository(pool)
	settlementRepo := repository.NewFreightSettlementRepository(pool)
	obligationLookup := repository.NewPaymentObligationLookupRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	registerSvc := service.NewBillingRegisterServiceWithPayments(registerRepo, obligationLookup, nil)
	cfg := config.Config{
		InternalServiceToken: "test-token",
		Environment:          "test",
		ServiceName:          "billing-register-service",
	}
	return billinghttp.NewRouter(slog.Default(), pool, cfg, registerSvc, nil, nil, settlementRepo, registerRepo, handlers.NewSettlementActorResolver(membershipRepo))
}

func seedSignedRegister(t *testing.T, pool *pgxpool.Pool, fix fixture) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	registerID := uuid.New()
	period := time.Now().UTC()
	if _, err := pool.Exec(ctx, `INSERT INTO billing.billing_registers (
		id, tenant_id, register_number, customer_company_id, contractor_company_id,
		period_from, period_to, currency_code, status,
		total_without_vat, vat_amount, total_with_vat
	) VALUES ($1,$2,$3,$4,$5,$6,$7,'RUB','SIGNED_BY_COUNTERPARTY',$8,$9,$10)`,
		registerID, fix.TenantID, "REG-"+registerID.String()[:8], fix.BuyerID, fix.CarrierA,
		period, period, "83.33", "16.67", obligationAmount); err != nil {
		t.Fatalf("signed register: %v", err)
	}
	return registerID
}

func seedObligationForRegister(t *testing.T, pool *pgxpool.Pool, fix fixture, registerID uuid.UUID, status string) {
	t.Helper()
	ctx := context.Background()
	paid := "0"
	outstanding := obligationAmount
	if status == "PAID" {
		paid = obligationAmount
		outstanding = "0"
	}
	if _, err := pool.Exec(ctx, `INSERT INTO billing.payment_obligations (
		tenant_id, obligation_number, payer_company_id, payee_company_id,
		source_type, source_id, currency_code, original_amount, paid_amount, outstanding_amount, status
	) VALUES ($1,$2,$3,$4,'BILLING_REGISTER',$5,'RUB',$6,$7,$8,$9)`,
		fix.TenantID, "OBL-"+registerID.String()[:8], fix.BuyerID, fix.CarrierA,
		registerID, obligationAmount, paid, outstanding, status); err != nil {
		t.Fatalf("seed obligation: %v", err)
	}
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
	fix := seedFixture(t, env.pool)
	registerID := seedSignedRegister(t, env.pool, fix)
	seedObligationForRegister(t, env.pool, fix, registerID, "OPEN")
	router := setupMarkPaidHTTPRouter(t, env.pool)

	rec := postMarkPaidHTTP(router, registerID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, domain.SettlementActorBuyer, fix.TenantID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_OPEN_DENY expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	if status := loadRegisterStatus(t, env.pool, registerID, fix.TenantID); status != domain.RegisterStatusSignedByCounterparty {
		t.Fatalf("register must remain SIGNED, got %s", status)
	}
}

func TestLegacyMarkPaidHTTPPaidObligationAllows(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	registerID := seedSignedRegister(t, env.pool, fix)
	seedObligationForRegister(t, env.pool, fix, registerID, "PAID")
	router := setupMarkPaidHTTPRouter(t, env.pool)

	rec := postMarkPaidHTTP(router, registerID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, domain.SettlementActorBuyer, fix.TenantID)
	if rec.Code != http.StatusOK {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_PAID_ALLOW expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if status := loadRegisterStatus(t, env.pool, registerID, fix.TenantID); status != domain.RegisterStatusPaid {
		t.Fatalf("register must become PAID, got %s", status)
	}
}

func TestLegacyMarkPaidHTTPCrossCompanyDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	registerID := seedSignedRegister(t, env.pool, fix)
	seedObligationForRegister(t, env.pool, fix, registerID, "OPEN")
	router := setupMarkPaidHTTPRouter(t, env.pool)
	before := loadRegisterStatus(t, env.pool, registerID, fix.TenantID)

	rec := postMarkPaidHTTP(router, registerID, fix.TenantID, fix.BuyerUserID, fix.ForeignBuyer, domain.SettlementActorBuyer, fix.TenantID)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_CROSS_COMPANY expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if after := loadRegisterStatus(t, env.pool, registerID, fix.TenantID); after != before {
		t.Fatalf("register must remain unchanged")
	}
}

func TestLegacyMarkPaidHTTPCrossTenantDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	registerID := seedSignedRegister(t, env.pool, fix)
	seedObligationForRegister(t, env.pool, fix, registerID, "OPEN")
	router := setupMarkPaidHTTPRouter(t, env.pool)
	before := loadRegisterStatus(t, env.pool, registerID, fix.TenantID)

	rec := postMarkPaidHTTP(router, registerID, fix.OtherTenantID, fix.BuyerUserID, fix.BuyerID, domain.SettlementActorBuyer, fix.OtherTenantID)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_CROSS_TENANT expected deny, got %d body=%s", rec.Code, rec.Body.String())
	}
	if after := loadRegisterStatus(t, env.pool, registerID, fix.TenantID); after != before {
		t.Fatalf("register must remain unchanged")
	}
}

func TestLegacyMarkPaidHTTPRepeatIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	registerID := seedSignedRegister(t, env.pool, fix)
	seedObligationForRegister(t, env.pool, fix, registerID, "PAID")
	router := setupMarkPaidHTTPRouter(t, env.pool)

	first := postMarkPaidHTTP(router, registerID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, domain.SettlementActorBuyer, fix.TenantID)
	if first.Code != http.StatusOK {
		t.Fatalf("first mark-paid: %d body=%s", first.Code, first.Body.String())
	}
	auditAfterFirst := countMarkPaidAuditEvents(t, env.pool, registerID)

	second := postMarkPaidHTTP(router, registerID, fix.TenantID, fix.BuyerUserID, fix.BuyerID, domain.SettlementActorBuyer, fix.TenantID)
	if second.Code != http.StatusOK {
		t.Fatalf("LEGACY_MARK_PAID_HTTP_REPEAT_IDEMPOTENT expected 200, got %d body=%s", second.Code, second.Body.String())
	}
	if auditAfterSecond := countMarkPaidAuditEvents(t, env.pool, registerID); auditAfterSecond != auditAfterFirst {
		t.Fatalf("expected no duplicate MARKED_PAID audit, before=%d after=%d", auditAfterFirst, auditAfterSecond)
	}
	if status := loadRegisterStatus(t, env.pool, registerID, fix.TenantID); status != domain.RegisterStatusPaid {
		t.Fatalf("register must remain PAID, got %s", status)
	}
}

func postSyncPaidHTTP(router http.Handler, registerID, tenantID uuid.UUID, token string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"tenant_id": tenantID.String()})
	path := "/internal/v1/billing-registers/" + registerID.String() + "/sync-paid"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-Internal-Service-Token", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSyncPaidClosedAlreadySatisfied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	registerID := seedSignedRegister(t, env.pool, fix)
	seedObligationForRegister(t, env.pool, fix, registerID, "PAID")
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `UPDATE billing.billing_registers SET status='PAID' WHERE id=$1`, registerID); err != nil {
		t.Fatalf("set paid: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE billing.billing_registers SET status='CLOSED' WHERE id=$1`, registerID); err != nil {
		t.Fatalf("set closed: %v", err)
	}
	router := setupMarkPaidHTTPRouter(t, env.pool)
	auditBefore := countMarkPaidAuditEvents(t, env.pool, registerID)

	rec := postSyncPaidHTTP(router, registerID, fix.TenantID, "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("F6_SYNC_PAID_CLOSED expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if status := loadRegisterStatus(t, env.pool, registerID, fix.TenantID); status != domain.RegisterStatusClosed {
		t.Fatalf("register must remain CLOSED, got %s", status)
	}
	if auditAfter := countMarkPaidAuditEvents(t, env.pool, registerID); auditAfter != auditBefore {
		t.Fatalf("CLOSED sync must not create duplicate MARKED_PAID audit")
	}
}
