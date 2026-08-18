//go:build integration

package executiontracking

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type env struct {
	pool           *pgxpool.Pool
	driverOps      *service.DriverOperationsService
	orderExecution *service.OrderExecutionService
}

func setupEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	shipmentRepo := repository.NewShipmentRepository(pool)
	orderExecutionRepo := repository.NewOrderExecutionRepository(pool)
	driverRepo := repository.NewDriverRepository(pool)
	vehicleRepo := repository.NewVehicleRepository(pool)
	driverOpsRepo := repository.NewDriverOperationsRepository(pool)
	shipmentSvc := service.NewShipmentService(shipmentRepo, driverRepo, vehicleRepo)
	driverOpsSvc := service.NewDriverOperationsService(driverRepo, shipmentRepo, driverOpsRepo)
	orderExecutionSvc := service.NewOrderExecutionService(orderExecutionRepo, shipmentSvc)

	return &env{
		pool:           pool,
		driverOps:      driverOpsSvc,
		orderExecution: orderExecutionSvc,
	}
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
			msg := execErr.Error()
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
				continue
			}
			return execErr
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
	return "", os.ErrNotExist
}

type fixture struct {
	TenantID   uuid.UUID
	BuyerID    uuid.UUID
	CarrierA   uuid.UUID
	CarrierB   uuid.UUID
	OrderID    uuid.UUID
	EventID    uuid.UUID
	UserID     uuid.UUID
	DriverA    uuid.UUID
	DriverB    uuid.UUID
	VehicleA   uuid.UUID
	OriginID   uuid.UUID
	DestID     uuid.UUID
	CargoID    uuid.UUID
	ShipmentID uuid.UUID
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantID: uuid.New(),
		BuyerID:  uuid.New(),
		CarrierA: uuid.New(),
		CarrierB: uuid.New(),
		OrderID:  uuid.New(),
		EventID:  uuid.New(),
		UserID:   uuid.New(),
		DriverA:  uuid.New(),
		DriverB:  uuid.New(),
		VehicleA: uuid.New(),
		OriginID: uuid.New(),
		DestID:   uuid.New(),
		CargoID:  uuid.New(),
	}
	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Tracking Tenant")
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	for _, row := range []struct {
		id        uuid.UUID
		typ, name string
	}{
		{fix.BuyerID, "SHIPPER", "Buyer Co"},
		{fix.CarrierA, "CARRIER", "Carrier A"},
		{fix.CarrierB, "CARRIER", "Carrier B"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, row.id, fix.TenantID, row.typ, row.name)
		if err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestID, "Destination"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantID, fix.BuyerID, loc.name)
		if err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		VALUES ($1,$2,'GENERAL','Cargo',1000)`, fix.CargoID, fix.TenantID)
	if err != nil {
		t.Fatalf("cargo: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.transport_orders (
		id, tenant_id, order_number, shipper_company_id, consignee_company_id,
		origin_location_id, destination_location_id, cargo_id, transport_mode, status, source_system
	) VALUES ($1,$2,$3,$4,$4,$5,$6,$7,'ROAD','DRAFT','rfx_award')`,
		fix.OrderID, fix.TenantID, "TO-"+fix.OrderID.String()[:8], fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	awardID := uuid.New()
	responseID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO rfx.rfx_events (id, tenant_id, owner_company_id, title, rfx_type, category, rfx_number, status)
		VALUES ($1,$2,$3,'Award Event','SPOT_RFQ','FREIGHT',$4,'AWARDED')`,
		fix.EventID, fix.TenantID, fix.BuyerID, "RFX-"+fix.EventID.String()[:8])
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO rfx.rfx_responses (id, tenant_id, rfx_event_id, participant_company_id, status)
		VALUES ($1,$2,$3,$4,'SUBMITTED')`, responseID, fix.TenantID, fix.EventID, fix.CarrierA)
	if err != nil {
		t.Fatalf("response: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO rfx.rfx_awards (id, tenant_id, rfx_event_id, rfx_response_id, carrier_company_id, awarded_at)
		VALUES ($1,$2,$3,$4,$5,now())`, awardID, fix.TenantID, fix.EventID, responseID, fix.CarrierA)
	if err != nil {
		t.Fatalf("award: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO rfx.rfx_award_transport_orders (
		tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, transport_order_id,
		carrier_company_id, buyer_company_id, amount, currency_code
	) VALUES ($1,$2,$3,$4,$5,$6,$7,100000,'RUB')`,
		fix.TenantID, fix.EventID, awardID, responseID, fix.OrderID, fix.CarrierA, fix.BuyerID)
	if err != nil {
		t.Fatalf("award link: %v", err)
	}
	driverUserA := uuid.New()
	driverUserB := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,'Driver A','ACTIVE'), ($5,$2,$6,$7,'Driver B','ACTIVE')`,
		fix.DriverA, fix.TenantID, fix.CarrierA, driverUserA, fix.DriverB, fix.CarrierB, driverUserB)
	if err != nil {
		t.Fatalf("drivers: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.vehicles (id, tenant_id, carrier_company_id, plate_number, vehicle_type, status)
		VALUES ($1,$2,$3,'A123BC77','TRUCK','ACTIVE')`, fix.VehicleA, fix.TenantID, fix.CarrierA)
	if err != nil {
		t.Fatalf("vehicle: %v", err)
	}
	fix.UserID = driverUserA
	return fix
}

func transition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func TestDriverMilestonePickupToLoadedAndOutboxActuals(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	execResult, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-TRACK-1",
	}, transition(fix.UserID))
	if err != nil || execResult.Shipment == nil {
		t.Fatalf("execute: %v shipment=%v", err, execResult != nil && execResult.Shipment != nil)
	}
	fix.ShipmentID = execResult.Shipment.ID

	shipmentRepo := repository.NewShipmentRepository(env.pool)
	shipmentSvc := service.NewShipmentService(shipmentRepo, repository.NewDriverRepository(env.pool), repository.NewVehicleRepository(env.pool))
	if _, err := shipmentSvc.AssignDriver(ctx, fix.TenantID, fix.ShipmentID, fix.DriverA, transition(fix.UserID)); err != nil {
		t.Fatalf("assign driver: %v", err)
	}
	if _, err := shipmentSvc.AssignVehicle(ctx, fix.TenantID, fix.ShipmentID, fix.VehicleA, transition(fix.UserID)); err != nil {
		t.Fatalf("assign vehicle: %v", err)
	}
	if _, err := env.orderExecution.StartExecution(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, transition(fix.UserID)); err != nil {
		t.Fatalf("start execution: %v", err)
	}

	first, err := env.driverOps.RecordOperationalEvent(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverOperationalEventInput{
		Type:           "PICKUP_COMPLETED",
		IdempotencyKey: "pickup-complete-1",
	}, transition(fix.UserID))
	if err != nil {
		t.Fatalf("pickup completed: %v", err)
	}
	if first.ShipmentStatus != domain.ShipmentStatusLoaded {
		t.Fatalf("status=%s want LOADED", first.ShipmentStatus)
	}

	replay, err := env.driverOps.RecordOperationalEvent(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverOperationalEventInput{
		Type:           "PICKUP_COMPLETED",
		IdempotencyKey: "pickup-complete-1",
	}, transition(fix.UserID))
	if err != nil || !replay.Replayed {
		t.Fatalf("idempotent replay: err=%v replayed=%v", err, replay.Replayed)
	}

	var actualPickup *time.Time
	if err := env.pool.QueryRow(ctx, `SELECT actual_pickup_at FROM transport.shipments WHERE id=$1`, fix.ShipmentID).Scan(&actualPickup); err != nil {
		t.Fatalf("actual pickup: %v", err)
	}
	if actualPickup == nil {
		t.Fatal("actual_pickup_at not persisted")
	}

	outboxPayload, _ := fetchLatestOutboxPayload(t, ctx, env.pool, fix.ShipmentID)
	var envelope struct {
		Data struct {
			ActualPickupAt string `json:"actualPickupAt"`
			ToStatus       string `json:"toStatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(outboxPayload, &envelope); err != nil {
		t.Fatalf("outbox json: %v", err)
	}
	if envelope.Data.ToStatus != domain.ShipmentStatusLoaded {
		t.Fatalf("outbox toStatus=%s want LOADED", envelope.Data.ToStatus)
	}
	if strings.TrimSpace(envelope.Data.ActualPickupAt) == "" {
		t.Fatal("outbox actualPickupAt missing")
	}
}

func TestForeignDriverMilestoneDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	execResult, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-TRACK-2",
	}, transition(fix.UserID))
	if err != nil || execResult.Shipment == nil {
		t.Fatalf("execute: %v", err)
	}
	fix.ShipmentID = execResult.Shipment.ID

	shipmentRepo := repository.NewShipmentRepository(env.pool)
	shipmentSvc := service.NewShipmentService(shipmentRepo, repository.NewDriverRepository(env.pool), repository.NewVehicleRepository(env.pool))
	if _, err := shipmentSvc.AssignDriver(ctx, fix.TenantID, fix.ShipmentID, fix.DriverA, transition(fix.UserID)); err != nil {
		t.Fatalf("assign driver: %v", err)
	}

	driverUserB := uuid.New()
	_, err = env.pool.Exec(ctx, `UPDATE transport.drivers SET user_id=$1 WHERE id=$2`, driverUserB, fix.DriverB)
	if err != nil {
		t.Fatalf("driver b user: %v", err)
	}

	_, err = env.driverOps.RecordOperationalEvent(ctx, fix.TenantID, driverUserB, fix.ShipmentID, domain.DriverOperationalEventInput{
		Type:           "PICKUP_COMPLETED",
		IdempotencyKey: "foreign-driver",
	}, transition(driverUserB))
	if err == nil {
		t.Fatal("foreign driver should be denied")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestBuyerOrderListIsolation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	if _, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-TRACK-3",
	}, transition(fix.UserID)); err != nil {
		t.Fatalf("execute: %v", err)
	}

	items, total, err := env.orderExecution.ListBuyerTransportOrders(ctx, domain.ListBuyerTransportOrdersFilter{
		TenantID: fix.TenantID, BuyerCompanyID: fix.BuyerID, Limit: 20,
	})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("buyer list: err=%v total=%d len=%d", err, total, len(items))
	}

	otherBuyer := uuid.New()
	_, err = env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1,$2,'SHIPPER','Other Buyer','ACTIVE')`, otherBuyer, fix.TenantID)
	if err != nil {
		t.Fatalf("other buyer: %v", err)
	}
	items, total, err = env.orderExecution.ListBuyerTransportOrders(ctx, domain.ListBuyerTransportOrdersFilter{
		TenantID: fix.TenantID, BuyerCompanyID: otherBuyer, Limit: 20,
	})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("foreign buyer list: err=%v total=%d len=%d", err, total, len(items))
	}
}

func fetchLatestOutboxPayload(t *testing.T, ctx context.Context, pool *pgxpool.Pool, shipmentID uuid.UUID) ([]byte, uuid.UUID) {
	t.Helper()
	var payload []byte
	var sourceEventID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT payload, source_event_id
		FROM transport.shipment_event_outbox
		WHERE aggregate_id = $1
		ORDER BY aggregate_version DESC
		LIMIT 1
	`, shipmentID).Scan(&payload, &sourceEventID)
	if err != nil {
		t.Fatalf("outbox: %v", err)
	}
	return payload, sourceEventID
}
