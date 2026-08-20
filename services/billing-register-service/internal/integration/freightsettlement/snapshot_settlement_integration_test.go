//go:build integration

package freightsettlement

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

func TestCSet001ContractRateTotalConsumed(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	contractID := uuid.New()
	rateCardID := uuid.New()
	rateVersionID := uuid.New()
	rateLineID := uuid.New()
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "CONTRACT_RATE",
		TotalAmount:   "87500.50",
		ContractID:    &contractID,
		RateCardID:    &rateCardID,
		RateVersionID: &rateVersionID,
		RateLineID:    &rateLineID,
	})
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if !ctxData.AgreedFreightAmount.Equal(decimal.RequireFromString("87500.50")) {
		t.Fatalf("expected contract total 87500.50, got %s", ctxData.AgreedFreightAmount.StringFixed(2))
	}
	settlement := createSettlement(t, env, fix, shipmentID, "cset-001")
	if querySettlementBaseAmountText(t, env.pool, settlement.ID) != "87500.50" {
		t.Fatalf("settlement base should equal contract snapshot total")
	}
}

func TestCSet002BaseFuelNotDoubleCounted(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	base := "100000.00"
	components := `[{"component_type":"FUEL","amount":"5000.00"}]`
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource:            "CONTRACT_RATE",
		TotalAmount:              "105000.00",
		BaseAmount:               &base,
		ComponentBreakdownStatus: "AVAILABLE",
		ComponentsJSON:           components,
		ContractID:               ptrUUID(uuid.New()),
		RateCardID:               ptrUUID(uuid.New()),
		RateVersionID:            ptrUUID(uuid.New()),
		RateLineID:               ptrUUID(uuid.New()),
	})
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	want := decimal.RequireFromString("105000.00")
	if !ctxData.AgreedFreightAmount.Equal(want) {
		t.Fatalf("expected snapshot total_amount 105000.00 (no base+fuel double count), got %s", ctxData.AgreedFreightAmount.StringFixed(2))
	}
	wrongDoubleCount := decimal.RequireFromString("100000.00").Add(decimal.RequireFromString("5000.00")).Add(decimal.RequireFromString("5000.00"))
	if ctxData.AgreedFreightAmount.Equal(wrongDoubleCount) {
		t.Fatal("settlement principal appears to double-count fuel component")
	}
}

func TestCSet004SpotBidWithoutAwardLinkSettles(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	bidID := uuid.New()
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "SPOT_BID",
		TotalAmount:   "62000.00",
		BidID:         &bidID,
	})
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if ctxData.AwardLinkID != nil {
		t.Fatalf("expected no award link for SPOT_BID snapshot, got %v", *ctxData.AwardLinkID)
	}
	if !ctxData.AgreedFreightAmount.Equal(decimal.RequireFromString("62000.00")) {
		t.Fatalf("expected 62000.00, got %s", ctxData.AgreedFreightAmount.StringFixed(2))
	}
	settlement := createSettlement(t, env, fix, shipmentID, "cset-004")
	if settlement.AwardLinkID != nil {
		t.Fatal("settlement should not require award link for SPOT_BID snapshot order")
	}
}

func TestCSet005ManualWithoutAwardLinkSettles(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	auditID := uuid.New()
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource:     "MANUAL_SPOT",
		TotalAmount:       "33500.00",
		ManualSpotAuditID: &auditID,
	})
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if ctxData.AwardLinkID != nil {
		t.Fatalf("expected no award link for MANUAL_SPOT snapshot")
	}
	settlement := createSettlement(t, env, fix, shipmentID, "cset-005")
	if querySettlementBaseAmountText(t, env.pool, settlement.ID) != "33500.00" {
		t.Fatalf("manual spot settlement principal mismatch")
	}
}

func TestCSet006AccessorialsSeparate(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "RFQ_AWARD",
		TotalAmount:   "90000.00",
	})
	settlement := createSettlement(t, env, fix, shipmentID, "cset-006")
	baseBefore := querySettlementBaseAmountText(t, env.pool, settlement.ID)
	if _, err := env.settlements.ProposeAccessorial(context.Background(), settlement.ID, domain.ProposeAccessorialInput{
		SettlementActorInput: carrierActor(fix),
		ChargeCode:           "DETENTION", Amount: 4500,
	}); err != nil {
		t.Fatalf("propose accessorial: %v", err)
	}
	baseAfter := querySettlementBaseAmountText(t, env.pool, settlement.ID)
	if baseBefore != baseAfter || baseAfter != "90000.00" {
		t.Fatalf("accessorial proposal changed base freight: before=%s after=%s", baseBefore, baseAfter)
	}
}

func TestCSet007ContractChangeDoesNotChangeSettlementPrincipal(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	contractID := uuid.New()
	rateCardID := uuid.New()
	rateVersionID := uuid.New()
	rateLineID := uuid.New()
	orderID, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "CONTRACT_RATE",
		TotalAmount:   "55000.00",
		ContractID:    &contractID,
		RateCardID:    &rateCardID,
		RateVersionID: &rateVersionID,
		RateLineID:    &rateLineID,
	})
	awardLinkID := uuid.New()
	otherEventID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_award_transport_orders (
			id, tenant_id, rfx_event_id, rfx_award_id, rfx_response_id, transport_order_id,
			carrier_company_id, buyer_company_id, amount, currency_code
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'RUB')`,
		awardLinkID, fix.TenantID, otherEventID, fix.AwardID, fix.ResponseID, orderID, fix.CarrierA, fix.BuyerID, 120000.00); err != nil {
		t.Fatalf("award link: %v", err)
	}
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if !ctxData.AgreedFreightAmount.Equal(decimal.RequireFromString("55000.00")) {
		t.Fatalf("snapshot principal must ignore changed award link amount, got %s", ctxData.AgreedFreightAmount.StringFixed(2))
	}
}

func TestCSet008LegacyHistoricalAwardFallback(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, fix.ShipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if !ctxData.AgreedFreightAmount.Equal(decimal.RequireFromString("100000.00")) {
		t.Fatalf("legacy award fallback expected 100000.00, got %s", ctxData.AgreedFreightAmount.StringFixed(2))
	}
	if ctxData.AwardLinkID == nil {
		t.Fatal("legacy path should populate award link id")
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestCSet010CurrencyPreserved(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	fix := seedFixture(t, env.pool)
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "MANUAL_SPOT",
		TotalAmount:   "1800.00",
		CurrencyCode:  "USD",
		ManualSpotAuditID: ptrUUID(uuid.New()),
	})
	ctxData, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, shipmentID)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if ctxData.CurrencyCode != "USD" {
		t.Fatalf("currency=%s want USD", ctxData.CurrencyCode)
	}
	settlement := createSettlement(t, env, fix, shipmentID, "cset-010")
	if settlement.CurrencyCode != "USD" {
		t.Fatalf("settlement currency=%s want USD", settlement.CurrencyCode)
	}
}

func TestCSet011SnapshotPrincipalNeverFloat64(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	const preciseAmount = "9007199254740993.01"
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource:     "MANUAL_SPOT",
		TotalAmount:       preciseAmount,
		ManualSpotAuditID: ptrUUID(uuid.New()),
	})
	settlement := createSettlement(t, env, fix, shipmentID, "cset-011")
	got := querySettlementBaseAmountText(t, env.pool, settlement.ID)
	if got != preciseAmount {
		t.Fatalf("expected exact NUMERIC %s in DB, got %s", preciseAmount, got)
	}
	floatRoundTrip := 9007199254740992.0
	if got == decimal.NewFromFloat(floatRoundTrip).StringFixed(2) {
		t.Fatalf("amount was corrupted by float64 round-trip")
	}
}

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
	if !ctxData.AgreedFreightAmount.Equal(decimal.RequireFromString("108000.00")) {
		t.Fatalf("expected settlement principal 108000.00, got %s", ctxData.AgreedFreightAmount.StringFixed(2))
	}
}
