//go:build integration

package statussnapshot

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
	snap "github.com/freight-platform/shipment-service/internal/statussnapshot"
)

type testEnv struct {
	pool *pgxpool.Pool
	repo *repository.ShipmentRepository
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &testEnv{pool: pool, repo: repository.NewShipmentRepository(pool)}
}

func createTempDatabase(ctx context.Context, adminURL string) (string, string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, err
	}
	dbName := "freight_snapshot_export_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
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
		url.QueryEscape(testCfg.ConnConfig.User), url.QueryEscape(testCfg.ConnConfig.Password),
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
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	for _, c := range []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", os.ErrNotExist
}

type fixture struct {
	TenantA, TenantB                          uuid.UUID
	ShipperA, ConsigneeA, CarrierA            uuid.UUID
	OriginA, DestA, TransportOrderA           uuid.UUID
	ShipperB, ConsigneeB, CarrierB            uuid.UUID
	OriginB, DestB, TransportOrderB           uuid.UUID
}

func seedTenant(ctx context.Context, pool *pgxpool.Pool, code string) (tenantID, shipper, consignee, carrier, origin, dest, transportOrder uuid.UUID) {
	tenantID = uuid.New()
	shipper, consignee, carrier = uuid.New(), uuid.New(), uuid.New()
	origin, dest = uuid.New(), uuid.New()
	transportOrder = uuid.New()
	_, _ = pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, tenantID, code, code)
	for _, row := range []struct{ id uuid.UUID; typ, name string }{
		{shipper, "SHIPPER", "Shipper"}, {consignee, "CONSIGNEE", "Consignee"}, {carrier, "CARRIER", "Carrier"},
	} {
		_, _ = pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, tenantID, row.name, row.typ)
	}
	for _, loc := range []struct{ id uuid.UUID; name string }{{origin, "Origin"}, {dest, "Destination"}} {
		_, _ = pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, tenantID, loc.name)
	}
	_, _ = pool.Exec(ctx, `
INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')`, transportOrder, tenantID, "TO-"+code, shipper, consignee, origin, dest)
	return
}

func (env *testEnv) seedFixtures(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	f := fixture{}
	f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA = seedTenant(ctx, env.pool, "TA")
	f.TenantB, f.ShipperB, f.ConsigneeB, f.CarrierB, f.OriginB, f.DestB, f.TransportOrderB = seedTenant(ctx, env.pool, "TB")
	return f
}

func createParams(f fixture, tenant uuid.UUID, shipper, consignee, carrier, origin, dest, transportOrder uuid.UUID, number string) repository.CreateShipmentParams {
	return repository.CreateShipmentParams{
		TenantID: tenant, ShipmentNumber: number, TransportOrderID: transportOrder,
		ShipperCompanyID: shipper, ConsigneeCompanyID: consignee, CarrierCompanyID: carrier,
		OriginLocationID: origin, DestinationLocationID: dest, TransportMode: "ROAD",
	}
}

func userTransition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func TestPostgresRepositoryTenantScope(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	_, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-A1"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.repo.CreateShipment(ctx, createParams(f, f.TenantB, f.ShipperB, f.ConsigneeB, f.CarrierB, f.OriginB, f.DestB, f.TransportOrderB, "SHP-B1"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}

	repo := snap.NewPostgresSnapshotRepository(env.pool)
	var rows []snap.ShipmentSnapshotRow
	stats, err := repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowCount != 1 || len(rows) != 1 {
		t.Fatalf("want 1 row, got stats=%+v rows=%d", stats, len(rows))
	}
	if rows[0].TenantID != f.TenantA {
		t.Fatal("tenant isolation failed")
	}
	if rows[0].CurrentStatus != domain.ShipmentStatusCarrierAssigned {
		t.Fatalf("status=%s", rows[0].CurrentStatus)
	}
	if rows[0].LastSourceEventID == nil {
		t.Fatal("expected source event id")
	}
}

func TestPostgresRepositoryEmptyTenant(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	stats, err := repo.StreamShipmentStatusSnapshot(context.Background(), snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowCount != 0 || stats.TenantCount != 0 {
		t.Fatalf("expected empty tenant snapshot, got %+v", stats)
	}
}

func TestPostgresRepositoryCreatedStatusRejected(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	shipmentID := uuid.New()
	_, err := env.pool.Exec(ctx, `
INSERT INTO transport.shipments (
    id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
    origin_location_id, destination_location_id, transport_mode, status, version
) VALUES ($1,$2,'SHP-CREATED',$3,$4,$5,$6,$7,'ROAD','CREATED',1)`,
		shipmentID, f.TenantA, f.TransportOrderA, f.ShipperA, f.ConsigneeA, f.OriginA, f.DestA)
	if err != nil {
		t.Fatal(err)
	}
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	if err == nil || snap.ExportErrorCode(err) != snap.CodeUnsupportedShipmentStatus {
		t.Fatalf("expected UNSUPPORTED_SHIPMENT_STATUS, got %v", err)
	}
}

func TestPostgresRepositoryMissingHistoryRejected(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	shipmentID := uuid.New()
	_, err := env.pool.Exec(ctx, `
INSERT INTO transport.shipments (
    id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
    origin_location_id, destination_location_id, transport_mode, status, version
) VALUES ($1,$2,'SHP-NOHIST',$3,$4,$5,$6,$7,'ROAD','CARRIER_ASSIGNED',1)`,
		shipmentID, f.TenantA, f.TransportOrderA, f.ShipperA, f.ConsigneeA, f.OriginA, f.DestA)
	if err != nil {
		t.Fatal(err)
	}
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	if err == nil || snap.ExportErrorCode(err) != snap.CodeMissingCanonicalStatusHistory {
		t.Fatalf("expected MISSING_CANONICAL_STATUS_HISTORY, got %v", err)
	}
}

func TestPostgresRepositorySoftDeleteExcluded(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-DEL"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.pool.Exec(ctx, `UPDATE transport.shipments SET deleted_at=NOW() WHERE id=$1`, shipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	stats, err := repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowCount != 0 {
		t.Fatalf("soft-deleted row exported: %+v", stats)
	}
}

func TestPostgresRepositoryRepeatableReadConsistency(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-RR"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}

	tx, err := env.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var versionBefore int
	if err := tx.QueryRow(ctx, `SELECT version FROM transport.shipments WHERE id=$1`, shipment.ID).Scan(&versionBefore); err != nil {
		t.Fatal(err)
	}

	_, err = env.repo.UpdateStatus(ctx, shipment.ID, f.TenantA, domain.ShipmentStatusCarrierAssigned, domain.ShipmentStatusInTransit, nil, nil, versionBefore, userTransition(user))
	if err != nil {
		t.Fatal(err)
	}

	var versionInTx int
	if err := tx.QueryRow(ctx, `SELECT version FROM transport.shipments WHERE id=$1`, shipment.ID).Scan(&versionInTx); err != nil {
		t.Fatal(err)
	}
	if versionInTx != versionBefore {
		t.Fatalf("RR snapshot saw mutated version: before=%d inTx=%d", versionBefore, versionInTx)
	}
}

func TestPostgresRepositoryAllScopeMultiTenant(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	_, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-ALL-A"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.repo.CreateShipment(ctx, createParams(f, f.TenantB, f.ShipperB, f.ConsigneeB, f.CarrierB, f.OriginB, f.DestB, f.TransportOrderB, "SHP-ALL-B"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	stats, err := repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "ALL"}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowCount != 2 || stats.TenantCount != 2 {
		t.Fatalf("expected 2 rows/tenants, got %+v", stats)
	}
}

func TestPostgresRepositoryEventIDAsymmetryAllowed(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	shipmentID := uuid.New()
	historyID := uuid.New()
	now := time.Now().UTC()
	_, err := env.pool.Exec(ctx, `
INSERT INTO transport.shipments (
    id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
    origin_location_id, destination_location_id, transport_mode, status, version
) VALUES ($1,$2,'SHP-NOOUTBOX',$3,$4,$5,$6,$7,'ROAD','CARRIER_ASSIGNED',1)`,
		shipmentID, f.TenantA, f.TransportOrderA, f.ShipperA, f.ConsigneeA, f.OriginA, f.DestA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.pool.Exec(ctx, `
INSERT INTO transport.shipment_status_history (id, tenant_id, shipment_id, shipment_version, from_status, to_status, source, actor_type, occurred_at)
VALUES ($1,$2,$3,1,NULL,'CARRIER_ASSIGNED','SHIPMENT_SERVICE','SYSTEM',$4)`,
		historyID, f.TenantA, shipmentID, now)
	if err != nil {
		t.Fatal(err)
	}
	var row snap.ShipmentSnapshotRow
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error {
		row = r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.LastSourceEventID == nil || *row.LastSourceEventID != historyID {
		t.Fatal("expected source event id from history")
	}
	if row.LastEventID != nil {
		t.Fatal("expected absent outbox event id")
	}
}

func TestPostgresRepositoryStatusHistoryMismatch(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-MIS"), userTransition(user))
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.pool.Exec(ctx, `UPDATE transport.shipments SET status='IN_TRANSIT' WHERE id=$1`, shipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	if err == nil || snap.ExportErrorCode(err) != snap.CodeAuthoritativeStatusMismatch {
		t.Fatalf("expected AUTHORITATIVE_STATUS_MISMATCH, got %v", err)
	}
}

func TestPostgresRepositoryLargeStreamingExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large streaming test in short mode")
	}
	rowTarget := 20000
	if v := strings.TrimSpace(os.Getenv("SNAPSHOT_LARGE_TEST_ROWS")); v != "" {
		fmt.Sscanf(v, "%d", &rowTarget)
	}
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	start := time.Now()
	_, err := env.pool.Exec(ctx, `
WITH nums AS (SELECT generate_series(1, $1) AS n)
INSERT INTO transport.shipments (id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, origin_location_id, destination_location_id, transport_mode, status, version)
SELECT gen_random_uuid(), $2, 'BULK-' || n::text, $3, $4, $5, $6, $7, $8, 'ROAD', 'CARRIER_ASSIGNED', 1 FROM nums`,
		rowTarget, f.TenantA, f.TransportOrderA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.pool.Exec(ctx, `
INSERT INTO transport.shipment_status_history (id, tenant_id, shipment_id, shipment_version, from_status, to_status, source, actor_type, occurred_at)
SELECT gen_random_uuid(), s.tenant_id, s.id, 1, NULL, 'CARRIER_ASSIGNED', 'SHIPMENT_SERVICE', 'SYSTEM', NOW()
FROM transport.shipments s WHERE s.tenant_id=$1 AND s.shipment_number LIKE 'BULK-%'`, f.TenantA)
	if err != nil {
		t.Fatal(err)
	}
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	stats, err := repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(row snap.ShipmentSnapshotRow) error {
		return nil
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowCount != int64(rowTarget) {
		t.Fatalf("expected %d rows, got %d", rowTarget, stats.RowCount)
	}
	t.Logf("large streaming export: rows=%d elapsed=%s", stats.RowCount, elapsed)
}
