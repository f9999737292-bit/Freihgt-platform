//go:build integration

package contractrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
	"github.com/freight-platform/contract-rate-service/internal/repository"
)

const maxMigrationNumber = 48

type testEnv struct {
	Pool      *pgxpool.Pool
	TenantID  uuid.UUID
	BuyerID   uuid.UUID
	CarrierID uuid.UUID
	Contracts *repository.ContractRepository
	RateCards *repository.RateCardRepository
	Actor     domain.ActorInput
	Today     time.Time
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	pool := setupPool(t)
	ctx := context.Background()
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	seedTenantAndCompanies(t, ctx, pool, tenantID, buyerID, carrierID)
	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	rateCards := repository.NewRateCardRepository(pool, contracts, audit)
	actor := domain.ActorInput{
		TenantID: tenantID, ActorUserID: uuid.New(),
		ActorCompanyID: buyerID, ActorKind: domain.ActorKindBuyer,
	}
	return &testEnv{
		Pool: pool, TenantID: tenantID, BuyerID: buyerID, CarrierID: carrierID,
		Contracts: contracts, RateCards: rateCards, Actor: actor,
		Today: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
	}
}

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := requireTestDatabaseURL(t)
	ctx := context.Background()
	pool, cleanup := createTempDB(t, ctx, adminURL)
	t.Cleanup(cleanup)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return pool
}

func requireTestDatabaseURL(t *testing.T) string {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return adminURL
}

func (e *testEnv) createDraftContract(t *testing.T, number string) *domain.TransportContract {
	t.Helper()
	created, err := e.Contracts.Create(context.Background(), domain.CreateContractInput{
		TenantID: e.TenantID, BuyerCompanyID: e.BuyerID, CarrierCompanyID: e.CarrierID,
		ContractNumber: number, Name: "Contract " + number, ValidFrom: e.Today,
		CurrencyCode: "RUB", Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create draft %s: %v", number, err)
	}
	return created
}

func seedTenantAndCompanies(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, buyerID, carrierID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name, status)
		VALUES ($1, $2, 'Test Tenant', 'ACTIVE')
		ON CONFLICT DO NOTHING`, tenantID, "T-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{buyerID, tenantID, "SHIPPER", "Buyer Co"},
		{carrierID, tenantID, "CARRIER", "Carrier Co"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')
			ON CONFLICT DO NOTHING`, row.id, row.tenant, row.typ, row.name)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
}

func seedTenantOnly(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name, status)
		VALUES ($1, $2, 'Other Tenant', 'ACTIVE')
		ON CONFLICT DO NOTHING`, tenantID, "T-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
}

func createTempDB(t *testing.T, ctx context.Context, adminURL string) (*pgxpool.Pool, func()) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	dbName := "freight_contract_rate_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		adminPool.Close()
		t.Fatalf("create db: %v", err)
	}
	adminPool.Close()

	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, testCfg)
	if err != nil {
		t.Fatalf("test pool: %v", err)
	}
	cleanup := func() {
		pool.Close()
		adminPool, _ = pgxpool.NewWithConfig(context.Background(), adminCfg)
		if adminPool != nil {
			_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
			adminPool.Close()
		}
	}
	return pool, cleanup
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "infrastructure", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(name, "%d", &num); err != nil || num > maxMigrationNumber {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "infrastructure", "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found")
		}
		dir = parent
	}
}

func strPtr(v string) *string { return &v }

func auditCount(t *testing.T, pool *pgxpool.Pool, tenantID, entityID uuid.UUID, action string) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM contract_rate.audit_event
		WHERE tenant_id=$1 AND entity_id=$2 AND action=$3`, tenantID, entityID, action).Scan(&count)
	if err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return count
}

func isAppErrorCode(err error, code apperrors.Code) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == code
}

func activateRateVersionSQL(t *testing.T, env *testEnv, versionID uuid.UUID) {
	t.Helper()
	_, err := env.Pool.Exec(context.Background(), `
		UPDATE contract_rate.rate_card_version
		SET status='ACTIVE', activated_at=now()
		WHERE tenant_id=$1 AND id=$2`, env.TenantID, versionID)
	if err != nil {
		t.Fatalf("activate version sql: %v", err)
	}
}
