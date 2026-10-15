//go:build integration

package chainorch_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func startPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	port := pickPort(t)
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
		DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("bno_test").
		Username("freight").Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/bno_test?sslmode=disable", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func pickPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func mustReadMigration(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
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

func applyThrough000097(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
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
		if !ok || version > 96 {
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
	if applied == 0 || !strings.HasPrefix(last, "000096") {
		t.Fatalf("expected last applied 000096*, got last=%s applied=%d", last, applied)
	}
	if _, err := pool.Exec(ctx, mustReadMigration(t, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql")); err != nil {
		t.Fatalf("up 000097: %v", err)
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

func insertTimedSearch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant, id, owner uuid.UUID, status string, createdOffset time.Duration) {
	t.Helper()
	one := 1
	capID := uuid.New()
	searchComplete := status == repository.ChainSearchCompleted
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.chain_search_runs (
			id, tenant_id, status, created_at, started_at,
			request_fingerprint, policy_version, algorithm_version, catalog_fingerprint,
			routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_snapshot_json, horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version,
			candidate_count, evaluated_pair_count, provider_call_count,
			search_complete, global_optimum, execution_supported, reason_counts_json
		) VALUES (
			$1,$2,$3,
			now() + $4::interval,
			CASE WHEN $3 = 'RUNNING' THEN now() + $4::interval ELSE NULL END,
			$5,'NLO-0.9C-I0/BOUNDED_TWO_LOAD','BOUNDED_TWO_ORDER_COMPARISON','catalog',
			'UNKNOWN',
			(now() + $4::interval) + interval '20 seconds',
			$6,
			(now() + $4::interval) + interval '25 seconds',
			'{"state":"UNKNOWN","pickup_known":false,"delivery_known":false}'::jsonb,
			now(), now() + interval '48 hours', 'MANUAL_AVAILABLE_CAPACITY',
			$7,$8,
			0,0,0,
			$9,false,false,'{}'::jsonb
		)`,
		id, tenant, status, createdOffset.String(), "fp-"+id.String(), owner, capID, one, searchComplete,
	); err != nil {
		t.Fatalf("insert timed search: %v", err)
	}
}

func expireClaimWindow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, searchID uuid.UUID) {
	t.Helper()
	tag, err := pool.Exec(ctx, `
		UPDATE network_optimizer.chain_search_runs
		SET created_at = now() - interval '26 seconds',
		    deadline_at = now() - interval '6 seconds',
		    claim_expires_at = now() - interval '1 second',
		    started_at = CASE WHEN status = 'RUNNING' THEN now() - interval '26 seconds' ELSE started_at END
		WHERE id = $1`, searchID)
	if err != nil {
		t.Fatalf("expire claim window: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("expire claim window rows=%d", tag.RowsAffected())
	}
}

func getRun(t *testing.T, ctx context.Context, store *repository.Postgres, tenant, id uuid.UUID) repository.ChainSearchRun {
	t.Helper()
	var got repository.ChainSearchRun
	if err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var err error
		got, err = tx.GetChainSearchRun(ctx, tenant, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertStatus(t *testing.T, ctx context.Context, store *repository.Postgres, tenant, id uuid.UUID, want string) {
	t.Helper()
	got := getRun(t, ctx, store, tenant, id)
	if got.Status != want {
		t.Fatalf("status=%s want %s", got.Status, want)
	}
}

func markRunning(ctx context.Context, store *repository.Postgres, tenant, id, owner uuid.UUID) (bool, error) {
	var ok bool
	err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var err error
		ok, err = tx.MarkChainSearchRunning(ctx, tenant, id, owner)
		return err
	})
	return ok, err
}

func ownerPersist(ctx context.Context, store *repository.Postgres, run repository.ChainSearchRun, owner uuid.UUID, expected string) (bool, error) {
	var ok bool
	err := store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var err error
		ok, err = tx.OwnerPersistChainSearchRun(ctx, run, owner, expected)
		return err
	})
	return ok, err
}

// keep fmt used for port string construction in startPostgres
var _ = fmt.Sprintf
