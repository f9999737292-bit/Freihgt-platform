//go:build integration

package scoringv3

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestMigration000067Fresh(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	var exists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_score_models'
		)`).Scan(&exists); err != nil {
		t.Fatalf("check score_models: %v", err)
	}
	if !exists {
		t.Fatal("migration 000067: rfx_score_models missing")
	}
}

func TestMigration000067Legacy(t *testing.T) {
	env, _ := setupLegacyMigrationTestEnv(t)
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Legacy Scoring",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-LEG-067",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO rfx.rfx_responses (tenant_id, rfx_event_id, participant_company_id, status, commercial_score, total_score)
		VALUES ($1,$2,$3,'SUBMITTED',80,80) RETURNING commercial_score`,
		fix.TenantID, event.ID, fix.CarrierID).Scan(new(float64)); err != nil {
		t.Fatalf("insert legacy response: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000067_rfx_scoring_v3_0d.up.sql"); err != nil {
		t.Fatalf("apply 067: %v", err)
	}
	var after float64
	if err := env.pool.QueryRow(ctx, `SELECT commercial_score FROM rfx.rfx_responses WHERE rfx_event_id=$1`, event.ID).Scan(&after); err != nil {
		t.Fatalf("read legacy score: %v", err)
	}
	if after != 80 {
		t.Fatalf("legacy commercial_score changed: %v", after)
	}
}

func TestScoreModelCreatePublishAndImmutable(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()
	putDeterministicScoreModel(t, env, fix, sf)
	view, err := env.scoreModelSvc.GetScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if view.Model.Status != domain.ScoreModelStatusPublished {
		t.Fatalf("expected published, got %s", view.Model.Status)
	}
	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{{CriterionCode: "X", Name: "X", Weight: 100,
			NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)}},
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestCriteriaWeightInvalid(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()
	result, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("validate empty: %v", err)
	}
	if result.Ready {
		t.Fatal("empty model should not be ready")
	}
	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "A", Name: "A", Weight: 30, NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "B", Name: "B", Weight: 30, NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "A", QuestionCode: "ADR_AVAILABLE"},
			{CriterionCode: "B", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("put partial model: %v", err)
	}
	result, err = env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("validate invalid weights: %v", err)
	}
	if result.Ready {
		t.Fatal("weights 30+30 should fail readiness")
	}
}

func TestDeterministicScoringCarrierAAndB(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	ctx := context.Background()

	carrierA := submitCarrierAnswers(t, env, fix, sf, fix.CarrierID, fix.CarrierAct, true, 50)
	carrierB := submitCarrierAnswers(t, env, fix, sf, fix.CarrierBID, fix.CarrierBAct, false, 100)

	scoreA, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, carrierA, env.rfxSvc)
	if err != nil {
		t.Fatalf("score A: %v", err)
	}
	if scoreA.Qualification.Status != domain.QualificationStatusQualified {
		t.Fatalf("carrier A status=%s", scoreA.Qualification.Status)
	}
	if scoreA.Qualification.TotalScore == nil || *scoreA.Qualification.TotalScore != 70 {
		t.Fatalf("carrier A total=%v want 70", scoreA.Qualification.TotalScore)
	}

	scoreB, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, carrierB, env.rfxSvc)
	if err != nil {
		t.Fatalf("score B: %v", err)
	}
	if scoreB.Qualification.Status != domain.QualificationStatusRejected {
		t.Fatalf("carrier B status=%s want REJECTED", scoreB.Qualification.Status)
	}
	if !scoreB.Qualification.KnockoutTriggered {
		t.Fatal("carrier B knockout expected")
	}
	if scoreB.Qualification.TotalScore == nil || *scoreB.Qualification.TotalScore != 60 {
		t.Fatalf("carrier B total=%v want 60", scoreB.Qualification.TotalScore)
	}

	var adrValue string
	if err := env.pool.QueryRow(ctx, `
		SELECT answer_value_json::text FROM rfx.rfx_answers a
		WHERE a.rfx_response_id=$1 AND a.question_id=$2`, carrierB, sf.ADRQuestion.ID).Scan(&adrValue); err != nil {
		t.Fatalf("read adr answer: %v", err)
	}
	if adrValue != "false" {
		t.Fatalf("knockout answer not preserved: %s", adrValue)
	}
}

func TestSubmitTriggersScoreAndIdempotent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	responseID := submitCarrierAnswers(t, env, fix, sf, fix.CarrierID, fix.CarrierAct, true, 50)
	ctx := context.Background()
	first, err := env.scoringSvc.CalculateForSubmittedResponse(ctx, fix.TenantID, responseID)
	if err != nil {
		t.Fatalf("recalc: %v", err)
	}
	second, err := env.scoringSvc.CalculateForSubmittedResponse(ctx, fix.TenantID, responseID)
	if err != nil {
		t.Fatalf("recalc2: %v", err)
	}
	if first.Qualification.TotalScore == nil || second.Qualification.TotalScore == nil {
		t.Fatal("missing totals")
	}
	if *first.Qualification.TotalScore != *second.Qualification.TotalScore {
		t.Fatalf("idempotency drift: %v vs %v", *first.Qualification.TotalScore, *second.Qualification.TotalScore)
	}
}

func TestCrossTenantAndCarrierMutationDeny(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()
	_, err := env.scoreModelSvc.GetScoreModel(ctx, fix.CrossTenant, sf.Event.ID)
	if err == nil {
		t.Fatal("expected cross tenant deny")
	}
	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.CarrierAct, sf.Event.ID, domain.PutScoreModelInput{})
	if err == nil {
		t.Fatal("expected carrier mutation deny")
	}
	_, err = env.scoreModelSvc.GetScoreModel(ctx, fix.BuyerB, sf.Event.ID)
	if err == nil {
		t.Fatal("expected cross company deny")
	}
}

func TestExplainabilityComplete(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	responseID := submitCarrierAnswers(t, env, fix, sf, fix.CarrierID, fix.CarrierAct, true, 50)
	ctx := context.Background()
	explanations, err := env.scoringSvc.GetResponseScoreExplanation(ctx, fix.BuyerA, sf.Event.ID, responseID, env.rfxSvc)
	if err != nil {
		t.Fatalf("explanation: %v", err)
	}
	if len(explanations) != 2 {
		t.Fatalf("expected 2 explanations, got %d", len(explanations))
	}
	for _, e := range explanations {
		if e.Source == "" || e.ScoreModelVersion != 1 || e.CriterionCode == "" {
			t.Fatalf("incomplete explanation: %+v", e)
		}
	}
}

func submitCarrierAnswers(t *testing.T, env *testEnv, fix buyerFixture, sf scoringFixture, carrierID uuid.UUID, actor domain.ActorContext, adr bool, fleet float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	ws, err := env.crSvc.StartOrResume(ctx, actor, sf.Event.ID, carrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	adrJSON, _ := json.Marshal(adr)
	fleetJSON, _ := json.Marshal(fleet)
	saved, err := env.crSvc.SaveAnswers(ctx, actor, sf.Event.ID, carrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ADRQuestion.ID, Value: adrJSON},
			{QuestionID: sf.FleetQuestion.ID, Value: fleetJSON},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := env.crSvc.Submit(ctx, actor, sf.Event.ID, carrierID, saved.SaveVersion); err != nil {
		t.Fatalf("submit: %v", err)
	}
	resp, err := env.rfxRepo.GetResponseByEventAndCompany(ctx, sf.Event.ID, carrierID, fix.TenantID)
	if err != nil {
		t.Fatalf("get response: %v", err)
	}
	return resp.ID
}
