//go:build integration

package freightsettlement

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCSet009NewOrderMissingSnapshotFailClosed(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	var orderID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, equipment_type,
			status, pricing_model_version
		) VALUES ($1,'SNAP-MISSING',$2,$2,$3,$4,$5,'ROAD','TAUTLINER','CONVERTED_TO_SHIPMENT','SNAPSHOT_V1')
		RETURNING id`, fix.TenantID, fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID).Scan(&orderID); err != nil {
		t.Fatalf("order: %v", err)
	}
	fix.OrderID = orderID
	fix.ShipmentID = uuid.New()
	seedDeliveredShipment(t, env.pool, fix, "DELIVERED", true)
	fix.PODDocumentID = seedPODDocument(t, env.pool, fix.TenantID, fix.CarrierA, fix.ShipmentID, "POD-"+fix.ShipmentID.String()[:8])
	_, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, fix.ShipmentID)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "snapshot") {
		t.Fatalf("expected fail-closed missing snapshot, got %v", err)
	}
}

func TestCSet003AggregateNullBaseSettlementPass(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	var orderID, snapshotID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, equipment_type,
			status, pricing_model_version
		) VALUES ($1,'SNAP-AGG',$2,$2,$3,$4,$5,'ROAD','TAUTLINER','CONVERTED_TO_SHIPMENT','SNAPSHOT_V1')
		RETURNING id`, fix.TenantID, fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID).Scan(&orderID); err != nil {
		t.Fatalf("order: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, rfx_event_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',NULL,108000.00,CURRENT_DATE,now(),'test','v2.0C','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')
		RETURNING id`, fix.TenantID, orderID, fix.BuyerID, fix.CarrierA, eventID, fix.OriginID, fix.DestID).Scan(&snapshotID); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	_ = snapshotID
	fix.OrderID = orderID
	fix.ShipmentID = uuid.New()
	seedDeliveredShipment(t, env.pool, fix, "DELIVERED", true)
	fix.PODDocumentID = seedPODDocument(t, env.pool, fix.TenantID, fix.CarrierA, fix.ShipmentID, "POD-"+fix.ShipmentID.String()[:8])
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, fix.ShipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if ctxData.BaseAmount != 108000 {
		t.Fatalf("expected settlement principal 108000, got %v", ctxData.BaseAmount)
	}
}

func strPtr(v string) *string {
	return &v
}
