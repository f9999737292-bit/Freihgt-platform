//go:build integration

package controltowerreadmodelintegration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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

	testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })

	pool, err := pgxpool.New(ctx, testURL)
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

func createTempDatabase(ctx context.Context, adminURL string) (testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	dbName := "freight_platform_gateway_rm_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
		_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return testURL, cleanup, nil
}

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

func startReadModelProcess(t *testing.T, databaseURL string) (baseURL string, stop func(), err error) {
	t.Helper()
	port, err := freeTCPPort()
	if err != nil {
		return "", nil, err
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return "", nil, err
	}
	serviceDir := filepath.Join(repoRoot, "services", "control-tower-read-model-service")
	binName := "read-model-blackbox-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)

	build := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	build.Dir = serviceDir
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		return "", nil, fmt.Errorf("build read-model: %w: %s", buildErr, string(out))
	}

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"CONTROL_TOWER_DATABASE_URL="+databaseURL,
		fmt.Sprintf("CONTROL_TOWER_READ_MODEL_SERVICE_PORT=%d", port),
		"CONTROL_TOWER_CONSUMER_ENABLED=false",
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
	)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start read-model: %w", err)
	}

	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForHTTP200(baseURL+"/health", 30*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return "", nil, err
	}

	stop = func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return baseURL, stop, nil
}

func locateRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")), nil
}

func freeTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForHTTP200(endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(endpoint)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", endpoint)
}
