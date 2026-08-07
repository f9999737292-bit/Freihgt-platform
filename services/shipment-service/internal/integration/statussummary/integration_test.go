//go:build integration

package statussummary

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
	"github.com/freight-platform/shipment-service/internal/service"
)

type testEnv struct {
	pool *pgxpool.Pool
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping live PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
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

	return &testEnv{pool: pool}
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}

	dbName = "freight_platform_status_summary_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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

type tenantFixture struct {
	TenantID         uuid.UUID
	ShipperID        uuid.UUID
	ConsigneeID      uuid.UUID
	CarrierID        uuid.UUID
	OriginID         uuid.UUID
	DestinationID    uuid.UUID
	TransportOrderID uuid.UUID
}

func seedTenantFixture(t *testing.T, pool *pgxpool.Pool, suffix string) tenantFixture {
	t.Helper()
	ctx := context.Background()
	fix := tenantFixture{
		TenantID:      uuid.New(),
		ShipperID:     uuid.New(),
		ConsigneeID:   uuid.New(),
		CarrierID:     uuid.New(),
		OriginID:      uuid.New(),
		DestinationID: uuid.New(),
	}
	fix.TransportOrderID = uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)
	`, fix.TenantID, "t-"+suffix, "Status Summary Tenant "+suffix)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id   uuid.UUID
		typ  string
		name string
	}{
		{fix.ShipperID, "SHIPPER", "Shipper " + suffix},
		{fix.ConsigneeID, "CONSIGNEE", "Consignee " + suffix},
		{fix.CarrierID, "CARRIER", "Carrier " + suffix},
	} {
		_, err = pool.Exec(ctx, `
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
		{fix.OriginID, "Origin " + suffix},
		{fix.DestinationID, "Destination " + suffix},
	} {
		_, err = pool.Exec(ctx, `
			INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code)
			VALUES ($1, $2, 'WAREHOUSE', $3, 'RU')
		`, loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, transport_mode
		) VALUES ($1, $2, $3, 'ASSIGNED', $4, $5, $6, $7, 'ROAD')
	`, fix.TransportOrderID, fix.TenantID, "TO-"+suffix,
		fix.ShipperID, fix.ConsigneeID, fix.OriginID, fix.DestinationID)
	if err != nil {
		t.Fatalf("seed transport order: %v", err)
	}
	return fix
}

func insertShipments(t *testing.T, pool *pgxpool.Pool, fix tenantFixture, status string, count int, softDeleted bool) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		id := uuid.New()
		number := fmt.Sprintf("SHP-%s-%s-%d", fix.TenantID.String()[:8], status, i)
		if softDeleted {
			_, err := pool.Exec(ctx, `
				INSERT INTO transport.shipments (
					id, tenant_id, shipment_number, transport_order_id,
					shipper_company_id, consignee_company_id, carrier_company_id,
					origin_location_id, destination_location_id, transport_mode, status, version, deleted_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1,now())
			`, id, fix.TenantID, number, fix.TransportOrderID,
				fix.ShipperID, fix.ConsigneeID, fix.CarrierID, fix.OriginID, fix.DestinationID, status)
			if err != nil {
				t.Fatalf("insert soft-deleted shipment: %v", err)
			}
			continue
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO transport.shipments (
				id, tenant_id, shipment_number, transport_order_id,
				shipper_company_id, consignee_company_id, carrier_company_id,
				origin_location_id, destination_location_id, transport_mode, status, version
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1)
		`, id, fix.TenantID, number, fix.TransportOrderID,
			fix.ShipperID, fix.ConsigneeID, fix.CarrierID, fix.OriginID, fix.DestinationID, status)
		if err != nil {
			t.Fatalf("insert shipment: %v", err)
		}
	}
}

func TestStatusSummaryIntegration(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	tenantA := seedTenantFixture(t, env.pool, "A")
	tenantB := seedTenantFixture(t, env.pool, "B")
	emptyTenant := seedTenantFixture(t, env.pool, "empty")

	insertShipments(t, env.pool, tenantA, domain.ShipmentStatusCarrierAssigned, 2, false)
	insertShipments(t, env.pool, tenantA, domain.ShipmentStatusInTransit, 3, false)
	insertShipments(t, env.pool, tenantA, domain.ShipmentStatusDelivered, 4, false)
	insertShipments(t, env.pool, tenantA, domain.ShipmentStatusCancelled, 1, true)

	insertShipments(t, env.pool, tenantB, domain.ShipmentStatusCancelled, 5, false)

	repo := repository.NewShipmentStatusSummaryRepository(env.pool)
	svc := service.NewStatusSummaryService(repo)

	summaryA, err := svc.GetStatusSummary(ctx, tenantA.TenantID)
	if err != nil {
		t.Fatalf("tenant A summary: %v", err)
	}
	if summaryA.TotalShipments != 9 || summaryA.CountedShipments != 9 {
		t.Fatalf("tenant A totals=%d counted=%d want 9/9", summaryA.TotalShipments, summaryA.CountedShipments)
	}
	if summaryA.ByStatus[domain.ShipmentStatusCarrierAssigned] != 2 {
		t.Fatalf("tenant A carrier assigned=%d want 2", summaryA.ByStatus[domain.ShipmentStatusCarrierAssigned])
	}
	if summaryA.ByStatus[domain.ShipmentStatusInTransit] != 3 {
		t.Fatalf("tenant A in transit=%d want 3", summaryA.ByStatus[domain.ShipmentStatusInTransit])
	}
	if summaryA.ByStatus[domain.ShipmentStatusDelivered] != 4 {
		t.Fatalf("tenant A delivered=%d want 4", summaryA.ByStatus[domain.ShipmentStatusDelivered])
	}
	if _, ok := summaryA.ByStatus[domain.ShipmentStatusCancelled]; ok {
		t.Fatal("tenant A must exclude soft-deleted cancelled shipment")
	}
	if !summaryA.Complete {
		t.Fatal("tenant A summary must be complete")
	}

	summaryB, err := svc.GetStatusSummary(ctx, tenantB.TenantID)
	if err != nil {
		t.Fatalf("tenant B summary: %v", err)
	}
	if summaryB.TotalShipments != 5 || summaryB.CountedShipments != 5 {
		t.Fatalf("tenant B totals=%d counted=%d want 5/5", summaryB.TotalShipments, summaryB.CountedShipments)
	}
	if summaryB.ByStatus[domain.ShipmentStatusCancelled] != 5 {
		t.Fatalf("tenant B cancelled=%d want 5", summaryB.ByStatus[domain.ShipmentStatusCancelled])
	}
	if summaryB.ByStatus[domain.ShipmentStatusCarrierAssigned] != 0 {
		t.Fatalf("tenant B must not include tenant A shipments")
	}

	summaryEmpty, err := svc.GetStatusSummary(ctx, emptyTenant.TenantID)
	if err != nil {
		t.Fatalf("empty tenant summary: %v", err)
	}
	if summaryEmpty.TotalShipments != 0 || summaryEmpty.CountedShipments != 0 {
		t.Fatalf("empty tenant totals=%d counted=%d want 0/0", summaryEmpty.TotalShipments, summaryEmpty.CountedShipments)
	}
	if len(summaryEmpty.ByStatus) != 0 {
		t.Fatalf("empty tenant byStatus=%#v want empty", summaryEmpty.ByStatus)
	}
	if !summaryEmpty.Complete {
		t.Fatal("empty tenant summary must be complete")
	}

	rowsB, err := repo.GetStatusSummary(ctx, tenantB.TenantID)
	if err != nil {
		t.Fatalf("tenant B repository rows: %v", err)
	}
	if len(rowsB) != 1 || rowsB[0].Status != domain.ShipmentStatusCancelled || rowsB[0].ShipmentCount != 5 {
		t.Fatalf("tenant B repository rows=%#v", rowsB)
	}
}
