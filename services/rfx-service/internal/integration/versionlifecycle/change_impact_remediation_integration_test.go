//go:build integration

package versionlifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE3REM001RepublishStructuralZeroResponsesNoAnalysis422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R01")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-01-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-01-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e3-rem-01-pub", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: eventRow.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Missing confirmation",
	})
	appErr := assertAppErrorCode(t, err, apperrors.CodeValidation)
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactRequired {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE3REM002NonMaterialRepublishWithoutAnalysis422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R02")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-02-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-02-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabeled fleet size")
	})
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e3-rem-02-pub", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: eventRow.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Label-only republish without confirmation",
	})
	appErr := assertAppErrorCode(t, err, apperrors.CodeValidation)
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactRequired {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE3REM003RepublishZeroResponsesWithAnalysisPass(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R03")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-03-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-03-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := publishWithConfirmation(t, env, fix, event.ID, "e3-rem-03-pub", fix.BuyerA, eventRow, draft, analysis, "Confirmed zero-response republish"); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func TestE3REM004FirstPublishWithoutAnalysisPass(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R04")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e3-rem-04-pub", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: eventRow.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "First publish",
	}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
}

func TestE3REM005ScoringOnlyBindingNotKnockout(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R05")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-05-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-05-fork", func(ctx context.Context, draftID uuid.UUID) {
		addScoringOnlyBindingOnDraft(t, env, fix, v1.ID, draftID)
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassScoringAffecting)
	assertImpactClassesNotContain(t, analysis.ImpactClasses, domain.ChangeImpactClassKnockoutAffecting)
}

func TestE3REM006KnockoutBindingAddedClassifiesKnockout(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R06")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-06-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-06-fork", func(ctx context.Context, draftID uuid.UUID) {
		addKnockoutBindingOnDraft(t, env, fix, draftID)
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassKnockoutAffecting)
}

func TestE3REM007AddedRequiredQuestionCountsExistingDraftResponse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R07")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-07-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start draft response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-07-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NEW_REQ", "New required")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassMaterialWithDraftResponses)
	if analysis.AffectedDraftResponseCount != 1 {
		t.Fatalf("draft count=%d want=1", analysis.AffectedDraftResponseCount)
	}
}

func TestE3REM008AddedRequiredQuestionCountsSubmittedResponse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R08")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-08-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	qFleet := questionIDByCode(t, ws, "FLEET_SIZE")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qFleet, Value: json.RawMessage(`"10"`) }},
	}); err != nil {
		t.Fatalf("save answer: %v", err)
	}
	ws2, err := env.crSvc.GetWorkspace(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if _, err := env.crSvc.Submit(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, ws2.Response.SaveVersion); err != nil {
		t.Fatalf("submit: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-08-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NEW_REQ", "New required")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassMaterialWithSubmitted)
	if analysis.AffectedSubmittedResponseCount != 1 {
		t.Fatalf("submitted count=%d want=1", analysis.AffectedSubmittedResponseCount)
	}
}

func TestE3REM009TwoAffectedAnswersSameResponseCountOnce(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R09")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-09-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	qFleet := questionIDByCode(t, ws, "FLEET_SIZE")
	qHSE := questionIDByCode(t, ws, "HSE_OK")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: qFleet, Value: json.RawMessage(`"10"`)},
			{QuestionID: qHSE, Value: json.RawMessage(`true`)},
		},
	}); err != nil {
		t.Fatalf("save answers: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-09-fork", func(ctx context.Context, draftID uuid.UUID) {
		requiredFalse := false
		requiredTrue := true
		updateQuestionRequired(t, env, fix, event.ID, draftID, "FLEET_SIZE", &requiredFalse)
		updateQuestionRequired(t, env, fix, event.ID, draftID, "HSE_OK", &requiredTrue)
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassMaterialWithDraftResponses)
	if analysis.AffectedDraftResponseCount != 1 {
		t.Fatalf("expected distinct response count=1, got draft=%d submitted=%d", analysis.AffectedDraftResponseCount, analysis.AffectedSubmittedResponseCount)
	}
}

func TestE3REM010OptionalUnfilledQuestionLabelChangeNoAffectedResponses(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R10")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-10-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	qFleet := questionIDByCode(t, ws, "FLEET_SIZE")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qFleet, Value: json.RawMessage(`"10"`) }},
	}); err != nil {
		t.Fatalf("save answer: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-10-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "HSE_OK", "HSE relabeled optional")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassNonMaterial)
	if analysis.AffectedDraftResponseCount != 0 || analysis.AffectedSubmittedResponseCount != 0 {
		t.Fatalf("expected zero affected counts for optional unfilled label-only change, got draft=%d submitted=%d",
			analysis.AffectedDraftResponseCount, analysis.AffectedSubmittedResponseCount)
	}
}

func TestE3REM011TwoCarriersDistinctAffectedCounts(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R11")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-11-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	carrierB, carrierBAct := seedSecondCarrier(t, env, fix)
	addSecondCarrierParticipant(t, env, fix, event.ID, carrierB)

	wsDraft, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start draft carrier: %v", err)
	}
	qDraft := questionIDByCode(t, wsDraft, "FLEET_SIZE")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: wsDraft.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qDraft, Value: json.RawMessage(`"10"`) }},
	}); err != nil {
		t.Fatalf("save draft carrier answer: %v", err)
	}

	wsSubmitted, err := env.crSvc.StartOrResume(context.Background(), carrierBAct, event.ID, carrierB)
	if err != nil {
		t.Fatalf("start submitted carrier: %v", err)
	}
	qSubmitted := questionIDByCode(t, wsSubmitted, "FLEET_SIZE")
	if _, err := env.crSvc.SaveAnswers(context.Background(), carrierBAct, event.ID, carrierB, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: wsSubmitted.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qSubmitted, Value: json.RawMessage(`"20"`) }},
	}); err != nil {
		t.Fatalf("save submitted carrier answer: %v", err)
	}
	wsSubmitted2, err := env.crSvc.GetWorkspace(context.Background(), carrierBAct, event.ID, carrierB)
	if err != nil {
		t.Fatalf("workspace submitted: %v", err)
	}
	if _, err := env.crSvc.Submit(context.Background(), carrierBAct, event.ID, carrierB, wsSubmitted2.Response.SaveVersion); err != nil {
		t.Fatalf("submit carrier b: %v", err)
	}

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-11-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NEW_REQ", "New required")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	if analysis.AffectedDraftResponseCount != 1 || analysis.AffectedSubmittedResponseCount != 1 {
		t.Fatalf("counts draft=%d submitted=%d want 1/1", analysis.AffectedDraftResponseCount, analysis.AffectedSubmittedResponseCount)
	}
}

func TestE3REM012BuyerManageDelegateCanConfirmPublish(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	delegate := seedOwnerCompanyBuyerDelegate(t, env, fix)
	event := createDraftEvent(t, env, fix, "RFX-E3-R12")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-12-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-12-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := publishWithConfirmation(t, env, fix, event.ID, "e3-rem-12-pub", delegate, eventRow, draft, analysis, "Delegate confirm"); err != nil {
		t.Fatalf("delegate publish: %v", err)
	}
	if analysis.ActorID != fix.BuyerA.UserID {
		t.Fatal("preview actor_id must remain original preview author")
	}
}

func TestE3REM013UnauthorizedSameTenantUserDenied403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	unauthorized := seedOwnerCompanyNonBuyerUser(t, env, fix)
	event := createDraftEvent(t, env, fix, "RFX-E3-R13")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-13-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-13-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabel")
	})
	_, err := env.versionSvc.PreviewChangeImpact(context.Background(), unauthorized, event.ID, domain.PreviewChangeImpactInput{
		CandidateVersionID: draft.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE3REM014UnknownImpactClassDeniedByPostgreSQL(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R14")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-14-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-14-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabel")
	})
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_change_impact_analyses (
			id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, '["UNKNOWN_CLASS"]'::jsonb, now() + interval '15 minutes')`,
		uuid.New(), fix.TenantID, event.ID, v1.ID, draft.ID, fix.BuyerA.UserID, "deadbeef")
	assertPostgreSQLErrorCode(t, err, "23514")
}

func TestE3REM015AllSixImpactClassesAllowedInPostgreSQL(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R15")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-15-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-rem-15-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabel")
	})
	classes := `[
		"NON_MATERIAL",
		"MATERIAL_NO_RESPONSES",
		"MATERIAL_WITH_DRAFT_RESPONSES",
		"MATERIAL_WITH_SUBMITTED_RESPONSES",
		"SCORING_AFFECTING",
		"KNOCKOUT_AFFECTING"
	]`
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_change_impact_analyses (
			id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, now() + interval '15 minutes')`,
		uuid.New(), fix.TenantID, event.ID, v1.ID, draft.ID, fix.BuyerA.UserID, "deadbeef", classes)
	if err != nil {
		t.Fatalf("insert allowed classes: %v", err)
	}
}

func TestE3REM016MigrationDownWithPublicSearchPath(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql"); err != nil {
		t.Fatalf("apply 000068: %v", err)
	}
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-R16")
	publishedID := ensurePublishedVersion(t, env, fix, event.ID, "e3-rem-16-v1").ID
	if err := applyMigrationFile(ctx, env.pool, "000069_rfx_change_impact_v3_0e3.up.sql"); err != nil {
		t.Fatalf("apply 000069 up: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `SET search_path TO public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000069_rfx_change_impact_v3_0e3.down.sql"); err != nil {
		t.Fatalf("apply 000069 down: %v", err)
	}
	if legacyTableExists(t, env, "rfx_change_impact_analyses") {
		t.Fatal("change impact table must be removed after down migration with public search_path")
	}
	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.rfx_versions WHERE id = $1`, publishedID).Scan(&status); err != nil {
		t.Fatalf("reload published version: %v", err)
	}
	if status != domain.RfxVersionStatusPublished {
		t.Fatalf("published version altered: status=%s", status)
	}
}

func addScoringOnlyBindingOnDraft(t *testing.T, env *testEnv, fix buyerFixture, publishedVersionID, draftVersionID uuid.UUID) {
	t.Helper()
	copyPublishedScoringToDraft(t, env, fix, publishedVersionID, draftVersionID)
	ctx := context.Background()
	model, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draftVersionID)
	if err != nil {
		t.Fatalf("load draft score model: %v", err)
	}
	criterionID := uuid.New()
	criteria := []domain.ScoreCriterion{{
		ID:                criterionID,
		CriterionCode:     "PRICE",
		Name:              "Price",
		Weight:            100,
		SortOrder:         1,
		NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
	}}
	questionID := mustQuestionByCode(t, env, fix, draftVersionID, "FLEET_SIZE").ID
	bindings := []domain.ScoreBinding{{
		CriterionID:     criterionID,
		QuestionID:      questionID,
		BindingType:     "AUTOMATIC",
		ScoringRuleJSON: json.RawMessage(`{"type":"BOOLEAN_MAP"}`),
	}}
	if err := env.scoreRepo.ReplaceDraftDefinition(ctx, model.ID, fix.TenantID, criteria, bindings); err != nil {
		t.Fatalf("replace scoring-only draft definition: %v", err)
	}
}

func addKnockoutBindingOnDraft(t *testing.T, env *testEnv, fix buyerFixture, draftVersionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	model, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draftVersionID)
	if err != nil {
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			t.Fatalf("load draft score model: %v", err)
		}
		if _, err := env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_score_models (tenant_id, rfx_version_id, model_version, status, model_type, definition_json)
			VALUES ($1, $2, 1, 'DRAFT', 'WEIGHTED', '{}'::jsonb)
		`, fix.TenantID, draftVersionID); err != nil {
			t.Fatalf("create draft score model: %v", err)
		}
		model, err = env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draftVersionID)
		if err != nil {
			t.Fatalf("reload draft score model: %v", err)
		}
	}
	criterionID := uuid.New()
	criteria := []domain.ScoreCriterion{{
		ID:                criterionID,
		CriterionCode:     "HSE",
		Name:              "HSE",
		Weight:            100,
		SortOrder:         1,
		NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
	}}
	questionID := mustQuestionByCode(t, env, fix, draftVersionID, "HSE_OK").ID
	bindings := []domain.ScoreBinding{{
		CriterionID:      criterionID,
		QuestionID:       questionID,
		BindingType:      "AUTOMATIC",
		ScoringRuleJSON:  json.RawMessage(`{"type":"BOOLEAN_MAP"}`),
		KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`),
	}}
	if err := env.scoreRepo.ReplaceDraftDefinition(ctx, model.ID, fix.TenantID, criteria, bindings); err != nil {
		t.Fatalf("replace knockout draft definition: %v", err)
	}
}

func seedOwnerCompanyBuyerDelegate(t *testing.T, env *testEnv, fix buyerFixture) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	delegate := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		delegate.UserID, fix.TenantID, "delegate@test.local", "delegate@test.local"); err != nil {
		t.Fatalf("seed delegate user: %v", err)
	}
	var roleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_ADMIN' LIMIT 1`).Scan(&roleID); err != nil {
		t.Fatalf("lookup shipper admin role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		fix.TenantID, fix.CompanyA, delegate.UserID); err != nil {
		t.Fatalf("seed delegate membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, delegate.UserID, fix.CompanyA, roleID); err != nil {
		t.Fatalf("seed delegate role: %v", err)
	}
	return delegate
}

func seedOwnerCompanyNonBuyerUser(t *testing.T, env *testEnv, fix buyerFixture) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	user := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		user.UserID, fix.TenantID, "finance@test.local", "finance@test.local"); err != nil {
		t.Fatalf("seed finance user: %v", err)
	}
	var roleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'FINANCE_MANAGER' LIMIT 1`).Scan(&roleID); err != nil {
		t.Fatalf("lookup finance role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		fix.TenantID, fix.CompanyA, user.UserID); err != nil {
		t.Fatalf("seed finance membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, user.UserID, fix.CompanyA, roleID); err != nil {
		t.Fatalf("seed finance role: %v", err)
	}
	return user
}

func assertImpactClassesNotContain(t *testing.T, classes []string, absent string) {
	t.Helper()
	for _, class := range classes {
		if class == absent {
			t.Fatalf("unexpected class %s in %v", absent, classes)
		}
	}
}
