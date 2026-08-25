//go:build integration

package systemwave2

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

// TestSYSTEM_WAVE2_GOLDEN_FLOW continues procurement chain:
// accepted bid → shipment → driver/vehicle → FSM → outbox with lineage preservation.
func TestSYSTEM_WAVE2_GOLDEN_FLOW(t *testing.T) {
	env := setupEnv(t)
	fix := seedWave2Fixture(t, env.pool)
	ctx := context.Background()
	tr := transition(fix.UserID)

	shipment, err := env.shipmentSvc.CreateFromBid(ctx, fix.TenantA, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SH-W2-GOLDEN", BidID: fix.BidID, TransportOrderID: fix.OrderID,
	}, tr)
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}
	if shipment.TenantID != fix.TenantA || shipment.CarrierCompanyID == nil || *shipment.CarrierCompanyID != fix.CarrierA1 {
		t.Fatalf("shipment ownership: tenant=%s carrier=%v", shipment.TenantID, shipment.CarrierCompanyID)
	}
	if shipment.Status != domain.ShipmentStatusCarrierAssigned {
		t.Fatalf("initial status=%s want CARRIER_ASSIGNED", shipment.Status)
	}

	if _, err := env.shipmentSvc.AssignDriver(ctx, fix.TenantA, shipment.ID, fix.DriverA1, tr); err != nil {
		t.Fatalf("assign driver A1: %v", err)
	}
	if _, err := env.shipmentSvc.AssignVehicle(ctx, fix.TenantA, shipment.ID, fix.VehicleA1, tr); err != nil {
		t.Fatalf("assign vehicle A1: %v", err)
	}

	legalStates := []string{
		domain.ShipmentStatusPickupSlotBooked,
		domain.ShipmentStatusInPickup,
		domain.ShipmentStatusLoaded,
		domain.ShipmentStatusInTransit,
		domain.ShipmentStatusArrivedAtConsignee,
		domain.ShipmentStatusUnloading,
		domain.ShipmentStatusDelivered,
		domain.ShipmentStatusDeliveryConfirmed,
		domain.ShipmentStatusDocumentsCompleted,
		domain.ShipmentStatusReadyForBilling,
	}
	current := shipment
	for _, st := range legalStates {
		var actualTime *time.Time
		if st == domain.ShipmentStatusLoaded || st == domain.ShipmentStatusDelivered {
			now := time.Now().UTC()
			actualTime = &now
		}
		current, err = env.shipmentSvc.UpdateStatus(ctx, fix.TenantA, current.ID, domain.UpdateShipmentStatusInput{
			Status: st, ActualTime: actualTime,
		}, tr)
		if err != nil {
			t.Fatalf("transition to %s: %v", st, err)
		}
		if current.Status != st {
			t.Fatalf("persisted status=%s want %s", current.Status, st)
		}
	}

	var outboxCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE tenant_id=$1 AND shipment_id=$2`,
		fix.TenantA, shipment.ID).Scan(&outboxCount); err != nil {
		t.Fatalf("outbox count: %v", err)
	}
	if outboxCount == 0 {
		t.Fatal("expected shipment outbox events after FSM progression")
	}

	var bidAmount float64
	var bidCurrency string
	if err := env.pool.QueryRow(ctx, `SELECT total_amount, currency_code FROM rfx.bids WHERE id=$1`, fix.BidID).Scan(&bidAmount, &bidCurrency); err != nil {
		t.Fatalf("bid amount: %v", err)
	}
	if bidAmount != 100000 || bidCurrency != "RUB" {
		t.Fatalf("commercial lineage broken: amount=%v currency=%s", bidAmount, bidCurrency)
	}

	t.Logf("LINEAGE freight_request=%s bid=%s transport_order=%s shipment=%s outbox_events=%d",
		fix.FreightReqID, fix.BidID, fix.OrderID, shipment.ID, outboxCount)
}

func TestW2_ForeignDriverAssignmentDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedWave2Fixture(t, env.pool)
	ctx := context.Background()
	tr := transition(fix.UserID)

	shipment, err := env.shipmentSvc.CreateFromBid(ctx, fix.TenantA, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SH-W2-DRV", BidID: fix.BidID, TransportOrderID: fix.OrderID,
	}, tr)
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}
	_, err = env.shipmentSvc.AssignDriver(ctx, fix.TenantA, shipment.ID, fix.DriverB1, tr)
	if err == nil {
		t.Fatal("tenant B driver must not assign to tenant A shipment")
	}
}

func TestW2_ForeignVehicleAssignmentDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedWave2Fixture(t, env.pool)
	ctx := context.Background()
	tr := transition(fix.UserID)

	shipment, _ := env.shipmentSvc.CreateFromBid(ctx, fix.TenantA, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SH-W2-VEH", BidID: fix.BidID, TransportOrderID: fix.OrderID,
	}, tr)
	_, err := env.shipmentSvc.AssignVehicle(ctx, fix.TenantA, shipment.ID, fix.VehicleB1, tr)
	if err == nil {
		t.Fatal("tenant B vehicle must not assign to tenant A shipment")
	}
}

func TestW2_InvalidFSMTransitionFailClosed(t *testing.T) {
	env := setupEnv(t)
	fix := seedWave2Fixture(t, env.pool)
	ctx := context.Background()
	tr := transition(fix.UserID)

	shipment, _ := env.shipmentSvc.CreateFromBid(ctx, fix.TenantA, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SH-W2-FSM", BidID: fix.BidID, TransportOrderID: fix.OrderID,
	}, tr)
	before := shipment.Status

	_, err := env.shipmentSvc.UpdateStatus(ctx, fix.TenantA, shipment.ID, domain.UpdateShipmentStatusInput{
		Status: domain.ShipmentStatusInTransit,
	}, tr)
	if err == nil {
		t.Fatal("skip-state transition must fail")
	}
	after, _ := env.shipmentSvc.GetByIDAndTenant(ctx, fix.TenantA, shipment.ID)
	if after.Status != before {
		t.Fatalf("state changed on invalid transition: before=%s after=%s", before, after.Status)
	}
}

func TestW2_ShipmentReadyForBillingEnablesSettlementContext(t *testing.T) {
	env := setupEnv(t)
	fix := seedWave2Fixture(t, env.pool)
	shipment := advanceToReadyForBilling(t, env, fix)

	var status string
	if err := env.pool.QueryRow(context.Background(), `SELECT status FROM transport.shipments WHERE id=$1`, shipment.ID).Scan(&status); err != nil {
		t.Fatalf("load shipment: %v", err)
	}
	if status != domain.ShipmentStatusReadyForBilling {
		t.Fatalf("status=%s want READY_FOR_BILLING", status)
	}
}

func TestW2_DataIntegrityOrphanCheck(t *testing.T) {
	env := setupEnv(t)
	_ = seedWave2Fixture(t, env.pool)
	ctx := context.Background()

	queries := []struct {
		name  string
		query string
	}{
		{"bid_without_tender", `SELECT COUNT(*) FROM rfx.bids b LEFT JOIN rfx.freight_requests fr ON fr.id=b.freight_request_id WHERE fr.id IS NULL`},
		{"shipment_without_order", `SELECT COUNT(*) FROM transport.shipments s LEFT JOIN transport.transport_orders o ON o.id=s.transport_order_id WHERE o.id IS NULL`},
		{"settlement_without_shipment", `SELECT COUNT(*) FROM billing.freight_settlements fs LEFT JOIN transport.shipments s ON s.id=fs.shipment_id WHERE s.id IS NULL AND fs.deleted_at IS NULL`},
	}
	for _, q := range queries {
		var count int
		if err := env.pool.QueryRow(ctx, q.query).Scan(&count); err != nil {
			t.Fatalf("%s: %v", q.name, err)
		}
		if count > 0 {
			t.Fatalf("orphan records detected for %s: count=%d", q.name, count)
		}
	}
}

func TestW2_CrossServiceTenantLineage(t *testing.T) {
	env := setupEnv(t)
	fix := seedWave2Fixture(t, env.pool)
	ctx := context.Background()
	tr := transition(fix.UserID)

	shipment, _ := env.shipmentSvc.CreateFromBid(ctx, fix.TenantA, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SH-W2-TEN", BidID: fix.BidID, TransportOrderID: fix.OrderID,
	}, tr)

	var orderTenant, bidTenant uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT tenant_id FROM transport.transport_orders WHERE id=$1`, fix.OrderID).Scan(&orderTenant); err != nil {
		t.Fatalf("order tenant: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT tenant_id FROM rfx.bids WHERE id=$1`, fix.BidID).Scan(&bidTenant); err != nil {
		t.Fatalf("bid tenant: %v", err)
	}
	if orderTenant != fix.TenantA || bidTenant != fix.TenantA || shipment.TenantID != fix.TenantA {
		t.Fatalf("tenant lineage mismatch order=%s bid=%s shipment=%s", orderTenant, bidTenant, shipment.TenantID)
	}
}

var _ = uuid.Nil
