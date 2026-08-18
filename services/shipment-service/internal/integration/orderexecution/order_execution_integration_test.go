//go:build integration

package orderexecution

import (
	"context"
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
	pool            *pgxpool.Pool
	orderExecution  *service.OrderExecutionService
	shipmentSvc     *service.ShipmentService
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
	shipmentSvc := service.NewShipmentService(shipmentRepo, driverRepo, vehicleRepo)
	orderExecutionSvc := service.NewOrderExecutionService(orderExecutionRepo, shipmentSvc)
	return &env{pool: pool, orderExecution: orderExecutionSvc, shipmentSvc: shipmentSvc}
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
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Order Execution Tenant")
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	for _, row := range []struct {
		id uuid.UUID
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
		id uuid.UUID
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
	_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,'Driver A','ACTIVE'), ($5,$2,$6,$7,'Driver B','ACTIVE')`,
		fix.DriverA, fix.TenantID, fix.CarrierA, uuid.New(), fix.DriverB, fix.CarrierB, uuid.New())
	if err != nil {
		t.Fatalf("drivers: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.vehicles (id, tenant_id, carrier_company_id, plate_number, vehicle_type, status)
		VALUES ($1,$2,$3,'A123BC77','TRUCK','ACTIVE')`, fix.VehicleA, fix.TenantID, fix.CarrierA)
	if err != nil {
		t.Fatalf("vehicle: %v", err)
	}
	return fix
}

func transition(userID uuid.UUID) domain.StatusTransitionContext {
	return domain.NewUserTransitionContext(userID, nil, time.Now().UTC())
}

func TestExecuteCreatesShipmentAndIsIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	first, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EXEC-1",
	}, transition(fix.UserID))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if first == nil || !first.Created || first.Shipment == nil {
		t.Fatalf("execute result invalid: created=%v shipment=%v", first != nil && first.Created, first != nil && first.Shipment != nil)
	}
	if first.Shipment.Status != domain.ShipmentStatusAcceptedByCarrier {
		t.Fatalf("expected implicit accept, got %s", first.Shipment.Status)
	}

	second, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EXEC-1",
	}, transition(fix.UserID))
	if err != nil || second.Created {
		t.Fatalf("idempotent retry: err=%v created=%v", err, second.Created)
	}
	if second.Shipment.ID != first.Shipment.ID {
		t.Fatalf("shipment id changed on retry")
	}
	var count int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.shipments WHERE tenant_id=$1 AND transport_order_id=$2`,
		fix.TenantID, fix.OrderID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 shipment, got %d", count)
	}
}

func TestCompetitorCarrierExecuteDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	_, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierB, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EXEC-2",
	}, transition(fix.UserID))
	var appErr *apperrors.AppError
	if err == nil || !errorsAs(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDriverAssignmentIsolation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	result, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EXEC-3",
	}, transition(fix.UserID))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := env.shipmentSvc.AssignDriver(ctx, fix.TenantID, result.Shipment.ID, fix.DriverA, transition(fix.UserID)); err != nil {
		t.Fatalf("own driver assign: %v", err)
	}
	if _, err := env.shipmentSvc.AssignDriver(ctx, fix.TenantID, result.Shipment.ID, fix.DriverB, transition(fix.UserID)); err == nil {
		t.Fatal("competitor driver assignment must be denied")
	}
}

func TestStartExecutionWithoutAssignmentsDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	if _, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EXEC-4",
	}, transition(fix.UserID)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	_, err := env.orderExecution.StartExecution(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, transition(fix.UserID))
	var appErr *apperrors.AppError
	if err == nil || !errorsAs(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func errorsAs(err error, target **apperrors.AppError) bool {
	for wrapped := err; wrapped != nil; {
		if e, ok := wrapped.(*apperrors.AppError); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := wrapped.(unwrapper)
		if !ok {
			break
		}
		wrapped = u.Unwrap()
	}
	return false
}
