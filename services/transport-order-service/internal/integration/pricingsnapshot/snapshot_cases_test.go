//go:build integration

package pricingsnapshot

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	todomain "github.com/freight-platform/transport-order-service/internal/domain"
)

func TestCSnap001ContractRateAvailable(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "snap-contract")
	snap := sampleContractSnapshot(tenantID, buyerID, carrierID, originID, destID)
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, testRequestHash("contract"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.RateSnapshot.PricingSource != "CONTRACT_RATE" {
		t.Fatalf("expected CONTRACT_RATE, got %s", result.RateSnapshot.PricingSource)
	}
	if result.RateSnapshot.ComponentBreakdownStatus != "AVAILABLE" {
		t.Fatalf("expected AVAILABLE breakdown, got %s", result.RateSnapshot.ComponentBreakdownStatus)
	}
	if result.RateSnapshot.BaseAmount == nil {
		t.Fatal("expected base_amount for AVAILABLE contract snapshot")
	}
	if !result.RateSnapshot.TotalAmount.Equal(decimal.RequireFromString("2500.00")) {
		t.Fatalf("unexpected total: %s", result.RateSnapshot.TotalAmount.StringFixed(2))
	}
	if result.RateSnapshot.ContractID == nil || result.RateSnapshot.RateLineID == nil {
		t.Fatal("expected contract provenance fields")
	}
}

func TestCSnap002RFQUnavailableBreakdown(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "snap-rfq")
	snap := sampleRFQUnavailableSnapshot(tenantID, buyerID, carrierID, originID, destID)
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, testRequestHash("rfq"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.RateSnapshot.PricingSource != "RFQ_AWARD" {
		t.Fatalf("expected RFQ_AWARD, got %s", result.RateSnapshot.PricingSource)
	}
	if result.RateSnapshot.ComponentBreakdownStatus != "UNAVAILABLE" {
		t.Fatalf("expected UNAVAILABLE breakdown, got %s", result.RateSnapshot.ComponentBreakdownStatus)
	}
	if result.RateSnapshot.BaseAmount != nil {
		t.Fatal("expected NULL base_amount for aggregate RFQ snapshot")
	}
	if result.RateSnapshot.RfxEventID == nil {
		t.Fatal("expected rfx_event_id provenance")
	}
}

func TestCSnap003SpotBidUnavailable(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "snap-bid")
	snap := sampleSpotBidUnavailableSnapshot(tenantID, buyerID, carrierID, originID, destID)
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, testRequestHash("bid"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.RateSnapshot.PricingSource != "SPOT_BID" {
		t.Fatalf("expected SPOT_BID, got %s", result.RateSnapshot.PricingSource)
	}
	if result.RateSnapshot.ComponentBreakdownStatus != "UNAVAILABLE" {
		t.Fatalf("expected UNAVAILABLE breakdown, got %s", result.RateSnapshot.ComponentBreakdownStatus)
	}
	if result.RateSnapshot.BidID == nil {
		t.Fatal("expected bid_id provenance")
	}
}

func TestCSnap004ManualProvenance(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "snap-manual")
	snap := sampleManualSnapshot(tenantID, buyerID, carrierID, originID, destID)
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, testRequestHash("manual"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.RateSnapshot.ManualSpotAuditID == nil {
		t.Fatal("expected manual_spot_audit_id on MANUAL_SPOT snapshot")
	}

	orderID := insertBareTransportOrder(t, env.pool, tenantID, buyerID, originID, destID, cargoID, "TO-MANUAL-PROV")
	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'MANUAL_SPOT',$5,$6,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',1500,CURRENT_DATE,now(),'test','v2.0C',$7)`,
		tenantID, orderID, buyerID, carrierID, originID, destID, strings.Repeat("m", 64))
	if err == nil {
		t.Fatal("expected manual provenance check failure without manual_spot_audit_id")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "chk_snapshot_manual_provenance") {
		t.Fatalf("expected manual provenance constraint, got %v", err)
	}
}

func TestCSnap005TotalRequired(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	orderID := insertBareTransportOrder(t, env.pool, tenantID, buyerID, originID, destID, cargoID, "TO-NO-TOTAL")
	eventID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, rfx_event_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',NULL,CURRENT_DATE,now(),'test','v2.0C',$8)`,
		tenantID, orderID, buyerID, carrierID, eventID, originID, destID, strings.Repeat("t", 64))
	if err == nil {
		t.Fatal("expected NOT NULL violation for missing total_amount")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "null") && !strings.Contains(strings.ToLower(err.Error()), "not-null") {
		t.Fatalf("expected total_amount required, got %v", err)
	}
}

func TestCSnap006UnavailableWithBaseDeny(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	orderID := insertBareTransportOrder(t, env.pool, tenantID, buyerID, originID, destID, cargoID, "TO-BAD-BASE")
	eventID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, rfx_event_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			base_amount, total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',1000.00,1500.00,CURRENT_DATE,now(),'test','v2.0C',$8)`,
		tenantID, orderID, buyerID, carrierID, eventID, originID, destID, strings.Repeat("b", 64))
	if err == nil {
		t.Fatal("expected UNAVAILABLE + base_amount invariant violation")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "chk_snapshot_unavailable_invariant") {
		t.Fatalf("expected unavailable invariant check, got %v", err)
	}
}

func TestCSnap010CrossTenantLookupDeny(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantA := seedSecondTenantCompanies(t, env.pool)
	tenantB := seedSecondTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantA.tenantID, tenantA.buyerID, tenantA.carrierID, tenantA.originID, tenantA.destID, tenantA.cargoID, "tenant-a")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantA.tenantID, tenantA.buyerID, tenantA.carrierID, tenantA.originID, tenantA.destID), testRequestHash("tenant-a"))
	if err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	_, err = env.pricedOrders.GetPricedResult(ctx, tenantB.tenantID, result.Order.ID, result.RateSnapshot.ID)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Fatalf("expected cross-tenant lookup denial, got %v", err)
	}
}

func TestCSnap011CrossTenantParentFKDeny(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantA := seedSecondTenantCompanies(t, env.pool)
	tenantB := seedSecondTenantCompanies(t, env.pool)
	orderB := insertBareTransportOrder(t, env.pool, tenantB.tenantID, tenantB.buyerID, tenantB.originID, tenantB.destID, tenantB.cargoID, "TO-TENANT-B")
	eventID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, rfx_event_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',1500,CURRENT_DATE,now(),'test','v2.0C',$8)`,
		tenantA.tenantID, orderB, tenantA.buyerID, tenantA.carrierID, eventID, tenantA.originID, tenantA.destID, strings.Repeat("x", 64))
	if err == nil {
		t.Fatal("expected cross-tenant parent FK denial")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "fk_transport_order_rate_snapshots_order_tenant") &&
		!strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("expected tenant-scoped FK violation, got %v", err)
	}
}

func TestCSnap012ContractChangeLeavesSnapshotUnchanged(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	snap := sampleContractSnapshot(tenantID, buyerID, carrierID, originID, destID)
	contractID := *snap.ContractID
	_, err := env.pool.Exec(ctx, `
		INSERT INTO contract_rate.transport_contract (
			id, tenant_id, buyer_company_id, carrier_company_id,
			contract_number, name, status, valid_from, currency_code
		) VALUES ($1,$2,$3,$4,$5,$6,'ACTIVE',CURRENT_DATE,'RUB')`,
		contractID, tenantID, buyerID, carrierID, "CTR-001", "Lane Contract")
	if err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "snap-immutable-contract")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, testRequestHash("immutable"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	originalTotal := result.RateSnapshot.TotalAmount.StringFixed(2)
	originalContractNumber := todomain.DerefString(result.RateSnapshot.ContractNumber)

	if _, err := env.pool.Exec(ctx, `
		UPDATE contract_rate.transport_contract SET contract_number = $1 WHERE id = $2 AND tenant_id = $3`,
		"CTR-REVISED", contractID, tenantID); err != nil {
		t.Fatalf("update contract: %v", err)
	}

	reloaded, err := env.pricedOrders.GetPricedResult(ctx, tenantID, result.Order.ID, result.RateSnapshot.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.RateSnapshot.TotalAmount.StringFixed(2) != originalTotal {
		t.Fatalf("snapshot total changed after contract update: got %s want %s",
			reloaded.RateSnapshot.TotalAmount.StringFixed(2), originalTotal)
	}
	if todomain.DerefString(reloaded.RateSnapshot.ContractNumber) != originalContractNumber {
		t.Fatalf("snapshot contract_number changed after contract update: got %q want %q",
			todomain.DerefString(reloaded.RateSnapshot.ContractNumber), originalContractNumber)
	}
}
