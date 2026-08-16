//go:build integration

package deadlineworker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/worker"
)

type testEnv struct {
	pool      *pgxpool.Pool
	rfxRepo   *repository.RfxRepository
	auditRepo *repository.AuditRepository
	rfxSvc    *service.RfxService
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		adminURL = "postgres://freight:freight_password@localhost:5433/rfx_deadline_worker_pg?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Skipf("isolated postgres unavailable: %v", err)
	}

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
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
	return &testEnv{pool: pool, rfxRepo: rfxRepo, auditRepo: auditRepo, rfxSvc: rfxSvc}
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
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	return "", fmt.Errorf("migrations dir not found from %s", wd)
}

type seededEvents struct {
	TenantID  uuid.UUID
	OwnerID   uuid.UUID
	EventA    uuid.UUID
	EventB    uuid.UUID
	EventC    uuid.UUID
	EventD    uuid.UUID
	Now       time.Time
	Past      time.Time
	Future    time.Time
}

func seedEvents(t *testing.T, env *testEnv) seededEvents {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)
	fix := seededEvents{
		TenantID: uuid.New(),
		OwnerID:  uuid.New(),
		EventA:   uuid.New(),
		EventB:   uuid.New(),
		EventC:   uuid.New(),
		EventD:   uuid.New(),
		Now:      now,
		Past:     past,
		Future:   future,
	}

	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Deadline Worker Tenant")
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		fix.OwnerID, fix.TenantID, "Owner", "SHIPPER")
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}

	insertEvent := func(id uuid.UUID, status string, deadline time.Time) {
		_, err := env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_events (
				id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline
			) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', $4, $5, $6, $7)
		`, id, fix.TenantID, "RFX-"+id.String()[:8], "Event "+id.String()[:8], fix.OwnerID, status, deadline)
		if err != nil {
			t.Fatalf("seed event %s: %v", id, err)
		}
	}
	insertEvent(fix.EventA, domain.RfxStatusResponsesOpen, past)
	insertEvent(fix.EventB, domain.RfxStatusResponsesOpen, future)
	insertEvent(fix.EventC, domain.RfxStatusResponsesClosed, past)
	insertEvent(fix.EventD, domain.RfxStatusCancelled, past)
	return fix
}

func eventStatus(t *testing.T, env *testEnv, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := env.pool.QueryRow(context.Background(), `SELECT status FROM rfx.rfx_events WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("load status: %v", err)
	}
	return status
}

func autoCloseAuditCount(t *testing.T, env *testEnv, eventID uuid.UUID) int {
	t.Helper()
	var count int
	err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE entity_type = 'rfx_event' AND entity_id = $1 AND action = 'auto_close_responses'
	`, eventID).Scan(&count)
	if err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return count
}

func TestMigration000037Applied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	var exists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'rfx' AND table_name = 'audit_events'
		)
	`).Scan(&exists); err != nil || !exists {
		t.Fatalf("audit table missing: exists=%v err=%v", exists, err)
	}
	var indexExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes WHERE schemaname = 'rfx' AND indexname = 'idx_rfx_audit_events_tenant_id'
		)
	`).Scan(&indexExists); err != nil || !indexExists {
		t.Fatalf("tenant index missing: exists=%v err=%v", indexExists, err)
	}
}

func TestIsolatedWorkerRuntime(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	workerCfg := config.DeadlineWorkerConfig{Enabled: true, Interval: time.Second, BatchSize: 10}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	deadlineWorker := worker.NewDeadlineWorker(workerCfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration"))
	deadlineWorker.RunOnce(context.Background())

	if got := eventStatus(t, env, fix.EventA); got != domain.RfxStatusResponsesClosed {
		t.Fatalf("EVENT_A=%s", got)
	}
	if got := eventStatus(t, env, fix.EventB); got != domain.RfxStatusResponsesOpen {
		t.Fatalf("EVENT_B=%s", got)
	}
	if got := eventStatus(t, env, fix.EventC); got != domain.RfxStatusResponsesClosed {
		t.Fatalf("EVENT_C=%s", got)
	}
	if got := eventStatus(t, env, fix.EventD); got != domain.RfxStatusCancelled {
		t.Fatalf("EVENT_D=%s", got)
	}
	if autoCloseAuditCount(t, env, fix.EventA) != 1 {
		t.Fatal("expected one auto-close audit for EVENT_A")
	}
}

func TestRestartIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	workerCfg := config.DeadlineWorkerConfig{Enabled: true, Interval: time.Second, BatchSize: 10}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	deadlineWorker := worker.NewDeadlineWorker(workerCfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration-restart"))
	deadlineWorker.RunOnce(context.Background())
	deadlineWorker.RunOnce(context.Background())
	if autoCloseAuditCount(t, env, fix.EventA) != 1 {
		t.Fatal("expected no duplicate audit after restart scan")
	}
}

func TestConcurrentWorkerRuntime(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	eventID := uuid.New()
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline
		) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', 'Concurrent', $4, $5, $6)
	`, eventID, fix.TenantID, "RFX-CONC", fix.OwnerID, domain.RfxStatusResponsesOpen, fix.Past)
	if err != nil {
		t.Fatalf("seed concurrent event: %v", err)
	}
	workerCfg := config.DeadlineWorkerConfig{Enabled: true, Interval: time.Second, BatchSize: 10}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w1 := worker.NewDeadlineWorker(workerCfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration-a"))
	w2 := worker.NewDeadlineWorker(workerCfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration-b"))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); w1.RunOnce(context.Background()) }()
	go func() { defer wg.Done(); w2.RunOnce(context.Background()) }()
	wg.Wait()
	if got := eventStatus(t, env, eventID); got != domain.RfxStatusResponsesClosed {
		t.Fatalf("concurrent event=%s", got)
	}
	if autoCloseAuditCount(t, env, eventID) != 1 {
		t.Fatal("expected single audit for concurrent close")
	}
}
