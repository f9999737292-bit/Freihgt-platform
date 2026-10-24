//go:build integration

package podaction

import (
	"bytes"
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

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

var (
	adminURL string
	stopPG   func() error
	env      *actionEnv
)

type actionEnv struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	svc    *service.PODActionEvidenceService
	legacy *service.PODUploadService
	migDir string
}

type actionFixture struct {
	operating, shipmentTenant, company, driver, authorCompany uuid.UUID
	execution, action, stop, shipment, cargo                  uuid.UUID
}

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	admin, stop, err := startPostgres()
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres: %v\n", err)
		os.Exit(1)
	}
	stopPG = stop
	adminURL = admin
	dbName := "pod_action_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	testURL, cleanup, err := createDatabase(ctx, adminURL, dbName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create db: %v\n", err)
		_ = stopPG()
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		cleanup(context.Background())
		_ = stopPG()
		os.Exit(1)
	}
	dir, err := locateMigrations()
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrations: %v\n", err)
		pool.Close()
		cleanup(context.Background())
		_ = stopPG()
		os.Exit(1)
	}
	if err := applyAllUp(ctx, pool, dir); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		pool.Close()
		cleanup(context.Background())
		_ = stopPG()
		os.Exit(1)
	}
	store, err := storage.NewLocalObjectStore(filepath.Join(os.TempDir(), "pod-action-objects-"+uuid.NewString()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage: %v\n", err)
		pool.Close()
		cleanup(context.Background())
		_ = stopPG()
		os.Exit(1)
	}
	docRepo := repository.NewDocumentRepository(pool)
	env = &actionEnv{
		ctx: ctx, pool: pool, migDir: dir,
		svc:    service.NewPODActionEvidenceService(repository.NewPODActionEvidenceRepository(pool), store, 1<<20, service.ActionObjectPlacement{StorageProvider: "local"}),
		legacy: service.NewPODUploadService(pool, service.NewDocumentService(docRepo), store, 1<<20),
	}
	code := m.Run()
	pool.Close()
	cleanup(context.Background())
	_ = stopPG()
	os.Exit(code)
}

func startPostgres() (string, func() error, error) {
	port := freePort()
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(os.TempDir(), "pg-pod-action-rt-"+uuid.NewString()[:8])).
		DataPath(filepath.Join(os.TempDir(), "pg-pod-action-data-"+uuid.NewString()[:8])).
		Database("postgres").
		Username("freight").Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/postgres?sslmode=disable", port), pg.Stop, nil
}

func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 55432
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func createDatabase(ctx context.Context, admin, dbName string) (string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(admin)
	if err != nil {
		return "", nil, err
	}
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	p, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", nil, err
	}
	defer p.Close()
	if _, err := p.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", nil, err
	}
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.ConnConfig.User, cfg.ConnConfig.Password, cfg.ConnConfig.Host, cfg.ConnConfig.Port, dbName)
	cleanup := func(cctx context.Context) {
		cp, err := pgxpool.NewWithConfig(cctx, adminCfg)
		if err != nil {
			return
		}
		defer cp.Close()
		_, _ = cp.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return url, cleanup, nil
}

func locateMigrations() (string, error) {
	for _, candidate := range []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("migrations not found")
}

func applyAllUp(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func seedAction(t *testing.T, crossShipper bool) actionFixture {
	t.Helper()
	fix := actionFixture{
		operating: uuid.New(), shipmentTenant: uuid.New(), company: uuid.New(),
		driver: uuid.New(), authorCompany: uuid.New(), execution: uuid.New(),
		action: uuid.New(), stop: uuid.New(), shipment: uuid.New(), cargo: uuid.New(),
	}
	if !crossShipper {
		fix.shipmentTenant = fix.operating
	}
	insertTenant(t, fix.operating, "op")
	if fix.shipmentTenant != fix.operating {
		insertTenant(t, fix.shipmentTenant, "sh")
	}
	insertCompany(t, fix.company, fix.operating, "CARRIER")
	insertCompany(t, fix.authorCompany, fix.operating, "CARRIER")
	shipper, consignee := uuid.New(), uuid.New()
	insertCompany(t, shipper, fix.shipmentTenant, "SHIPPER")
	insertCompany(t, consignee, fix.shipmentTenant, "CONSIGNEE")
	origin, dest := uuid.New(), uuid.New()
	insertLocation(t, origin, fix.shipmentTenant, "Origin")
	insertLocation(t, dest, fix.shipmentTenant, "Dest")
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type) VALUES ($1,$2,'GENERAL')`, fix.cargo, fix.shipmentTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, full_name, status) VALUES ($1,$2,$3,'Assigned Driver','ACTIVE')`,
		fix.driver, fix.shipmentTenant, shipper); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, driver_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'ROAD','CREATED',1)`,
		fix.shipment, fix.shipmentTenant, "SHP-"+fix.shipment.String()[:8], shipper, consignee, origin, dest, fix.driver); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_executions (
			id, operating_tenant_id, carrier_company_id, driver_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,now(),now())`,
		fix.execution, fix.operating, fix.company, fix.driver); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_stops (
			id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind,
			latitude, longitude, status, version, created_at, updated_at
		) VALUES ($1,$2,$3,1,'CARGO','POSITION_ANCHOR',55,37,'PLANNED',1,now(),now())`,
		fix.stop, fix.execution, fix.operating); err != nil {
		t.Fatal(err)
	}
	insertAction(t, fix, "DELIVERY", 0)
	return fix
}

func extraAction(t *testing.T, fix actionFixture, actionType string, ordinal int) actionFixture {
	t.Helper()
	fix.action = uuid.New()
	insertAction(t, fix, actionType, ordinal)
	return fix
}

func insertAction(t *testing.T, fix actionFixture, actionType string, ordinal int) {
	t.Helper()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_actions (
			id, execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, cargo_version,
			route_subject_type, route_subject_id, action_type, ordinal, status, version
		) VALUES ($1,$2,$3,$4,$5,1,'SHIPMENT_CARGO',$6,$7,$8,'PENDING',1)`,
		fix.action, fix.stop, fix.shipment, fix.shipmentTenant, fix.cargo, fix.cargo, actionType, ordinal); err != nil {
		t.Fatal(err)
	}
}

func insertTenant(t *testing.T, id uuid.UUID, prefix string) {
	t.Helper()
	code := prefix + strings.ReplaceAll(id.String(), "-", "")[:12]
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, id, code, code); err != nil {
		t.Fatal(err)
	}
}

func insertCompany(t *testing.T, id, tenant uuid.UUID, kind string) {
	t.Helper()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`, id, tenant, kind+" "+id.String()[:8], kind); err != nil {
		t.Fatal(err)
	}
}

func insertLocation(t *testing.T, id, tenant uuid.UUID, name string) {
	t.Helper()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, id, tenant, name); err != nil {
		t.Fatal(err)
	}
}

func (f actionFixture) input(key string) service.CreateActionPODIntentInput {
	return service.CreateActionPODIntentInput{
		ActionType: domain.PODActionTypeDelivery, DocumentTenantID: f.operating, OperatingTenantID: f.operating,
		ExecutionID: f.execution, ExecutionActionID: f.action, ExecutionStopID: f.stop,
		ShipmentID: f.shipment, ShipmentTenantID: f.shipmentTenant, CargoID: f.cargo,
		OwnerCompanyID: f.company, AuthorDriverID: f.driver, AuthorCompanyID: f.authorCompany,
		MimeType: "image/jpeg", FileName: "pod.jpg", IdempotencyKey: key,
	}
}

func finish(t *testing.T, in service.CreateActionPODIntentInput) *service.CompleteActionPODResult {
	t.Helper()
	intent, err := env.svc.CreateUploadIntent(env.ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := env.svc.UploadContent(env.ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("jpeg-bytes")), domain.SubjectConfirmation{}); err != nil {
		t.Fatalf("content: %v", err)
	}
	done, err := env.svc.CompleteUpload(env.ctx, intent.UploadID, "abc", domain.SubjectConfirmation{})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	return done
}

func countEvidence(t *testing.T, documentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM documents.pod_delivery_action_evidence WHERE document_id=$1`, documentID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func countDecisions(t *testing.T, actionID uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM transport.transport_execution_pod_requirement_decisions WHERE execution_action_id=$1`, actionID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
