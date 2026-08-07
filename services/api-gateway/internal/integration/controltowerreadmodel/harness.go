//go:build integration

package controltowerreadmodelintegration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	gatewayrm "github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

type readModelHarness struct {
	t       *testing.T
	pool    *pgxpool.Pool
	baseURL string
	stop    func()
}

func newReadModelHarness(t *testing.T) *readModelHarness {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping black-box read-model integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	testURL, dropDB, err := createTempDatabase(ctx, adminURL, "freight_platform_gateway_rm_")
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })

	pool, err := connectIntegrationPool(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := applyControlTowerMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	baseURL, stop, err := startReadModelProcess(t, testURL)
	if err != nil {
		t.Fatalf("start read-model process: %v", err)
	}
	t.Cleanup(stop)

	return &readModelHarness{t: t, pool: pool, baseURL: baseURL}
}

func (h *readModelHarness) seedProjections(tenantID uuid.UUID, statuses []string) {
	h.t.Helper()
	ctx := context.Background()
	for _, status := range statuses {
		shipmentID := uuid.New()
		eventID := uuid.New()
		sourceEventID := uuid.New()
		now := time.Now().UTC()
		_, err := h.pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
	tenant_id, shipment_id, shipment_version, current_status,
	last_event_id, last_source_event_id, last_event_type,
	last_occurred_at, last_consumed_at, complete, gap_detected
) VALUES ($1,$2,1,$3,$4,$5,'shipment.status.changed',$6,$6,true,false)`,
			tenantID, shipmentID, status, eventID, sourceEventID, now,
		)
		if err != nil {
			h.t.Fatalf("seed projection: %v", err)
		}
	}
}

func (h *readModelHarness) client(timeout time.Duration) *gatewayrm.Client {
	return gatewayrm.NewClient(&http.Client{Timeout: timeout}, gatewayrm.Config{
		BaseURL:          h.baseURL,
		Timeout:          timeout,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())
}

func createTempDatabase(ctx context.Context, adminURL, namePrefix string) (testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	dbName := namePrefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
	testURL = buildPostgresDSN(testCfg)

	cleanup = func(cctx context.Context) {
		cadmin, cerr := pgxpool.NewWithConfig(cctx, adminCfg)
		if cerr != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(cctx, `
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = $1 AND pid <> pg_backend_pid()`, dbName)
		_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return testURL, cleanup, nil
}

func connectIntegrationPool(ctx context.Context, testURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(testURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 2
	cfg.MinConns = 0
	return pgxpool.NewWithConfig(ctx, cfg)
}

const integrationDBPoolEnv = "DB_MAX_OPEN_CONNS=2"
const integrationDBIdleEnv = "DB_MAX_IDLE_CONNS=1"

func buildPostgresDSN(cfg *pgxpool.Config) string {
	user := url.QueryEscape(cfg.ConnConfig.User)
	pass := url.QueryEscape(cfg.ConnConfig.Password)
	host := cfg.ConnConfig.Host
	port := cfg.ConnConfig.Port
	db := cfg.ConnConfig.Database
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, pass, host, port, db)
}

func applyControlTowerMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "000015*.up.sql"))
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
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime caller unavailable")
	}
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "infrastructure", "migrations"))
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	return "", fmt.Errorf("migrations dir not found from %s", dir)
}

const (
	shipmentServiceBinaryKey  = "shipment-service-integration"
	readModelServiceBinaryKey = "read-model-integration"
)

func startReadModelProcess(t *testing.T, databaseURL string) (baseURL string, stop func(), err error) {
	t.Helper()
	binPath := buildServiceBinaryOnce(t, "services/control-tower-read-model-service", readModelServiceBinaryKey)
	baseURL = startManagedHTTPProcess(t, binPath, []string{
		"CONTROL_TOWER_DATABASE_URL=" + databaseURL,
		integrationDBPoolEnv,
		integrationDBIdleEnv,
		"CONTROL_TOWER_CONSUMER_ENABLED=false",
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
	}, "CONTROL_TOWER_READ_MODEL_SERVICE_PORT", "/ready")
	return baseURL, func() {}, nil
}

func locateRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")), nil
}
