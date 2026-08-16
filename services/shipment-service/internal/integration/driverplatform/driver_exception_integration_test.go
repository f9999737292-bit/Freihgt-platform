//go:build integration

package driverplatform

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestMigration000030Schema(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	var tableCount int64
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema='transport' AND table_name IN ('driver_operation_idempotency','driver_reported_exception')
	`).Scan(&tableCount))
	require.Equal(t, int64(2), tableCount)

	var idxCount int64
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE schemaname='transport' AND indexname='uq_drivers_tenant_user_active'
	`).Scan(&idxCount))
	require.Equal(t, int64(1), idxCount)
}

func TestMigration000030DuplicateDriverUserBindingRejected(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)

	driverB := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,$5,'ACTIVE')`,
		driverB, fix.TenantID, fix.CarrierID, fix.UserID, "Driver B")
	require.Error(t, err, "duplicate tenant+user_id binding must be rejected")
}

func TestMigration000030DuplicateExceptionIdempotencyRejected(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)

	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.driver_reported_exception
		(tenant_id, shipment_id, driver_id, category, occurred_at, received_at, idempotency_key)
		VALUES ($1,$2,$3,'TRAFFIC',NOW(),NOW(),'dup-key')`,
		fix.TenantID, fix.ShipmentID, fix.DriverID)
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.driver_reported_exception
		(tenant_id, shipment_id, driver_id, category, occurred_at, received_at, idempotency_key)
		VALUES ($1,$2,$3,'TRAFFIC',NOW(),NOW(),'dup-key')`,
		fix.TenantID, fix.ShipmentID, fix.DriverID)
	require.Error(t, err)
}

func TestConcurrentExceptionIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)
	corr := "req-concurrent-1"
	key := "idem-concurrent-1"

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = env.driverOps.ReportException(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverExceptionInput{
				Category:       "VEHICLE_BREAKDOWN",
				IdempotencyKey: key,
			}, &corr)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	excCount := countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM transport.driver_reported_exception
		WHERE tenant_id=$1 AND driver_id=$2 AND idempotency_key=$3`,
		fix.TenantID, fix.DriverID, key)
	require.Equal(t, int64(1), excCount)

	outboxCount := countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM transport.shipment_event_outbox
		WHERE tenant_id=$1 AND event_type='driver.problem.reported' AND aggregate_id=$2`,
		fix.TenantID, fix.ShipmentID)
	require.Equal(t, int64(1), outboxCount)
}

func TestSameTenantWrongDriverDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)

	userB := uuid.New()
	driverB := uuid.New()
	shipmentB := uuid.New()
	fixB := seedSecondDriverShipment(t, env.pool, fix.TenantID, fix.CarrierID, driverB, userB, shipmentB)

	_, err := env.driverOps.ReportException(ctx, fix.TenantID, fix.UserID, fixB.ShipmentID, domain.DriverExceptionInput{
		Category:       "TRAFFIC",
		IdempotencyKey: "wrong-driver-key",
	}, nil)
	require.Error(t, err)

	excCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_reported_exception WHERE shipment_id=$1`, fixB.ShipmentID)
	require.Equal(t, int64(0), excCount)
	outboxCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1`, fixB.ShipmentID)
	require.Equal(t, int64(0), outboxCount)
}

func seedSecondDriverShipment(t *testing.T, pool *pgxpool.Pool, tenantID, carrierID, driverID, userID, shipmentID uuid.UUID) driverFixture {
	t.Helper()
	ctx := context.Background()
	shipperID := uuid.New()
	consigneeID := uuid.New()
	originID := uuid.New()
	destID := uuid.New()
	orderID := uuid.New()
	for _, row := range []struct {
		id uuid.UUID
		typ, name string
	}{
		{shipperID, "SHIPPER", "Shipper B"},
		{consigneeID, "CONSIGNEE", "Consignee B"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, tenantID, row.name, row.typ)
		require.NoError(t, err)
	}
	for _, loc := range []struct {
		id uuid.UUID
		name string
	}{
		{originID, "Origin B"},
		{destID, "Dest B"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, tenantID, loc.name)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
		VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')`,
		orderID, tenantID, "TO-B", shipperID, consigneeID, originID, destID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, user_id, full_name, status)
		VALUES ($1,$2,$3,$4,$5,'ACTIVE')`,
		driverID, tenantID, carrierID, userID, "Driver B")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.shipments (id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, driver_id, origin_location_id, destination_location_id, transport_mode, status, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ROAD','PICKUP_SLOT_BOOKED',1)`,
		shipmentID, tenantID, "SHP-B", orderID, shipperID, consigneeID, carrierID, driverID, originID, destID)
	require.NoError(t, err)
	return driverFixture{TenantID: tenantID, UserID: userID, DriverID: driverID, CarrierID: carrierID, ShipmentID: shipmentID}
}

func TestStatusEventIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)
	transition := userTransition(fix.UserID)
	key := "status-idem-1"

	_, err := env.driverOps.RecordOperationalEvent(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverOperationalEventInput{
		Type:           "ARRIVED_AT_PICKUP",
		IdempotencyKey: key,
	}, transition)
	require.NoError(t, err)

	_, err = env.driverOps.RecordOperationalEvent(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverOperationalEventInput{
		Type:           "ARRIVED_AT_PICKUP",
		IdempotencyKey: key,
	}, transition)
	require.NoError(t, err)

	idemCount := countRows(ctx, env.pool, `
		SELECT COUNT(*) FROM transport.driver_operation_idempotency
		WHERE tenant_id=$1 AND driver_id=$2 AND operation_type='status_event' AND idempotency_key=$3`,
		fix.TenantID, fix.DriverID, key)
	require.Equal(t, int64(1), idemCount)
}
