//go:build integration

package scoringv3

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
)

type failingScoringTrigger struct {
	err error
}

func (f failingScoringTrigger) CalculateForSubmittedResponse(ctx context.Context, tenantID, responseID uuid.UUID) (*domain.ScoringRunResult, error) {
	return nil, f.err
}

func TestScoreModelDraftUpdateAndRead(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()

	view, err := env.scoreModelSvc.GetScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if view.Model.Status != domain.ScoreModelStatusDraft {
		t.Fatalf("expected draft, got %s", view.Model.Status)
	}

	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "HSE", Name: "HSE", Weight: 50, SortOrder: 1,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 50, SortOrder: 2,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "HSE", QuestionCode: "ADR_AVAILABLE"},
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("put draft: %v", err)
	}
	updated, err := env.scoreModelSvc.GetScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}
	if len(updated.Criteria) != 2 || len(updated.Bindings) != 2 {
		t.Fatalf("expected 2 criteria and 2 bindings, got %d/%d", len(updated.Criteria), len(updated.Bindings))
	}

	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "HSE", Name: "HSE", Weight: 40, SortOrder: 1,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 60, SortOrder: 2,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "HSE", QuestionCode: "ADR_AVAILABLE",
				KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)},
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	final, err := env.scoreModelSvc.GetScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("read final draft: %v", err)
	}
	if final.Criteria[0].Weight != 40 || final.Criteria[1].Weight != 60 {
		t.Fatalf("authoritative weights not updated: %+v", final.Criteria)
	}

	published, err := env.scoreModelSvc.PublishScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Model.Status != domain.ScoreModelStatusPublished {
		t.Fatalf("expected published, got %s", published.Model.Status)
	}
	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestCriteriaReadinessConstraints(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()

	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "HSE", Name: "HSE", Weight: 40,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 60,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "HSE", QuestionCode: "ADR_AVAILABLE",
				KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)},
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("put valid model: %v", err)
	}
	valid, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("validate valid: %v", err)
	}
	if !valid.Ready {
		t.Fatalf("CRITERIA_WEIGHT_VALID expected ready, errors=%+v", valid.Errors)
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
		t.Fatalf("put invalid weights: %v", err)
	}
	invalid, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("validate invalid weights: %v", err)
	}
	if invalid.Ready || !hasReadinessCode(*invalid, "CRITERIA_WEIGHT_TOTAL_INVALID") {
		t.Fatalf("CRITERIA_WEIGHT_INVALID expected, got ready=%v errors=%+v", invalid.Ready, invalid.Errors)
	}

	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "DUP", Name: "One", Weight: 50, NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "DUP", Name: "Two", Weight: 50, NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "DUP", QuestionCode: "ADR_AVAILABLE"},
		},
	})
	assertAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestBindingVersionSafety(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sfA := seedScoringFixture(t, env, fix)
	ctx := context.Background()

	sfB := seedForeignVersionQuestion(t, env, fix)
	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sfA.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "HSE", Name: "HSE", Weight: 40,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 60,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "HSE", QuestionCode: "ADR_AVAILABLE",
				KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)},
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("put draft model: %v", err)
	}

	view, err := env.scoreModelSvc.GetScoreModel(ctx, fix.BuyerA, sfA.Event.ID)
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if len(view.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(view.Bindings))
	}
	bindingID := view.Bindings[0].ID
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_score_bindings SET question_id=$1 WHERE id=$2`, sfB.QuestionID, bindingID); err != nil {
		t.Fatalf("corrupt binding to foreign version question: %v", err)
	}

	readiness, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sfA.Event.ID)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if readiness.Ready || !hasReadinessCode(*readiness, "BINDING_QUESTION_INVALID") {
		t.Fatalf("BINDING_WRONG_VERSION_DENY expected, ready=%v errors=%+v", readiness.Ready, readiness.Errors)
	}

	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sfA.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "HSE", Name: "HSE", Weight: 40,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 60,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "HSE", QuestionCode: "ADR_AVAILABLE",
				KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)},
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("BINDING_SAME_VERSION_REQUIRED repair put: %v", err)
	}
	repaired, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sfA.Event.ID)
	if err != nil || !repaired.Ready {
		t.Fatalf("expected same-version bindings ready, err=%v errors=%+v", err, repaired.Errors)
	}
}

func TestSingleSelectScoring(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedSelectScoringFixture(t, env, fix)
	ctx := context.Background()

	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "SERVICE", Name: "Service", Weight: 100,
				NormalizationJSON: json.RawMessage(`{"type":"OPTION_MAP","option_scores":{"GOLD":100,"SILVER":60,"BRONZE":20}}`)},
		},
		Bindings: []domain.ScoreBindingInput{{CriterionCode: "SERVICE", QuestionCode: "SERVICE_LEVEL"}},
	})
	if err != nil {
		t.Fatalf("put select model: %v", err)
	}
	if _, err := env.scoreModelSvc.PublishScoreModel(ctx, fix.BuyerA, sf.Event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ServiceQuestion.ID, Value: json.RawMessage(`"GOLD"`)},
			{QuestionID: sf.MultiQuestion.ID, Value: json.RawMessage(`["A"]`)},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	submit, err := env.crSvc.Submit(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, saved.SaveVersion)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	score, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, submit.ResponseID, env.rfxSvc)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if score.Qualification.TotalScore == nil || *score.Qualification.TotalScore != 100 {
		t.Fatalf("SINGLE_SELECT_SCORE total=%v want 100", score.Qualification.TotalScore)
	}
}

func TestMultiSelectSumCappedScoring(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedSelectScoringFixture(t, env, fix)
	ctx := context.Background()

	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "FEATURES", Name: "Features", Weight: 100,
				NormalizationJSON: json.RawMessage(`{"type":"MULTI_SELECT","aggregation":"SUM_CAPPED","cap":100,"option_scores":{"A":30,"B":40,"C":50}}`)},
		},
		Bindings: []domain.ScoreBindingInput{{CriterionCode: "FEATURES", QuestionCode: "FEATURES"}},
	})
	if err != nil {
		t.Fatalf("put multi model: %v", err)
	}
	if _, err := env.scoreModelSvc.PublishScoreModel(ctx, fix.BuyerA, sf.Event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ServiceQuestion.ID, Value: json.RawMessage(`"BRONZE"`)},
			{QuestionID: sf.MultiQuestion.ID, Value: json.RawMessage(`["A","B"]`)},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	submit, err := env.crSvc.Submit(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, saved.SaveVersion)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	score, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, submit.ResponseID, env.rfxSvc)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if score.Qualification.TotalScore == nil || *score.Qualification.TotalScore != 70 {
		t.Fatalf("MULTI_SELECT_SUM_CAPPED total=%v want 70", score.Qualification.TotalScore)
	}
}

func TestInvalidOptionScoringConfigDeny(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedSelectScoringFixture(t, env, fix)
	ctx := context.Background()
	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "SERVICE", Name: "Service", Weight: 100,
				NormalizationJSON: json.RawMessage(`{"type":"OPTION_MAP","option_scores":{"PLATINUM":100}}`)},
		},
		Bindings: []domain.ScoreBindingInput{{CriterionCode: "SERVICE", QuestionCode: "SERVICE_LEVEL"}},
	})
	if err != nil {
		t.Fatalf("put invalid option model: %v", err)
	}
	invalidOpt, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("validate invalid option: %v", err)
	}
	if invalidOpt.Ready || !hasReadinessCode(*invalidOpt, "OPTION_SCORE_INVALID") {
		t.Fatalf("INVALID_OPTION_SCORING_CONFIG_DENY expected, errors=%+v", invalidOpt.Errors)
	}
}

func TestMultiSelectUnsupportedAggregationDeny(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedSelectScoringFixture(t, env, fix)
	ctx := context.Background()
	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "FEATURES", Name: "Features", Weight: 100,
				NormalizationJSON: json.RawMessage(`{"type":"MULTI_SELECT","aggregation":"MEDIAN","option_scores":{"A":30,"B":40}}`)},
		},
		Bindings: []domain.ScoreBindingInput{{CriterionCode: "FEATURES", QuestionCode: "FEATURES"}},
	})
	if err != nil {
		t.Fatalf("put unsupported aggregation: %v", err)
	}
	unsupported, err := env.scoreModelSvc.ValidateScoreModel(ctx, fix.BuyerA, sf.Event.ID)
	if err != nil {
		t.Fatalf("validate unsupported: %v", err)
	}
	if unsupported.Ready || (!hasReadinessCode(*unsupported, "MULTI_SELECT_AGGREGATION_INVALID") && !hasReadinessCode(*unsupported, "NORMALIZATION_INVALID")) {
		t.Fatalf("MODEL_READINESS_DENY expected, errors=%+v", unsupported.Errors)
	}
}

func TestNumberLinearBoundaries(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()

	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 100,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("put model: %v", err)
	}
	if _, err := env.scoreModelSvc.PublishScoreModel(ctx, fix.BuyerA, sf.Event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	cases := []struct {
		name  string
		fleet float64
		want  float64
	}{
		{"min", 0, 0},
		{"midpoint", 50, 50},
		{"max", 100, 100},
		{"below_min", -10, 0},
		{"above_max", 150, 100},
	}
	responseID := submitCarrierAnswers(t, env, fix, sf, fix.CarrierID, fix.CarrierAct, true, 50)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fleetJSON, _ := json.Marshal(tc.fleet)
			if _, err := env.pool.Exec(ctx, `
				UPDATE rfx.rfx_answers SET answer_value_json=$1::jsonb
				WHERE rfx_response_id=$2 AND question_id=$3`, string(fleetJSON), responseID, sf.FleetQuestion.ID); err != nil {
				t.Fatalf("update fleet answer: %v", err)
			}
			if _, err := env.scoringSvc.CalculateForSubmittedResponse(ctx, fix.TenantID, responseID); err != nil {
				t.Fatalf("recalc: %v", err)
			}
			score, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, responseID, env.rfxSvc)
			if err != nil {
				t.Fatalf("score: %v", err)
			}
			if score.Qualification.TotalScore == nil || *score.Qualification.TotalScore != tc.want {
				t.Fatalf("fleet=%v total=%v want %v", tc.fleet, score.Qualification.TotalScore, tc.want)
			}
		})
	}
}

func TestPersistedValidAnswerOnly(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	ctx := context.Background()
	crNoScore := service.NewCarrierResponseService(env.pool, env.rfxRepo, env.answerRepo, env.qRepo, env.auditRepo, env.membershipRepo, env.rfxSvc)

	ws, err := crNoScore.StartOrResume(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := crNoScore.SaveAnswers(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ADRQuestion.ID, Value: json.RawMessage(`true`)},
			{QuestionID: sf.FleetQuestion.ID, Value: json.RawMessage(`50`)},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := crNoScore.Submit(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, saved.SaveVersion); err != nil {
		t.Fatalf("submit: %v", err)
	}
	resp, err := env.rfxRepo.GetResponseByEventAndCompany(ctx, sf.Event.ID, fix.CarrierID, fix.TenantID)
	if err != nil {
		t.Fatalf("response: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_answers SET answer_value_json='100'::jsonb
		WHERE rfx_response_id=$1 AND question_id=$2`, resp.ID, sf.FleetQuestion.ID); err != nil {
		t.Fatalf("sql update authoritative answer: %v", err)
	}
	if _, err := env.scoringSvc.CalculateForSubmittedResponse(ctx, fix.TenantID, resp.ID); err != nil {
		t.Fatalf("recalc: %v", err)
	}
	score, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, resp.ID, env.rfxSvc)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if score.Qualification.TotalScore == nil || *score.Qualification.TotalScore != 100 {
		t.Fatalf("PERSISTED_VALID_ANSWER_ONLY total=%v want 100 (DB fleet=100 not save-time 50)", score.Qualification.TotalScore)
	}
}

func TestInvalidAndPreviewAnswerSafety(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	ctx := context.Background()
	crNoScore := service.NewCarrierResponseService(env.pool, env.rfxRepo, env.answerRepo, env.qRepo, env.auditRepo, env.membershipRepo, env.rfxSvc)

	ws, err := crNoScore.StartOrResume(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := crNoScore.SaveAnswers(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ADRQuestion.ID, Value: json.RawMessage(`true`)},
			{QuestionID: sf.FleetQuestion.ID, Value: json.RawMessage(`50`)},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := crNoScore.Submit(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, saved.SaveVersion); err != nil {
		t.Fatalf("submit: %v", err)
	}
	resp, err := env.rfxRepo.GetResponseByEventAndCompany(ctx, sf.Event.ID, fix.CarrierID, fix.TenantID)
	if err != nil {
		t.Fatalf("response: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_answers SET answer_value_json='"not-a-number"'::jsonb
		WHERE rfx_response_id=$1 AND question_id=$2`, resp.ID, sf.FleetQuestion.ID); err != nil {
		t.Fatalf("corrupt fleet answer: %v", err)
	}
	_, err = env.scoringSvc.CalculateForSubmittedResponse(ctx, fix.TenantID, resp.ID)
	if err == nil {
		t.Fatal("expected scoring failure for corrupt answer")
	}
	qualCount, err := countQualificationResults(ctx, env.pool, resp.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("count qualification: %v", err)
	}
	scoreCount, err := countAnswerScores(ctx, env.pool, resp.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("count answer scores: %v", err)
	}
	if qualCount != 0 || scoreCount != 0 {
		t.Fatalf("INVALID_ANSWER_NOT_SCOREABLE expected 0 rows, qual=%d scores=%d", qualCount, scoreCount)
	}

	previewEnv := setupTestEnv(t)
	previewFix := seedBuyerFixture(t, previewEnv)
	previewSF := seedScoringFixture(t, previewEnv, previewFix)
	putDeterministicScoreModel(t, previewEnv, previewFix, previewSF)
	previewCtx := context.Background()
	ws, err := previewEnv.crSvc.StartOrResume(previewCtx, previewFix.CarrierAct, previewSF.Event.ID, previewFix.CarrierID)
	if err != nil {
		t.Fatalf("preview start: %v", err)
	}
	adrJSON, _ := json.Marshal(true)
	fleetJSON, _ := json.Marshal(50.0)
	if _, err := previewEnv.crSvc.SaveAnswers(previewCtx, previewFix.CarrierAct, previewSF.Event.ID, previewFix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: previewSF.ADRQuestion.ID, Value: adrJSON},
			{QuestionID: previewSF.FleetQuestion.ID, Value: fleetJSON},
		},
	}); err != nil {
		t.Fatalf("preview save: %v", err)
	}
	_, err = previewEnv.scoringSvc.CalculateForSubmittedResponse(previewCtx, previewFix.TenantID, ws.Response.ID)
	if err == nil {
		t.Fatal("expected preview/non-submitted scoring deny")
	}
	pQual, _ := countQualificationResults(previewCtx, previewEnv.pool, ws.Response.ID, previewFix.TenantID)
	pScores, _ := countAnswerScores(previewCtx, previewEnv.pool, ws.Response.ID, previewFix.TenantID)
	if pQual != 0 || pScores != 0 {
		t.Fatalf("PREVIEW_SCORE_PERSISTENCE expected 0, qual=%d scores=%d", pQual, pScores)
	}
}

func TestKnockoutSaveSubmitExplicitProofs(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	ctx := context.Background()

	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierBAct, sf.Event.ID, fix.CarrierBID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	adrJSON, _ := json.Marshal(false)
	fleetJSON, _ := json.Marshal(100.0)
	saved, err := env.crSvc.SaveAnswers(ctx, fix.CarrierBAct, sf.Event.ID, fix.CarrierBID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ADRQuestion.ID, Value: adrJSON},
			{QuestionID: sf.FleetQuestion.ID, Value: fleetJSON},
		},
	})
	if err != nil {
		t.Fatalf("KNOCKOUT_BLOCKS_SAVE_NO: save failed: %v", err)
	}
	submit, err := env.crSvc.Submit(ctx, fix.CarrierBAct, sf.Event.ID, fix.CarrierBID, saved.SaveVersion)
	if err != nil {
		t.Fatalf("KNOCKOUT_BLOCKS_SUBMIT_NO: submit failed: %v", err)
	}
	score, err := env.scoringSvc.GetResponseScore(ctx, fix.BuyerA, sf.Event.ID, submit.ResponseID, env.rfxSvc)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if score.Qualification.Status != domain.QualificationStatusRejected || !score.Qualification.KnockoutTriggered {
		t.Fatalf("KNOCKOUT_AFTER_SUBMIT expected REJECTED knockout, got status=%s knockout=%v",
			score.Qualification.Status, score.Qualification.KnockoutTriggered)
	}
	var adrValue string
	if err := env.pool.QueryRow(ctx, `
		SELECT answer_value_json::text FROM rfx.rfx_answers
		WHERE rfx_response_id=$1 AND question_id=$2`, submit.ResponseID, sf.ADRQuestion.ID).Scan(&adrValue); err != nil {
		t.Fatalf("read adr: %v", err)
	}
	if adrValue != "false" {
		t.Fatalf("KNOCKOUT_ANSWER_PRESERVED got %s", adrValue)
	}
}

func TestScorePersistenceInvariants(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	responseID := submitCarrierAnswers(t, env, fix, sf, fix.CarrierID, fix.CarrierAct, true, 50)
	ctx := context.Background()

	var modelVersion int
	var modelID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT score_model_version, score_model_id FROM rfx.rfx_qualification_results
		WHERE rfx_response_id=$1 AND tenant_id=$2`, responseID, fix.TenantID).Scan(&modelVersion, &modelID); err != nil {
		t.Fatalf("qualification row: %v", err)
	}
	if modelVersion != 1 {
		t.Fatalf("SCORE_MODEL_VERSION_PINNED qual version=%d want 1", modelVersion)
	}
	var answerModelVersion int
	if err := env.pool.QueryRow(ctx, `
		SELECT score_model_version FROM rfx.rfx_answer_scores
		WHERE rfx_response_id=$1 AND tenant_id=$2 LIMIT 1`, responseID, fix.TenantID).Scan(&answerModelVersion); err != nil {
		t.Fatalf("answer score row: %v", err)
	}
	if answerModelVersion != 1 {
		t.Fatalf("answer score version=%d want 1", answerModelVersion)
	}

	if _, err := env.scoringSvc.CalculateForSubmittedResponse(ctx, fix.TenantID, responseID); err != nil {
		t.Fatalf("recalc: %v", err)
	}
	qualCount, err := countQualificationResults(ctx, env.pool, responseID, fix.TenantID)
	if err != nil {
		t.Fatalf("count qual: %v", err)
	}
	scoreCount, err := countAnswerScores(ctx, env.pool, responseID, fix.TenantID)
	if err != nil {
		t.Fatalf("count scores: %v", err)
	}
	if qualCount != 1 || scoreCount != 2 {
		t.Fatalf("SCORING_IDEMPOTENT expected 1 qual and 2 scores, got qual=%d scores=%d", qualCount, scoreCount)
	}
}

func TestScoringFailureDoesNotRollbackSubmit(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	putDeterministicScoreModel(t, env, fix, sf)
	ctx := context.Background()

	injected := apperrors.Internal("SCORING_FAILURE_INJECTED", nil)
	failSvc := service.NewCarrierResponseServiceWithScoring(
		env.pool, env.rfxRepo, env.answerRepo, env.qRepo, env.auditRepo, env.membershipRepo, env.rfxSvc,
		failingScoringTrigger{err: injected},
	)

	ws, err := failSvc.StartOrResume(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	saved, err := failSvc.SaveAnswers(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: sf.ADRQuestion.ID, Value: json.RawMessage(`true`)},
			{QuestionID: sf.FleetQuestion.ID, Value: json.RawMessage(`50`)},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	submit, err := failSvc.Submit(ctx, fix.CarrierAct, sf.Event.ID, fix.CarrierID, saved.SaveVersion)
	if err != nil {
		t.Fatalf("SUBMIT_COMMITTED_BEFORE_SCORING: submit must succeed: %v", err)
	}
	if submit.Status != domain.CarrierResponseStatusSubmitted {
		t.Fatalf("expected SUBMITTED status, got %s", submit.Status)
	}
	resp, err := env.rfxRepo.GetResponseByID(ctx, submit.ResponseID, fix.TenantID)
	if err != nil || resp.Status != domain.RfxResponseStatusSubmitted {
		t.Fatalf("SCORING_FAILURE_DOES_NOT_ROLLBACK_SUBMIT status=%s err=%v", resp.Status, err)
	}
	scoreCount, _ := countAnswerScores(ctx, env.pool, submit.ResponseID, fix.TenantID)
	if scoreCount != 0 {
		t.Fatalf("PARTIAL_SCORE_PERSISTENCE expected 0, got %d", scoreCount)
	}
	auditEvents, err := env.auditRepo.ListByEntity(ctx, fix.TenantID, "rfx_response", submit.ResponseID, 20)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	auditCount := 0
	for _, ev := range auditEvents {
		if ev.Action == "response.scoring_failed" {
			auditCount++
		}
	}
	if auditCount < 1 {
		t.Fatalf("SCORING_FAILURE_AUDITED count=%d events=%+v", auditCount, auditEvents)
	}
}

func TestNonMemberDeny(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	ctx := context.Background()

	_, err := env.scoreModelSvc.GetScoreModel(ctx, fix.NonMember, sf.Event.ID)
	if err == nil {
		t.Fatal("NON_MEMBER_DENY expected get deny")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || (appErr.Code != apperrors.CodeForbidden && appErr.Code != apperrors.CodeNotFound) {
		t.Fatalf("expected forbidden/not found, got %v", err)
	}
	_, err = env.scoreModelSvc.PutScoreModel(ctx, fix.NonMember, sf.Event.ID, domain.PutScoreModelInput{})
	if err == nil {
		t.Fatal("NON_MEMBER_DENY expected put deny")
	}
}

type foreignVersionFixture struct {
	QuestionID uuid.UUID
}

func seedForeignVersionQuestion(t *testing.T, env *testEnv, fix buyerFixture) foreignVersionFixture {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Foreign Version Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-FV-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create foreign event: %v", err)
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
	q, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_AVAILABLE", QuestionType: domain.QuestionTypeYesNo, Label: "ADR", Required: true,
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	publishVersion(t, env, version.ID)
	return foreignVersionFixture{QuestionID: q.ID}
}

type selectScoringFixture struct {
	Event           *domain.RfxEvent
	ServiceQuestion *domain.Question
	MultiQuestion   *domain.Question
}

func seedSelectScoringFixture(t *testing.T, env *testEnv, fix buyerFixture) selectScoringFixture {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Select Scoring Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-SEL-" + uuid.NewString()[:8],
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
	serviceQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "SERVICE_LEVEL", QuestionType: domain.QuestionTypeSingleSelect, Label: "Service Level", Required: true,
	})
	if err != nil {
		t.Fatalf("service question: %v", err)
	}
	for _, opt := range []struct{ code, label string }{{"GOLD", "Gold"}, {"SILVER", "Silver"}, {"BRONZE", "Bronze"}} {
		if _, err := env.qSvc.CreateOption(ctx, fix.BuyerA, event.ID, serviceQ.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.code, Label: opt.label,
		}); err != nil {
			t.Fatalf("option %s: %v", opt.code, err)
		}
	}
	multiQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "FEATURES", QuestionType: domain.QuestionTypeMultiSelect, Label: "Features", Required: true,
	})
	if err != nil {
		t.Fatalf("multi question: %v", err)
	}
	for _, opt := range []struct{ code, label string }{{"A", "A"}, {"B", "B"}, {"C", "C"}} {
		if _, err := env.qSvc.CreateOption(ctx, fix.BuyerA, event.ID, multiQ.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.code, Label: opt.label,
		}); err != nil {
			t.Fatalf("multi option %s: %v", opt.code, err)
		}
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return selectScoringFixture{Event: event, ServiceQuestion: serviceQ, MultiQuestion: multiQ}
}
