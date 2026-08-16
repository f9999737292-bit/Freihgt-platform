//go:build integration

package driverplatform

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestCrossTenantDelayReportDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)

	tenantB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`, tenantB, "tb", "Tenant B")
	require.NoError(t, err)

	shipmentB := uuid.New()
	carrierB := uuid.New()
	driverB := uuid.New()
	userB := uuid.New()
	seedTenantBShipment(t, env.pool, tenantB, carrierB, driverB, userB, shipmentB)

	_, err = env.driverOps.ReportDelay(ctx, fix.TenantID, fix.UserID, shipmentB, domain.DriverDelayInput{
		ReasonCode: "TRAFFIC", IdempotencyKey: "cross-tenant-delay",
	}, nil)
	require.Error(t, err)

	delayCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_reported_delay WHERE shipment_id=$1`, shipmentB)
	require.Equal(t, int64(0), delayCount)
	outboxCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1 AND event_type='driver.delay.reported'`, shipmentB)
	require.Equal(t, int64(0), outboxCount)
}

func TestSameTenantWrongDriverDelayDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)

	userB := uuid.New()
	driverB := uuid.New()
	shipmentB := uuid.New()
	seedSecondDriverShipment(t, env.pool, fix.TenantID, fix.CarrierID, driverB, userB, shipmentB)

	_, err := env.driverOps.ReportDelay(ctx, fix.TenantID, fix.UserID, shipmentB, domain.DriverDelayInput{
		ReasonCode: "TRAFFIC", IdempotencyKey: "wrong-driver-delay",
	}, nil)
	require.Error(t, err)

	delayCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_reported_delay WHERE shipment_id=$1`, shipmentB)
	require.Equal(t, int64(0), delayCount)
}

func TestDelayIdempotencySingleOutboxEvent(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedDriverFixture(t, env.pool)
	key := "delay-idem-1"

	_, err := env.driverOps.ReportDelay(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverDelayInput{
		ReasonCode: "TRAFFIC", IdempotencyKey: key,
	}, nil)
	require.NoError(t, err)
	_, err = env.driverOps.ReportDelay(ctx, fix.TenantID, fix.UserID, fix.ShipmentID, domain.DriverDelayInput{
		ReasonCode: "TRAFFIC", IdempotencyKey: key,
	}, nil)
	require.NoError(t, err)

	delayCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.driver_reported_delay WHERE tenant_id=$1 AND idempotency_key=$2`, fix.TenantID, key)
	require.Equal(t, int64(1), delayCount)
	outboxCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE tenant_id=$1 AND event_type='driver.delay.reported' AND aggregate_id=$2`, fix.TenantID, fix.ShipmentID)
	require.Equal(t, int64(1), outboxCount)
}

func seedTenantBShipment(t *testing.T, pool *pgxpool.Pool, tenantID, carrierID, driverID, userID, shipmentID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	shipperID := uuid.New()
	consigneeID := uuid.New()
	originID := uuid.New()
	destID := uuid.New()
	orderID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		carrierID, tenantID, "Carrier B", "CARRIER")
	require.NoError(t, err)
	for _, row := range []struct {
		id uuid.UUID
		typ, name string
	}{
		{shipperID, "SHIPPER", "Shipper B"}, {consigneeID, "CONSIGNEE", "Consignee B"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
			row.id, tenantID, row.name, row.typ)
		require.NoError(t, err)
	}
	for _, loc := range []struct {
		id uuid.UUID
		name string
	}{
		{originID, "Origin B"}, {destID, "Dest B"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			loc.id, tenantID, loc.name)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `
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
}
