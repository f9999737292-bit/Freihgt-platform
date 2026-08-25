//go:build integration

package systemwave2

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

type wave2Fixture struct {
	TenantA        uuid.UUID
	TenantB        uuid.UUID
	BuyerA         uuid.UUID
	CarrierA1      uuid.UUID
	CarrierA2      uuid.UUID
	BuyerB         uuid.UUID
	CarrierB1      uuid.UUID
	ConsigneeA     uuid.UUID
	BuyerAdminA    domain.ActorContext
	BuyerOperatorA domain.ActorContext
	CarrierA1Act   domain.ActorContext
	CarrierA2Act   domain.ActorContext
	BuyerAdminB    domain.ActorContext
	CarrierB1Act   domain.ActorContext
	OriginID       uuid.UUID
	DestID         uuid.UUID
	CargoID        uuid.UUID
	TransportOrder uuid.UUID
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
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
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

func seedWave2Fixture(t *testing.T, env *testEnv) wave2Fixture {
	t.Helper()
	ctx := context.Background()
	fix := wave2Fixture{
		TenantA:        uuid.New(),
		TenantB:        uuid.New(),
		BuyerA:         uuid.New(),
		CarrierA1:      uuid.New(),
		CarrierA2:      uuid.New(),
		BuyerB:         uuid.New(),
		CarrierB1:      uuid.New(),
		ConsigneeA:     uuid.New(),
		BuyerAdminA:    domain.ActorContext{TenantID: uuid.Nil, UserID: uuid.New()},
		BuyerOperatorA: domain.ActorContext{TenantID: uuid.Nil, UserID: uuid.New()},
		CarrierA1Act:   domain.ActorContext{TenantID: uuid.Nil, UserID: uuid.New()},
		CarrierA2Act:   domain.ActorContext{TenantID: uuid.Nil, UserID: uuid.New()},
		BuyerAdminB:    domain.ActorContext{TenantID: uuid.Nil, UserID: uuid.New()},
		CarrierB1Act:   domain.ActorContext{TenantID: uuid.Nil, UserID: uuid.New()},
		OriginID:       uuid.New(),
		DestID:         uuid.New(),
		CargoID:        uuid.New(),
		TransportOrder: uuid.New(),
	}
	fix.BuyerAdminA.TenantID = fix.TenantA
	fix.BuyerOperatorA.TenantID = fix.TenantA
	fix.CarrierA1Act.TenantID = fix.TenantA
	fix.CarrierA2Act.TenantID = fix.TenantA
	fix.BuyerAdminB.TenantID = fix.TenantB
	fix.CarrierB1Act.TenantID = fix.TenantB

	for _, row := range []struct {
		id   uuid.UUID
		code string
		name string
	}{
		{fix.TenantA, "tenant-a-" + fix.TenantA.String()[:8], "FP Test Tenant A"},
		{fix.TenantB, "tenant-b-" + fix.TenantB.String()[:8], "FP Test Tenant B"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name, country_code, default_locale, default_currency)
			VALUES ($1,$2,$3,'RU','ru-RU','RUB')`, row.id, row.code, row.name); err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}

	type companySeed struct {
		id, tenant uuid.UUID
		name, typ  string
	}
	for _, c := range []companySeed{
		{fix.BuyerA, fix.TenantA, "Buyer Company A", "SHIPPER"},
		{fix.CarrierA1, fix.TenantA, "Carrier A1", "CARRIER"},
		{fix.CarrierA2, fix.TenantA, "Carrier A2", "CARRIER"},
		{fix.ConsigneeA, fix.TenantA, "Consignee A", "CONSIGNEE"},
		{fix.BuyerB, fix.TenantB, "Buyer Company B", "SHIPPER"},
		{fix.CarrierB1, fix.TenantB, "Carrier B1", "CARRIER"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, c.id, c.tenant, c.name, c.typ); err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}

	type userSeed struct {
		act   *domain.ActorContext
		email string
	}
	for _, u := range []userSeed{
		{&fix.BuyerAdminA, "a-buyer-admin@test.local"},
		{&fix.BuyerOperatorA, "a-buyer-operator@test.local"},
		{&fix.CarrierA1Act, "a-carrier1@test.local"},
		{&fix.CarrierA2Act, "a-carrier2@test.local"},
		{&fix.BuyerAdminB, "b-buyer-admin@test.local"},
		{&fix.CarrierB1Act, "b-carrier1@test.local"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name)
			VALUES ($1,$2,$3,$4)`, u.act.UserID, u.act.TenantID, u.email, u.email); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES
		($1,$2,$3),($1,$4,$5),($1,$6,$7),($1,$8,$9),
		($10,$11,$12),($10,$13,$14)`,
		fix.TenantA, fix.BuyerA, fix.BuyerAdminA.UserID,
		fix.BuyerA, fix.BuyerOperatorA.UserID,
		fix.CarrierA1, fix.CarrierA1Act.UserID,
		fix.CarrierA2, fix.CarrierA2Act.UserID,
		fix.TenantB, fix.BuyerB, fix.BuyerAdminB.UserID,
		fix.CarrierB1, fix.CarrierB1Act.UserID,
	); err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	var buyerRoleID, carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&buyerRoleID); err != nil {
		t.Fatalf("lookup buyer role: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES
		($1,$2,$3,$4),($1,$5,$3,$4),($1,$6,$7,$8),($1,$9,$10,$8),
		($11,$12,$13,$4),($11,$14,$15,$8)`,
		fix.TenantA, fix.BuyerAdminA.UserID, fix.BuyerA, buyerRoleID,
		fix.BuyerOperatorA.UserID,
		fix.CarrierA1Act.UserID, fix.CarrierA1, carrierRoleID,
		fix.CarrierA2Act.UserID, fix.CarrierA2,
		fix.TenantB, fix.BuyerAdminB.UserID, fix.BuyerB,
		fix.CarrierB1Act.UserID, fix.CarrierB1,
	); err != nil {
		t.Fatalf("seed roles: %v", err)
	}

	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Moscow WH"},
		{fix.DestID, "SPB DC"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantA, fix.BuyerA, loc.name); err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		VALUES ($1,$2,'FMCG','Wave2 cargo',20000)`, fix.CargoID, fix.TenantA); err != nil {
		t.Fatalf("seed cargo: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO transport.transport_orders (
		id, tenant_id, order_number, shipper_company_id, consignee_company_id,
		origin_location_id, destination_location_id, cargo_id, transport_mode, status, source_system
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'ROAD','READY_FOR_SOURCING','wave2_test')`,
		fix.TransportOrder, fix.TenantA, "TO-W2-"+fix.TransportOrder.String()[:8],
		fix.BuyerA, fix.ConsigneeA, fix.OriginID, fix.DestID, fix.CargoID,
	); err != nil {
		t.Fatalf("seed transport order: %v", err)
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
	dbName = "rfx_wave2_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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

func ptrFloat(v float64) *float64 { return &v }

func ptrString(v string) *string { return &v }

func futureDeadline() *time.Time {
	t := time.Now().UTC().Add(72 * time.Hour)
	return &t
}
