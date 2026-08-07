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
	require.NoError(t, applyMigrationsThrough18(context.Background(), pool))
	return pool
}

func applyMigrationsThrough18(ctx context.Context, pool *pgxpool.Pool) error {
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
		if strings.HasPrefix(base, "000020") {
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
		if strings.HasPrefix(base, "000017") || strings.HasPrefix(base, "000018") || strings.HasPrefix(base, "000019") {
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

func applySingleMigration(ctx context.Context, pool *pgxpool.Pool, prefix string) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, prefix+".up.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("apply %s: %w", prefix, err)
	}
	return nil
}

func applySingleMigrationDown(ctx context.Context, pool *pgxpool.Pool, prefix string) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, prefix+".down.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("apply down %s: %w", prefix, err)
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

func setupMigrationDBAt16(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg, err := pgxpool.ParseConfig(adminURL)
	require.NoError(t, err)
	dbName := "freight_rebuild_upg_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
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

func TestMigrationUpgrade000016To000018(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDBAt16(t)
	t.Cleanup(pool.Close)

	snapshotID := uuid.New()
	tenantID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection_rebuild_job (
    snapshot_id, schema_version, scope, tenant_id, state, started_at, import_started_at, created_at, updated_at
) VALUES ($1, 1, 'TENANT', $2, 'VALIDATED', NOW(), NOW(), NOW(), NOW())`,
		snapshotID, tenantID)
	require.NoError(t, err)

	shipID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at,
    projection_source
) VALUES ($1,$2,3,'IN_TRANSIT',$3,$4,'shipment.status.changed',NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
		tenantID, shipID, uuid.New(), uuid.New())
	require.NoError(t, err)

	var inboxBefore, dlBefore, projBefore int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox`).Scan(&inboxBefore))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter`).Scan(&dlBefore))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection`).Scan(&projBefore))

	require.NoError(t, applySingleMigration(ctx, pool, "000017_add_projection_rebuild_activation_v0.1"))
	require.NoError(t, applySingleMigration(ctx, pool, "000018_projection_rebuild_last_event_type_v0.1"))

	var jobState string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT state FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`, snapshotID).Scan(&jobState))
	require.Equal(t, "VALIDATED", jobState)

	var inboxAfter, dlAfter, projAfter int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox`).Scan(&inboxAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter`).Scan(&dlAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection`).Scan(&projAfter))
	require.Equal(t, inboxBefore, inboxAfter)
	require.Equal(t, dlBefore, dlAfter)
	require.Equal(t, projBefore, projAfter)

	var nullable bool
	require.NoError(t, pool.QueryRow(ctx, `
SELECT is_nullable='YES' FROM information_schema.columns
WHERE table_schema='control_tower' AND table_name='shipment_status_projection' AND column_name='last_event_type'
`).Scan(&nullable))
	require.True(t, nullable)

	var stageCol bool
	require.NoError(t, pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='control_tower' AND table_name='shipment_status_projection_rebuild_stage' AND column_name='last_event_type'
)`).Scan(&stageCol))
	require.True(t, stageCol)
}

func TestMigration000018DownRequiresNoNullLastEventType(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	tenantID, shipID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at,
    projection_source
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,NULL,NOW(),NOW(),TRUE,FALSE,NOW(),NOW(),'LIVE_EVENT')`,
		tenantID, shipID, uuid.New(), uuid.New())
	require.NoError(t, err)

	err = applySingleMigrationDown(ctx, pool, "000018_projection_rebuild_last_event_type_v0.1")
	require.Error(t, err, "down migration must fail when NULL last_event_type rows exist")
}

func TestMigration000019BackupLastEventTypeNullable(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	require.NoError(t, applySingleMigration(ctx, pool, "000019_projection_rebuild_backup_last_event_type_nullable_v0.1"))

	var nullable bool
	require.NoError(t, pool.QueryRow(ctx, `
SELECT is_nullable='YES' FROM information_schema.columns
WHERE table_schema='control_tower'
  AND table_name='shipment_status_projection_rebuild_backup'
  AND column_name='last_event_type'
`).Scan(&nullable))
	require.True(t, nullable)
}

func TestMigration000019DownRequiresNoNullBackupLastEventType(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	snapshotID := uuid.New()
	require.NoError(t, applySingleMigration(ctx, pool, "000019_projection_rebuild_backup_last_event_type_nullable_v0.1"))
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection_rebuild_backup (
    snapshot_id, tenant_id, shipment_id, shipment_version, current_status,
    last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
    complete, gap_detected, projection_source, created_at, updated_at, backed_up_at
) VALUES (
    $1, $2, $3, 1, 'IN_TRANSIT',
    $4, $5, NULL, NOW(), NOW(),
    TRUE, FALSE, 'LIVE_EVENT', NOW(), NOW(), NOW()
)`, snapshotID, uuid.New(), uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)

	err = applySingleMigrationDown(ctx, pool, "000019_projection_rebuild_backup_last_event_type_nullable_v0.1")
	require.Error(t, err, "down migration must fail when NULL backup last_event_type rows exist")
}
