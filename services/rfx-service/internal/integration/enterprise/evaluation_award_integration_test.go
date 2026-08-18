//go:build integration

package enterprise

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func seedCarrierUser(t *testing.T, env *testEnv, fix buyerFixture, companyID uuid.UUID) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		userID, fix.TenantID, "carrier-"+userID.String()[:8]+"@test.local", "Carrier User")
	if err != nil {
		t.Fatalf("seed carrier user: %v", err)
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		fix.TenantID, companyID, userID)
	if err != nil {
		t.Fatalf("seed carrier membership: %v", err)
	}
	var carrierRoleID uuid.UUID
	err = env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID)
	if err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	_, err = env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, userID, companyID, carrierRoleID)
	if err != nil {
		t.Fatalf("seed carrier role: %v", err)
	}
	return domain.ActorContext{TenantID: fix.TenantID, UserID: userID}
}

func seedEvaluationEvent(t *testing.T, env *testEnv, fix buyerFixture) (*domain.RfxEvent, uuid.UUID, domain.ActorContext) {
	t.Helper()
	ctx := context.Background()
	carrierB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		carrierB, fix.TenantID, "Carrier B", "CARRIER")
	if err != nil {
		t.Fatalf("seed carrier B: %v", err)
	}
	deadline := time.Now().UTC().Add(24 * time.Hour)
	currency := "RUB"
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Evaluation Tender",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-E-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline, CurrencyCode: &currency,
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
	return event, carrierB, seedCarrierUser(t, env, fix, carrierB)
}

func TestCarrierDraftCommercialUpdateAndSubmittedImmutability(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedEvaluationEvent(t, env, fix)

	response, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("create response: %v", err)
	}
	updated, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, response.ID, []domain.UpsertOfferLineInput{
		{Amount: 100000, CurrencyCode: "RUB"},
	})
	if err != nil {
		t.Fatalf("update commercial: %v", err)
	}
	if len(updated.OfferLines) != 1 || updated.OfferLines[0].Amount != 100000 {
		t.Fatalf("expected offer line persisted")
	}
	submitted, err := env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, response.ID)
	if err != nil || submitted.Status != domain.RfxResponseStatusSubmitted {
		t.Fatalf("submit: %v", err)
	}
	_, err = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, response.ID, []domain.UpsertOfferLineInput{
		{Amount: 90000, CurrencyCode: "RUB"},
	})
	if err == nil {
		t.Fatal("expected submitted response update denied")
	}
}

func TestBuyerEvaluationIsolationAndAward(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event, carrierB, carrierBAct := seedEvaluationEvent(t, env, fix)

	respA, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("create response A: %v", err)
	}
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, respA.ID, []domain.UpsertOfferLineInput{
		{Amount: 100000, CurrencyCode: "RUB"},
	}); err != nil {
		t.Fatalf("update A: %v", err)
	}
	if _, err := env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, respA.ID); err != nil {
		t.Fatalf("submit A: %v", err)
	}

	respB, err := env.rfxSvc.CreateResponse(ctx, carrierBAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: carrierB,
	})
	if err != nil {
		t.Fatalf("create response B: %v", err)
	}
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, carrierBAct, respB.ID, []domain.UpsertOfferLineInput{
		{Amount: 95000, CurrencyCode: "RUB"},
	}); err != nil {
		t.Fatalf("update B: %v", err)
	}
	if _, err := env.rfxSvc.SubmitResponse(ctx, carrierBAct, respB.ID); err != nil {
		t.Fatalf("submit B: %v", err)
	}

	_, err = env.rfxSvc.ListEvaluationResponses(ctx, fix.BuyerB, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)

	views, err := env.rfxSvc.ListEvaluationResponses(ctx, fix.BuyerA, event.ID)
	if err != nil || len(views) != 2 {
		t.Fatalf("buyer list responses: %v len=%d", err, len(views))
	}
	recalculated, err := env.rfxSvc.RecalculateEvaluation(ctx, fix.BuyerA, event.ID)
	if err != nil || len(recalculated) != 2 {
		t.Fatalf("recalculate: %v", err)
	}
	if recalculated[0].Response.EvaluationRank == nil {
		t.Fatal("expected rank after recalculate")
	}

	if err := env.rfxSvc.AddToShortlist(ctx, fix.BuyerA, respB.ID); err != nil {
		t.Fatalf("shortlist: %v", err)
	}
	award, err := env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, respB.ID)
	if err != nil {
		t.Fatalf("award: %v", err)
	}
	if award.RfxResponseID != respB.ID {
		t.Fatalf("unexpected winner")
	}
	_, err = env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, respA.ID)
	assertAppErrorCode(t, err, apperrors.CodeConflict)

	_, err = env.rfxSvc.ListEvaluationResponses(ctx, fix.CarrierAct, event.ID)
	if err == nil {
		t.Fatal("carrier must not read buyer evaluation list")
	}
}

func TestConcurrentDoubleAwardProtection(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedEvaluationEvent(t, env, fix)

	respA, _ := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, respA.ID, []domain.UpsertOfferLineInput{{Amount: 100000, CurrencyCode: "RUB"}})
	_, _ = env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, respA.ID)

	carrierB := uuid.New()
	_, _ = env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		carrierB, fix.TenantID, "Carrier Race", "CARRIER")
	_, _ = env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: carrierB, ParticipantType: "CARRIER",
	})
	carrierBAct := seedCarrierUser(t, env, fix, carrierB)
	respB, _ := env.rfxSvc.CreateResponse(ctx, carrierBAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: carrierB,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, carrierBAct, respB.ID, []domain.UpsertOfferLineInput{{Amount: 90000, CurrencyCode: "RUB"}})
	_, _ = env.rfxSvc.SubmitResponse(ctx, carrierBAct, respB.ID)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, responseID := range []uuid.UUID{respA.ID, respB.ID} {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			_, err := env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, id)
			results <- err
		}(responseID)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful award, got %d", successes)
	}
	var awardCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_awards WHERE rfx_event_id = $1`, event.ID).Scan(&awardCount); err != nil {
		t.Fatalf("count awards: %v", err)
	}
	if awardCount != 1 {
		t.Fatalf("expected one award row, got %d", awardCount)
	}
}

func TestCompetitorCommercialIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event, carrierB, carrierBAct := seedEvaluationEvent(t, env, fix)

	respA, _ := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, respA.ID, []domain.UpsertOfferLineInput{{Amount: 100000, CurrencyCode: "RUB"}})
	_, _ = env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, respA.ID)

	respB, _ := env.rfxSvc.CreateResponse(ctx, carrierBAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: carrierB,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, carrierBAct, respB.ID, []domain.UpsertOfferLineInput{{Amount: 80000, CurrencyCode: "RUB"}})
	_, _ = env.rfxSvc.SubmitResponse(ctx, carrierBAct, respB.ID)

	_, err := env.rfxSvc.GetResponse(ctx, fix.CarrierAct, respB.ID)
	if err == nil {
		t.Fatal("carrier A must not read carrier B response")
	}
	_, err = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, respB.ID, []domain.UpsertOfferLineInput{{Amount: 1, CurrencyCode: "RUB"}})
	if err == nil {
		t.Fatal("carrier A must not update carrier B response")
	}
}

func TestCarrierOwnAwardVisibility(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedEvaluationEvent(t, env, fix)

	resp, _ := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, resp.ID, []domain.UpsertOfferLineInput{{Amount: 100000, CurrencyCode: "RUB"}})
	_, _ = env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, resp.ID)
	_, err := env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, resp.ID)
	if err != nil {
		t.Fatalf("award: %v", err)
	}
	award, err := env.rfxSvc.GetOwnAward(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil || award.RfxResponseID != resp.ID {
		t.Fatalf("carrier own award: %v", err)
	}
}

func TestAuditFailureRollsBackAward(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedEvaluationEvent(t, env, fix)
	resp, _ := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, resp.ID, []domain.UpsertOfferLineInput{{Amount: 100000, CurrencyCode: "RUB"}})
	_, _ = env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, resp.ID)

	env.auditRepo.SetInjectRecordFailure(true)
	_, err := env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, resp.ID)
	if err == nil {
		t.Fatal("expected award failure when audit fails")
	}
	env.auditRepo.SetInjectRecordFailure(false)
	var awardCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_awards WHERE rfx_event_id = $1`, event.ID).Scan(&awardCount); err != nil {
		t.Fatalf("count awards: %v", err)
	}
	if awardCount != 0 {
		t.Fatalf("expected award rollback, got %d rows", awardCount)
	}
}
