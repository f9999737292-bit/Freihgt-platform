//go:build integration

package executiontracking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func Test01DriverOwnMilestoneAllow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	started := prepareStartedShipment(t, env, fix, "SHP-EG-01")
	result := recordMilestone(t, env, fix, started.shipmentID, "PICKUP_COMPLETED", "eg-01-pickup")
	if result.ShipmentStatus != domain.ShipmentStatusLoaded {
		t.Fatalf("status=%s want LOADED", result.ShipmentStatus)
	}
}

func Test02ForeignDriverMilestoneDeny(t *testing.T) {
	TestForeignDriverMilestoneDenied(t)
}

func Test03DuplicateMilestoneRetrySafe(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	started := prepareStartedShipment(t, env, fix, "SHP-EG-03")
	recordMilestone(t, env, fix, started.shipmentID, "PICKUP_COMPLETED", "eg-03-pickup")
	pickupBefore, _ := scanActualTimes(t, env.pool, started.shipmentID)
	historyBefore := countStatusHistory(t, env.pool, started.shipmentID)
	outboxBefore := countOutboxRows(t, env.pool, started.shipmentID)

	replay, err := env.driverOps.RecordOperationalEvent(context.Background(), fix.TenantID, fix.UserID, started.shipmentID, domain.DriverOperationalEventInput{
		Type: "PICKUP_COMPLETED", IdempotencyKey: "eg-03-pickup",
	}, transition(fix.UserID))
	if err != nil || !replay.Replayed {
		t.Fatalf("replay: err=%v replayed=%v", err, replay.Replayed)
	}
	pickupAfter, _ := scanActualTimes(t, env.pool, started.shipmentID)
	if pickupBefore == nil || pickupAfter == nil || !pickupBefore.Equal(*pickupAfter) {
		t.Fatal("actual_pickup_at changed on idempotent replay")
	}
	if countStatusHistory(t, env.pool, started.shipmentID) != historyBefore {
		t.Fatal("status history grew on idempotent replay")
	}
	if countOutboxRows(t, env.pool, started.shipmentID) != outboxBefore {
		t.Fatal("outbox grew on idempotent replay")
	}
}

func Test04ActualPickupPersisted(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	started := prepareStartedShipment(t, env, fix, "SHP-EG-04")
	recordMilestone(t, env, fix, started.shipmentID, "PICKUP_COMPLETED", "eg-04-pickup")
	pickup, _ := scanActualTimes(t, env.pool, started.shipmentID)
	if pickup == nil {
		t.Fatal("actual_pickup_at not persisted")
	}
}

func Test05OutboxContainsActualPickup(t *testing.T) {
	TestDriverMilestonePickupToLoadedAndOutboxActuals(t)
}

func Test08BuyerTenantIsolation(t *testing.T) {
	TestBuyerOrderListIsolation(t)
}

func Test09BuyerSameTenantCompanyIsolation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	if _, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EG-09",
	}, transition(fix.UserID)); err != nil {
		t.Fatalf("execute: %v", err)
	}

	otherBuyer := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1,$2,'SHIPPER','Buyer B same tenant','ACTIVE')`, otherBuyer, fix.TenantID)
	if err != nil {
		t.Fatalf("other buyer: %v", err)
	}

	_, err = env.orderExecution.GetExecution(ctx, fix.TenantID, fix.OrderID, otherBuyer, domain.ExecutionActorBuyer)
	if err == nil {
		t.Fatal("buyer B should not read buyer A order")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func Test10CarrierExecutionIsolation(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()

	if _, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EG-10",
	}, transition(fix.UserID)); err != nil {
		t.Fatalf("execute: %v", err)
	}

	_, err := env.orderExecution.GetExecution(ctx, fix.TenantID, fix.OrderID, fix.CarrierB, domain.ExecutionActorCarrier)
	if err == nil {
		t.Fatal("carrier B should not read carrier A execution")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func Test13PODAppearsInExecutionDetail(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	started := prepareStartedShipment(t, env, fix, "SHP-EG-13")
	docID := seedPODDocument(t, env.pool, fix.TenantID, started.shipmentID, "POD-EG-13")

	view, err := env.orderExecution.GetExecution(ctx, fix.TenantID, started.orderID, fix.BuyerID, domain.ExecutionActorBuyer)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if len(view.PODDocuments) != 1 || view.PODDocuments[0].ID != docID {
		t.Fatalf("pod_documents=%v want doc %s", view.PODDocuments, docID)
	}
}

func Test14ForeignPODReadDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	started := prepareStartedShipment(t, env, fix, "SHP-EG-14")
	seedPODDocument(t, env.pool, fix.TenantID, started.shipmentID, "POD-EG-14")

	otherBuyer := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1,$2,'SHIPPER','Foreign buyer','ACTIVE')`, otherBuyer, fix.TenantID)
	if err != nil {
		t.Fatalf("other buyer: %v", err)
	}

	_, err = env.orderExecution.GetExecution(ctx, fix.TenantID, started.orderID, otherBuyer, domain.ExecutionActorBuyer)
	if err == nil {
		t.Fatal("foreign buyer must not read execution with POD")
	}
}

func Test15DeliveryConfirmationValid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	started := prepareStartedShipment(t, env, fix, "SHP-EG-15")
	advanceToUnloading(t, env, fix, started.shipmentID)
	result := recordMilestone(t, env, fix, started.shipmentID, "DELIVERY_COMPLETED", "eg-15-deliver")
	if result.ShipmentStatus != domain.ShipmentStatusDelivered {
		t.Fatalf("status=%s want DELIVERED", result.ShipmentStatus)
	}
	_, delivery := scanActualTimes(t, env.pool, started.shipmentID)
	if delivery == nil {
		t.Fatal("actual_delivery_at not persisted")
	}
}

func Test16InvalidDeliveryTransitionDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	started := prepareStartedShipment(t, env, fix, "SHP-EG-16")
	_, err := env.driverOps.RecordOperationalEvent(context.Background(), fix.TenantID, fix.UserID, started.shipmentID, domain.DriverOperationalEventInput{
		Type: "DELIVERY_COMPLETED", IdempotencyKey: "eg-16-invalid",
	}, transition(fix.UserID))
	if err == nil {
		t.Fatal("delivery from IN_PICKUP should be denied")
	}
}

func Test17DuplicateDeliveryIdempotent(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	started := prepareStartedShipment(t, env, fix, "SHP-EG-17")
	advanceToUnloading(t, env, fix, started.shipmentID)
	recordMilestone(t, env, fix, started.shipmentID, "DELIVERY_COMPLETED", "eg-17-deliver")
	_, deliveryBefore := scanActualTimes(t, env.pool, started.shipmentID)
	historyBefore := countStatusHistory(t, env.pool, started.shipmentID)

	replay, err := env.driverOps.RecordOperationalEvent(context.Background(), fix.TenantID, fix.UserID, started.shipmentID, domain.DriverOperationalEventInput{
		Type: "DELIVERY_COMPLETED", IdempotencyKey: "eg-17-deliver",
	}, transition(fix.UserID))
	if err != nil || !replay.Replayed {
		t.Fatalf("delivery replay: err=%v replayed=%v", err, replay.Replayed)
	}
	_, deliveryAfter := scanActualTimes(t, env.pool, started.shipmentID)
	if deliveryBefore == nil || deliveryAfter == nil || !deliveryBefore.Equal(*deliveryAfter) {
		t.Fatal("actual_delivery_at changed on idempotent replay")
	}
	if countStatusHistory(t, env.pool, started.shipmentID) != historyBefore {
		t.Fatal("duplicate delivery changed history")
	}
}

func Test18ActualDeliveryPersisted(t *testing.T) {
	Test15DeliveryConfirmationValid(t)
}

func Test20SLALateSignal(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	started := prepareStartedShipment(t, env, fix, "SHP-EG-20")

	past := time.Now().UTC().Add(-2 * time.Hour)
	_, err := env.pool.Exec(ctx, `UPDATE transport.shipments SET planned_pickup_at=$2 WHERE id=$1`, started.shipmentID, past)
	if err != nil {
		t.Fatalf("set planned pickup: %v", err)
	}

	view, err := env.orderExecution.GetExecution(ctx, fix.TenantID, started.orderID, fix.BuyerID, domain.ExecutionActorBuyer)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	found := false
	for _, signal := range view.SLASignals {
		if signal.Code == "PICKUP_LATE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected PICKUP_LATE signal, got %v", view.SLASignals)
	}
}

func TestSLAOnTimeNoPickupLate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	started := prepareStartedShipment(t, env, fix, "SHP-EG-SLA-OK")
	recordMilestone(t, env, fix, started.shipmentID, "PICKUP_COMPLETED", "eg-sla-ok")

	view, err := env.orderExecution.GetExecution(ctx, fix.TenantID, started.orderID, fix.BuyerID, domain.ExecutionActorBuyer)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	for _, signal := range view.SLASignals {
		if signal.Code == "PICKUP_LATE" {
			t.Fatal("on-time pickup should not emit PICKUP_LATE")
		}
	}
}

func TestCarrierOwnExecutionAllow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	started := prepareStartedShipment(t, env, fix, "SHP-EG-CARRIER-OK")

	if _, err := env.orderExecution.GetExecution(ctx, fix.TenantID, started.orderID, fix.CarrierA, domain.ExecutionActorCarrier); err != nil {
		t.Fatalf("carrier own execution: %v", err)
	}
}

func TestBuyerOwnOrderListAllow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	if _, err := env.orderExecution.ExecuteAwardOrder(ctx, fix.TenantID, fix.OrderID, fix.CarrierA, domain.ExecuteTransportOrderInput{
		ShipmentNumber: "SHP-EG-BUYER-LIST",
	}, transition(fix.UserID)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	items, total, err := env.orderExecution.ListBuyerTransportOrders(ctx, domain.ListBuyerTransportOrdersFilter{
		TenantID: fix.TenantID, BuyerCompanyID: fix.BuyerID, Limit: 20,
	})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("buyer list: err=%v total=%d len=%d", err, total, len(items))
	}
}

func TestTrackingNotStoredInShipmentService(t *testing.T) {
	ctx := context.Background()
	var exists bool
	err := envSetupPool(t).QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='transport' AND table_name='shipments'
			  AND column_name IN ('latitude','longitude','last_latitude','last_longitude')
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("schema check: %v", err)
	}
	if exists {
		t.Fatal("shipment table must not store live GPS coordinates")
	}
}

func envSetupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	env := setupEnv(t)
	return env.pool
}
