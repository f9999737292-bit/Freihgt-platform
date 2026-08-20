//go:build integration

package enterprise

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

type testEnv struct {
	pool           *pgxpool.Pool
	rfxRepo        *repository.RfxRepository
	auditRepo      *repository.AuditRepository
	bidRepo        *repository.BidRepository
	frRepo         *repository.FreightRequestRepository
	membershipRepo *repository.MembershipRepository
	rfxSvc         *service.RfxService
	bidSvc         *service.BidService
	frSvc          *service.FreightRequestService
}

type buyerFixture struct {
	TenantID   uuid.UUID
	CompanyA   uuid.UUID
	CompanyB   uuid.UUID
	BuyerA     domain.ActorContext
	BuyerB     domain.ActorContext
	CarrierID  uuid.UUID
	CarrierAct domain.ActorContext
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres unavailable: %v", err)
	}

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rfxRepo := repository.NewRfxRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	bidRepo := repository.NewBidRepository(pool)
	frRepo := repository.NewFreightRequestRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionTOStub(pool))
	bidSvc := service.NewBidServiceWithAtomic(pool, bidRepo, frRepo, membershipRepo, auditRepo)
	frSvc := service.NewFreightRequestServiceWithAuth(frRepo, membershipRepo)
	t.Logf("isolated database=%s", dbName)
	return &testEnv{
		pool:           pool,
		rfxRepo:        rfxRepo,
		auditRepo:      auditRepo,
		bidRepo:        bidRepo,
		frRepo:         frRepo,
		membershipRepo: membershipRepo,
		rfxSvc:         rfxSvc,
		bidSvc:         bidSvc,
		frSvc:          frSvc,
	}
}

func seedBuyerFixture(t *testing.T, env *testEnv) buyerFixture {
	t.Helper()
	ctx := context.Background()
	fix := buyerFixture{
		TenantID:  uuid.New(),
		CompanyA:  uuid.New(),
		CompanyB:  uuid.New(),
		CarrierID: uuid.New(),
	}
	fix.BuyerA = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.BuyerB = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CarrierAct = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}

	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Enterprise Auth Tenant")
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, company := range []struct {
		id   uuid.UUID
		name string
		typ  string
	}{
		{fix.CompanyA, "Company A", "SHIPPER"},
		{fix.CompanyB, "Company B", "SHIPPER"},
		{fix.CarrierID, "Carrier C", "CARRIER"},
	} {
		_, err = env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
			company.id, fix.TenantID, company.name, company.typ)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, user := range []struct {
		id    uuid.UUID
		email string
	}{
		{fix.BuyerA.UserID, "buyer-a@test.local"},
		{fix.BuyerB.UserID, "buyer-b@test.local"},
		{fix.CarrierAct.UserID, "carrier@test.local"},
	} {
		_, err = env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
			user.id, fix.TenantID, user.email, user.email)
		if err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3), ($1, $4, $5), ($1, $6, $7)`,
		fix.TenantID, fix.CompanyA, fix.BuyerA.UserID, fix.CompanyB, fix.BuyerB.UserID, fix.CarrierID, fix.CarrierAct.UserID)
	if err != nil {
		t.Fatalf("seed memberships: %v", err)
	}
	var roleID uuid.UUID
	err = env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&roleID)
	if err != nil {
		t.Fatalf("lookup role: %v", err)
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4), ($1, $5, $6, $4)`,
		fix.TenantID, fix.BuyerA.UserID, fix.CompanyA, roleID, fix.BuyerB.UserID, fix.CompanyB)
	if err != nil {
		t.Fatalf("seed buyer roles: %v", err)
	}
	var carrierRoleID uuid.UUID
	err = env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID)
	if err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, fix.CarrierAct.UserID, fix.CarrierID, carrierRoleID)
	if err != nil {
		t.Fatalf("seed carrier role: %v", err)
	}
	return fix
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName = "rfx_enterprise_auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = adminDB
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, err
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, err
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL = buildDSN(testCfg)
	cleanup = func(cctx context.Context) {
		cadmin, cerr := pgxpool.NewWithConfig(cctx, adminCfg)
		if cerr != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return dbName, testURL, cleanup, nil
}

func buildDSN(cfg *pgxpool.Config) string {
	user := url.QueryEscape(cfg.ConnConfig.User)
	pass := url.QueryEscape(cfg.ConnConfig.Password)
	host := cfg.ConnConfig.Host
	port := cfg.ConnConfig.Port
	db := cfg.ConnConfig.Database
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, pass, host, port, db)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	return "", fmt.Errorf("migrations dir not found from %s", wd)
}

func countAuditEvents(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, entityID uuid.UUID, action string) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id = $1 AND entity_id = $2 AND action = $3
	`, tenantID, entityID, action).Scan(&count)
	return count, err
}

func countRfxEvents(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id = $1`, tenantID).Scan(&count)
	return count, err
}
