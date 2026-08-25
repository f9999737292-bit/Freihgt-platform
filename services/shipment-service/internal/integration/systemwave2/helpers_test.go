//go:build integration

package systemwave2

import (
	"context"
	"errors"
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
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type env struct {
	pool        *pgxpool.Pool
	shipmentSvc *service.ShipmentService
}

type wave2Fixture struct {
	TenantA      uuid.UUID
	TenantB      uuid.UUID
	BuyerA       uuid.UUID
	CarrierA1    uuid.UUID
	CarrierA2    uuid.UUID
	CarrierB1    uuid.UUID
	ConsigneeA   uuid.UUID
	OrderID      uuid.UUID
	BidID        uuid.UUID
	FreightReqID uuid.UUID
	UserID       uuid.UUID
	DriverA1     uuid.UUID
	DriverB1     uuid.UUID
	VehicleA1    uuid.UUID
	VehicleB1    uuid.UUID
}

func setupEnv(t *testing.T) *env {
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
		t.Fatalf("isolated postgres: %v", err)
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
	t.Logf("isolated database=%s", dbName)

	shipmentRepo := repository.NewShipmentRepository(pool)
	driverRepo := repository.NewDriverRepository(pool)
	vehicleRepo := repository.NewVehicleRepository(pool)
	shipmentSvc := service.NewShipmentService(shipmentRepo, driverRepo, vehicleRepo)

	return &env{pool: pool, shipmentSvc: shipmentSvc}
}

func seedWave2Fixture(t *testing.T, pool *pgxpool.Pool) wave2Fixture {
	t.Helper()
	ctx := context.Background()
	fix := wave2Fixture{
		TenantA: uuid.New(), TenantB: uuid.New(),
		BuyerA: uuid.New(), CarrierA1: uuid.New(), CarrierA2: uuid.New(), CarrierB1: uuid.New(),
		ConsigneeA: uuid.New(), OrderID: uuid.New(), BidID: uuid.New(), FreightReqID: uuid.New(),
		UserID: uuid.New(), DriverA1: uuid.New(), DriverB1: uuid.New(), VehicleA1: uuid.New(), VehicleB1: uuid.New(),
	}
	for _, row := range []struct{ id uuid.UUID; code, name string }{
		{fix.TenantA, "ta-" + fix.TenantA.String()[:8], "Tenant A"},
		{fix.TenantB, "tb-" + fix.TenantB.String()[:8], "Tenant B"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,$2,$3)`, row.id, row.code, row.name); err != nil {
			t.Fatalf("tenant: %v", err)
		}
	}
	for _, c := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{fix.BuyerA, fix.TenantA, "SHIPPER", "Buyer A"},
		{fix.CarrierA1, fix.TenantA, "CARRIER", "Carrier A1"},
		{fix.CarrierA2, fix.TenantA, "CARRIER", "Carrier A2"},
		{fix.ConsigneeA, fix.TenantA, "CONSIGNEE", "Consignee A"},
		{fix.CarrierB1, fix.TenantB, "CARRIER", "Carrier B1"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,company_type,legal_name,status) VALUES ($1,$2,$3,$4,'ACTIVE')`,
			c.id, c.tenant, c.typ, c.name); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	originID, destID, cargoID := uuid.New(), uuid.New(), uuid.New()
	for _, loc := range []struct{ id uuid.UUID; name string }{
		{originID, "Origin"}, {destID, "Dest"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO transport.locations (id,tenant_id,company_id,location_type,name,country_code,city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantA, fix.BuyerA, loc.name); err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.cargoes (id,tenant_id,cargo_type,description,gross_weight) VALUES ($1,$2,'GEN','c',1000)`,
		cargoID, fix.TenantA); err != nil {
		t.Fatalf("cargo: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.transport_orders (
		id,tenant_id,order_number,shipper_company_id,consignee_company_id,origin_location_id,destination_location_id,cargo_id,transport_mode,status,source_system
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'ROAD','READY_FOR_SOURCING','wave2')`,
		fix.OrderID, fix.TenantA, "TO-W2", fix.BuyerA, fix.ConsigneeA, originID, destID, cargoID); err != nil {
		t.Fatalf("order: %v", err)
	}
	deadline := time.Now().UTC().Add(48 * time.Hour)
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.freight_requests (
		id,tenant_id,transport_order_id,freight_request_number,request_type,shipper_company_id,status,response_deadline,currency_code
	) VALUES ($1,$2,$3,$4,'MINI_TENDER',$5,'AWARDED',$6,'RUB')`,
		fix.FreightReqID, fix.TenantA, fix.OrderID, "FR-W2", fix.BuyerA, deadline); err != nil {
		t.Fatalf("freight request: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rfx.bids (
		id,tenant_id,freight_request_id,carrier_company_id,bid_number,status,total_amount,currency_code,total_amount_with_vat
	) VALUES ($1,$2,$3,$4,$5,'ACCEPTED',100000,'RUB',120000)`,
		fix.BidID, fix.TenantA, fix.FreightReqID, fix.CarrierA1, "BID-W2"); err != nil {
		t.Fatalf("bid: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status)
		VALUES ($1,$2,$3,$4,'Driver A1','ACTIVE'),($5,$6,$7,$8,'Driver B1','ACTIVE')`,
		fix.DriverA1, fix.TenantA, fix.CarrierA1, uuid.New(),
		fix.DriverB1, fix.TenantB, fix.CarrierB1, uuid.New()); err != nil {
		t.Fatalf("drivers: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.vehicles (id,tenant_id,carrier_company_id,plate_number,vehicle_type,status)
		VALUES ($1,$2,$3,'A111AA77','TRUCK','ACTIVE'),($4,$5,$6,'B222BB77','TRUCK','ACTIVE')`,
		fix.VehicleA1, fix.TenantA, fix.CarrierA1, fix.VehicleB1, fix.TenantB, fix.CarrierB1); err != nil {
		t.Fatalf("vehicles: %v", err)
	}
	return fix
}

func transition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func advanceToReadyForBilling(t *testing.T, env *env, fix wave2Fixture) *domain.Shipment {
	t.Helper()
	ctx := context.Background()
	tr := transition(fix.UserID)
	shipment, err := env.shipmentSvc.CreateFromBid(ctx, fix.TenantA, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SH-W2-" + fix.BidID.String()[:8], BidID: fix.BidID, TransportOrderID: fix.OrderID,
	}, tr)
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}
	if _, err := env.shipmentSvc.AssignDriver(ctx, fix.TenantA, shipment.ID, fix.DriverA1, tr); err != nil {
		t.Fatalf("assign driver: %v", err)
	}
	if _, err := env.shipmentSvc.AssignVehicle(ctx, fix.TenantA, shipment.ID, fix.VehicleA1, tr); err != nil {
		t.Fatalf("assign vehicle: %v", err)
	}
	states := []string{
		domain.ShipmentStatusPickupSlotBooked, domain.ShipmentStatusInPickup, domain.ShipmentStatusLoaded,
		domain.ShipmentStatusInTransit, domain.ShipmentStatusArrivedAtConsignee, domain.ShipmentStatusUnloading,
		domain.ShipmentStatusDelivered, domain.ShipmentStatusDeliveryConfirmed,
		domain.ShipmentStatusDocumentsCompleted, domain.ShipmentStatusReadyForBilling,
	}
	current := shipment
	for _, st := range states {
		var actualTime *time.Time
		if st == domain.ShipmentStatusLoaded || st == domain.ShipmentStatusDelivered {
			now := time.Now().UTC()
			actualTime = &now
		}
		current, err = env.shipmentSvc.UpdateStatus(ctx, fix.TenantA, current.ID, domain.UpdateShipmentStatusInput{
			Status: st, ActualTime: actualTime,
		}, tr)
		if err != nil {
			t.Fatalf("transition %s: %v", st, err)
		}
	}
	return current
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
	dbName := "ship_wave2_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
		url.QueryEscape(testCfg.ConnConfig.User), url.QueryEscape(testCfg.ConnConfig.Password),
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, testCfg.ConnConfig.Database)
	cleanup := func(cctx context.Context) {
		p, _ := pgxpool.NewWithConfig(cctx, adminCfg)
		if p != nil {
			defer p.Close()
			_, _ = p.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		}
	}
	return dbName, testURL, cleanup, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	sort.Strings(files)
	for _, file := range files {
		content, _ := os.ReadFile(file)
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	for _, c := range []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", os.ErrNotExist
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
