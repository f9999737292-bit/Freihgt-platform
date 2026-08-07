//go:build integration

package projectionrebuild

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type dualDBEnv struct {
	shipmentPool   *pgxpool.Pool
	readModelPool  *pgxpool.Pool
	shipmentDBURL  string
	readModelDBURL string
	repo           *repository.ShipmentRepository
}

type activeStateCounts struct {
	Projection int64
	Inbox      int64
	DeadLetter int64
	Jobs       int64
	Stage      int64
}

func setupDualDB(t *testing.T) *dualDBEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)

	shipmentURL, dropShipment := createTempDatabase(t, ctx, adminURL, "freight_cli_ship_")
	readModelURL, dropReadModel := createTempDatabase(t, ctx, adminURL, "freight_cli_rm_")
	t.Cleanup(func() {
		dropShipment(context.Background())
		dropReadModel(context.Background())
	})

	shipmentPool, err := pgxpool.New(ctx, shipmentURL)
	require.NoError(t, err)
	t.Cleanup(shipmentPool.Close)
	require.NoError(t, applyAllMigrations(ctx, shipmentPool))

	readModelPool, err := pgxpool.New(ctx, readModelURL)
	require.NoError(t, err)
	t.Cleanup(readModelPool.Close)
	require.NoError(t, applyReadModelMigrations(ctx, readModelPool))

	require.NoError(t, shipmentPool.Ping(ctx))
	require.NoError(t, readModelPool.Ping(ctx))

	var shipmentDBName, readModelDBName string
	require.NoError(t, shipmentPool.QueryRow(ctx, `SELECT current_database()`).Scan(&shipmentDBName))
	require.NoError(t, readModelPool.QueryRow(ctx, `SELECT current_database()`).Scan(&readModelDBName))
	require.NotEqual(t, shipmentDBName, readModelDBName)

	return &dualDBEnv{
		shipmentPool:   shipmentPool,
		readModelPool:  readModelPool,
		shipmentDBURL:  shipmentPool.Config().ConnString(),
		readModelDBURL: readModelPool.Config().ConnString(),
		repo:           repository.NewShipmentRepository(shipmentPool),
	}
}

func createTempDatabase(t *testing.T, ctx context.Context, adminURL, prefix string) (string, func(context.Context)) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(adminURL)
	require.NoError(t, err)
	dbName := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	require.NoError(t, err)
	t.Cleanup(adminPool.Close)
	_, err = adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
	require.NoError(t, err)
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(testCfg.ConnConfig.User), url.QueryEscape(string(testCfg.ConnConfig.Password)),
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, dbName)
	cleanup := func(cctx context.Context) {
		cadmin, _ := pgxpool.NewWithConfig(cctx, adminCfg)
		if cadmin != nil {
			defer cadmin.Close()
			_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		}
	}
	return testURL, cleanup
}

func applyAllMigrations(ctx context.Context, pool *pgxpool.Pool) error {
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

func applyReadModelMigrations(ctx context.Context, pool *pgxpool.Pool) error {
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
		if strings.HasPrefix(base, "000017") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("%s: %w", base, err)
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

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "services", "shipment-service", "go.mod")); err == nil {
			if _, err2 := os.Stat(filepath.Join(dir, "services", "control-tower-read-model-service", "go.mod")); err2 == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}

func buildCLIBinaries(t *testing.T) (exporterPath, importerPath string) {
	t.Helper()
	binaryOnce.Do(func() {
		root := findRepoRoot(t)
		binDir := filepath.Join(os.TempDir(), "freight_projection_rebuild_cli_bins")
		_ = os.MkdirAll(binDir, 0o755)
		cachedExporter = filepath.Join(binDir, "shipment-status-snapshot-export.exe")
		cachedImporter = filepath.Join(binDir, "control-tower-status-snapshot-import.exe")
		if st, err := os.Stat(cachedExporter); err != nil || st.Size() == 0 {
			require.NoError(t, runGoBuild(t, root, filepath.Join("services", "shipment-service"), cachedExporter, "./cmd/shipment-status-snapshot-export"))
		}
		if st, err := os.Stat(cachedImporter); err != nil || st.Size() == 0 {
			require.NoError(t, runGoBuild(t, root, filepath.Join("services", "control-tower-read-model-service"), cachedImporter, "./cmd/control-tower-status-snapshot-import"))
		}
	})
	return cachedExporter, cachedImporter
}

var (
	binaryOnce     sync.Once
	cachedExporter string
	cachedImporter string
)

func runGoBuild(t *testing.T, root, moduleDir, output, pkg string) error {
	t.Helper()
	cmd := execCommand(t, "go", "build", "-o", output, pkg)
	cmd.Dir = filepath.Join(root, moduleDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build %s: %w\n%s", pkg, err, out)
	}
	return nil
}

func seedTenant(ctx context.Context, pool *pgxpool.Pool, code string) (tenantID, shipper, consignee, carrier, origin, dest, transportOrder uuid.UUID) {
	tenantID = uuid.New()
	shipper, consignee, carrier = uuid.New(), uuid.New(), uuid.New()
	origin, dest = uuid.New(), uuid.New()
	transportOrder = uuid.New()
	_, _ = pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, tenantID, code, code)
	for _, row := range []struct {
		id   uuid.UUID
		typ  string
		name string
	}{
		{shipper, "SHIPPER", "Shipper"}, {consignee, "CONSIGNEE", "Consignee"}, {carrier, "CARRIER", "Carrier"},
	} {
		_, _ = pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, tenantID, row.name, row.typ)
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{{origin, "Origin"}, {dest, "Destination"}} {
		_, _ = pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, tenantID, loc.name)
	}
	_, _ = pool.Exec(ctx, `
INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')`, transportOrder, tenantID, "TO-"+code, shipper, consignee, origin, dest)
	return
}

func createShipment(t *testing.T, env *dualDBEnv, tenantID, shipper, consignee, carrier, origin, dest, transportOrder uuid.UUID, number string) *domain.Shipment {
	t.Helper()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(context.Background(), repository.CreateShipmentParams{
		TenantID: tenantID, ShipmentNumber: number, TransportOrderID: transportOrder,
		ShipperCompanyID: shipper, ConsigneeCompanyID: consignee, CarrierCompanyID: carrier,
		OriginLocationID: origin, DestinationLocationID: dest, TransportMode: "ROAD",
	}, domain.NewUserTransitionContext(user, nil, time.Now().UTC()))
	require.NoError(t, err)
	return shipment
}

func snapshotActiveState(t *testing.T, pool *pgxpool.Pool) activeStateCounts {
	t.Helper()
	ctx := context.Background()
	var counts activeStateCounts
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection`).Scan(&counts.Projection))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox`).Scan(&counts.Inbox))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter`).Scan(&counts.DeadLetter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job`).Scan(&counts.Jobs))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_stage`).Scan(&counts.Stage))
	return counts
}

func seedReadModelActiveRow(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tenantID, shipmentID := uuid.New(), uuid.New()
	eventID, sourceID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,'shipment.created',NOW(),NOW(),TRUE,FALSE,NOW(),NOW())`,
		tenantID, shipmentID, eventID, sourceID)
	require.NoError(t, err)
}
