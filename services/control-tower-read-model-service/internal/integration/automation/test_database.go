//go:build integration

package automation

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

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

type testEnv struct {
	pool *pgxpool.Pool
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	var stopEmbedded func() error
	if adminURL == "" {
		port := pickPort(t)
		pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
			Port(uint32(port)).RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
			DataPath(filepath.Join(t.TempDir(), "pg-data")).Database("freight_ct_auto").
			Username("freight").Password("freight").Version(embeddedpostgres.V16))
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

	dbName := "freight_ct_auto_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
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
	return &testEnv{pool: pool}
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
	return "", fmt.Errorf("migrations not found")
}

type fixture struct {
	TenantID, UserID, DriverID, CarrierID, ShipmentID, ExceptionID uuid.UUID
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(),
		CarrierID: uuid.New(), ShipmentID: uuid.New(), ExceptionID: uuid.New(),
	}
	shipper, consignee, origin, dest, orderID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,$2,$3)`, fix.TenantID, "t1", "Tenant A")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id uuid.UUID; typ, name string }{
		{fix.CarrierID, "CARRIER", "Carrier"}, {shipper, "SHIPPER", "Shipper"}, {consignee, "CONSIGNEE", "Consignee"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,$3,$4)`,
			row.id, fix.TenantID, row.name, row.typ)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, loc := range []struct{ id uuid.UUID; name string }{{origin, "O"}, {dest, "D"}} {
		_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id,tenant_id,location_type,name,country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.transport_orders (id,tenant_id,order_number,status,shipper_company_id,consignee_company_id,origin_location_id,destination_location_id,transport_mode) VALUES ($1,$2,'TO', 'ASSIGNED',$3,$4,$5,$6,'ROAD')`,
		orderID, fix.TenantID, shipper, consignee, origin, dest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status) VALUES ($1,$2,$3,$4,'Driver','ACTIVE')`,
		fix.DriverID, fix.TenantID, fix.CarrierID, fix.UserID)
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

func insertDriverException(t *testing.T, pool *pgxpool.Pool, fix fixture, idempotencyKey string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO transport.driver_reported_exception
		(id, tenant_id, shipment_id, driver_id, category, occurred_at, received_at, source, idempotency_key)
		VALUES ($1,$2,$3,$4,'VEHICLE_BREAKDOWN',NOW(),NOW(),'driver',$5)`,
		fix.ExceptionID, fix.TenantID, fix.ShipmentID, fix.DriverID, idempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf(`{"eventId":"%s","eventType":"driver.exception_reported","shipmentId":"%s"}`, fix.ExceptionID, fix.ShipmentID)
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.shipment_event_outbox
		(id, tenant_id, aggregate_type, aggregate_id, aggregate_version, event_type, schema_version, source_event_id, payload, status)
		VALUES ($1,$2,'shipment',$3,1,'driver.exception_reported',1,$4,$5::jsonb,'PENDING')`,
		uuid.New(), fix.TenantID, fix.ShipmentID, fix.ExceptionID, payload)
	if err != nil {
		t.Fatal(err)
	}
}

func seedMatchingRule(t *testing.T, ctx context.Context, repo *repository.AutomationRepository, pool *pgxpool.Pool, tenantID, userID uuid.UUID) (domain.AutomationRule, domain.OperationalPlaybook) {
	t.Helper()
	steps := []domain.PlaybookStepInput{{Sequence: 1, Title: "Contact driver", StepType: domain.StepTypeInstruction, ActionCode: "contact_driver", Required: true}}
	p, pv, err := repo.CreatePlaybook(ctx, tenantID, userID, domain.CreatePlaybookInput{Name: "Driver breakdown", Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1, updated_at=NOW() WHERE tenant_id=$1 AND id=$2`, tenantID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, tenantID, pv.ID)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := repo.GetActivePlaybookVersions(ctx, tenantID, []uuid.UUID{p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := versions[p.ID]; !ok {
		t.Fatalf("playbook %s has no active version after activation", p.ID)
	}
	rule, err := repo.CreateRule(ctx, tenantID, userID, domain.CreateRuleInput{
		Name: "Breakdown rule", TriggerType: "exception_created", ExecutionMode: domain.ExecutionModeRecommend, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "exceptionCategory", Operator: "eq", Value: []byte(`"vehicle_breakdown"`)},
			{Field: "priority", Operator: "eq", Value: []byte(`"high"`)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.SetRuleStatus(ctx, tenantID, userID, rule.ID, domain.RuleStatusActive)
	if err != nil {
		t.Fatal(err)
	}
	return rule, p
}

func countRows(ctx context.Context, pool *pgxpool.Pool, q string, args ...any) int64 {
	var n int64
	if err := pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		panic(err)
	}
	return n
}

func eventIDHex(id uuid.UUID) string {
	return strings.ReplaceAll(id.String(), "-", "")
}

func newAutomationStack(pool *pgxpool.Pool) (*repository.AutomationRepository, *repository.WorkflowRepository, *service.AutomationService) {
	autoRepo := repository.NewAutomationRepository(pool)
	workflowRepo := repository.NewWorkflowRepository(pool)
	return autoRepo, workflowRepo, service.NewAutomationService(autoRepo)
}
