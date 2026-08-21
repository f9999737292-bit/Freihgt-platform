//go:build integration

package laneownership

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
	membershipRepo *repository.MembershipRepository
	rfxSvc         *service.RfxService
}

type laneFixture struct {
	TenantA   uuid.UUID
	TenantB   uuid.UUID
	CompanyA  uuid.UUID
	CompanyB  uuid.UUID
	CompanyC  uuid.UUID
	BuyerA    domain.ActorContext
	BuyerB    domain.ActorContext
	BuyerC    domain.ActorContext
	EventA    uuid.UUID
	EventB    uuid.UUID
	EventC    uuid.UUID
	LotA      uuid.UUID
	LotB      uuid.UUID
	LotC      uuid.UUID
	OriginID  uuid.UUID
	DestID    uuid.UUID
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
	membershipRepo := repository.NewMembershipRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, repository.NewAuditRepository(pool), membershipRepo, nil)
	t.Logf("isolated database=%s", dbName)
	return &testEnv{
		pool:           pool,
		rfxRepo:        rfxRepo,
		membershipRepo: membershipRepo,
		rfxSvc:         rfxSvc,
	}
}

func seedLaneFixture(t *testing.T, env *testEnv) laneFixture {
	t.Helper()
	ctx := context.Background()
	fix := laneFixture{
		TenantA:  uuid.New(),
		TenantB:  uuid.New(),
		CompanyA: uuid.New(),
		CompanyB: uuid.New(),
		CompanyC: uuid.New(),
		EventA:   uuid.New(),
		EventB:   uuid.New(),
		EventC:   uuid.New(),
		LotA:     uuid.New(),
		LotB:     uuid.New(),
		LotC:     uuid.New(),
		OriginID: uuid.New(),
		DestID:   uuid.New(),
	}
	fix.BuyerA = domain.ActorContext{TenantID: fix.TenantA, UserID: uuid.New()}
	fix.BuyerB = domain.ActorContext{TenantID: fix.TenantA, UserID: uuid.New()}
	fix.BuyerC = domain.ActorContext{TenantID: fix.TenantB, UserID: uuid.New()}

	for _, tenant := range []struct {
		id   uuid.UUID
		code string
	}{
		{fix.TenantA, "ta-" + fix.TenantA.String()[:8]},
		{fix.TenantB, "tb-" + fix.TenantB.String()[:8]},
	} {
		_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
			tenant.id, tenant.code, "Lane Tenant "+tenant.code)
		if err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}

	companies := []struct {
		id       uuid.UUID
		tenantID uuid.UUID
		name     string
	}{
		{fix.CompanyA, fix.TenantA, "Company A"},
		{fix.CompanyB, fix.TenantA, "Company B"},
		{fix.CompanyC, fix.TenantB, "Company C"},
	}
	for _, c := range companies {
		_, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, 'SHIPPER')`,
			c.id, c.tenantID, c.name)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}

	users := []struct {
		id       uuid.UUID
		tenantID uuid.UUID
		email    string
	}{
		{fix.BuyerA.UserID, fix.TenantA, "buyer-a@test.local"},
		{fix.BuyerB.UserID, fix.TenantA, "buyer-b@test.local"},
		{fix.BuyerC.UserID, fix.TenantB, "buyer-c@test.local"},
	}
	for _, u := range users {
		_, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
			u.id, u.tenantID, u.email, u.email)
		if err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	_, err := env.pool.Exec(ctx, `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES
		($1, $2, $3), ($1, $4, $5), ($6, $7, $8)
	`, fix.TenantA, fix.CompanyA, fix.BuyerA.UserID, fix.CompanyB, fix.BuyerB.UserID, fix.TenantB, fix.CompanyC, fix.BuyerC.UserID)
	if err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	var roleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&roleID); err != nil {
		t.Fatalf("lookup role: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES
		($1, $2, $3, $4), ($1, $5, $6, $4), ($7, $8, $9, $4)
	`, fix.TenantA, fix.BuyerA.UserID, fix.CompanyA, roleID, fix.BuyerB.UserID, fix.CompanyB, fix.TenantB, fix.BuyerC.UserID, fix.CompanyC)
	if err != nil {
		t.Fatalf("seed buyer roles: %v", err)
	}

	events := []struct {
		id              uuid.UUID
		tenantID        uuid.UUID
		number          string
		ownerCompanyID  uuid.UUID
	}{
		{fix.EventA, fix.TenantA, "RFX-A-1", fix.CompanyA},
		{fix.EventB, fix.TenantA, "RFX-B-1", fix.CompanyB},
		{fix.EventC, fix.TenantB, "RFX-C-1", fix.CompanyC},
	}
	for _, e := range events {
		_, err = env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_events (id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status)
			VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', $4, $5, 'DRAFT')
		`, e.id, e.tenantID, e.number, "Event "+e.number, e.ownerCompanyID)
		if err != nil {
			t.Fatalf("seed event: %v", err)
		}
	}

	lots := []struct {
		id       uuid.UUID
		tenantID uuid.UUID
		eventID  uuid.UUID
		number   string
	}{
		{fix.LotA, fix.TenantA, fix.EventA, "LOT-A"},
		{fix.LotB, fix.TenantA, fix.EventB, "LOT-B"},
		{fix.LotC, fix.TenantB, fix.EventC, "LOT-C"},
	}
	for _, l := range lots {
		_, err = env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_lots (id, tenant_id, rfx_event_id, lot_number, name, status)
			VALUES ($1, $2, $3, $4, $5, 'ACTIVE')
		`, l.id, l.tenantID, l.eventID, l.number, "Lot "+l.number)
		if err != nil {
			t.Fatalf("seed lot: %v", err)
		}
	}
	return fix
}

func laneInput(tenantID, lotID, originID, destID uuid.UUID) domain.CreateRfxLaneInput {
	return domain.CreateRfxLaneInput{
		TenantID:              tenantID,
		RfxLotID:              lotID,
		OriginLocationID:      originID,
		DestinationLocationID: destID,
		TransportMode:         "ROAD",
	}
}

func countLanes(ctx context.Context, pool *pgxpool.Pool, lotID uuid.UUID) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_lanes WHERE rfx_lot_id = $1`, lotID).Scan(&count)
	return count, err
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
	dbName = "rfx_lane_ownership_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
