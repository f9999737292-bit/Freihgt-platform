//go:build integration

package trackingloss

import (
	"context"
	"fmt"
	"log/slog"
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

	"github.com/freight-platform/tracking-service/internal/outbox"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

type testEnv struct {
	pool     *pgxpool.Pool
	repo     *repository.TrackingRepository
	outbox   *outbox.Publisher
	detector *service.TrackingLossDetector
}

type fixture struct {
	TenantID, DriverID, CarrierID, ShipmentID uuid.UUID
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	var stopEmbedded func() error
	if adminURL == "" {
		port := pickPort(t)
		pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
			Port(uint32(port)).
			RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
			DataPath(filepath.Join(t.TempDir(), "pg-data")).
			Database("freight_track_loss").
			Username("freight").Password("freight").
			Version(embeddedpostgres.V16))
		if err := pg.Start(); err != nil {
			t.Fatalf("embedded postgres: %v", err)
		}
		stopEmbedded = pg.Stop
		adminURL = fmt.Sprintf("postgres://freight:freight@localhost:%d/postgres?sslmode=disable", port)
	}
	t.Cleanup(func() {
		if stopEmbedded != nil {
			_ = stopEmbedded()
		}
	})

	dbName := "freight_track_loss_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	testURL, cleanup, err := createDB(ctx, adminURL, dbName)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { cleanup(context.Background()) })

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := applyTrackingAutomationStateTable(ctx, pool); err != nil {
		t.Fatalf("tracking automation schema: %v", err)
	}

	repo := repository.NewTrackingRepository(pool)
	outboxPub := outbox.NewPublisher(pool)
	detector := service.NewTrackingLossDetector(repo, outboxPub, 5*time.Minute, time.Minute, 100, slog.Default())

	return &testEnv{pool: pool, repo: repo, outbox: outboxPub, detector: detector}
}

func pickPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func createDB(ctx context.Context, adminURL, dbName string) (string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", nil, err
	}
	admin := cfg.Copy()
	admin.ConnConfig.Database = "postgres"
	p, err := pgxpool.NewWithConfig(ctx, admin)
	if err != nil {
		return "", nil, err
	}
	defer p.Close()
	if _, err := p.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", nil, err
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testCfg.ConnConfig.User, testCfg.ConnConfig.Password,
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, dbName)
	cleanup := func(cctx context.Context) {
		cp, _ := pgxpool.NewWithConfig(cctx, admin)
		if cp != nil {
			defer cp.Close()
			_, _ = cp.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		}
	}
	return url, cleanup, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		num := migrationNumber(base)
		if num > 14 && num != 26 && num != 31 {
			continue
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", base, execErr)
		}
	}
	return nil
}

func applyTrackingAutomationStateTable(ctx context.Context, pool *pgxpool.Pool) error {
	const q = `
CREATE TABLE IF NOT EXISTS tracking.driver_tracking_automation_state (
	tenant_id                   UUID NOT NULL REFERENCES core.tenants(id),
	shipment_id                 UUID NOT NULL,
	automation_state            VARCHAR(32) NOT NULL DEFAULT 'TRACKING_OK',
	last_location_recorded_at   TIMESTAMPTZ,
	last_transition_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (tenant_id, shipment_id),
	CONSTRAINT chk_driver_tracking_automation_state
		CHECK (automation_state IN ('TRACKING_OK', 'TRACKING_LOST'))
);
CREATE INDEX IF NOT EXISTS idx_driver_tracking_automation_state_lost
	ON tracking.driver_tracking_automation_state (tenant_id, automation_state, last_location_recorded_at);`
	_, err := pool.Exec(ctx, q)
	return err
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
	for _, c := range []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("migrations not found")
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantID: uuid.New(), DriverID: uuid.New(), CarrierID: uuid.New(), ShipmentID: uuid.New(),
	}
	userID := uuid.New()
	shipper, consignee, origin, dest, orderID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,'t1','Tenant')`, fix.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id uuid.UUID
		typ, name string
	}{
		{fix.CarrierID, "CARRIER", "Carrier"}, {shipper, "SHIPPER", "Shipper"}, {consignee, "CONSIGNEE", "Consignee"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,$3,$4)`,
			row.id, fix.TenantID, row.name, row.typ)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, loc := range []struct {
		id uuid.UUID
		name string
	}{
		{origin, "O"}, {dest, "D"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id,tenant_id,location_type,name,country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.transport_orders (id,tenant_id,order_number,status,shipper_company_id,consignee_company_id,origin_location_id,destination_location_id,transport_mode) VALUES ($1,$2,'TO','ASSIGNED',$3,$4,$5,$6,'ROAD')`,
		orderID, fix.TenantID, shipper, consignee, origin, dest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status) VALUES ($1,$2,$3,$4,'Driver','ACTIVE')`,
		fix.DriverID, fix.TenantID, fix.CarrierID, userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.shipments (id,tenant_id,shipment_number,transport_order_id,shipper_company_id,consignee_company_id,carrier_company_id,driver_id,origin_location_id,destination_location_id,transport_mode,status,version) VALUES ($1,$2,'SHP',$3,$4,$5,$6,$7,$8,$9,'ROAD','PICKUP_SLOT_BOOKED',1)`,
		fix.ShipmentID, fix.TenantID, orderID, shipper, consignee, fix.CarrierID, fix.DriverID, origin, dest)
	if err != nil {
		t.Fatal(err)
	}
	return fix
}

func seedTrackingBindingAndState(t *testing.T, pool *pgxpool.Pool, fix fixture, recordedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	deviceID := fix.DriverID.String()
	_, err := pool.Exec(ctx, `
		INSERT INTO tracking.shipment_tracking_binding
		(tenant_id, shipment_id, driver_id, provider_code, provider_device_id, status, active_from)
		VALUES ($1,$2,$3,'driver_mobile',$4,'active',NOW())`,
		fix.TenantID, fix.ShipmentID, fix.DriverID, deviceID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO tracking.shipment_tracking_state
		(tenant_id, shipment_id, tracking_status, provider_code, last_latitude, last_longitude,
		 last_recorded_at, last_received_at, freshness_status, quality_status, updated_at)
		VALUES ($1,$2,'active','driver_mobile',55.75,37.62,$3,$3,'fresh','good',NOW())`,
		fix.TenantID, fix.ShipmentID, recordedAt.UTC())
	if err != nil {
		t.Fatal(err)
	}
}

func updateTrackingRecordedAt(t *testing.T, pool *pgxpool.Pool, fix fixture, recordedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		UPDATE tracking.shipment_tracking_state
		SET last_recorded_at=$3, last_received_at=$3, updated_at=NOW()
		WHERE tenant_id=$1 AND shipment_id=$2`,
		fix.TenantID, fix.ShipmentID, recordedAt.UTC())
	if err != nil {
		t.Fatal(err)
	}
}

func countOutboxEvents(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, shipmentID uuid.UUID, eventType string) int64 {
	var n int64
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND aggregate_id=$2 AND event_type=$3`,
		tenantID, shipmentID, eventType).Scan(&n)
	return n
}

func automationState(ctx context.Context, pool *pgxpool.Pool, fix fixture) string {
	var state string
	_ = pool.QueryRow(ctx, `
		SELECT COALESCE(automation_state,'TRACKING_OK') FROM tracking.driver_tracking_automation_state
		WHERE tenant_id=$1 AND shipment_id=$2`, fix.TenantID, fix.ShipmentID).Scan(&state)
	if state == "" {
		return repository.TrackingAutomationOK
	}
	return state
}
