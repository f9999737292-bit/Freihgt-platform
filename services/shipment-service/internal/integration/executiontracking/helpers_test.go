//go:build integration

package executiontracking

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type startedShipment struct {
	fix        fixture
	shipmentID uuid.UUID
	orderID    uuid.UUID
}

func prepareStartedShipment(t *testing.T, env *env, fix fixture, shipmentNumber string) startedShipment {
	t.Helper()
	ctx := context.Background()

	execResult, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: shipmentNumber,
	}, transition(fix.UserID))
	if err != nil || execResult.Shipment == nil {
		t.Fatalf("execute: %v", err)
	}
	shipmentID := execResult.Shipment.ID

	shipmentRepo := repository.NewShipmentRepository(env.pool)
	shipmentSvc := service.NewShipmentService(shipmentRepo, repository.NewDriverRepository(env.pool), repository.NewVehicleRepository(env.pool))
	if _, err := shipmentSvc.AssignDriver(ctx, fix.TenantID, shipmentID, fix.DriverA, transition(fix.UserID)); err != nil {
		t.Fatalf("assign driver: %v", err)
	}
	if _, err := shipmentSvc.AssignVehicle(ctx, fix.TenantID, shipmentID, fix.VehicleA, transition(fix.UserID)); err != nil {
		t.Fatalf("assign vehicle: %v", err)
	}
	if _, err := env.orderExecution.StartExecution(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, transition(fix.UserID)); err != nil {
		t.Fatalf("start execution: %v", err)
	}
	return startedShipment{fix: fix, shipmentID: shipmentID, orderID: fix.OrderID}
}

func recordMilestone(
	t *testing.T,
	env *env,
	fix fixture,
	shipmentID uuid.UUID,
	eventType, idempotencyKey string,
) service.DriverOperationalEventResult {
	t.Helper()
	result, err := env.driverOps.RecordOperationalEvent(context.Background(), fix.TenantID, fix.UserID, shipmentID, domain.DriverOperationalEventInput{
		Type:           eventType,
		IdempotencyKey: idempotencyKey,
	}, transition(fix.UserID))
	if err != nil {
		t.Fatalf("milestone %s: %v", eventType, err)
	}
	return result
}

func advanceToUnloading(t *testing.T, env *env, fix fixture, shipmentID uuid.UUID) {
	t.Helper()
	for _, step := range []struct {
		eventType string
		key       string
	}{
		{"PICKUP_COMPLETED", "pickup-complete"},
		{"DEPARTED_PICKUP", "depart-pickup"},
		{"ARRIVED_AT_DELIVERY", "arrive-delivery"},
		{"UNLOADING_STARTED", "unloading-started"},
	} {
		recordMilestone(t, env, fix, shipmentID, step.eventType, step.key)
	}
}

func seedPODDocument(t *testing.T, pool *pgxpool.Pool, tenantID, shipmentID uuid.UUID, docNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	docID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, status,
			related_entity_type, related_entity_id, created_at, updated_at
		) VALUES ($1,$2,$3,'POD','COMPLETED','SHIPMENT',$4,now(),now())`,
		docID, tenantID, docNumber, shipmentID)
	if err != nil {
		t.Fatalf("seed pod document: %v", err)
	}
	return docID
}

func scanActualTimes(t *testing.T, pool *pgxpool.Pool, shipmentID uuid.UUID) (pickup, delivery *time.Time) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `
		SELECT actual_pickup_at, actual_delivery_at FROM transport.shipments WHERE id=$1`, shipmentID).
		Scan(&pickup, &delivery)
	if err != nil {
		t.Fatalf("scan actual times: %v", err)
	}
	return pickup, delivery
}

func countStatusHistory(t *testing.T, pool *pgxpool.Pool, shipmentID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id=$1`, shipmentID).Scan(&count)
	if err != nil {
		t.Fatalf("count history: %v", err)
	}
	return count
}

func countOutboxRows(t *testing.T, pool *pgxpool.Pool, shipmentID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id=$1`, shipmentID).Scan(&count)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return count
}
