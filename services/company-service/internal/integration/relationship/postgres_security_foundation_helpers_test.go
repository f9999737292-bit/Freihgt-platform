//go:build integration

package relationship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	secDigest  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	secDigestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	secFinger  = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	secFingerB = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
)

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func setupSecurityPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres: %v", err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
		dropSecurityRoles(adminURL)
	})
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Logf("isolated database=%s", dbName)
	return pool
}

func expectSecurityObjects(ctx context.Context, pool *pgxpool.Pool, present bool) error {
	var roles, trigger, column int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM pg_roles WHERE rolname IN ('freight_migrations', 'company_runtime', 'shipment_runtime')),
			(SELECT count(*) FROM pg_trigger WHERE tgname = 'trg_new_activation_authorization' AND NOT tgisinternal),
			(SELECT count(*) FROM information_schema.columns
			  WHERE table_schema = 'transport' AND table_name = 'transport_execution_revisions'
			    AND column_name = 'pin_set_fingerprint')
	`).Scan(&roles, &trigger, &column); err != nil {
		return err
	}
	if trigger != 0 {
		return fmt.Errorf("enforcement trigger present during migration cycle")
	}
	wantRoles, wantColumn := 0, 0
	if present {
		wantRoles, wantColumn = 3, 1
	}
	if roles != wantRoles || column != wantColumn {
		return fmt.Errorf("roles=%d column=%d", roles, column)
	}
	return nil
}

func assertFunctionHardened(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var definer bool
	var config []string
	var body string
	if err := pool.QueryRow(context.Background(), `
		SELECT p.prosecdef, p.proconfig, p.prosrc
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'company' AND p.proname = 'authorize_carrier_relationship_activation'
	`).Scan(&definer, &config, &body); err != nil {
		t.Fatal(err)
	}
	if !definer || !strings.Contains(strings.Join(config, ","), "search_path=pg_catalog") {
		t.Fatalf("authorizer definer=%v config=%v", definer, config)
	}
	if strings.Contains(body, "EXECUTE") || strings.Contains(body, "current_setting") {
		t.Fatal("authorizer uses dynamic SQL or a session setting")
	}
}

func poolAs(t *testing.T, pool *pgxpool.Pool, role string) *pgxpool.Pool {
	t.Helper()
	cfg := pool.Config().Copy()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+role)
		return err
	}
	next, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(next.Close)
	return next
}

func withRole(t *testing.T, pool *pgxpool.Pool, role string, fn func(context.Context, pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+role); err != nil {
		t.Fatal(err)
	}
	fn(ctx, tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("%s commit: %v", role, err)
	}
}

func beginRole(t *testing.T, pool *pgxpool.Pool, role string) (context.Context, pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+role); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	return ctx, tx
}

func insertExecutionRevision(t *testing.T, pool *pgxpool.Pool, tenant, carrier, activation uuid.UUID, supersedes *uuid.UUID, digest string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	execID, revID := uuid.New(), uuid.New()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO transport.transport_executions (
			id, operating_tenant_id, carrier_company_id, current_revision_id, created_at, updated_at
		) VALUES ($1,$2,$3,NULL,clock_timestamp(),clock_timestamp())
	`, execID, tenant, carrier); err != nil {
		t.Fatal(err)
	}
	insertRevisionTx(t, tx, execID, revID, tenant, activation, supersedes, digest)
	if _, err := tx.Exec(ctx, `UPDATE transport.transport_executions SET current_revision_id = $2 WHERE id = $1`, execID, revID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return execID, revID
}

func insertExecutionTx(t *testing.T, tx pgx.Tx, execID, tenant, carrier uuid.UUID) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO transport.transport_executions (
			id, operating_tenant_id, carrier_company_id, current_revision_id, created_at, updated_at
		) VALUES ($1,$2,$3,NULL,clock_timestamp(),clock_timestamp())
	`, execID, tenant, carrier); err != nil {
		t.Fatal(err)
	}
}

func insertRevisionTx(t *testing.T, tx pgx.Tx, execID, revID, tenant, activation uuid.UUID, supersedes *uuid.UUID, digest string) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			supersedes_revision_id, status, version, contract_sha256, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,1,$5,1,'security','DEPOT_START',$6,'ACTIVE',1,$7,clock_timestamp(),clock_timestamp()
		)
	`, revID, execID, tenant, uuid.New(), activation, supersedes, digest); err != nil {
		t.Fatal(err)
	}
}

func authorizerSQL() string {
	return `
		SELECT outcome, authorization_id, execution_id, revision_id
		FROM company.authorize_carrier_relationship_activation(
			$1,$2,$3,$4,$5,
			ARRAY[ROW($6,$7,$8)::company.activation_subject_input]
		)`
}

func callAuthorizer(ctx context.Context, q queryRower, activation, operatingTenant, operatingCompany uuid.UUID, digest, finger string, shipperTenant, shipment, shipperCarrier uuid.UUID) (string, uuid.UUID, error) {
	var outcome string
	var authID *uuid.UUID
	var execID, revID *uuid.UUID
	err := q.QueryRow(ctx, authorizerSQL(), activation, operatingTenant, operatingCompany, digest, finger, shipperTenant, shipment, shipperCarrier).Scan(&outcome, &authID, &execID, &revID)
	if err != nil {
		return "", uuid.Nil, err
	}
	if authID == nil {
		return outcome, uuid.Nil, nil
	}
	return outcome, *authID, nil
}

func permissionDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "42501") || strings.Contains(msg, "permission denied")
}

func applyEnforcementGate(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "..", "postgres", "tms_activation_enforcement_gate_v0.1.sql"))
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(raw))
	return err
}

func triggerCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return scalarInt(t, pool, `SELECT count(*) FROM pg_trigger WHERE tgname = 'trg_new_activation_authorization' AND NOT tgisinternal`)
}

func scalarInt(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func dropSecurityRoles(adminURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return
	}
	defer admin.Close()
	for _, role := range []string{"shipment_runtime", "company_runtime", "freight_migrations"} {
		_, _ = admin.Exec(ctx, "DROP OWNED BY "+role)
		_, _ = admin.Exec(ctx, "DROP ROLE IF EXISTS "+role)
	}
	var nologin bool
	if err := admin.QueryRow(ctx, `SELECT NOT rolcanlogin FROM pg_roles WHERE rolname = 'freight'`).Scan(&nologin); err == nil && nologin {
		_, _ = admin.Exec(ctx, "DROP OWNED BY freight")
		_, _ = admin.Exec(ctx, "DROP ROLE IF EXISTS freight")
	}
}
