//go:build integration

package analytics

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var v22MigrationPairs = []struct {
	up   string
	down string
}{
	{"000061_freight_cost_analytics_projection_v2.2B.up.sql", "000061_freight_cost_analytics_projection_v2.2B.down.sql"},
	{"000062_freight_cost_lane_carrier_intelligence_v2.2C.up.sql", "000062_freight_cost_lane_carrier_intelligence_v2.2C.down.sql"},
	{"000063_freight_cost_accessorial_enrichment_v2.2D.up.sql", "000063_freight_cost_accessorial_enrichment_v2.2D.down.sql"},
	{"000064_freight_cost_benchmark_savings_v2.2E.up.sql", "000064_freight_cost_benchmark_savings_v2.2E.down.sql"},
}

func TestFC22G_MigrationGateV22UpDown(t *testing.T) {
	ctx := context.Background()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	dir := filepath.Join(root, "infrastructure", "migrations")

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply up chain: %v", err)
	}

	for i := len(v22MigrationPairs) - 1; i >= 0; i-- {
		pair := v22MigrationPairs[i]
		raw, err := os.ReadFile(filepath.Join(dir, pair.down))
		if err != nil {
			t.Fatalf("read down %s: %v", pair.down, err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("apply down %s: %v", pair.down, err)
		}
		t.Logf("applied down migration %s", pair.down)
	}

	for _, pair := range v22MigrationPairs {
		raw, err := os.ReadFile(filepath.Join(dir, pair.up))
		if err != nil {
			t.Fatalf("read up %s: %v", pair.up, err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("re-apply up %s: %v", pair.up, err)
		}
	}
}
