//go:build integration

package kafka

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

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type pgTestEnv struct {
	pool *pgxpool.Pool
	repo *repository.ShipmentRepository
}

func setupPGTestEnv(t *testing.T) *pgTestEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL + Kafka end-to-end tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
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
	_ = dbName
	return &pgTestEnv{pool: pool, repo: repository.NewShipmentRepository(pool)}
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	dbName = "freight_platform_kafka_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
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
		cadmin, _ := pgxpool.NewWithConfig(cctx, adminCfg)
		if cadmin == nil {
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
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		user, pass, cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	var migrationsDir string
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			migrationsDir = candidate
			break
		}
	}
	if migrationsDir == "" {
		return fmt.Errorf("migrations dir not found")
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

type seedFixture struct {
	TenantID         uuid.UUID
	ShipperID        uuid.UUID
	ConsigneeID      uuid.UUID
	CarrierID        uuid.UUID
	OriginID         uuid.UUID
	DestinationID    uuid.UUID
	TransportOrderID uuid.UUID
	UserID           uuid.UUID
}

func (env *pgTestEnv) seedFixture(t *testing.T) seedFixture {
	t.Helper()
	ctx := context.Background()
	fix := seedFixture{
		TenantID:      uuid.New(),
		ShipperID:     uuid.New(),
		ConsigneeID:   uuid.New(),
		CarrierID:     uuid.New(),
		OriginID:      uuid.New(),
		DestinationID: uuid.New(),
		UserID:        uuid.New(),
	}
	fix.TransportOrderID = uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Kafka E2E Tenant")
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id   uuid.UUID
		typ  string
		name string
	}{
		{fix.ShipperID, "SHIPPER", "Shipper"},
		{fix.ConsigneeID, "CONSIGNEE", "Consignee"},
		{fix.CarrierID, "CARRIER", "Carrier"},
	} {
		_, err = env.pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, legal_name, company_type)
			VALUES ($1, $2, $3, $4)
		`, row.id, fix.TenantID, row.name, row.typ)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestinationID, "Destination"},
	} {
		_, err = env.pool.Exec(ctx, `
			INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code)
			VALUES ($1, $2, 'WAREHOUSE', $3, 'RU')
		`, loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, transport_mode
		) VALUES ($1, $2, $3, 'ASSIGNED', $4, $5, $6, $7, 'ROAD')
	`, fix.TransportOrderID, fix.TenantID, "TO-"+fix.TransportOrderID.String()[:8],
		fix.ShipperID, fix.ConsigneeID, fix.OriginID, fix.DestinationID)
	if err != nil {
		t.Fatalf("seed transport order: %v", err)
	}
	return fix
}

func repositoryCreateParams(fix seedFixture, number string) repository.CreateShipmentParams {
	return repository.CreateShipmentParams{
		TenantID:              fix.TenantID,
		ShipmentNumber:        number,
		TransportOrderID:      fix.TransportOrderID,
		ShipperCompanyID:      fix.ShipperID,
		ConsigneeCompanyID:    fix.ConsigneeID,
		CarrierCompanyID:      fix.CarrierID,
		OriginLocationID:      fix.OriginID,
		DestinationLocationID: fix.DestinationID,
		TransportMode:         "ROAD",
	}
}

func userTransition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func claimNow() time.Time {
	return time.Now().UTC().Add(time.Minute)
}
