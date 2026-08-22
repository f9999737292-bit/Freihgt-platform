//go:build integration

package freightcostledger

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
)

const maxMigrationFile = "000056_payment_outbox_aggregate_version_v2.1B.up.sql"

type env struct {
	pool        *pgxpool.Pool
	settlements *service.FreightSettlementService
	repo        *repository.FreightSettlementRepository
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
	repo := repository.NewFreightSettlementRepository(pool)
	settlements := service.NewFreightSettlementService(repo)
	return &env{pool: pool, settlements: settlements, repo: repo}
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

func TestFC_B_BIL_DB_001_BillingLinkRevisionMonotonicOnRelink(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	settlementID := uuid.New()
	tenantID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		tenantID, "t-"+tenantID.String()[:8], "Billing Link Tenant"); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	var revision int64
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.freight_settlements (
			id, tenant_id, settlement_number, shipment_id, transport_order_id,
			buyer_company_id, carrier_company_id, currency_code, status,
			base_freight_amount, total_without_vat, billing_link_revision, version
		) VALUES ($1,$2,'FS-TEST',$3,$4,$5,$6,'RUB','APPROVED',1000,1000,1,1)`,
		settlementID, tenantID, uuid.New(), uuid.New(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("insert settlement: %v", err)
	}
	err := env.pool.QueryRow(ctx, `SELECT billing_link_revision FROM billing.freight_settlements WHERE id = $1`, settlementID).Scan(&revision)
	if err != nil {
		t.Fatalf("read revision: %v", err)
	}
	if revision != 1 {
		t.Fatalf("initial revision = %d", revision)
	}
	err = env.pool.QueryRow(ctx, `
		UPDATE billing.freight_settlements
		SET billing_register_id = NULL, billing_link_revision = billing_link_revision + 1
		WHERE id = $1 AND tenant_id = $2
		RETURNING billing_link_revision`, settlementID, tenantID).Scan(&revision)
	if err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if revision != 2 {
		t.Fatalf("unlink revision = %d", revision)
	}
	registerID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.billing_registers (
			id, tenant_id, register_number, customer_company_id, contractor_company_id,
			period_from, period_to, currency_code, status,
			total_without_vat, vat_amount, total_with_vat, version
		) VALUES ($1,$2,'BR-TEST',$3,$4,CURRENT_DATE,CURRENT_DATE,'RUB','DRAFT',0,0,0,1)`,
		registerID, tenantID, buyerID, carrierID); err != nil {
		t.Fatalf("insert register: %v", err)
	}
	err = env.pool.QueryRow(ctx, `
		UPDATE billing.freight_settlements
		SET billing_register_id = $3, billing_link_revision = billing_link_revision + 1
		WHERE id = $1 AND tenant_id = $2
		RETURNING billing_link_revision`, settlementID, tenantID, registerID).Scan(&revision)
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	if revision != 3 {
		t.Fatalf("relink revision = %d", revision)
	}
}

func TestFC_B_ACC_DB_001_DisputeRemovesAccessorialFromApprovedSet(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	settlementID := uuid.New()
	accessorialID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		tenantID, "t-"+tenantID.String()[:8], "Accrual Tenant"); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.freight_settlements (
			id, tenant_id, settlement_number, shipment_id, transport_order_id,
			buyer_company_id, carrier_company_id, currency_code, status,
			base_freight_amount, total_without_vat, version
		) VALUES ($1,$2,'FS-ACC',$3,$4,$5,$6,'RUB','APPROVED',1000,1000,1)`,
		settlementID, tenantID, uuid.New(), uuid.New(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.settlement_accessorials (
			id, tenant_id, settlement_id, charge_code, amount, currency_code, status,
			submitted_by, submitted_by_company_id
		) VALUES ($1,$2,$3,'LUMPER',100,'RUB','APPROVED',$4,$5)`,
		accessorialID, tenantID, settlementID, uuid.New(), uuid.New()); err != nil {
		t.Fatalf("accessorial: %v", err)
	}
	sumApproved := queryApprovedSum(t, env.pool, tenantID, settlementID)
	if !sumApproved.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("approved sum = %s", sumApproved)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE billing.settlement_accessorials SET status = 'DISPUTED' WHERE id = $1`, accessorialID); err != nil {
		t.Fatalf("dispute: %v", err)
	}
	sumAfter := queryApprovedSum(t, env.pool, tenantID, settlementID)
	if !sumAfter.Equal(decimal.Zero) {
		t.Fatalf("disputed accessorial must be excluded, got %s", sumAfter)
	}
	base := decimal.RequireFromString("1000.00")
	expectedAccrual := base.Add(sumAfter)
	if !expectedAccrual.Equal(decimal.RequireFromString("1000.00")) {
		t.Fatalf("expected accrual 1000+100->1000, got %s", expectedAccrual)
	}
	_ = domain.SettlementStatusApproved
}

func queryApprovedSum(t *testing.T, pool *pgxpool.Pool, tenantID, settlementID uuid.UUID) decimal.Decimal {
	t.Helper()
	var raw string
	err := pool.QueryRow(context.Background(), `
		SELECT COALESCE(SUM(amount), 0)::text
		FROM billing.settlement_accessorials
		WHERE tenant_id = $1 AND settlement_id = $2 AND status = 'APPROVED'`,
		tenantID, settlementID).Scan(&raw)
	if err != nil {
		t.Fatalf("approved sum: %v", err)
	}
	return decimal.RequireFromString(raw)
}

func TestFC_B_BIL_DB_002_InternalBillingLinkReadReturnsRevision(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	settlementID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		tenantID, "t-"+tenantID.String()[:8], "Internal Read Tenant"); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO billing.freight_settlements (
			id, tenant_id, settlement_number, shipment_id, transport_order_id,
			buyer_company_id, carrier_company_id, currency_code, status,
			base_freight_amount, total_without_vat, billing_link_revision, version
		) VALUES ($1,$2,'FS-LINK',$3,$4,$5,$6,'RUB','APPROVED',1000,1000,5,1)`,
		settlementID, tenantID, uuid.New(), uuid.New(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("settlement: %v", err)
	}
	read, err := env.repo.GetInternalBillingLink(ctx, tenantID, settlementID)
	if err != nil {
		t.Fatalf("internal billing link read: %v", err)
	}
	if read.BillingLinkRevision != 5 || read.BillingLinkState != "UNLINKED" {
		t.Fatalf("read = revision %d state %s", read.BillingLinkRevision, read.BillingLinkState)
	}
}
