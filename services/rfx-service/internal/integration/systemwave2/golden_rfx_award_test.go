//go:build integration

package systemwave2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// TestSYSTEM_WAVE2_GOLDEN_RFX_TO_AWARD exercises Tenant A procurement chain:
// transport order → freight request → publish → dual bids → award (accept) → lineage checks.
func TestSYSTEM_WAVE2_GOLDEN_RFX_TO_AWARD(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedWave2Fixture(t, env)
	ctx := context.Background()

	fr, err := env.frSvc.CreateFromTransportOrder(ctx, fix.BuyerAdminA, domain.CreateFreightRequestFromOrderInput{
		TenantID:           fix.TenantA,
		TransportOrderID:   fix.TransportOrder,
		FreightRequestNumber: "FR-W2-" + fix.TransportOrder.String()[:8],
		RequestType:        "MINI_TENDER",
		ShipperCompanyID:   fix.BuyerA,
		ResponseDeadline:   futureDeadline(),
		CurrencyCode:       ptrString("RUB"),
	})
	if err != nil {
		t.Fatalf("create freight request: %v", err)
	}
	if fr.TenantID != fix.TenantA || fr.ShipperCompanyID != fix.BuyerA {
		t.Fatalf("freight request ownership invalid: tenant=%s buyer=%s", fr.TenantID, fr.ShipperCompanyID)
	}

	published, err := env.frSvc.Publish(ctx, fix.BuyerAdminA, fr.ID)
	if err != nil || published.Status != domain.FreightRequestStatusPublished {
		t.Fatalf("publish: err=%v status=%s", err, published.Status)
	}

	bidA1, err := env.bidSvc.CreateBid(ctx, fix.CarrierA1Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA1,
		BidNumber: "BID-A1-" + fr.ID.String()[:8], CurrencyCode: ptrString("RUB"),
		VATRate: ptrFloat(20), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 100000, FuelSurcharge: 0, TollAmount: 0, ExtraCharges: 0, VATRate: ptrFloat(20)}},
	})
	if err != nil {
		t.Fatalf("create bid A1: %v", err)
	}
	bidA2, err := env.bidSvc.CreateBid(ctx, fix.CarrierA2Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA2,
		BidNumber: "BID-A2-" + fr.ID.String()[:8], CurrencyCode: ptrString("RUB"),
		VATRate: ptrFloat(20), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 110000, FuelSurcharge: 0, TollAmount: 0, ExtraCharges: 0, VATRate: ptrFloat(20)}},
	})
	if err != nil {
		t.Fatalf("create bid A2: %v", err)
	}

	if _, err := env.bidSvc.SubmitBid(ctx, fix.CarrierA1Act, bidA1.ID); err != nil {
		t.Fatalf("submit bid A1: %v", err)
	}
	if _, err := env.bidSvc.SubmitBid(ctx, fix.CarrierA2Act, bidA2.ID); err != nil {
		t.Fatalf("submit bid A2: %v", err)
	}

	buyerBids, err := env.bidSvc.ListBids(ctx, fix.BuyerAdminA, fr.ID, nil)
	if err != nil || len(buyerBids) != 2 {
		t.Fatalf("buyer compare bids: err=%v count=%d", err, len(buyerBids))
	}

	_, err = env.bidSvc.GetByID(ctx, fix.CarrierA1Act, bidA2.ID)
	if err == nil {
		t.Fatal("carrier A1 must not read competitor bid before award")
	}

	awarded, err := env.bidSvc.AcceptBid(ctx, fix.BuyerAdminA, bidA1.ID)
	if err != nil || awarded.Status != domain.BidStatusAccepted {
		t.Fatalf("accept bid A1: err=%v status=%s", err, awarded.Status)
	}
	if awarded.CarrierCompanyID != fix.CarrierA1 {
		t.Fatalf("award winner=%s want carrier A1", awarded.CarrierCompanyID)
	}

	rejected, err := env.bidSvc.GetByID(ctx, fix.BuyerAdminA, bidA2.ID)
	if err != nil || rejected.Status == domain.BidStatusAccepted {
		t.Fatalf("carrier A2 must not be awarded: status=%s", rejected.Status)
	}

	awardedFR, err := env.frSvc.GetByID(ctx, fix.BuyerAdminA, fr.ID)
	if err != nil || awardedFR.Status != domain.FreightRequestStatusAwarded {
		t.Fatalf("freight request awarded status: err=%v status=%s", err, awardedFR.Status)
	}

	var orderTenant uuid.UUID
	var orderCarrier uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT tenant_id, shipper_company_id FROM transport.transport_orders WHERE id = $1`,
		fix.TransportOrder).Scan(&orderTenant, &orderCarrier); err != nil {
		t.Fatalf("load transport order: %v", err)
	}
	if orderTenant != fix.TenantA || orderCarrier != fix.BuyerA {
		t.Fatalf("transport order lineage broken: tenant=%s buyer=%s", orderTenant, orderCarrier)
	}

	t.Logf("LINEAGE tender_id=%s bid_a1=%s bid_a2=%s award_bid=%s transport_order=%s",
		fr.ID, bidA1.ID, bidA2.ID, awarded.ID, fix.TransportOrder)
}

func TestW2_CrossTenantFreightRequestDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedWave2Fixture(t, env)
	ctx := context.Background()

	fr, err := env.frSvc.CreateFromTransportOrder(ctx, fix.BuyerAdminA, domain.CreateFreightRequestFromOrderInput{
		TenantID: fix.TenantA, TransportOrderID: fix.TransportOrder,
		FreightRequestNumber: "FR-X-" + fix.TransportOrder.String()[:8],
		RequestType: "MINI_TENDER", ShipperCompanyID: fix.BuyerA,
		ResponseDeadline: futureDeadline(), CurrencyCode: ptrString("RUB"),
	})
	if err != nil {
		t.Fatalf("create fr: %v", err)
	}
	if _, err := env.frSvc.Publish(ctx, fix.BuyerAdminA, fr.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	_, err = env.frSvc.GetByID(ctx, fix.BuyerAdminB, fr.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)

	_, err = env.bidSvc.CreateBid(ctx, fix.CarrierB1Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantB, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierB1,
		BidNumber: "BID-B1", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 50000, VATRate: ptrFloat(20)}},
	})
	if err == nil {
		t.Fatal("tenant B carrier must not bid on tenant A tender")
	}
}

func TestW2_CrossCompanyBidMutationDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedWave2Fixture(t, env)
	ctx := context.Background()

	fr, _ := env.frSvc.CreateFromTransportOrder(ctx, fix.BuyerAdminA, domain.CreateFreightRequestFromOrderInput{
		TenantID: fix.TenantA, TransportOrderID: fix.TransportOrder,
		FreightRequestNumber: "FR-CC-" + fix.TransportOrder.String()[:8],
		RequestType: "MINI_TENDER", ShipperCompanyID: fix.BuyerA,
		ResponseDeadline: futureDeadline(), CurrencyCode: ptrString("RUB"),
	})
	_, _ = env.frSvc.Publish(ctx, fix.BuyerAdminA, fr.ID)

	bidA1, _ := env.bidSvc.CreateBid(ctx, fix.CarrierA1Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA1,
		BidNumber: "BID-CC-A1", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 100000, VATRate: ptrFloat(20)}},
	})
	bidA2, _ := env.bidSvc.CreateBid(ctx, fix.CarrierA2Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA2,
		BidNumber: "BID-CC-A2", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 110000, VATRate: ptrFloat(20)}},
	})
	_, _ = env.bidSvc.SubmitBid(ctx, fix.CarrierA1Act, bidA1.ID)
	_, _ = env.bidSvc.SubmitBid(ctx, fix.CarrierA2Act, bidA2.ID)

	_, err := env.bidSvc.SubmitBid(ctx, fix.CarrierA1Act, bidA2.ID)
	if err == nil {
		t.Fatal("carrier A1 must not submit/mutate A2 bid")
	}
}

func TestW2_DuplicateAcceptBidProtection(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedWave2Fixture(t, env)
	ctx := context.Background()

	fr, _ := env.frSvc.CreateFromTransportOrder(ctx, fix.BuyerAdminA, domain.CreateFreightRequestFromOrderInput{
		TenantID: fix.TenantA, TransportOrderID: fix.TransportOrder,
		FreightRequestNumber: "FR-DUP-" + fix.TransportOrder.String()[:8],
		RequestType: "MINI_TENDER", ShipperCompanyID: fix.BuyerA,
		ResponseDeadline: futureDeadline(), CurrencyCode: ptrString("RUB"),
	})
	_, _ = env.frSvc.Publish(ctx, fix.BuyerAdminA, fr.ID)

	bidA1, _ := env.bidSvc.CreateBid(ctx, fix.CarrierA1Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA1,
		BidNumber: "BID-DUP-A1", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 100000, VATRate: ptrFloat(20)}},
	})
	bidA2, _ := env.bidSvc.CreateBid(ctx, fix.CarrierA2Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA2,
		BidNumber: "BID-DUP-A2", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 110000, VATRate: ptrFloat(20)}},
	})
	_, _ = env.bidSvc.SubmitBid(ctx, fix.CarrierA1Act, bidA1.ID)
	_, _ = env.bidSvc.SubmitBid(ctx, fix.CarrierA2Act, bidA2.ID)

	if _, err := env.bidSvc.AcceptBid(ctx, fix.BuyerAdminA, bidA1.ID); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	_, err := env.bidSvc.AcceptBid(ctx, fix.BuyerAdminA, bidA2.ID)
	if err == nil {
		t.Fatal("second accept must fail when freight request already awarded")
	}

	var acceptedCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.bids WHERE freight_request_id = $1 AND status = 'ACCEPTED'`, fr.ID).Scan(&acceptedCount); err != nil {
		t.Fatalf("count accepted: %v", err)
	}
	if acceptedCount != 1 {
		t.Fatalf("duplicate business entity: accepted bids=%d", acceptedCount)
	}
}

func TestW2_IDORMatrixForeignTenantBid(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedWave2Fixture(t, env)
	ctx := context.Background()

	fr, _ := env.frSvc.CreateFromTransportOrder(ctx, fix.BuyerAdminA, domain.CreateFreightRequestFromOrderInput{
		TenantID: fix.TenantA, TransportOrderID: fix.TransportOrder,
		FreightRequestNumber: "FR-IDOR-" + fix.TransportOrder.String()[:8],
		RequestType: "MINI_TENDER", ShipperCompanyID: fix.BuyerA,
		ResponseDeadline: futureDeadline(), CurrencyCode: ptrString("RUB"),
	})
	_, _ = env.frSvc.Publish(ctx, fix.BuyerAdminA, fr.ID)
	bid, _ := env.bidSvc.CreateBid(ctx, fix.CarrierA1Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA1,
		BidNumber: "BID-IDOR", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: 100000, VATRate: ptrFloat(20)}},
	})

	cases := []struct {
		name  string
		actor domain.ActorContext
		id    uuid.UUID
	}{
		{"foreign_tenant", fix.BuyerAdminB, bid.ID},
		{"random_uuid", fix.BuyerAdminA, uuid.New()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env.bidSvc.GetByID(ctx, tc.actor, tc.id)
			if err == nil {
				t.Fatal("expected access denied / not found")
			}
		})
	}
}

func TestW2_MoneyIntegrityNonIntegerBid(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedWave2Fixture(t, env)
	ctx := context.Background()

	fr, _ := env.frSvc.CreateFromTransportOrder(ctx, fix.BuyerAdminA, domain.CreateFreightRequestFromOrderInput{
		TenantID: fix.TenantA, TransportOrderID: fix.TransportOrder,
		FreightRequestNumber: "FR-MNY-" + fix.TransportOrder.String()[:8],
		RequestType: "MINI_TENDER", ShipperCompanyID: fix.BuyerA,
		ResponseDeadline: futureDeadline(), CurrencyCode: ptrString("RUB"),
	})
	_, _ = env.frSvc.Publish(ctx, fix.BuyerAdminA, fr.ID)

	amount := 123456.78
	bid, err := env.bidSvc.CreateBid(ctx, fix.CarrierA1Act, fr.ID, domain.CreateBidInput{
		TenantID: fix.TenantA, FreightRequestID: fr.ID, CarrierCompanyID: fix.CarrierA1,
		BidNumber: "BID-MNY", CurrencyCode: ptrString("RUB"), ValidUntil: futureDeadline(),
		Items: []domain.CreateBidItemInput{{BaseAmount: amount, VATRate: ptrFloat(20)}},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}
	if bid.TotalAmount != amount {
		t.Fatalf("amount corrupted: got %v want %v", bid.TotalAmount, amount)
	}
	if bid.CurrencyCode == nil || *bid.CurrencyCode != "RUB" {
		t.Fatalf("currency lost: %v", bid.CurrencyCode)
	}
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s", code)
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error=%v want code=%s", err, code)
	}
}

// Guard against unused import when tests are refactored.
var _ = time.Now
