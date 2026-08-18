//go:build integration

package enterprise

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestCarrierInvitedListAndIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Carrier Tender",
		RfxType: "LANE_TENDER", Category: "FREIGHT", RfxNumber: "RFX-C-1",
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	items, total, err := env.rfxSvc.ListCarrierInvitedEvents(ctx, fix.CarrierAct, domain.ListCarrierInvitedEventsFilter{
		TenantID: fix.TenantID, CarrierCompanyID: fix.CarrierID, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list carrier invited: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 invited event, got total=%d len=%d", total, len(items))
	}
	if items[0].OwnResponseStatus != domain.CarrierOwnResponseNotStarted {
		t.Fatalf("expected NOT_STARTED, got %s", items[0].OwnResponseStatus)
	}

	_, err = env.rfxSvc.GetEvent(ctx, fix.BuyerB, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)

	otherCarrier := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	_, err = env.rfxSvc.GetEvent(ctx, otherCarrier, event.ID)
	if err == nil {
		t.Fatal("expected non-participant carrier denied")
	}
}

func TestCarrierOwnResponseCreateSubmitAndCompetitorIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	carrierB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		carrierB, fix.TenantID, "Carrier B", "CARRIER")
	if err != nil {
		t.Fatalf("seed carrier B: %v", err)
	}

	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Bid Isolation",
		RfxType: "LANE_TENDER", Category: "FREIGHT", RfxNumber: "RFX-C-2",
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	for _, companyID := range []uuid.UUID{fix.CarrierID, carrierB} {
		if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
			TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: companyID, ParticipantType: "CARRIER",
		}); err != nil {
			t.Fatalf("add participant: %v", err)
		}
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	responseA, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("create response A: %v", err)
	}

	carrierBAct := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	_, err = env.rfxSvc.GetResponse(ctx, carrierBAct, responseA.ID)
	if err == nil {
		t.Fatal("expected competitor cannot read response")
	}

	own, err := env.rfxSvc.GetOwnResponse(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil || own.ID != responseA.ID {
		t.Fatalf("get own response: %v", err)
	}

	submitted, err := env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, responseA.ID)
	if err != nil {
		t.Fatalf("submit response: %v", err)
	}
	if submitted.Status != domain.RfxResponseStatusSubmitted {
		t.Fatalf("expected SUBMITTED, got %s", submitted.Status)
	}
}

func TestCarrierCannotPublishRfx(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Buyer only",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-C-3",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	_, err = env.rfxSvc.PublishEvent(ctx, fix.CarrierAct, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}
