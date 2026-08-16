//go:build integration

package deadlineworker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/worker"
)

type seededEvents struct {
	TenantID uuid.UUID
	OwnerID  uuid.UUID
	EventA   uuid.UUID
	EventB   uuid.UUID
	EventC   uuid.UUID
	EventD   uuid.UUID
	Now      time.Time
	Past     time.Time
	Future   time.Time
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

func assertAutoCloseAudit(t *testing.T, env *testEnv, eventID, tenantID uuid.UUID) {
	t.Helper()
	var action, metadataRaw string
	var auditTenant uuid.UUID
	err := env.pool.QueryRow(context.Background(), `
		SELECT action, tenant_id, metadata::text
		FROM rfx.audit_events
		WHERE entity_type = 'rfx_event' AND entity_id = $1 AND action = 'auto_close_responses'
	`, eventID).Scan(&action, &auditTenant, &metadataRaw)
	if err != nil {
		t.Fatalf("load audit: %v", err)
	}
	if action != "auto_close_responses" {
		t.Fatalf("action=%s", action)
	}
	if auditTenant != tenantID {
		t.Fatalf("audit tenant mismatch")
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
		t.Fatalf("metadata json: %v", err)
	}
	if metadata["actor_type"] != domain.AuditActorTypeSystem {
		t.Fatalf("actor_type=%v", metadata["actor_type"])
	}
}

func newTestWorker(t *testing.T, env *testEnv, fix seededEvents, batchSize int) *worker.DeadlineWorker {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DeadlineWorkerConfig{Enabled: true, Interval: time.Second, BatchSize: batchSize}
	return worker.NewDeadlineWorker(cfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration"))
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
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("locate migrations: %v", err)
	}
	migrationPath := filepath.Join(migrationsDir, "000037_add_rfx_audit_events_v1.0.up.sql")
	if _, err := os.Stat(migrationPath); err != nil {
		t.Fatalf("migration 000037 file missing: %v", err)
	}
}

func TestIsolatedWorkerRuntime(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	newTestWorker(t, env, fix, 10).RunOnce(context.Background())

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
	assertAutoCloseAudit(t, env, fix.EventA, fix.TenantID)
}

func TestRestartIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	w := newTestWorker(t, env, fix, 10)
	w.RunOnce(context.Background())
	w.RunOnce(context.Background())
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
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DeadlineWorkerConfig{Enabled: true, Interval: time.Second, BatchSize: 10}
	w1 := worker.NewDeadlineWorker(cfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration-a"))
	w2 := worker.NewDeadlineWorker(cfg, env.rfxSvc, worker.NewFixedClock(fix.Now), log, worker.NewMetrics("rfx-service-integration-b"))
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

func TestBatchProcessing(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	ctx := context.Background()
	ids := make([]uuid.UUID, 0, 5)
	for i := 0; i < 5; i++ {
		id := uuid.New()
		ids = append(ids, id)
		_, err := env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_events (
				id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline
			) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', $4, $5, $6, $7)
		`, id, fix.TenantID, "RFX-B"+id.String()[:6], "Batch", fix.OwnerID, domain.RfxStatusResponsesOpen, fix.Past)
		if err != nil {
			t.Fatalf("seed batch event: %v", err)
		}
	}
	newTestWorker(t, env, fix, 2).RunOnce(context.Background())
	for _, id := range ids {
		if got := eventStatus(t, env, id); got != domain.RfxStatusResponsesClosed {
			t.Fatalf("batch event %s status=%s", id, got)
		}
	}
}

func TestDeadlineBoundary(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 15, 0, 0, 0, time.UTC)
	tenantID := uuid.New()
	ownerID := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`, tenantID, "tb", "Boundary Tenant")
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`, ownerID, tenantID, "Owner", "SHIPPER")
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
	pastID := uuid.New()
	equalID := uuid.New()
	futureID := uuid.New()
	insert := func(id uuid.UUID, deadline time.Time) {
		_, err := env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_events (
				id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline
			) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', $4, $5, $6, $7)
		`, id, tenantID, "RFX-"+id.String()[:8], "Boundary", ownerID, domain.RfxStatusResponsesOpen, deadline)
		if err != nil {
			t.Fatalf("seed boundary event: %v", err)
		}
	}
	insert(pastID, now.Add(-time.Minute))
	insert(equalID, now)
	insert(futureID, now.Add(time.Minute))

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DeadlineWorkerConfig{Enabled: true, Interval: time.Second, BatchSize: 10}
	worker.NewDeadlineWorker(cfg, env.rfxSvc, worker.NewFixedClock(now), log, worker.NewMetrics("rfx-service-boundary")).RunOnce(context.Background())

	if got := eventStatus(t, env, pastID); got != domain.RfxStatusResponsesClosed {
		t.Fatalf("past=%s", got)
	}
	if got := eventStatus(t, env, equalID); got != domain.RfxStatusResponsesClosed {
		t.Fatalf("equal=%s", got)
	}
	if got := eventStatus(t, env, futureID); got != domain.RfxStatusResponsesOpen {
		t.Fatalf("future=%s", got)
	}
}

func TestFailureSafetyAlreadyClosedRace(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)
	ctx := context.Background()
	_, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_events SET status = $1 WHERE id = $2`,
		domain.RfxStatusResponsesClosed, fix.EventA)
	if err != nil {
		t.Fatalf("pre-close event: %v", err)
	}
	newTestWorker(t, env, fix, 10).RunOnce(ctx)
	if autoCloseAuditCount(t, env, fix.EventA) != 0 {
		t.Fatal("expected no audit when event already closed before worker")
	}
	if got := eventStatus(t, env, fix.EventA); got != domain.RfxStatusResponsesClosed {
		t.Fatalf("status=%s", got)
	}
}

func TestServiceProcessRuntime(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedEvents(t, env)

	serviceRoot, err := locateRfxServiceRoot()
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(t.TempDir(), "rfx-service")
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/server")
	build.Dir = serviceRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build rfx-service: %v\n%s", err, string(out))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+env.testDBURL,
		"RFX_SERVICE_PORT=18084",
		"RFX_DEADLINE_WORKER_ENABLED=true",
		"RFX_DEADLINE_WORKER_INTERVAL_SECONDS=1",
		"RFX_DEADLINE_WORKER_BATCH_SIZE=10",
		"LOG_LEVEL=info",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	})

	pollUntil(t, 10*time.Second, func() bool {
		return eventStatus(t, env, fix.EventA) == domain.RfxStatusResponsesClosed
	})
	if autoCloseAuditCount(t, env, fix.EventA) != 1 {
		t.Fatal("expected audit after service worker tick")
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("sigterm: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "signal") {
			t.Fatalf("service exit: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("service did not shut down gracefully")
	}

	newTestWorker(t, env, fix, 10).RunOnce(context.Background())
	if autoCloseAuditCount(t, env, fix.EventA) != 1 {
		t.Fatal("expected no duplicate audit after service restart tick")
	}
}
