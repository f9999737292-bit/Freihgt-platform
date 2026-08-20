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
	var orderID, shipmentID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, equipment_type,
			status, pricing_model_version
		) VALUES ($1,'SNAP-MISSING',$2,$2,$3,$4,$5,'ROAD','TAUTLINER','CONVERTED_TO_SHIPMENT','SNAPSHOT_V1')
		RETURNING id`, fix.TenantID, fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID).Scan(&orderID); err != nil {
		t.Fatalf("order: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.shipments (tenant_id, transport_order_id, carrier_company_id, status)
		VALUES ($1,$2,$3,'DELIVERED') RETURNING id`, fix.TenantID, orderID, fix.CarrierA).Scan(&shipmentID); err != nil {
		t.Fatalf("shipment: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO documents.documents (tenant_id, related_entity_type, related_entity_id, document_type, status)
		VALUES ($1,'SHIPMENT',$2,'POD','ACTIVE')`, fix.TenantID, shipmentID); err != nil {
		t.Fatalf("pod: %v", err)
	}
	_, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "snapshot") {
		t.Fatalf("expected fail-closed missing snapshot, got %v", err)
	}
}

func TestCSet003AggregateNullBaseSettlementPass(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	var orderID, snapshotID, shipmentID uuid.UUID
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
			pricing_source, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',NULL,108000.00,CURRENT_DATE,now(),'test','v2.0C','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')
		RETURNING id`, fix.TenantID, orderID, fix.BuyerID, fix.CarrierA, fix.OriginID, fix.DestID).Scan(&snapshotID); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	_ = snapshotID
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.shipments (tenant_id, transport_order_id, carrier_company_id, status)
		VALUES ($1,$2,$3,'DELIVERED') RETURNING id`, fix.TenantID, orderID, fix.CarrierA).Scan(&shipmentID); err != nil {
		t.Fatalf("shipment: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO documents.documents (tenant_id, related_entity_type, related_entity_id, document_type, status)
		VALUES ($1,'SHIPMENT',$2,'POD','ACTIVE')`, fix.TenantID, shipmentID); err != nil {
		t.Fatalf("pod: %v", err)
	}
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if ctxData.BaseAmount != 108000 {
		t.Fatalf("expected settlement principal 108000, got %v", ctxData.BaseAmount)
	}
}
