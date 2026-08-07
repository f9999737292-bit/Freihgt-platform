//go:build integration

package outbox

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

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type testEnv struct {
	t        *testing.T
	pool     *pgxpool.Pool
	repo     *repository.ShipmentRepository
	adminURL string
	dbName   string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping live PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return &testEnv{
		t:        t,
		pool:     pool,
		repo:     repository.NewShipmentRepository(pool),
		adminURL: adminURL,
		dbName:   dbName,
	}
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}

	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}

	dbName = "freight_platform_outbox_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"

	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, fmt.Errorf("connect admin database: %w", err)
	}
	defer adminPool.Close()

	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, fmt.Errorf("create database: %w", err)
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
	ssl := "disable"
	if cfg.ConnConfig.TLSConfig != nil {
		ssl = "require"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, pass, host, port, db, ssl)
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

type seedFixture struct {
	TenantID         uuid.UUID
	ShipperID        uuid.UUID
	ConsigneeID      uuid.UUID
	CarrierID        uuid.UUID
	OriginID         uuid.UUID
	DestinationID    uuid.UUID
	TransportOrderID uuid.UUID
	UserID           uuid.UUID
}

func (env *testEnv) seedFixture(t *testing.T) seedFixture {
	t.Helper()
	ctx := context.Background()
	fix := seedFixture{
		TenantID:      uuid.New(),
		ShipperID:     uuid.New(),
		ConsigneeID:   uuid.New(),
		CarrierID:     uuid.New(),
		OriginID:      uuid.New(),
		DestinationID: uuid.New(),
		UserID:        uuid.New(),
	}
	fix.TransportOrderID = uuid.New()

	_, err := env.pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)
	`, fix.TenantID, "t-"+fix.TenantID.String()[:8], "Outbox Test Tenant")
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id   uuid.UUID
		typ  string
		name string
	}{
		{fix.ShipperID, "SHIPPER", "Shipper"},
		{fix.ConsigneeID, "CONSIGNEE", "Consignee"},
		{fix.CarrierID, "CARRIER", "Carrier"},
	} {
		_, err = env.pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, legal_name, company_type)
			VALUES ($1, $2, $3, $4)
		`, row.id, fix.TenantID, row.name, row.typ)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestinationID, "Destination"},
	} {
		_, err = env.pool.Exec(ctx, `
			INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code)
			VALUES ($1, $2, 'WAREHOUSE', $3, 'RU')
		`, loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, transport_mode
		) VALUES ($1, $2, $3, 'ASSIGNED', $4, $5, $6, $7, 'ROAD')
	`, fix.TransportOrderID, fix.TenantID, "TO-"+fix.TransportOrderID.String()[:8],
		fix.ShipperID, fix.ConsigneeID, fix.OriginID, fix.DestinationID)
	if err != nil {
		t.Fatalf("seed transport order: %v", err)
	}
	return fix
}

func userTransition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func claimNow() time.Time {
	return time.Now().UTC().Add(time.Minute)
}

func countRows(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int64 {
	var n int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		panic(err)
	}
	return n
}
