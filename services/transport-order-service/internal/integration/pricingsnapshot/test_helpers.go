//go:build integration

package pricingsnapshot

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
	"github.com/shopspring/decimal"

	todomain "github.com/freight-platform/transport-order-service/internal/domain"
	torepo "github.com/freight-platform/transport-order-service/internal/repository"
)

type testEnv struct {
	pool         *pgxpool.Pool
	pricedOrders *torepo.PricedOrderRepository
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("postgres unavailable: %v", err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Logf("database=%s", dbName)
	return &testEnv{pool: pool, pricedOrders: torepo.NewPricedOrderRepository(pool)}
}

func createTempDatabase(ctx context.Context, adminURL string) (string, string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, err
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName := "to_pricing_snapshot_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = adminDB
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
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(testCfg.ConnConfig.User),
		url.QueryEscape(testCfg.ConnConfig.Password),
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, testCfg.ConnConfig.Database)
	cleanup := func(cctx context.Context) {
		cadmin, _ := pgxpool.NewWithConfig(cctx, adminCfg)
		if cadmin != nil {
			defer cadmin.Close()
			_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		}
	}
	return dbName, testURL, cleanup, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
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
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), execErr)
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
	return "", fmt.Errorf("migrations dir not found")
}

func seedTenantCompanies(t *testing.T, pool *pgxpool.Pool) (tenantID, buyerID, carrierID, originID, destID, cargoID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID = uuid.New()
	buyerID = uuid.New()
	carrierID = uuid.New()
	originID = uuid.New()
	destID = uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, tenantID, "t-"+tenantID.String()[:8], "TO Pricing Tenant")
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	for _, row := range []struct {
		id   uuid.UUID
		name string
		typ  string
	}{
		{buyerID, "Buyer", "SHIPPER"},
		{carrierID, "Carrier", "CARRIER"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, tenantID, row.name, row.typ); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	for _, locID := range []uuid.UUID{originID, destID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code, city)
			VALUES ($1,$2,'WAREHOUSE','WH','RU','Moscow')`, locID, tenantID); err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	if err := pool.QueryRow(ctx, `INSERT INTO transport.cargoes (tenant_id, cargo_type, description) VALUES ($1,'GENERAL','test') RETURNING id`, tenantID).Scan(&cargoID); err != nil {
		t.Fatalf("cargo: %v", err)
	}
	return tenantID, buyerID, carrierID, originID, destID, cargoID
}

func sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID uuid.UUID, key string) todomain.CreatePricedTransportOrderInput {
	equip := "TAUTLINER"
	return todomain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: todomain.CreateTransportOrderInput{
			TenantID:              tenantID,
			OrderNumber:           "TO-" + uuid.NewString()[:8],
			ShipperCompanyID:      buyerID,
			ConsigneeCompanyID:    buyerID,
			OriginLocationID:      originID,
			DestinationLocationID: destID,
			CargoID:               cargoID,
			TransportMode:         "ROAD",
			EquipmentType:         &equip,
		},
		Actor: todomain.InternalActor{
			TenantID:  tenantID,
			UserID:    uuid.New(),
			CompanyID: buyerID,
			ActorKind: "USER",
		},
		PricingContext: todomain.PricingContext{CarrierCompanyID: carrierID},
		IdempotencyKey: key,
	}
}

func sampleSnapshot(tenantID, buyerID, carrierID, originID, destID uuid.UUID) todomain.RateSnapshot {
	total := decimal.RequireFromString("1500.00")
	return todomain.RateSnapshot{
		TenantID:                 tenantID,
		BuyerCompanyID:           buyerID,
		CarrierCompanyID:         carrierID,
		PricingSource:            "RFQ_AWARD",
		OriginLocationID:         originID,
		DestinationLocationID:    destID,
		EquipmentType:            "TAUTLINER",
		TransportMode:            "ROAD",
		CurrencyCode:             "RUB",
		ComponentBreakdownStatus: "UNAVAILABLE",
		Components:               todomain.EmptyJSONArray(),
		AccessorialRules:         todomain.EmptyJSONArray(),
		TotalAmount:              total,
		PricingDate:              time.Now().UTC(),
		ResolvedAt:               time.Now().UTC(),
		ResolvedByService:        "test",
		ResolverVersion:          "v2.0C",
		ResolutionRequestHash:    strings.Repeat("a", 64),
	}
}
