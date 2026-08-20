//go:build integration

package pricing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestCRFX003AwardExactDecimalString(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 108000.50)

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if out.TotalAmount != "108000.50" {
		t.Fatalf("expected exact decimal string 108000.50, got %q", out.TotalAmount)
	}
}

func TestCRFX004AwardAggregateOnly(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 95000)

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if out.BaseAmount != nil || len(out.Components) != 0 || out.ComponentBreakdownStatus != domain.ComponentBreakdownUnavailable {
		t.Fatalf("expected aggregate-only normalization")
	}
}

func TestCRFX002AwardWrongTenantDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 1000)

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAwardScopeContext(ctx, uuid.New(), event.ID, &lotID)
	if err == nil {
		t.Fatal("expected not found for wrong tenant")
	}
}

func seedAwardedEvent(t *testing.T, env *testEnv, fix fixture, amount float64) (*domain.RfxEvent, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	currency := "RUB"
	event, err := env.rfxSvc.CreateEvent(ctx, fix.buyer, domain.CreateRfxEventInput{
		TenantID: fix.tenantID, OwnerCompanyID: fix.buyerCompany, Title: "Pricing Tender",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-P-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline, CurrencyCode: &currency,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.buyer, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.tenantID, RfxEventID: event.ID, CompanyID: fix.carrierCompany, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("participant: %v", err)
	}
	lot, err := env.rfxSvc.CreateLot(ctx, fix.buyer, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.tenantID, RfxEventID: event.ID, LotNumber: "LOT-1", Name: "Lot 1",
	})
	if err != nil {
		t.Fatalf("lot: %v", err)
	}
	equip := "TAUTLINER"
	if _, err := env.rfxSvc.CreateLane(ctx, fix.buyer, lot.ID, domain.CreateRfxLaneInput{
		TenantID: fix.tenantID, RfxLotID: lot.ID,
		OriginLocationID: fix.originID, DestinationLocationID: fix.destID,
		TransportMode: "ROAD", EquipmentType: &equip,
	}); err != nil {
		t.Fatalf("lane: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.buyer, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	resp, err := env.rfxSvc.CreateResponse(ctx, fix.carrier, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.tenantID, ParticipantCompanyID: fix.carrierCompany,
	})
	if err != nil {
		t.Fatalf("response: %v", err)
	}
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.carrier, resp.ID, []domain.UpsertOfferLineInput{
		{RfxLotID: lot.ID, Amount: amount, CurrencyCode: "RUB"},
	}); err != nil {
		t.Fatalf("commercial: %v", err)
	}
	if _, err := env.rfxSvc.SubmitResponse(ctx, fix.carrier, resp.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.rfxSvc.AwardResponse(ctx, fix.buyer, event.ID, resp.ID); err != nil {
		t.Fatalf("award: %v", err)
	}
	return event, lot.ID
}
