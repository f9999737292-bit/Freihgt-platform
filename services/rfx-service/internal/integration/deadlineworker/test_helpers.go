//go:build integration

package deadlineworker

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

	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

type testEnv struct {
	pool      *pgxpool.Pool
	testDBURL string
	rfxRepo   *repository.RfxRepository
	auditRepo *repository.AuditRepository
	rfxSvc    *service.RfxService
}

func requireTestDatabase(t *testing.T) bool {
	t.Helper()
	return os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true")
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if requireTestDatabase(t) {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		if requireTestDatabase(t) {
			t.Fatalf("isolated postgres unavailable: %v", err)
		}
		t.Skipf("isolated postgres unavailable: %v", err)
	}

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		if requireTestDatabase(t) {
			t.Fatalf("connect test database: %v", err)
		}
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rfxRepo := repository.NewRfxRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	rfxSvc := service.NewRfxService(rfxRepo, auditRepo, nil)
	t.Logf("isolated database=%s", dbName)
	return &testEnv{
		pool:      pool,
		testDBURL: testURL,
		rfxRepo:   rfxRepo,
		auditRepo: auditRepo,
		rfxSvc:    rfxSvc,
	}
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName = "rfx_deadline_worker_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
		filepath.Join("..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	return "", fmt.Errorf("migrations dir not found from %s", wd)
}

func pollUntil(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("poll timeout")
}
