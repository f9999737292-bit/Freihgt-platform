//go:build integration

package rebuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMigration000016Objects(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	assertTableExists(t, pool, "control_tower", "shipment_status_projection_rebuild_job")
	assertTableExists(t, pool, "control_tower", "shipment_status_projection_rebuild_stage")
	assertTableExists(t, pool, "control_tower", "shipment_status_projection_rebuild_backup")

	var inboxCount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox`).Scan(&inboxCount))
	var dlCount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter`).Scan(&dlCount))

	tenantID, shipmentID := uuid.New(), uuid.New()
	eventID, sourceID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,'shipment.created',NOW(),NOW(),TRUE,FALSE,NOW(),NOW())`,
		tenantID, shipmentID, eventID, sourceID)
	require.NoError(t, err)

	var projectionCount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1`, tenantID).Scan(&projectionCount))
	require.Equal(t, int64(1), projectionCount)
}

func setupMigrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg, err := pgxpool.ParseConfig(adminURL)
	require.NoError(t, err)
	dbName := "freight_rebuild_mig_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(context.Background(), adminCfg)
	require.NoError(t, err)
	t.Cleanup(adminPool.Close)
	_, err = adminPool.Exec(context.Background(), "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	})
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(context.Background(), testCfg)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsThrough16(context.Background(), pool))
	return pool
}

func applyMigrationsThrough16(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		if strings.HasPrefix(base, "000017") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply %s: %w", base, err)
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", os.ErrNotExist
}

func assertTableExists(t *testing.T, pool *pgxpool.Pool, schema, table string) {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(), `
SELECT EXISTS (
  SELECT 1 FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2
)`, schema, table).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists)
}
