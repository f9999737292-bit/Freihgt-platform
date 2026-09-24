//go:build integration

package repository

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigration000075UpDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	up := mustRead(t, "000075_bno_capacity_marketplace_foundation_v0_1a.up.sql")
	down := mustRead(t, "000075_bno_capacity_marketplace_foundation_v0_1a.down.sql")
	baseline := mustRead(t, "000001_create_schemas.up.sql")

	if _, err := pool.Exec(ctx, baseline); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("up: %v", err)
	}
	assertSchema(t, ctx, pool, "network_optimizer", true)
	assertSchema(t, ctx, pool, "transport", true)
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (
			id, owner_tenant_id, location_label, available_from, available_until, source,
			visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'Yard', now(), now() + interval '1 hour', 'CURRENT_SHIPMENT_PREDICTION', 'PRIVATE', 'AVAILABLE', 1, now(), now())`,
		uuid.New(), uuid.New()); err == nil {
		t.Fatal("predicted source insert must fail")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.load_opportunities (
			id, owner_tenant_id, source_type, source_id, visibility_scope, status, version, created_at, updated_at, weight_kg
		) VALUES ($1,$2,'TRANSPORT_ORDER',$3,'PRIVATE','DRAFT',1,now(),now(),0)`,
		uuid.New(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("zero weight insert must fail")
	}
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertSchema(t, ctx, pool, "network_optimizer", false)
	assertSchema(t, ctx, pool, "transport", true)
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("up again: %v", err)
	}
	assertSchema(t, ctx, pool, "network_optimizer", true)
}

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

func mustRead(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string, want bool) {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name=$1)`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("schema %s exists=%v want %v", name, exists, want)
	}
}
