//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestI2A_MIG_01_UpFrom096Head(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyOrderedMigrationsThrough(t, ctx, pool, 96)
	assertMigration096Artifacts(t, ctx, pool, true)
	if _, err := pool.Exec(ctx, mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql")); err != nil {
		t.Fatalf("I2A-MIG-01 up: %v", err)
	}
	assertChain097Present(t, ctx, pool, true)
	assertMigration096Artifacts(t, ctx, pool, true)
}

func TestI2A_MIG_02_DownSucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyOrderedMigrationsThrough(t, ctx, pool, 96)
	if _, err := pool.Exec(ctx, mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql")); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.down.sql")); err != nil {
		t.Fatalf("I2A-MIG-02 down: %v", err)
	}
	assertChain097Present(t, ctx, pool, false)
	assertMigration096Artifacts(t, ctx, pool, true)
}

func TestI2A_MIG_03_UpDownUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyOrderedMigrationsThrough(t, ctx, pool, 96)
	up := mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql")
	down := mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.down.sql")
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertMigration096Artifacts(t, ctx, pool, true)
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("I2A-MIG-03 up after down: %v", err)
	}
	assertChain097Present(t, ctx, pool, true)
	assertMigration096Artifacts(t, ctx, pool, true)
}

func TestI2A_MIG_04_ContiguousNoCollision(t *testing.T) {
	dir := migrationsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	var numbers []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		if len(name) < 6 {
			t.Fatalf("unexpected migration name %s", name)
		}
		num := name[:6]
		if prev, ok := seen[num]; ok {
			t.Fatalf("I2A-MIG-04 collision: %s and %s", prev, name)
		}
		seen[num] = name
		numbers = append(numbers, num)
	}
	sort.Strings(numbers)
	if len(numbers) == 0 {
		t.Fatal("I2A-MIG-04 found no migrations")
	}
	if _, ok := seen["000096"]; !ok {
		t.Fatal("I2A-MIG-04 missing 000096")
	}
	if _, ok := seen["000097"]; !ok {
		t.Fatal("I2A-MIG-04 missing 000097")
	}
}

func TestI2A_MIG_05_ExistingRoutePlansSurviveUnique(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyOrderedMigrationsThrough(t, ctx, pool, 96)
	tenant := uuid.New()
	planID := uuid.New()
	insertMinimalRoutePlan(t, ctx, pool, tenant, planID)
	if _, err := pool.Exec(ctx, mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql")); err != nil {
		t.Fatalf("I2A-MIG-05 up with existing plan: %v", err)
	}
	var still bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM network_optimizer.route_plans WHERE id=$1 AND tenant_id=$2)`, planID, tenant).Scan(&still); err != nil || !still {
		t.Fatalf("I2A-MIG-05 plan survived=%v err=%v", still, err)
	}
	var uniqueExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname='route_plans_tenant_id_uidx'
		)`).Scan(&uniqueExists); err != nil || !uniqueExists {
		t.Fatalf("I2A-MIG-05 unique missing err=%v exists=%v", err, uniqueExists)
	}
	assertMigration096Artifacts(t, ctx, pool, true)
}

func applyOrderedMigrationsThrough(t *testing.T, ctx context.Context, pool *pgxpool.Pool, through int) {
	t.Helper()
	dir := migrationsDir(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	applied := 0
	var last string
	for _, file := range files {
		base := filepath.Base(file)
		version, ok := migrationVersion(base)
		if !ok {
			t.Fatalf("unversioned migration file %s", base)
		}
		if version > through {
			continue
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			t.Fatalf("apply %s: %v", base, execErr)
		}
		applied++
		last = base
	}
	if applied == 0 || !strings.HasPrefix(last, fmt.Sprintf("%06d", through)) {
		t.Fatalf("expected last applied migration %06d*, got last=%s applied=%d", through, last, applied)
	}
}

func applyThrough000097(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applyOrderedMigrationsThrough(t, ctx, pool, 96)
	if _, err := pool.Exec(ctx, mustRead(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql")); err != nil {
		t.Fatalf("up 000097: %v", err)
	}
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "infrastructure", "migrations")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("migrations dir: %v", err)
	}
	return dir
}

func migrationVersion(name string) (int, bool) {
	if len(name) < 6 {
		return 0, false
	}
	n, err := strconv.Atoi(name[:6])
	if err != nil {
		return 0, false
	}
	return n, true
}

func assertChain097Present(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want bool) {
	t.Helper()
	for _, table := range []string{"chain_search_runs", "chain_candidates", "chain_search_idempotency"} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema='network_optimizer' AND table_name=$1
			)`, table).Scan(&exists); err != nil || exists != want {
			t.Fatalf("table %s exists=%v want %v err=%v", table, exists, want, err)
		}
	}
	var uniqueExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint WHERE conname='route_plans_tenant_id_uidx'
		)`).Scan(&uniqueExists); err != nil {
		t.Fatal(err)
	}
	if uniqueExists != want {
		t.Fatalf("route_plans_tenant_id_uidx exists=%v want %v", uniqueExists, want)
	}
}

func assertMigration096Artifacts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want bool) {
	t.Helper()
	for _, item := range []struct {
		schema string
		table  string
	}{
		{"documents", "document_attachments"},              // 000095
		{"documents", "attachment_signature_blobs"},        // 000096
		{"documents", "signature_verification_evidence"},   // 000096
		{"network_optimizer", "service_duration_policies"}, // 000094 (still present at 096 head)
		{"network_optimizer", "route_plans"},               // 000085
	} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema=$1 AND table_name=$2
			)`, item.schema, item.table).Scan(&exists); err != nil || exists != want {
			t.Fatalf("%s.%s exists=%v want %v err=%v", item.schema, item.table, exists, want, err)
		}
	}
}

func insertMinimalRoutePlan(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant, planID uuid.UUID) {
	t.Helper()
	capacity := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plans (
			id, tenant_id, version, status, planning_mode, result_status,
			capacity_id, capacity_version, context_fingerprint, evaluation_fingerprint,
			algorithm_policy_version, routing_policy_version, execution_supported, reason_codes, created_at
		) VALUES ($1,$2,1,'EVALUATED','DEPOT_START','INDETERMINATE_PLAN_FOUND',$3,1,'ctx','eval','alg','route',false,'{}',now())`,
		planID, tenant, capacity); err != nil {
		t.Fatalf("insert route plan: %v", err)
	}
}
