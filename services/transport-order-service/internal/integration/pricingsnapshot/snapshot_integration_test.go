//go:build integration

package pricingsnapshot

import (
	"context"
	"strings"
	"testing"

	todomain "github.com/freight-platform/transport-order-service/internal/domain"
)

func TestCSnap007SameTOSecondSnapshotDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "key-1")
	snap := sampleSnapshot(tenantID, buyerID, carrierID, originID, destID)
	first, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, "hash-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',2000,CURRENT_DATE,now(),'test','v2.0C',$7)`,
		tenantID, first.Order.ID, buyerID, carrierID, originID, destID, strings.Repeat("b", 64))
	if err == nil {
		t.Fatal("expected unique constraint violation for second snapshot on same TO")
	}
	if !strings.Contains(err.Error(), "uq_transport_order_rate_snapshot") && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("expected uniqueness violation, got %v", err)
	}
}

func TestCSnap008SnapshotUpdateDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "key-update")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), "hash-update")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.pool.Exec(ctx, `UPDATE transport.transport_order_rate_snapshots SET total_amount = 9999 WHERE id = $1`, result.RateSnapshot.ID)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable trigger denial, got %v", err)
	}
}

func TestCSnap009SnapshotDeleteDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "key-delete")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), "hash-delete")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.pool.Exec(ctx, `DELETE FROM transport.transport_order_rate_snapshots WHERE id = $1`, result.RateSnapshot.ID)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable trigger denial, got %v", err)
	}
}

func TestCTo006SameIdempotencyKeySameRequest(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "same-key")
	hash, err := todomain.ComputeCreateRequestHash(in)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	first, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), hash)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), hash)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Order.ID != second.Order.ID || first.RateSnapshot.ID != second.RateSnapshot.ID {
		t.Fatalf("idempotent retry returned different commercial result")
	}
}

func TestCTo007SameKeyDifferentRequestConflict(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "conflict-key")
	hash, err := todomain.ComputeCreateRequestHash(in)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), hash); err != nil {
		t.Fatalf("first: %v", err)
	}
	in.OrderNumber = "TO-DIFFERENT"
	hash2, err := todomain.ComputeCreateRequestHash(in)
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	_, err = env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), hash2)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "idempotency") {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func TestCTo010PricedDraftEquipmentUpdateDenied(t *testing.T) {
	order := &todomain.TransportOrder{PricingModelVersion: ptr("SNAPSHOT_V1")}
	newEquip := "REFRIGERATED"
	err := todomain.ValidateUpdateWithSnapshot(order, todomain.UpdateTransportOrderInput{EquipmentType: &newEquip})
	if err == nil {
		t.Fatal("expected equipment update denial")
	}
}

func ptr(v string) *string { return &v }
