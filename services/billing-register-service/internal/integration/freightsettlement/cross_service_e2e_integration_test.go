//go:build integration

package freightsettlement

import (
	"testing"

	"github.com/google/uuid"
)

func TestCE2E001RFQAwardPath(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	eventID := uuid.New()
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "RFQ_AWARD",
		TotalAmount:   "145000.00",
		RfxEventID:    &eventID,
	})
	settlement := createSettlement(t, env, fix, shipmentID, "e2e-rfq")
	if got := querySettlementBaseAmountText(t, env.pool, settlement.ID); got != "145000.00" {
		t.Fatalf("settlement base=%s want 145000.00", got)
	}
}

func TestCE2E002SpotBidPath(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	bidID := uuid.New()
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "SPOT_BID",
		TotalAmount:   "72000.00",
		BidID:         &bidID,
	})
	settlement := createSettlement(t, env, fix, shipmentID, "e2e-bid")
	if querySettlementBaseAmountText(t, env.pool, settlement.ID) != "72000.00" {
		t.Fatal("spot bid settlement principal mismatch")
	}
	if settlement.AwardLinkID != nil {
		t.Fatal("SPOT_BID settlement should not require award link")
	}
}

func TestCE2E003ContractRatePath(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource: "CONTRACT_RATE",
		TotalAmount:   "98300.25",
		ContractID:    ptrUUID(uuid.New()),
		RateCardID:    ptrUUID(uuid.New()),
		RateVersionID: ptrUUID(uuid.New()),
		RateLineID:    ptrUUID(uuid.New()),
	})
	settlement := createSettlement(t, env, fix, shipmentID, "e2e-contract")
	if querySettlementBaseAmountText(t, env.pool, settlement.ID) != "98300.25" {
		t.Fatal("contract rate settlement principal mismatch")
	}
	if settlement.PricingSource == nil || *settlement.PricingSource != "CONTRACT_RATE" {
		t.Fatalf("pricing_source=%v want CONTRACT_RATE", settlement.PricingSource)
	}
}

func TestCE2E004ManualSpotPath(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	_, _, shipmentID := seedSnapshotOrderWithShipment(t, env.pool, fix, snapshotOrderOpts{
		PricingSource:     "MANUAL_SPOT",
		TotalAmount:       "41000.00",
		ManualSpotAuditID: ptrUUID(uuid.New()),
	})
	settlement := createSettlement(t, env, fix, shipmentID, "e2e-manual")
	if querySettlementBaseAmountText(t, env.pool, settlement.ID) != "41000.00" {
		t.Fatal("manual spot settlement principal mismatch")
	}
}
