//go:build integration

package postgres

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

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

const maxMigrationNumber = 9999 // apply all platform migrations for integration parity

type TestEnv struct {
	T    *testing.T
	Pool *pgxpool.Pool
	Repo *repository.ProjectionRepository
}

func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping live PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return &TestEnv{
		T:    t,
		Pool: pool,
		Repo: repository.NewProjectionRepository(pool),
	}
}

func setupTestEnv(t *testing.T) *TestEnv {
	return SetupTestEnv(t)
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}

	dbName = "freight_platform_control_tower_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"

	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, fmt.Errorf("connect admin database: %w", err)
	}
	defer adminPool.Close()

	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, fmt.Errorf("create database: %w", err)
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
	ssl := "disable"
	if cfg.ConnConfig.TLSConfig != nil {
		ssl = "require"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, pass, host, port, db, ssl)
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
		base := filepath.Base(file)
		num := 0
		if _, scanErr := fmt.Sscanf(base, "%d", &num); scanErr != nil {
			return fmt.Errorf("parse migration number from %s: %w", base, scanErr)
		}
		if num > maxMigrationNumber {
			continue
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("apply %s: %w", base, execErr)
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

func sampleProcessInput(tenantID, shipmentID uuid.UUID, version int, eventID, sourceEventID uuid.UUID, topic string, offset int64) repository.ProcessInput {
	now := time.Now().UTC()
	return repository.ProcessInput{
		Event: domain.ShipmentStatusEvent{
			EventID:       eventID,
			EventType:     domain.EventTypeStatusChanged,
			SchemaVersion: domain.SchemaVersionV1,
			OccurredAt:    now,
			TenantID:      tenantID,
			Aggregate: domain.ShipmentAggregate{
				Type:    domain.AggregateTypeShipment,
				ID:      shipmentID,
				Version: version,
			},
			SourceEventID: sourceEventID,
			Data: domain.ShipmentStatusEventData{
				ToStatus:  domain.StatusInTransit,
				ActorType: "SYSTEM",
			},
		},
		Meta: domain.KafkaRecordMeta{
			Topic:     topic,
			Partition: 0,
			Offset:    offset,
			Key:       shipmentID.String(),
		},
		ReceivedAt: now,
	}
}

func countRows(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int64 {
	var n int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		panic(err)
	}
	return n
}
