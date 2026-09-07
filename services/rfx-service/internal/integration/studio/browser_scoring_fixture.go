//go:build integration

package studio

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type browserScoringFixture struct {
	browserStudioFixture
	CarrierACompanyID uuid.UUID
	CarrierAUserID    uuid.UUID
	CarrierAJWT       string
	CarrierBCompanyID uuid.UUID
	CarrierBUserID    uuid.UUID
	CarrierBJWT       string
	LegacyEventID     uuid.UUID
	LegacyRfxNumber   string
}

func seedBrowserScoringV3Fixture(t *testing.T, env *testEnv) browserScoringFixture {
	t.Helper()
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	carrierBCompany := uuid.New()
	carrierBUser := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,'CARRIER')`,
		carrierBCompany, fix.TenantID, "Carrier B Browser"); err != nil {
		t.Fatalf("seed carrier b company: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`,
		carrierBUser, fix.TenantID, "carrier-b-scoring-e2e@freight.test", "Carrier B E2E"); err != nil {
		t.Fatalf("seed carrier b user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`,
		fix.TenantID, carrierBCompany, carrierBUser); err != nil {
		t.Fatalf("carrier b membership: %v", err)
	}
	var carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.TenantID, carrierBUser, carrierBCompany, carrierRoleID); err != nil {
		t.Fatalf("carrier b role: %v", err)
	}

	deadline := time.Now().UTC().Add(48 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Browser Scoring v3 Acceptance",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-SCORING-BROWSER-1",
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create scoring event: %v", err)
	}
	for _, carrier := range []uuid.UUID{fix.CarrierID, carrierBCompany} {
		if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
			TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: carrier, ParticipantType: "CARRIER",
		}); err != nil {
			t.Fatalf("add participant: %v", err)
		}
	}
	version, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("draft version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled = TRUE WHERE id = $1`, version.ID); err != nil {
		t.Fatalf("enable questionnaire: %v", err)
	}
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{SectionCode: "MAIN", Title: "Main"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_AVAILABLE", QuestionType: domain.QuestionTypeYesNo, Label: "ADR Available", Required: true,
	}); err != nil {
		t.Fatalf("adr question: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET_COUNT", QuestionType: domain.QuestionTypeNumber, Label: "Fleet Count", Required: true,
	}); err != nil {
		t.Fatalf("fleet question: %v", err)
	}
	eventRecord, err := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event before publish: %v", err)
	}
	version, err = env.qRepo.GetVersionByID(ctx, version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft version before publish: %v", err)
	}
	if _, err := env.versionSvc.PublishQuestionnaire(ctx, fix.BuyerA, event.ID, "browser-scoring-fixture-publish", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: eventRecord.Version,
		ExpectedDraftVersion: version.Version,
		ChangeSummary:        "Browser scoring fixture publish",
	}); err != nil {
		t.Fatalf("publish questionnaire: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	legacy := createDraftEvent(t, env, fix, "RFX-LEGACY-NO-SCORE")
	enableQuestionnaire(t, env, fix.BuyerA, legacy.ID)
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, legacy.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: legacy.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("legacy participant: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, legacy.ID); err != nil {
		t.Fatalf("publish legacy event: %v", err)
	}
	legacyResp, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, legacy.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("legacy response: %v", err)
	}
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, legacyResp.ID, []domain.UpsertOfferLineInput{
		{Amount: 88000, CurrencyCode: "RUB"},
	}); err != nil {
		t.Fatalf("legacy commercial: %v", err)
	}
	if _, err := env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, legacyResp.ID); err != nil {
		t.Fatalf("legacy submit: %v", err)
	}
	if _, err := env.rfxSvc.RecalculateEvaluation(ctx, fix.BuyerA, legacy.ID); err != nil {
		t.Fatalf("legacy recalculate: %v", err)
	}

	return browserScoringFixture{
		browserStudioFixture: browserStudioFixture{
			TenantID:  fix.TenantID,
			CompanyID: fix.CompanyA,
			UserID:    fix.BuyerA.UserID,
			EventID:   event.ID,
			JWT:       browserStudioJWT(fix.BuyerA.UserID, fix.TenantID),
			RfxNumber: event.RfxNumber,
		},
		CarrierACompanyID: fix.CarrierID,
		CarrierAUserID:    fix.CarrierAct.UserID,
		CarrierAJWT:       browserStudioJWT(fix.CarrierAct.UserID, fix.TenantID),
		CarrierBCompanyID: carrierBCompany,
		CarrierBUserID:    carrierBUser,
		CarrierBJWT:       browserStudioJWT(carrierBUser, fix.TenantID),
		LegacyEventID:     legacy.ID,
		LegacyRfxNumber:   legacy.RfxNumber,
	}
}

func forkDraftViaProductionGateway(t *testing.T, gatewayURL string, fix browserScoringFixture) {
	t.Helper()
	url := gatewayURL + "/api/v1/rfx-events/" + fix.EventID.String() + "/versions/fork-draft"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("fork-draft request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+fix.JWT)
	req.Header.Set("X-Company-ID", fix.CompanyID.String())
	req.Header.Set("Idempotency-Key", "browser-scoring-v3-fork-draft")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fork-draft do: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("fork-draft via gateway: status=%d body=%s", resp.StatusCode, string(body))
	}
}
