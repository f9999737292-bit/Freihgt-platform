//go:build integration

package bintranse2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kfake"
)

const internalToken = "bintrans-e2e-transport-flow-v0.1b"

type serviceProc struct {
	base string
	log  *bytes.Buffer
	cmd  *exec.Cmd
}

type flowEnv struct {
	pool      *pgxpool.Pool
	brokers   string
	rfx       *serviceProc
	orders    *serviceProc
	shipment  *serviceProc
	rates     *serviceProc
	documents *serviceProc
	tower     *serviceProc
	billing   *serviceProc
}

type flowFixture struct {
	TenantID           uuid.UUID
	ShipperID          uuid.UUID
	CarrierID          uuid.UUID
	OtherCarrier       uuid.UUID
	OtherShipper       uuid.UUID
	BuyerUser          uuid.UUID
	CarrierUser        uuid.UUID
	OtherCarrierUser   uuid.UUID
	OtherBuyerUser     uuid.UUID
	ForeignCarrierUser uuid.UUID
	DriverUser         uuid.UUID
	OtherDriverUser    uuid.UUID
	ForeignDriverUser  uuid.UUID
	OriginID           uuid.UUID
	DestID             uuid.UUID
	DriverID           uuid.UUID
	VehicleID          uuid.UUID
	OtherDriver        uuid.UUID
	OtherVehicle       uuid.UUID
	ForeignTenant      uuid.UUID
	ForeignCarrier     uuid.UUID
	ForeignDriver      uuid.UUID
	ForeignVehicle     uuid.UUID
}

func newFlowEnv(t *testing.T) *flowEnv {
	t.Helper()
	brokers := startKafka(t)
	mode := strings.TrimSpace(os.Getenv("BINTRANS_E2E_STAND_MODE"))

	var pool *pgxpool.Pool
	var testURL string
	switch mode {
	case "stand":
		pool, testURL = openStandDatabase(t)
	case "ci":
		pool, testURL = openCIDatabase(t)
	default:
		t.Fatalf("BINTRANS_E2E_STAND_MODE %q is not stand or ci", mode)
	}

	root := repoRoot(t)
	binDir := t.TempDir()
	shipmentLog := "error"
	towerLog := "error"
	if mode == "stand" {
		shipmentLog = "info"
		towerLog = "info"
	}
	listenHost := standHTTPListenHost(mode)
	documentRoot := t.TempDir()
	if mode == "stand" {
		if configured := strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_ROOT")); configured != "" {
			documentRoot = configured
		}
	}
	rfxPort := freePort(t)
	orderPort := freePort(t)
	ratePort := freePort(t)
	shipmentPort := freePort(t)
	documentPort := freePort(t)
	towerPort := freePort(t)
	billingPort := freePort(t)
	rfxURL := fmt.Sprintf("http://127.0.0.1:%d", rfxPort)
	orderURL := fmt.Sprintf("http://127.0.0.1:%d", orderPort)
	rateURL := fmt.Sprintf("http://127.0.0.1:%d", ratePort)
	shipmentURL := fmt.Sprintf("http://127.0.0.1:%d", shipmentPort)
	documentURL := fmt.Sprintf("http://127.0.0.1:%d", documentPort)
	towerURL := fmt.Sprintf("http://127.0.0.1:%d", towerPort)
	billingURL := fmt.Sprintf("http://127.0.0.1:%d", billingPort)

	env := &flowEnv{pool: pool, brokers: brokers}
	env.rates = startService(t, filepath.Join(root, "services", "contract-rate-service"), filepath.Join(binDir, "contract-rate.exe"), map[string]string{
		"DATABASE_URL":               testURL,
		"CONTRACT_RATE_SERVICE_PORT": fmt.Sprintf("%d", ratePort),
		"HTTP_PORT":                  fmt.Sprintf("%d", ratePort),
		"INTERNAL_SERVICE_TOKEN":     internalToken,
		"RFX_SERVICE_URL":            rfxURL,
		"LOG_LEVEL":                  "error",
		"ENVIRONMENT":                "test",
	})
	env.orders = startService(t, filepath.Join(root, "services", "transport-order-service"), filepath.Join(binDir, "transport-order.exe"), map[string]string{
		"DATABASE_URL":                 testURL,
		"TRANSPORT_ORDER_SERVICE_PORT": fmt.Sprintf("%d", orderPort),
		"HTTP_PORT":                    fmt.Sprintf("%d", orderPort),
		"INTERNAL_SERVICE_TOKEN":       internalToken,
		"CONTRACT_RATE_SERVICE_URL":    rateURL,
		"LOG_LEVEL":                    "error",
		"ENVIRONMENT":                  "test",
	})
	env.rfx = startService(t, filepath.Join(root, "services", "rfx-service"), filepath.Join(binDir, "rfx.exe"), map[string]string{
		"DATABASE_URL":                testURL,
		"RFX_SERVICE_PORT":            fmt.Sprintf("%d", rfxPort),
		"HTTP_PORT":                   fmt.Sprintf("%d", rfxPort),
		"INTERNAL_SERVICE_TOKEN":      internalToken,
		"TRANSPORT_ORDER_SERVICE_URL": orderURL,
		"RFX_DEADLINE_WORKER_ENABLED": "false",
		"LOG_LEVEL":                   "error",
		"ENVIRONMENT":                 "test",
	})
	env.shipment = startService(t, filepath.Join(root, "services", "shipment-service"), filepath.Join(binDir, "shipment.exe"), map[string]string{
		"DATABASE_URL":                        testURL,
		"SHIPMENT_SERVICE_PORT":               fmt.Sprintf("%d", shipmentPort),
		"HTTP_PORT":                           fmt.Sprintf("%d", shipmentPort),
		"INTERNAL_SERVICE_TOKEN":              internalToken,
		"DRIVER_NOTIFICATION_ENABLED":         "false",
		"SHIPMENT_OUTBOX_ENABLED":             "true",
		"SHIPMENT_OUTBOX_TRANSPORT":           "kafka",
		"SHIPMENT_KAFKA_BROKERS":              brokers,
		"SHIPMENT_KAFKA_TOPIC":                "shipment.status.v1",
		"SHIPMENT_KAFKA_DRIVER_TOPIC":         "driver.events.v1",
		"SHIPMENT_KAFKA_CLIENT_ID":            "shipment-service",
		"SHIPMENT_OUTBOX_POLL_INTERVAL":       "500ms",
		"SHIPMENT_OUTBOX_PUBLISH_TIMEOUT":     "5s",
		"SHIPMENT_OUTBOX_LEASE_TIMEOUT":       "30s",
		"TMS_RELATIONSHIP_ACTIVATION_ENABLED": "false",
		"SHIPMENT_HTTP_LISTEN_HOST":           listenHost,
		"LOG_LEVEL":                           shipmentLog,
		"ENVIRONMENT":                         "test",
	})
	env.documents = startService(t, filepath.Join(root, "services", "document-service"), filepath.Join(binDir, "document.exe"), map[string]string{
		"DATABASE_URL":          testURL,
		"DOCUMENT_SERVICE_PORT": fmt.Sprintf("%d", documentPort),
		"HTTP_PORT":             fmt.Sprintf("%d", documentPort),
		"DOCUMENT_STORAGE_ROOT": documentRoot,
		"LOG_LEVEL":             "error",
		"ENVIRONMENT":           "test",
	})
	env.billing = startService(t, filepath.Join(root, "services", "billing-register-service"), filepath.Join(binDir, "billing-register.exe"), map[string]string{
		"DATABASE_URL":                        testURL,
		"BILLING_REGISTER_SERVICE_PORT":       fmt.Sprintf("%d", billingPort),
		"HTTP_PORT":                           fmt.Sprintf("%d", billingPort),
		"INTERNAL_SERVICE_TOKEN":              internalToken,
		"BILLING_FREIGHT_COST_OUTBOX_ENABLED": "false",
		"LOG_LEVEL":                           "error",
		"ENVIRONMENT":                         "test",
	})
	env.tower = startService(t, filepath.Join(root, "services", "control-tower-read-model-service"), filepath.Join(binDir, "control-tower.exe"), map[string]string{
		"DATABASE_URL":                          testURL,
		"CONTROL_TOWER_DATABASE_URL":            testURL,
		"CONTROL_TOWER_READ_MODEL_SERVICE_PORT": fmt.Sprintf("%d", towerPort),
		"CONTROL_TOWER_HTTP_PORT":               fmt.Sprintf("%d", towerPort),
		"CONTROL_TOWER_CONSUMER_ENABLED":        "true",
		"CONTROL_TOWER_KAFKA_BROKERS":           brokers,
		"CONTROL_TOWER_KAFKA_TOPIC":             "shipment.status.v1",
		"CONTROL_TOWER_KAFKA_GROUP_ID":          "control-tower-e2e-" + uuid.NewString(),
		"CONTROL_TOWER_KAFKA_CLIENT_ID":         "control-tower-e2e",
		"CONTROL_TOWER_DRIVER_EVENTS_ENABLED":   "false",
		"SHIPMENT_SERVICE_URL":                  shipmentURL,
		"INTERNAL_SERVICE_TOKEN":                internalToken,
		"CONTROL_TOWER_HTTP_LISTEN_HOST":        listenHost,
		"LOG_LEVEL":                             towerLog,
		"ENVIRONMENT":                           "test",
	})
	env.rates.base = rateURL
	env.orders.base = orderURL
	env.rfx.base = rfxURL
	env.shipment.base = shipmentURL
	env.documents.base = documentURL
	env.tower.base = towerURL
	env.billing.base = billingURL
	waitReady(t, env.rates)
	waitReady(t, env.orders)
	waitReady(t, env.rfx)
	waitReady(t, env.shipment)
	waitReady(t, env.documents)
	waitReady(t, env.tower)
	waitReady(t, env.billing)
	return env
}

func startKafka(t *testing.T) string {
	t.Helper()
	mode := strings.TrimSpace(os.Getenv("BINTRANS_E2E_STAND_MODE"))
	broker := strings.TrimSpace(os.Getenv("BINTRANS_E2E_KAFKA_BROKERS"))
	allow := strings.TrimSpace(os.Getenv("BINTRANS_E2E_KFAKE_ALLOW"))
	modeLine, brokerLine := brokerLogLines(mode, broker)
	t.Log(modeLine)
	t.Log(brokerLine)
	plan, err := decideBroker(mode, broker, allow, dialStandBroker)
	if err != nil {
		t.Fatalf("broker: %v", err)
	}
	if plan.UseKfake {
		addr := startKfakeCluster(t)
		_, used := brokerLogLines(mode, addr)
		t.Log(used)
		return addr
	}
	return plan.Broker
}

func dialStandBroker(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}

func startKfakeCluster(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("BINTRANS_E2E_STAND_MODE")) == "stand" {
		t.Fatal("stand mode refuses kfake.NewCluster")
	}
	if strings.TrimSpace(os.Getenv("BINTRANS_E2E_KFAKE_ALLOW")) != "1" {
		t.Fatal("kfake.NewCluster requires BINTRANS_E2E_KFAKE_ALLOW=1")
	}
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, "shipment.status.v1", "driver.events.v1"),
	)
	if err != nil {
		t.Fatalf("kafka cluster: %v", err)
	}
	t.Cleanup(func() { cluster.Close() })
	addrs := cluster.ListenAddrs()
	if len(addrs) == 0 {
		t.Fatal("kafka cluster has no listen address")
	}
	return strings.Join(addrs, ",")
}

func openCIDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required")
		}
		adminURL = startEmbeddedPostgres(t)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres: %v", err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return pool, testURL
}

func openStandDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn, err := validateStandDatabaseURL(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("stand postgres connect failed")
	}
	t.Cleanup(pool.Close)
	if err := assertStandSchema(ctx, pool); err != nil {
		t.Fatalf("stand schema: %v", err)
	}
	return pool, dsn
}

func assertStandSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var state standSchemaState
	if err := pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&state.Version, &state.Dirty); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger WHERE tgname = 'trg_new_activation_authorization'`).Scan(&state.TriggerCount); err != nil {
		return err
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolcanlogin AND rolname <> current_user`).Scan(&state.ExtraLoginRoles); err != nil {
		return err
	}
	var nologin int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_roles
		WHERE rolname IN ('freight_migrations', 'company_runtime', 'shipment_runtime')
		  AND NOT rolcanlogin`).Scan(&nologin); err != nil {
		return err
	}
	state.RuntimeRolesNoLogin = nologin == 3
	return evaluateStandSchema(state)
}

func startService(t *testing.T, moduleDir, binPath string, environment map[string]string) *serviceProc {
	t.Helper()
	build := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	build.Dir = moduleDir
	buildOut, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", moduleDir, err, buildOut)
	}
	logBuf := &bytes.Buffer{}
	cmd := exec.Command(binPath)
	cmd.Env = mergedEnv(environment)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", binPath, err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	proc := &serviceProc{log: logBuf, cmd: cmd}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	return proc
}

func waitReady(t *testing.T, proc *serviceProc) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if proc.cmd.ProcessState != nil && proc.cmd.ProcessState.Exited() {
			t.Fatalf("service exited before ready: %s\n%s", proc.base, tail(proc.log.String()))
		}
		resp, err := http.Get(proc.base + "/ready")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("service not ready: %s\n%s", proc.base, tail(proc.log.String()))
}

func mergedEnv(overrides map[string]string) []string {
	skip := map[string]struct{}{}
	for key := range overrides {
		skip[strings.ToUpper(key)] = struct{}{}
	}
	out := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, found := skip[strings.ToUpper(key)]; found {
			continue
		}
		out = append(out, entry)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

func startEmbeddedPostgres(t *testing.T) string {
	t.Helper()
	port := freePort(t)
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
		DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("postgres").
		Username("freight").
		Password("freight").
		Logger(io.Discard).
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	return fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/postgres?sslmode=disable", port)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func seedFlowFixture(t *testing.T, pool *pgxpool.Pool) flowFixture {
	t.Helper()
	ctx := context.Background()
	fix := flowFixture{
		TenantID:           uuid.New(),
		ShipperID:          uuid.New(),
		CarrierID:          uuid.New(),
		OtherCarrier:       uuid.New(),
		OtherShipper:       uuid.New(),
		BuyerUser:          uuid.New(),
		CarrierUser:        uuid.New(),
		OtherCarrierUser:   uuid.New(),
		OtherBuyerUser:     uuid.New(),
		ForeignCarrierUser: uuid.New(),
		DriverUser:         uuid.New(),
		OtherDriverUser:    uuid.New(),
		ForeignDriverUser:  uuid.New(),
		OriginID:           uuid.New(),
		DestID:             uuid.New(),
		DriverID:           uuid.New(),
		VehicleID:          uuid.New(),
		OtherDriver:        uuid.New(),
		OtherVehicle:       uuid.New(),
		ForeignTenant:      uuid.New(),
		ForeignCarrier:     uuid.New(),
		ForeignDriver:      uuid.New(),
		ForeignVehicle:     uuid.New(),
	}
	mustExec(t, pool, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3), ($4,$5,$6)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "BINTRANS Shipper Tenant",
		fix.ForeignTenant, "t-"+fix.ForeignTenant.String()[:8], "Foreign Tenant")
	for _, company := range []struct {
		id, tenant uuid.UUID
		name, typ  string
	}{
		{fix.ShipperID, fix.TenantID, "Shipper Co", "SHIPPER"},
		{fix.OtherShipper, fix.TenantID, "Other Shipper", "SHIPPER"},
		{fix.CarrierID, fix.TenantID, "Awarded Carrier", "CARRIER"},
		{fix.OtherCarrier, fix.TenantID, "Other Carrier", "CARRIER"},
		{fix.ForeignCarrier, fix.ForeignTenant, "Foreign Carrier", "CARRIER"},
	} {
		mustExec(t, pool, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			company.id, company.tenant, company.name, company.typ)
	}
	mustExec(t, pool, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4), ($5,$2,$6,$7), ($8,$2,$9,$10), ($11,$2,$12,$13)`,
		fix.BuyerUser, fix.TenantID, "buyer-"+fix.BuyerUser.String()[:8]+"@bintrans.test", "Buyer",
		fix.CarrierUser, "carrier-"+fix.CarrierUser.String()[:8]+"@bintrans.test", "Carrier User",
		fix.DriverUser, "driver-"+fix.DriverUser.String()[:8]+"@bintrans.test", "Awarded Driver User",
		fix.OtherDriverUser, "other-driver-"+fix.OtherDriverUser.String()[:8]+"@bintrans.test", "Other Driver User")
	mustExec(t, pool, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4), ($5,$2,$6,$7)`,
		fix.OtherCarrierUser, fix.TenantID, "other-carrier-"+fix.OtherCarrierUser.String()[:8]+"@bintrans.test", "Other Carrier User",
		fix.OtherBuyerUser, "other-buyer-"+fix.OtherBuyerUser.String()[:8]+"@bintrans.test", "Other Buyer")
	mustExec(t, pool, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4), ($5,$2,$6,$7)`,
		fix.ForeignDriverUser, fix.ForeignTenant, "foreign-driver-"+fix.ForeignDriverUser.String()[:8]+"@bintrans.test", "Foreign Driver User",
		fix.ForeignCarrierUser, "foreign-carrier-"+fix.ForeignCarrierUser.String()[:8]+"@bintrans.test", "Foreign Carrier User")
	mustExec(t, pool, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3), ($1,$4,$5), ($1,$6,$7), ($1,$8,$9)`,
		fix.TenantID, fix.ShipperID, fix.BuyerUser, fix.CarrierID, fix.CarrierUser, fix.OtherCarrier, fix.OtherCarrierUser, fix.OtherShipper, fix.OtherBuyerUser)
	mustExec(t, pool, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`,
		fix.ForeignTenant, fix.ForeignCarrier, fix.ForeignCarrierUser)
	var buyerRole, carrierRole uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&buyerRole); err != nil {
		t.Fatalf("buyer role: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRole); err != nil {
		t.Fatalf("carrier role: %v", err)
	}
	mustExec(t, pool, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4), ($1,$5,$6,$7), ($1,$8,$9,$7), ($1,$10,$11,$4)`,
		fix.TenantID, fix.BuyerUser, fix.ShipperID, buyerRole, fix.CarrierUser, fix.CarrierID, carrierRole,
		fix.OtherCarrierUser, fix.OtherCarrier, fix.OtherBuyerUser, fix.OtherShipper)
	mustExec(t, pool, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.ForeignTenant, fix.ForeignCarrierUser, fix.ForeignCarrier, carrierRole)
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestID, "Destination"},
	} {
		mustExec(t, pool, `INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantID, fix.ShipperID, loc.name)
	}
	mustExec(t, pool, `INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,'Awarded Driver','ACTIVE'), ($5,$2,$6,$7,'Other Driver','ACTIVE'), ($8,$9,$10,$11,'Foreign Driver','ACTIVE')`,
		fix.DriverID, fix.TenantID, fix.CarrierID, fix.DriverUser,
		fix.OtherDriver, fix.OtherCarrier, fix.OtherDriverUser,
		fix.ForeignDriver, fix.ForeignTenant, fix.ForeignCarrier, fix.ForeignDriverUser)
	mustExec(t, pool, `INSERT INTO transport.vehicles (id, tenant_id, carrier_company_id, plate_number, vehicle_type, status)
		VALUES ($1,$2,$3,'A111AA77','TRUCK','ACTIVE'), ($4,$2,$5,'B222BB77','TRUCK','ACTIVE'), ($6,$7,$8,'C333CC77','TRUCK','ACTIVE')`,
		fix.VehicleID, fix.TenantID, fix.CarrierID, fix.OtherVehicle, fix.OtherCarrier, fix.ForeignVehicle, fix.ForeignTenant, fix.ForeignCarrier)
	return fix
}

func mustExec(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	if strings.TrimSpace(os.Getenv("BINTRANS_E2E_STAND_MODE")) == "stand" {
		return "", "", nil, fmt.Errorf("stand mode refuses CREATE DATABASE")
	}
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName = "bintrans_e2e_flow_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
	if strings.TrimSpace(os.Getenv("BINTRANS_E2E_STAND_MODE")) == "stand" {
		return fmt.Errorf("stand mode refuses applyMigrations")
	}
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no migrations in %s", dir)
	}
	last := filepath.Base(files[len(files)-1])
	if !strings.HasPrefix(last, "000105_") {
		return fmt.Errorf("migration head %s, want 000105", last)
	}
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
		filepath.Join("..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	return "", fmt.Errorf("migrations dir not found from %s", wd)
}

func (e *flowEnv) post(t *testing.T, proc *serviceProc, path string, body any, headers map[string]string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(http.MethodPost, proc.base+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v\n%s", path, err, e.logs())
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, payload
}

func (e *flowEnv) patch(t *testing.T, proc *serviceProc, path string, body any, headers map[string]string) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPatch, proc.base+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch %s: %v\n%s", path, err, e.logs())
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, payload
}

func (e *flowEnv) putRaw(t *testing.T, proc *serviceProc, path string, body []byte, headers map[string]string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, proc.base+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put %s: %v\n%s", path, err, e.logs())
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, payload
}

func (e *flowEnv) get(t *testing.T, proc *serviceProc, path string, headers map[string]string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, proc.base+path, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v\n%s", path, err, e.logs())
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, payload
}

func (e *flowEnv) logs() string {
	parts := []string{
		"rfx:\n" + tail(e.rfx.log.String()),
		"orders:\n" + tail(e.orders.log.String()),
		"rates:\n" + tail(e.rates.log.String()),
		"shipment:\n" + tail(e.shipment.log.String()),
	}
	if e.documents != nil {
		parts = append(parts, "documents:\n"+tail(e.documents.log.String()))
	}
	if e.tower != nil {
		parts = append(parts, "control-tower:\n"+tail(e.tower.log.String()))
	}
	if e.billing != nil {
		parts = append(parts, "billing:\n"+tail(e.billing.log.String()))
	}
	return strings.Join(parts, "\n")
}

func tail(text string) string {
	if len(text) > 4000 {
		return text[len(text)-4000:]
	}
	return text
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

func actorHeaders(tenantID, userID uuid.UUID) map[string]string {
	return map[string]string{
		"X-Tenant-ID": tenantID.String(),
		"X-User-ID":   userID.String(),
	}
}

func billingHeaders(tenantID, userID, companyID uuid.UUID, actorKind string) map[string]string {
	return map[string]string{
		"X-Tenant-ID":  tenantID.String(),
		"X-User-ID":    userID.String(),
		"X-Company-ID": companyID.String(),
		"X-Actor-Kind": actorKind,
	}
}

func jsonID(t *testing.T, payload []byte) string {
	t.Helper()
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("id json: %v body=%s", err, payload)
	}
	if body.ID == "" {
		t.Fatalf("missing id: %s", payload)
	}
	return body.ID
}

type shipmentView struct {
	ID               string  `json:"id"`
	TenantID         string  `json:"tenant_id"`
	TransportOrderID *string `json:"transport_order_id"`
	CarrierCompanyID *string `json:"carrier_company_id"`
	DriverID         *string `json:"driver_id"`
	VehicleID        *string `json:"vehicle_id"`
	Status           string  `json:"status"`
}

type executeView struct {
	Created          bool         `json:"created"`
	TransportOrderID string       `json:"transport_order_id"`
	RfxEventID       string       `json:"rfx_event_id"`
	RfxAwardID       string       `json:"rfx_award_id"`
	CarrierCompanyID string       `json:"carrier_company_id"`
	Shipment         shipmentView `json:"shipment"`
}

type convertView struct {
	Created bool `json:"created"`
	Items   []struct {
		RfxEventID       string `json:"rfx_event_id"`
		RfxAwardID       string `json:"rfx_award_id"`
		RfxLotID         string `json:"rfx_lot_id"`
		TransportOrderID string `json:"transport_order_id"`
		CarrierCompanyID string `json:"carrier_company_id"`
	} `json:"items"`
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
