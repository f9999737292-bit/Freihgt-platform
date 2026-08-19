//go:build integration

package freightpaymentscore

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	"github.com/freight-platform/payment-service/internal/repository"
	"github.com/freight-platform/payment-service/internal/service"
)

const maxMigrationFile = "000047_payment_allocation_void_metadata_v1.9.2B.up.sql"

type env struct {
	pool        *pgxpool.Pool
	payments    *service.PaymentService
	paymentRepo *repository.PaymentRepository
	outboxRepo  *repository.OutboxRepository
}

type fixture struct {
	TenantID      uuid.UUID
	BuyerID       uuid.UUID
	CarrierID     uuid.UUID
	BuyerUserID   uuid.UUID
	RegisterID    uuid.UUID
	RegisterTotal decimal.Decimal
}

func setupEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	paymentRepo := repository.NewPaymentRepository(pool)
	registerLookup := repository.NewBillingRegisterLookupRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	paymentSvc := service.NewPaymentService(paymentRepo, registerLookup, membershipRepo, nil, outboxRepo)
	return &env{pool: pool, payments: paymentSvc, paymentRepo: paymentRepo, outboxRepo: outboxRepo}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.Compare(filepath.Base(file), maxMigrationFile) > 0 {
			break
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			msg := execErr.Error()
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
				continue
			}
			return execErr
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantID:      uuid.New(),
		BuyerID:       uuid.New(),
		CarrierID:     uuid.New(),
		BuyerUserID:   uuid.New(),
		RegisterID:    uuid.New(),
		RegisterTotal: decimal.RequireFromString("100.00"),
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		fix.TenantID, "T-"+fix.TenantID.String()[:8], "Test Tenant"); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{fix.BuyerID, fix.TenantID, "SHIPPER", "Buyer Co"},
		{fix.CarrierID, fix.TenantID, "CARRIER", "Carrier Co"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, row.id, row.tenant, row.name, row.typ); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1,$2,$3,$4,'ACTIVE')`, fix.BuyerUserID, fix.TenantID, "buyer@test.local", "Buyer User"); err != nil {
		t.Fatalf("user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1,$2,$3,'ACTIVE')`, fix.TenantID, fix.BuyerID, fix.BuyerUserID); err != nil {
		t.Fatalf("membership: %v", err)
	}
	period := time.Now().UTC()
	if _, err := pool.Exec(ctx, `INSERT INTO billing.billing_registers (
		id, tenant_id, register_number, customer_company_id, contractor_company_id,
		period_from, period_to, currency_code, status,
		total_without_vat, vat_amount, total_with_vat
	) VALUES ($1,$2,$3,$4,$5,$6,$7,'RUB','SIGNED_BY_COUNTERPARTY',$8,$9,$10)`,
		fix.RegisterID, fix.TenantID, "REG-"+fix.RegisterID.String()[:8], fix.BuyerID, fix.CarrierID,
		period, period, "83.33", "16.67", fix.RegisterTotal.StringFixed(2)); err != nil {
		t.Fatalf("register: %v", err)
	}
	return fix
}

func buyerActor(fix fixture) domain.PaymentActorInput {
	return domain.PaymentActorInput{
		TenantID: fix.TenantID, ActorCompanyID: fix.BuyerID,
		ActorKind: domain.PaymentActorBuyer, ActorUserID: fix.BuyerUserID,
	}
}

func seedBillingRegister(t *testing.T, pool *pgxpool.Pool, fix fixture, registerID uuid.UUID, registerNumber, total string) {
	t.Helper()
	ctx := context.Background()
	period := time.Now().UTC()
	if _, err := pool.Exec(ctx, `INSERT INTO billing.billing_registers (
		id, tenant_id, register_number, customer_company_id, contractor_company_id,
		period_from, period_to, currency_code, status,
		total_without_vat, vat_amount, total_with_vat
	) VALUES ($1,$2,$3,$4,$5,$6,$7,'RUB','SIGNED_BY_COUNTERPARTY',$8,$9,$10)`,
		registerID, fix.TenantID, registerNumber, fix.BuyerID, fix.CarrierID,
		period, period, total, "0.00", total); err != nil {
		t.Fatalf("register %s: %v", registerNumber, err)
	}
}

func createManualPayment(t *testing.T, env *env, fix fixture, amount string) *domain.Payment {
	t.Helper()
	p, err := env.payments.CreateManualPayment(context.Background(), domain.CreateManualPaymentInput{
		Amount:         decimal.RequireFromString(amount),
		CurrencyCode:   "RUB",
		PaymentDate:    time.Now().UTC(),
		PayerCompanyID: fix.BuyerID,
		PayeeCompanyID: fix.CarrierID,
	}, buyerActor(fix))
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return p
}
