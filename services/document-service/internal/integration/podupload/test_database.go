//go:build integration

package podupload

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

	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

type testEnv struct {
	pool    *pgxpool.Pool
	pod     *service.PODUploadService
	storage storage.ObjectStore
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
			Database("freight_doc_pod").
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

	dbName := "freight_doc_pod_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
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

	store, err := storage.NewLocalObjectStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	docRepo := repository.NewDocumentRepository(pool)
	docSvc := service.NewDocumentService(docRepo)
	podSvc := service.NewPODUploadService(pool, docSvc, store, 10<<20)

	return &testEnv{pool: pool, pod: podSvc, storage: store}
}

func pickPort(t *testing.T) int {
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
		if num > 14 && num != 33 {
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

type fixture struct {
	TenantA, TenantB, CompanyA, CompanyB, DriverA, DriverB, ShipmentA, ShipmentB uuid.UUID
}

func seedFixtures(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantA: uuid.New(), TenantB: uuid.New(),
		CompanyA: uuid.New(), CompanyB: uuid.New(),
		DriverA: uuid.New(), DriverB: uuid.New(),
		ShipmentA: uuid.New(), ShipmentB: uuid.New(),
	}
	for _, row := range []struct {
		tenant, company, driver, shipment uuid.UUID
		code                              string
	}{
		{fix.TenantA, fix.CompanyA, fix.DriverA, fix.ShipmentA, "A"},
		{fix.TenantB, fix.CompanyB, fix.DriverB, fix.ShipmentB, "B"},
	} {
		shipper, consignee, origin, dest, orderID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
		userID := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,$2,$3)`,
			row.tenant, "t"+row.code, "Tenant "+row.code)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct{ id uuid.UUID; typ, name string }{
			{row.company, "CARRIER", "Carrier " + row.code},
			{shipper, "SHIPPER", "Shipper"},
			{consignee, "CONSIGNEE", "Consignee"},
		} {
			_, err = pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,$3,$4)`,
				c.id, row.tenant, c.name, c.typ)
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, loc := range []struct{ id uuid.UUID; name string }{{origin, "O"}, {dest, "D"}} {
			_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id,tenant_id,location_type,name,country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
				loc.id, row.tenant, loc.name)
			if err != nil {
				t.Fatal(err)
			}
		}
		_, err = pool.Exec(ctx, `INSERT INTO transport.transport_orders (id,tenant_id,order_number,status,shipper_company_id,consignee_company_id,origin_location_id,destination_location_id,transport_mode) VALUES ($1,$2,'TO','ASSIGNED',$3,$4,$5,$6,'ROAD')`,
			orderID, row.tenant, shipper, consignee, origin, dest)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status) VALUES ($1,$2,$3,$4,'Driver','ACTIVE')`,
			row.driver, row.tenant, row.company, userID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO transport.shipments (id,tenant_id,shipment_number,transport_order_id,shipper_company_id,consignee_company_id,carrier_company_id,driver_id,origin_location_id,destination_location_id,transport_mode,status,version) VALUES ($1,$2,'SHP',$3,$4,$5,$6,$7,$8,$9,'ROAD','PICKUP_SLOT_BOOKED',1)`,
			row.shipment, row.tenant, orderID, shipper, consignee, row.company, row.driver, origin, dest)
		if err != nil {
			t.Fatal(err)
		}
	}
	return fix
}
