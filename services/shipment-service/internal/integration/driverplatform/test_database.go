//go:build integration

package driverplatform

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type testEnv struct {
	pool           *pgxpool.Pool
	driverOps      *service.DriverOperationsService
	driverOpsRepo  *repository.DriverOperationsRepository
	shipmentRepo   *repository.ShipmentRepository
	embeddedStop   func() error
}

func setupTestEnv(t *testing.T) *testEnv {
	return setupTestEnvWithMigrations(t, false)
}

func setupFullPlatformTestEnv(t *testing.T) *testEnv {
	return setupTestEnvWithMigrations(t, true)
}

func setupTestEnvWithMigrations(t *testing.T, fullPlatform bool) *testEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	var embeddedStop func() error
	if adminURL == "" {
		port := pickFreePort(t)
		runtimePath := filepath.Join(t.TempDir(), "pg-runtime")
		dataPath := filepath.Join(t.TempDir(), "pg-data")
		pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
			Port(uint32(port)).
			RuntimePath(runtimePath).
			DataPath(dataPath).
			Database("freight_driver_test").
			Username("freight").
			Password("freight").
			Version(embeddedpostgres.V16))
		if err := pg.Start(); err != nil {
			t.Fatalf("start embedded postgres: %v", err)
		}
		embeddedStop = pg.Stop
		adminURL = fmt.Sprintf("postgres://freight:freight@localhost:%d/postgres?sslmode=disable", port)
	}

	dbName := "freight_driver_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	testURL, cleanupDB, err := createTempDatabase(ctx, adminURL, dbName)
	if err != nil {
		if embeddedStop != nil {
			_ = embeddedStop()
		}
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() {
		cleanupDB(context.Background())
		if embeddedStop != nil {
			_ = embeddedStop()
		}
	})

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if fullPlatform {
		if err := applyFullPlatformMigrations(ctx, pool); err != nil {
			t.Fatalf("apply migrations: %v", err)
		}
	} else if err := applyAllMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	driverRepo := repository.NewDriverRepository(pool)
	shipmentRepo := repository.NewShipmentRepository(pool)
	driverOpsRepo := repository.NewDriverOperationsRepository(pool)
	driverOpsSvc := service.NewDriverOperationsService(driverRepo, shipmentRepo, driverOpsRepo)

	return &testEnv{
		pool:          pool,
		driverOps:     driverOpsSvc,
		driverOpsRepo: driverOpsRepo,
		shipmentRepo:  shipmentRepo,
		embeddedStop:  embeddedStop,
	}
}

func pickFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func createTempDatabase(ctx context.Context, adminURL, dbName string) (testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", nil, err
	}
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", nil, err
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", nil, err
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testCfg.ConnConfig.User, testCfg.ConnConfig.Password,
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, dbName)
	cleanup = func(cctx context.Context) {
		cadmin, cerr := pgxpool.NewWithConfig(cctx, adminCfg)
		if cerr != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return testURL, cleanup, nil
}

func applyAllMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return applyMigrations(ctx, pool, false)
}

func applyFullPlatformMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return applyMigrations(ctx, pool, true)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, fullPlatform bool) error {
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
		base := filepath.Base(file)
		if !fullPlatform && strings.Contains(base, "000020") {
			continue
		}
		if !fullPlatform {
			num := migrationNumber(base)
			if num > 14 && num != 30 && num != 31 {
				continue
			}
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("apply %s: %w", base, execErr)
		}
	}
	return nil
}

func migrationNumber(filename string) int {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) == 0 {
		return 0
	}
	var n int
	fmt.Sscanf(parts[0], "%d", &n)
	return n
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

type driverFixture struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	DriverID   uuid.UUID
	CarrierID  uuid.UUID
	ShipmentID uuid.UUID
}

func seedDriverFixture(t *testing.T, pool *pgxpool.Pool) driverFixture {
	t.Helper()
	ctx := context.Background()
	fix := driverFixture{
		TenantID: uuid.New(),
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		CarrierID: uuid.New(),
		ShipmentID: uuid.New(),
	}
	shipperID := uuid.New()
	consigneeID := uuid.New()
	originID := uuid.New()
	destID := uuid.New()
	orderID := uuid.New()

	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Driver Test Tenant")
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id uuid.UUID
		typ, name string
	}{
		{fix.CarrierID, "CARRIER", "Carrier"},
		{shipperID, "SHIPPER", "Shipper"},
		{consigneeID, "CONSIGNEE", "Consignee"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, fix.TenantID, row.name, row.typ)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, loc := range []struct {
		id uuid.UUID
		name string
	}{
		{originID, "Origin"},
		{destID, "Destination"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
		VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')`,
		orderID, fix.TenantID, "TO-1", shipperID, consigneeID, originID, destID)
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,$5,'ACTIVE')`,
		fix.DriverID, fix.TenantID, fix.CarrierID, fix.UserID, "Driver A")
	if err != nil {
		t.Fatalf("seed driver: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.shipments (id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, driver_id, origin_location_id, destination_location_id, transport_mode, status, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ROAD','PICKUP_SLOT_BOOKED',1)`,
		fix.ShipmentID, fix.TenantID, "SHP-1", orderID, shipperID, consigneeID, fix.CarrierID, fix.DriverID, originID, destID)
	if err != nil {
		t.Fatalf("seed shipment: %v", err)
	}
	return fix
}

func userTransition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func countRows(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int64 {
	var n int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		panic(err)
	}
	return n
}
