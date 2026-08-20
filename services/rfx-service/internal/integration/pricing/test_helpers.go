//go:build integration

package pricing

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
	pool    *pgxpool.Pool
	rfxSvc  *service.RfxService
}

type fixture struct {
	tenantID       uuid.UUID
	buyerCompany   uuid.UUID
	carrierCompany uuid.UUID
	originID       uuid.UUID
	destID         uuid.UUID
	buyer          domain.ActorContext
	carrier        domain.ActorContext
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("postgres unavailable: %v", err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	rfxRepo := repository.NewRfxRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	return &testEnv{pool: pool, rfxSvc: service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, nil)}
}

func seedFixture(t *testing.T, env *testEnv) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		tenantID:       uuid.New(),
		buyerCompany:   uuid.New(),
		carrierCompany: uuid.New(),
		originID:       uuid.New(),
		destID:         uuid.New(),
	}
	fix.buyer = domain.ActorContext{TenantID: fix.tenantID, UserID: uuid.New()}
	fix.carrier = domain.ActorContext{TenantID: fix.tenantID, UserID: uuid.New()}
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, fix.tenantID, "t-"+fix.tenantID.String()[:8], "Pricing Tenant")
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	for _, row := range []struct {
		id uuid.UUID
		name, typ string
	}{
		{fix.buyerCompany, "Buyer", "SHIPPER"},
		{fix.carrierCompany, "Carrier", "CARRIER"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, fix.tenantID, row.name, row.typ); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	for _, locID := range []uuid.UUID{fix.originID, fix.destID} {
		if _, err := env.pool.Exec(ctx, `
			INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code, city)
			VALUES ($1,$2,'WAREHOUSE','WH','RU','Moscow')`, locID, fix.tenantID); err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	for _, user := range []struct {
		id    uuid.UUID
		email string
	}{
		{fix.buyer.UserID, "buyer@test.local"},
		{fix.carrier.UserID, "carrier@test.local"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`,
			user.id, fix.tenantID, user.email, user.email); err != nil {
			t.Fatalf("user: %v", err)
		}
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3), ($1,$4,$5)`,
		fix.tenantID, fix.buyerCompany, fix.buyer.UserID, fix.carrierCompany, fix.carrier.UserID); err != nil {
		t.Fatalf("membership: %v", err)
	}
	var buyerRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&buyerRoleID); err != nil {
		t.Fatalf("buyer role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.tenantID, fix.buyer.UserID, fix.buyerCompany, buyerRoleID); err != nil {
		t.Fatalf("buyer role assign: %v", err)
	}
	var carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.tenantID, fix.carrier.UserID, fix.carrierCompany, carrierRoleID); err != nil {
		t.Fatalf("carrier role assign: %v", err)
	}
	return fix
}

func createTempDatabase(ctx context.Context, adminURL string) (string, string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, err
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName := "rfx_pricing_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(testCfg.ConnConfig.User),
		url.QueryEscape(testCfg.ConnConfig.Password),
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, testCfg.ConnConfig.Database)
	cleanup := func(cctx context.Context) {
		cadmin, _ := pgxpool.NewWithConfig(cctx, adminCfg)
		if cadmin != nil {
			defer cadmin.Close()
			_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		}
	}
	return dbName, testURL, cleanup, nil
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
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), execErr)
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
	return "", fmt.Errorf("migrations dir not found")
}
