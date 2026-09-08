//go:build integration

package versionlifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/http/handlers"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestE3INT01LabelOnlyChangeNonMaterial(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-01")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-01-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-01-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Fleet size (updated label)")
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesOrdered(t, analysis.ImpactClasses, domain.ChangeImpactClassNonMaterial)
}

func TestE3INT02StructuralChangeNoResponsesMaterialNoResponses(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-02")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-02-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-02-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "EXTRA", "Extra question")
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesOrdered(t, analysis.ImpactClasses, domain.ChangeImpactClassMaterialNoResponses)
}

func TestE3INT03DraftResponsesAffectedMaterialWithDraftResponses(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-03")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-03-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	qID := questionIDByCode(t, ws, "FLEET_SIZE")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qID, Value: json.RawMessage(`"42"`) }},
	}); err != nil {
		t.Fatalf("save draft answer: %v", err)
	}
	_ = v1

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-03-fork", func(ctx context.Context, draftID uuid.UUID) {
		required := false
		updateQuestionRequired(t, env, fix, event.ID, draftID, "FLEET_SIZE", &required)
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassMaterialWithDraftResponses)
	if analysis.AffectedDraftResponseCount < 1 {
		t.Fatalf("expected affected draft responses, got %d", analysis.AffectedDraftResponseCount)
	}
}

func TestE3INT04SubmittedResponsesMaterialWithSubmitted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-04")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-04-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	qID := questionIDByCode(t, ws, "FLEET_SIZE")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: qID, Value: json.RawMessage(`"99"`) }},
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

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-04-fork", func(ctx context.Context, draftID uuid.UUID) {
		required := false
		updateQuestionRequired(t, env, fix, event.ID, draftID, "FLEET_SIZE", &required)
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassMaterialWithSubmitted)
	if analysis.AffectedSubmittedResponseCount < 1 {
		t.Fatalf("expected affected submitted responses, got %d", analysis.AffectedSubmittedResponseCount)
	}
}

func TestE3INT05ScoringChangeScoringAffecting(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-05")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-05-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-05-fork", func(ctx context.Context, draftID uuid.UUID) {
		copyPublishedScoringToDraft(t, env, fix, v1.ID, draftID)
		modifyScoringWeightOnDraft(t, env, fix, draftID, 50)
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassScoringAffecting)
	if !analysis.ScoringAffecting {
		t.Fatal("expected scoring_affecting=true")
	}
}

func TestE3INT06KnockoutChangeKnockoutAffecting(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-06")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-06-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-06-fork", func(ctx context.Context, draftID uuid.UUID) {
		copyPublishedScoringToDraft(t, env, fix, v1.ID, draftID)
		modifyKnockoutRuleOnDraft(t, env, fix, draftID)
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	assertImpactClassesContain(t, analysis.ImpactClasses, domain.ChangeImpactClassKnockoutAffecting)
	if !analysis.KnockoutAffecting {
		t.Fatal("expected knockout_affecting=true")
	}
}

func TestE3INT07MultipleClassesDeterministicOrder(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-07")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-07-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
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

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-07-fork", func(ctx context.Context, draftID uuid.UUID) {
		copyPublishedScoringToDraft(t, env, fix, v1.ID, draftID)
		required := false
		updateQuestionRequired(t, env, fix, event.ID, draftID, "FLEET_SIZE", &required)
		modifyScoringWeightOnDraft(t, env, fix, draftID, 60)
		modifyKnockoutRuleOnDraft(t, env, fix, draftID)
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	want := []string{
		domain.ChangeImpactClassMaterialWithDraftResponses,
		domain.ChangeImpactClassMaterialWithSubmitted,
		domain.ChangeImpactClassScoringAffecting,
		domain.ChangeImpactClassKnockoutAffecting,
	}
	assertImpactClassesOrdered(t, analysis.ImpactClasses, want...)
}

func TestE3INT08StableHashDespiteRowOrder(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-08")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-08-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-08-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})

	first := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_sections SET updated_at = now() + interval '1 second'
		WHERE rfx_version_id = $1 AND tenant_id = $2`, draft.ID, fix.TenantID); err != nil {
		t.Fatalf("shuffle section timestamps: %v", err)
	}
	second := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	if first.CanonicalDiffHash != second.CanonicalDiffHash {
		t.Fatalf("hash not stable: %s vs %s", first.CanonicalDiffHash, second.CanonicalDiffHash)
	}
}

func TestE3INT09PreviewPersistedImmutableNewIDOnSecondPreview(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-09")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-09-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-09-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})

	first := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	firstSnapshot := loadAnalysisSnapshot(t, env, first.ID, fix.TenantID)
	second := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	if first.ID == second.ID {
		t.Fatal("expected new impact analysis id on second preview")
	}
	afterFirst := loadAnalysisSnapshot(t, env, first.ID, fix.TenantID)
	if firstSnapshot.hash != afterFirst.hash || firstSnapshot.classes != afterFirst.classes || firstSnapshot.consumedAt != afterFirst.consumedAt {
		t.Fatalf("first preview row mutated: before=%+v after=%+v", firstSnapshot, afterFirst)
	}
}

func TestE3INT10PreviewAuditEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-10")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-10-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-10-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Fleet size relabeled")
	})

	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id = $1 AND action = 'rfx.change_impact.previewed.v1' AND entity_id = $2`,
		fix.TenantID, analysis.ID).Scan(&count); err != nil {
		t.Fatalf("count preview audit: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one preview audit event, got %d", count)
	}
}

func TestE3INT11CarrierPreviewForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-11")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-11-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-11-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabel")
	})

	_, err := env.versionSvc.PreviewChangeImpact(context.Background(), fix.CarrierAct, event.ID, domain.PreviewChangeImpactInput{
		CandidateVersionID: draft.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE3INT12CrossTenantPreviewNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-12")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-12-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-12-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabel")
	})

	_, err := env.versionSvc.PreviewChangeImpact(context.Background(), fix.CrossTenant, event.ID, domain.PreviewChangeImpactInput{
		CandidateVersionID: draft.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE3INT13ForeignEventCandidateNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	eventA := createDraftEvent(t, env, fix, "RFX-E3-13-A")
	eventB := createDraftEvent(t, env, fix, "RFX-E3-13-B")
	ensurePublishedVersion(t, env, fix, eventA.ID, "e3-int-13-a-v1")
	ensurePublishedVersion(t, env, fix, eventB.ID, "e3-int-13-b-v1")
	draftB := forkAndModifyDraft(t, env, fix, eventB.ID, "e3-int-13-b-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, eventB.ID, draftID, "FLEET_SIZE", "Relabel")
	})

	_, err := env.versionSvc.PreviewChangeImpact(context.Background(), fix.BuyerA, eventA.ID, domain.PreviewChangeImpactInput{
		CandidateVersionID: draftB.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE3INT14NonActiveDraftCandidateConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-14")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-14-v1")

	_, err := env.versionSvc.PreviewChangeImpact(context.Background(), fix.BuyerA, event.ID, domain.PreviewChangeImpactInput{
		CandidateVersionID: published.ID,
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE3INT15PublishWithoutConfirmationWhenResponsesExist422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-15")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-15-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-15-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e3-int-15-pub", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: eventRow.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Missing confirmation",
	})
	appErr := assertAppErrorCode(t, err, apperrors.CodeValidation)
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactRequired {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE3INT16StaleDiffAfterEdit409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-16")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-16-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-16-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)

	updateQuestionLabel(t, env, fix, event.ID, draft.ID, "FLEET_SIZE", "Edited after preview")
	draft, err := reloadDraftForEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	_, err = publishWithConfirmation(t, env, fix, event.ID, "e3-int-16-pub", eventRow, draft, analysis, "Stale diff publish")
	appErr := assertAppErrorCode(t, err, apperrors.CodeConflict)
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactStaleDiff {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE3INT17ExpiredAnalysis422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-17")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-17-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-17-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	expireChangeImpactAnalysis(t, env, analysis.ID, fix.TenantID)

	draft, err := reloadDraftForEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	_, err = publishWithConfirmation(t, env, fix, event.ID, "e3-int-17-pub", eventRow, draft, analysis, "Expired analysis publish")
	appErr := assertAppErrorCode(t, err, apperrors.CodeValidation)
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactExpired {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE3INT18AnalysisWrongEventTenant404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	eventA := createDraftEvent(t, env, fix, "RFX-E3-18-A")
	eventB := createDraftEvent(t, env, fix, "RFX-E3-18-B")
	ensurePublishedVersion(t, env, fix, eventA.ID, "e3-int-18-a-v1")
	ensurePublishedVersion(t, env, fix, eventB.ID, "e3-int-18-b-v1")
	addParticipantAndOpenResponses(t, env, fix, eventA.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, eventA.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response on A: %v", err)
	}
	draftA := forkAndModifyDraft(t, env, fix, eventA.ID, "e3-int-18-a-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, eventA.ID, "NOTE", "Note")
	})
	analysisA := previewChangeImpact(t, env, fix, eventA.ID, draftA.ID, fix.BuyerA)

	draftB := forkAndModifyDraft(t, env, fix, eventB.ID, "e3-int-18-b-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, eventB.ID, "NOTE", "Note")
	})
	eventBRow, err := reloadEvent(t, env, fix, eventB.ID)
	if err != nil {
		t.Fatalf("reload event B: %v", err)
	}

	_, err = publishWithConfirmation(t, env, fix, eventB.ID, "e3-int-18-b-pub", eventBRow, draftB, analysisA, "Wrong event analysis")
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE3INT19SuccessfulConfirmedPublish(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-19")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-19-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-19-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	v2, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-19-pub", eventRow, draft, analysis, "Confirmed publish")
	if err != nil {
		t.Fatalf("confirmed publish: %v", err)
	}
	if v2.Status != domain.RfxVersionStatusPublished || !v2.IsCurrentPublished {
		t.Fatalf("unexpected published version: %+v", v2)
	}
	v1Reload, err := env.qRepo.GetVersionByID(context.Background(), v1.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v1: %v", err)
	}
	if v1Reload.Status != domain.RfxVersionStatusSuperseded {
		t.Fatalf("v1 status=%s", v1Reload.Status)
	}
}

func TestE3INT20PublishConsumesAnalysis(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-20")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-20-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-20-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-20-pub", eventRow, draft, analysis, "Consume analysis"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	consumedAt := loadAnalysisConsumedAt(t, env, analysis.ID, fix.TenantID)
	if consumedAt == nil {
		t.Fatal("expected consumed_at to be set")
	}
}

func TestE3INT21ConsumedAnalysisNewIdempotencyKey409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-21")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-21-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-21-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-21-pub-k1", eventRow, draft, analysis, "First publish"); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	draft2 := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-21-fork-2", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE2", "Note 2")
	})
	eventRow2, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event after fork: %v", err)
	}

	_, err = publishWithConfirmation(t, env, fix, event.ID, "e3-int-21-pub-k2", eventRow2, draft2, analysis, "Reuse consumed analysis")
	appErr := assertAppErrorCode(t, err, apperrors.CodeConflict)
	if got := appErr.Details["code"]; got != domain.VersionLifecycleMachineCodeChangeImpactConsumed {
		t.Fatalf("details.code=%v", got)
	}
}

func TestE3INT22SameKeyBodyReplayAfterConsumption(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-22")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-22-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-22-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e3-int-22-pub"
	first, err := publishWithConfirmation(t, env, fix, event.ID, key, eventRow, draft, analysis, "Replay body")
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	second, err := publishWithConfirmation(t, env, fix, event.ID, key, eventRow, draft, analysis, "Replay body")
	if err != nil {
		t.Fatalf("replay publish: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected idempotent replay version id %s, got %s", first.ID, second.ID)
	}
}

func TestE3INT23SameKeyDifferentBody409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-23")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-23-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-23-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	key := "e3-int-23-pub"
	if _, err := publishWithConfirmation(t, env, fix, event.ID, key, eventRow, draft, analysis, "First body"); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	draft2 := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-23-fork-2", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE2", "Note 2")
	})
	analysis2 := previewChangeImpact(t, env, fix, event.ID, draft2.ID, fix.BuyerA)
	eventRow2, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event after fork: %v", err)
	}

	_, err = publishWithConfirmation(t, env, fix, event.ID, key, eventRow2, draft2, analysis2, "Different body")
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE3INT24ConcurrentPublishOneWinner(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-24")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-24-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-24-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]*domain.RfxVersion, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "e3-int-24-" + string(rune('a'+idx))
			results[idx], errs[idx] = publishWithConfirmation(t, env, fix, event.ID, key, eventRow, draft, analysis, "Concurrent publish")
		}(i)
	}
	wg.Wait()

	success := 0
	for i, err := range errs {
		if err == nil {
			success++
			if results[i] == nil {
				t.Fatal("nil result on success")
			}
			continue
		}
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
			t.Fatalf("unexpected error[%d]: %v", i, err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly one successful publish, got %d", success)
	}
}

func TestE3INT25RollbackLeavesAnalysisUnconsumed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-25")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-25-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-25-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	analysisID := analysis.ID
	hash := analysis.CanonicalDiffHash
	_, err = env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, event.ID, "e3-int-25-pub", domain.PublishQuestionnaireInput{
		ExpectedEventVersion: eventRow.Version,
		ExpectedDraftVersion: draft.Version + 999,
		ChangeSummary:        "Wrong draft version",
		ImpactAnalysisID:     &analysisID,
		CanonicalDiffHash:    hash,
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)

	consumedAt := loadAnalysisConsumedAt(t, env, analysis.ID, fix.TenantID)
	if consumedAt != nil {
		t.Fatal("expected consumed_at to remain NULL after rollback")
	}
}

func TestE3INT26ScoringPublishSetsRescoringRequiredTrue(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-26")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-26-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-26-fork", func(ctx context.Context, draftID uuid.UUID) {
		copyPublishedScoringToDraft(t, env, fix, v1.ID, draftID)
		modifyScoringWeightOnDraft(t, env, fix, draftID, 40)
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	v2, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-26-pub", eventRow, draft, analysis, "Scoring publish")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	reloaded, err := env.qRepo.GetVersionByID(context.Background(), v2.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v2: %v", err)
	}
	if !reloaded.RescoringRequired {
		t.Fatal("expected rescoring_required=true on scoring-affecting publish")
	}
}

func TestE3INT27NonScoringPublishRescoringRequiredFalse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-27")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-27-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	if _, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID); err != nil {
		t.Fatalf("start response: %v", err)
	}
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-27-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Fleet size relabeled")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}

	v2, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-27-pub", eventRow, draft, analysis, "Label-only publish")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	reloaded, err := env.qRepo.GetVersionByID(context.Background(), v2.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v2: %v", err)
	}
	if reloaded.RescoringRequired {
		t.Fatal("expected rescoring_required=false on non-scoring publish")
	}
}

func TestE3INT28OldResponsePinsUnchangedAfterPublish(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-28")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-28-v1")
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	if ws.Response.RfxVersionID == nil || *ws.Response.RfxVersionID != v1.ID {
		t.Fatalf("expected pin to v1 before publish")
	}

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-28-fork", func(ctx context.Context, draftID uuid.UUID) {
		addStructuralQuestion(t, env, fix, event.ID, "NOTE", "Note")
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-28-pub", eventRow, draft, analysis, "Publish v2"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	reloaded, err := env.rfxRepo.GetResponseByEventAndCompany(context.Background(), event.ID, fix.CarrierID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	if reloaded.RfxVersionID == nil || *reloaded.RfxVersionID != v1.ID {
		t.Fatalf("response pin changed: got %v want %s", reloaded.RfxVersionID, v1.ID)
	}
}

func TestE3INT29OldScoreQualificationHistoryUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-29")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-29-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	insertQualificationForResponse(t, env, fix.TenantID, &ws.Response)

	ctx := context.Background()
	var before int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results qr
		INNER JOIN rfx.rfx_responses rr ON rr.id = qr.rfx_response_id
		WHERE rr.rfx_event_id = $1`, event.ID).Scan(&before); err != nil {
		t.Fatalf("count qualification before: %v", err)
	}

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-29-fork", func(ctx context.Context, draftID uuid.UUID) {
		copyPublishedScoringToDraft(t, env, fix, v1.ID, draftID)
		modifyScoringWeightOnDraft(t, env, fix, draftID, 30)
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if _, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-29-pub", eventRow, draft, analysis, "Scoring publish"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var after int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results qr
		INNER JOIN rfx.rfx_responses rr ON rr.id = qr.rfx_response_id
		WHERE rr.rfx_event_id = $1`, event.ID).Scan(&after); err != nil {
		t.Fatalf("count qualification after: %v", err)
	}
	if before != after || before != 1 {
		t.Fatalf("qualification history changed: before=%d after=%d", before, after)
	}
	_ = v1
}

func TestE3INT30NoAutomaticRescore(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-30")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-30-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	qFleet := questionIDByCode(t, ws, "FLEET_SIZE")
	qID := questionIDByCode(t, ws, "HSE_OK")
	if _, err := env.crSvc.SaveAnswers(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers: []domain.AnswerPatchItem{
			{QuestionID: qFleet, Value: json.RawMessage(`"10"`)},
			{QuestionID: qID, Value: json.RawMessage(`true`)},
		},
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
	responseID := ws.Response.ID

	ctx := context.Background()
	var qualCountBefore int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results WHERE tenant_id = $1 AND rfx_response_id = $2`,
		fix.TenantID, responseID).Scan(&qualCountBefore); err != nil {
		t.Fatalf("count qualification before: %v", err)
	}

	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-30-fork", func(ctx context.Context, draftID uuid.UUID) {
		copyPublishedScoringToDraft(t, env, fix, v1.ID, draftID)
		modifyKnockoutRuleOnDraft(t, env, fix, draftID)
	})
	analysis := previewChangeImpact(t, env, fix, event.ID, draft.ID, fix.BuyerA)
	eventRow, err := reloadEvent(t, env, fix, event.ID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	v2, err := publishWithConfirmation(t, env, fix, event.ID, "e3-int-30-pub", eventRow, draft, analysis, "Knockout publish")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	reloadedV2, err := env.qRepo.GetVersionByID(ctx, v2.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v2: %v", err)
	}
	if !reloadedV2.RescoringRequired {
		t.Fatal("expected rescoring_required=true")
	}

	var qualCountAfter int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results WHERE tenant_id = $1 AND rfx_response_id = $2`,
		fix.TenantID, responseID).Scan(&qualCountAfter); err != nil {
		t.Fatalf("count qualification after: %v", err)
	}
	if qualCountBefore != qualCountAfter {
		t.Fatalf("automatic re-score occurred: before=%d after=%d", qualCountBefore, qualCountAfter)
	}
}

func TestE3INT31Migration000069Up(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql"); err != nil {
		t.Fatalf("apply 000068: %v", err)
	}
	if legacyTableExists(t, env, "rfx_change_impact_analyses") {
		t.Fatal("change impact table must not exist before migration 000069")
	}
	if err := applyMigrationFile(ctx, env.pool, "000069_rfx_change_impact_v3_0e3.up.sql"); err != nil {
		t.Fatalf("apply 000069: %v", err)
	}
	if !legacyTableExists(t, env, "rfx_change_impact_analyses") {
		t.Fatal("expected change impact table after migration 000069 up")
	}
	for _, indexName := range []string{
		"idx_rfx_change_impact_event_candidate_created",
		"idx_rfx_change_impact_expires_unconsumed",
		"uq_rfx_events_tenant_id",
	} {
		var exists bool
		if err := env.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'rfx' AND indexname = $1
			)`, indexName).Scan(&exists); err != nil {
			t.Fatalf("check index %s: %v", indexName, err)
		}
		if !exists {
			t.Fatalf("expected index %s after migration 000069 up", indexName)
		}
	}
	for _, constraintName := range []string{
		"fk_rfx_change_impact_event_composite",
		"fk_rfx_change_impact_candidate_composite",
		"fk_rfx_change_impact_source_composite",
	} {
		var exists bool
		if err := env.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = $1
			)`, constraintName).Scan(&exists); err != nil {
			t.Fatalf("check constraint %s: %v", constraintName, err)
		}
		if !exists {
			t.Fatalf("expected constraint %s after migration 000069 up", constraintName)
		}
	}
}

func TestE3INT32Migration000069DownPreservesLegacy(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql"); err != nil {
		t.Fatalf("apply 000068: %v", err)
	}
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-32")
	publishedID := ensurePublishedVersion(t, env, fix, event.ID, "e3-int-32-v1").ID
	if err := applyMigrationFile(ctx, env.pool, "000069_rfx_change_impact_v3_0e3.up.sql"); err != nil {
		t.Fatalf("apply 000069 up: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000069_rfx_change_impact_v3_0e3.down.sql"); err != nil {
		t.Fatalf("apply 000069 down: %v", err)
	}
	if legacyTableExists(t, env, "rfx_change_impact_analyses") {
		t.Fatal("change impact table must be removed after down migration")
	}
	var status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.rfx_versions WHERE id = $1`, publishedID).Scan(&status); err != nil {
		t.Fatalf("reload published version: %v", err)
	}
	if status != domain.RfxVersionStatusPublished {
		t.Fatalf("published version altered by down migration: status=%s", status)
	}
}

func TestE3INT33CompositeTenantEventVersionIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	eventA := createDraftEvent(t, env, fix, "RFX-E3-33-A")
	eventB := createDraftEvent(t, env, fix, "RFX-E3-33-B")
	v1A := ensurePublishedVersion(t, env, fix, eventA.ID, "e3-int-33-a-v1")
	ensurePublishedVersion(t, env, fix, eventB.ID, "e3-int-33-b-v1")
	draftB := forkAndModifyDraft(t, env, fix, eventB.ID, "e3-int-33-b-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, eventB.ID, draftID, "FLEET_SIZE", "Relabel")
	})

	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_change_impact_analyses (
			id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, '[]'::jsonb, now() + interval '15 minutes')`,
		uuid.New(), fix.TenantID, eventA.ID, v1A.ID, draftB.ID, fix.BuyerA.UserID, "deadbeef")
	assertPostgreSQLErrorCode(t, err, "23503")
}

func TestE3INT34OpenAPIHandlerHTTPStatusParity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E3-34")
	ensurePublishedVersion(t, env, fix, event.ID, "e3-int-34-v1")
	draft := forkAndModifyDraft(t, env, fix, event.ID, "e3-int-34-fork", func(ctx context.Context, draftID uuid.UUID) {
		updateQuestionLabel(t, env, fix, event.ID, draftID, "FLEET_SIZE", "Relabel")
	})

	handler := handlers.NewVersionLifecycleHandler(env.versionSvc)
	r := chi.NewRouter()
	r.Post("/v1/rfx-events/{id}/change-impact/preview", handler.PreviewChangeImpact)

	body, err := json.Marshal(domain.PreviewChangeImpactInput{CandidateVersionID: draft.ID})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/"+event.ID.String()+"/change-impact/preview", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("buyer preview HTTP status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}

	reqCarrier := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/"+event.ID.String()+"/change-impact/preview", bytes.NewReader(body))
	reqCarrier.Header.Set("X-Tenant-ID", fix.TenantID.String())
	reqCarrier.Header.Set("X-User-ID", fix.CarrierAct.UserID.String())
	recCarrier := httptest.NewRecorder()
	r.ServeHTTP(recCarrier, reqCarrier)
	if recCarrier.Code != http.StatusForbidden {
		t.Fatalf("carrier preview HTTP status=%d want=403 body=%s", recCarrier.Code, recCarrier.Body.String())
	}
}

type analysisSnapshot struct {
	hash       string
	classes    string
	consumedAt *time.Time
}

func previewChangeImpact(t *testing.T, env *testEnv, fix buyerFixture, eventID, candidateID uuid.UUID, actor domain.ActorContext) *domain.ChangeImpactAnalysis {
	t.Helper()
	analysis, err := env.versionSvc.PreviewChangeImpact(context.Background(), actor, eventID, domain.PreviewChangeImpactInput{
		CandidateVersionID: candidateID,
	})
	if err != nil {
		t.Fatalf("preview change impact: %v", err)
	}
	return analysis
}

func publishWithConfirmation(
	t *testing.T,
	env *testEnv,
	fix buyerFixture,
	eventID uuid.UUID,
	key string,
	event *domain.RfxEvent,
	draft *domain.RfxVersion,
	analysis *domain.ChangeImpactAnalysis,
	summary string,
) (*domain.RfxVersion, error) {
	t.Helper()
	analysisID := analysis.ID
	return env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, eventID, key, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        summary,
		ImpactAnalysisID:     &analysisID,
		CanonicalDiffHash:    analysis.CanonicalDiffHash,
	})
}

func forkAndModifyDraft(
	t *testing.T,
	env *testEnv,
	fix buyerFixture,
	eventID uuid.UUID,
	forkKey string,
	modify func(ctx context.Context, draftID uuid.UUID),
) *domain.RfxVersion {
	t.Helper()
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, eventID, forkKey)
	if err != nil {
		t.Fatalf("fork draft: %v", err)
	}
	if modify != nil {
		modify(context.Background(), draft.ID)
	}
	draft, err = reloadDraftForEvent(t, env, fix, eventID)
	if err != nil {
		t.Fatalf("reload draft after modify: %v", err)
	}
	return draft
}

func reloadEvent(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) (*domain.RfxEvent, error) {
	t.Helper()
	return env.rfxRepo.GetEventByID(context.Background(), eventID, fix.TenantID)
}

func reloadDraftForEvent(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) (*domain.RfxVersion, error) {
	t.Helper()
	state, err := env.qRepo.GetEventVersionState(context.Background(), eventID, fix.TenantID)
	if err != nil {
		return nil, err
	}
	if state.DraftVersionID == nil {
		t.Fatal("expected active draft version")
	}
	return env.qRepo.GetVersionByID(context.Background(), *state.DraftVersionID, fix.TenantID)
}

func updateQuestionLabel(t *testing.T, env *testEnv, fix buyerFixture, eventID, draftID uuid.UUID, questionCode, label string) {
	t.Helper()
	ctx := context.Background()
	q := mustQuestionByCode(t, env, fix, draftID, questionCode)
	labelCopy := label
	if _, err := env.qSvc.UpdateQuestion(ctx, fix.BuyerA, eventID, q.ID, domain.UpdateQuestionInput{
		Label:           &labelCopy,
		ExpectedVersion: q.Version,
	}); err != nil {
		t.Fatalf("update question label %s: %v", questionCode, err)
	}
}

func updateQuestionRequired(t *testing.T, env *testEnv, fix buyerFixture, eventID, draftID uuid.UUID, questionCode string, required *bool) {
	t.Helper()
	ctx := context.Background()
	q := mustQuestionByCode(t, env, fix, draftID, questionCode)
	if _, err := env.qSvc.UpdateQuestion(ctx, fix.BuyerA, eventID, q.ID, domain.UpdateQuestionInput{
		Required:        required,
		ExpectedVersion: q.Version,
	}); err != nil {
		t.Fatalf("update question required %s: %v", questionCode, err)
	}
}

func addStructuralQuestion(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, code, label string) {
	t.Helper()
	ctx := context.Background()
	section, err := env.qSvc.CreateSection(ctx, fix.BuyerA, eventID, domain.CreateSectionInput{
		SectionCode: "SEC_" + code,
		Title:       "Section " + code,
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, eventID, section.ID, domain.CreateQuestionInput{
		QuestionCode: code,
		QuestionType: domain.QuestionTypeText,
		Label:        label,
		Required:     true,
	}); err != nil {
		t.Fatalf("create question %s: %v", code, err)
	}
}

func modifyScoringWeight(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, weight float64) {
	t.Helper()
	state, err := env.qRepo.GetEventVersionState(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("event state: %v", err)
	}
	if state.DraftVersionID == nil || state.PublishedVersionID == nil {
		t.Fatal("expected published and draft versions for scoring modification")
	}
	copyPublishedScoringToDraft(t, env, fix, *state.PublishedVersionID, *state.DraftVersionID)
	modifyScoringWeightOnDraft(t, env, fix, *state.DraftVersionID, weight)
}

func modifyKnockoutRule(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) {
	t.Helper()
	state, err := env.qRepo.GetEventVersionState(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("event state: %v", err)
	}
	if state.DraftVersionID == nil || state.PublishedVersionID == nil {
		t.Fatal("expected published and draft versions for scoring modification")
	}
	copyPublishedScoringToDraft(t, env, fix, *state.PublishedVersionID, *state.DraftVersionID)
	modifyKnockoutRuleOnDraft(t, env, fix, *state.DraftVersionID)
}

func copyPublishedScoringToDraft(t *testing.T, env *testEnv, fix buyerFixture, publishedVersionID, draftVersionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	sourceQ, err := env.qRepo.LoadQuestionnaire(ctx, publishedVersionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load source questionnaire: %v", err)
	}
	targetQ, err := env.qRepo.LoadQuestionnaire(ctx, draftVersionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load target questionnaire: %v", err)
	}
	questionIDMap := mapQuestionIDsByCode(*sourceQ, *targetQ)
	if err := env.scoreRepo.CopyDraftScoringFromSource(ctx, fix.TenantID, publishedVersionID, draftVersionID, questionIDMap, sourceQ, targetQ); err != nil {
		t.Fatalf("copy draft scoring: %v", err)
	}
}

func mapQuestionIDsByCode(source, target domain.QuestionnaireDefinition) map[uuid.UUID]uuid.UUID {
	sourceByCode := map[string]uuid.UUID{}
	for _, section := range source.Sections {
		for _, q := range section.Questions {
			sourceByCode[section.Section.SectionCode+"\x00"+q.QuestionCode] = q.ID
		}
	}
	out := make(map[uuid.UUID]uuid.UUID)
	for _, section := range target.Sections {
		for _, q := range section.Questions {
			key := section.Section.SectionCode + "\x00" + q.QuestionCode
			if sourceID, ok := sourceByCode[key]; ok {
				out[sourceID] = q.ID
			}
		}
	}
	return out
}

func modifyScoringWeightOnDraft(t *testing.T, env *testEnv, fix buyerFixture, draftVersionID uuid.UUID, weight float64) {
	t.Helper()
	ctx := context.Background()
	model, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draftVersionID)
	if err != nil {
		t.Fatalf("load draft score model: %v", err)
	}
	criteria := []domain.ScoreCriterion{{
		CriterionCode:     "HSE",
		Name:              "HSE",
		Weight:            weight,
		SortOrder:         1,
		NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
	}}
	questionID := mustQuestionByCode(t, env, fix, draftVersionID, "HSE_OK").ID
	criterionID := uuid.New()
	criteria[0].ID = criterionID
	bindings := []domain.ScoreBinding{{
		CriterionID:      criterionID,
		QuestionID:       questionID,
		BindingType:      "AUTOMATIC",
		ScoringRuleJSON:  json.RawMessage(`{"type":"BOOLEAN_MAP"}`),
		KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`),
	}}
	if err := env.scoreRepo.ReplaceDraftDefinition(ctx, model.ID, fix.TenantID, criteria, bindings); err != nil {
		t.Fatalf("replace draft score definition: %v", err)
	}
}

func modifyKnockoutRuleOnDraft(t *testing.T, env *testEnv, fix buyerFixture, draftVersionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	model, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draftVersionID)
	if err != nil {
		t.Fatalf("load draft score model: %v", err)
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
		KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":true}`),
	}}
	if err := env.scoreRepo.ReplaceDraftDefinition(ctx, model.ID, fix.TenantID, criteria, bindings); err != nil {
		t.Fatalf("replace draft knockout definition: %v", err)
	}
}

func mustQuestionByCode(t *testing.T, env *testEnv, fix buyerFixture, versionID uuid.UUID, questionCode string) domain.Question {
	t.Helper()
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), versionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load questionnaire: %v", err)
	}
	for _, section := range graph.Sections {
		for _, q := range section.Questions {
			if q.QuestionCode == questionCode {
				return q
			}
		}
	}
	t.Fatalf("question code %s not found", questionCode)
	return domain.Question{}
}

func questionIDByCode(t *testing.T, ws *domain.CarrierResponseWorkspace, questionCode string) uuid.UUID {
	t.Helper()
	for _, section := range ws.Questionnaire.Sections {
		for _, q := range section.Questions {
			if q.QuestionCode == questionCode {
				return q.ID
			}
		}
	}
	t.Fatalf("question code %s not found in workspace", questionCode)
	return uuid.Nil
}

func seedSecondCarrier(t *testing.T, env *testEnv, fix buyerFixture) (uuid.UUID, domain.ActorContext) {
	t.Helper()
	ctx := context.Background()
	carrierID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		carrierID, fix.TenantID, "Carrier B", "CARRIER"); err != nil {
		t.Fatalf("seed carrier b: %v", err)
	}
	actor := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`,
		actor.UserID, fix.TenantID, "carrier-b-e3@test.local", "carrier-b-e3@test.local"); err != nil {
		t.Fatalf("seed carrier b user: %v", err)
	}
	var carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`,
		fix.TenantID, carrierID, actor.UserID); err != nil {
		t.Fatalf("membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		fix.TenantID, actor.UserID, carrierID, carrierRoleID); err != nil {
		t.Fatalf("role: %v", err)
	}
	return carrierID, actor
}

func addSecondCarrierParticipant(t *testing.T, env *testEnv, fix buyerFixture, eventID, carrierID uuid.UUID) {
	t.Helper()
	if _, err := env.rfxSvc.AddParticipant(context.Background(), fix.BuyerA, eventID, domain.AddRfxParticipantInput{
		TenantID:        fix.TenantID,
		RfxEventID:      eventID,
		CompanyID:       carrierID,
		ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add second carrier participant: %v", err)
	}
}

func assertImpactClassesOrdered(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("impact classes length mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("impact class order mismatch at %d: got=%v want=%v", i, got, want)
		}
	}
}

func assertImpactClassesContain(t *testing.T, got []string, class string) {
	t.Helper()
	for _, item := range got {
		if item == class {
			return
		}
	}
	t.Fatalf("expected impact class %s in %v", class, got)
}

func loadAnalysisSnapshot(t *testing.T, env *testEnv, id, tenantID uuid.UUID) analysisSnapshot {
	t.Helper()
	repo := repository.NewChangeImpactRepository(env.pool)
	analysis, err := repo.GetByID(context.Background(), id, tenantID)
	if err != nil {
		t.Fatalf("load analysis: %v", err)
	}
	classesJSON, err := json.Marshal(analysis.ImpactClasses)
	if err != nil {
		t.Fatalf("marshal classes: %v", err)
	}
	return analysisSnapshot{
		hash:       analysis.CanonicalDiffHash,
		classes:    string(classesJSON),
		consumedAt: analysis.ConsumedAt,
	}
}

func loadAnalysisConsumedAt(t *testing.T, env *testEnv, id, tenantID uuid.UUID) *time.Time {
	t.Helper()
	var consumedAt *time.Time
	if err := env.pool.QueryRow(context.Background(), `
		SELECT consumed_at FROM rfx.rfx_change_impact_analyses WHERE id = $1 AND tenant_id = $2`,
		id, tenantID).Scan(&consumedAt); err != nil {
		t.Fatalf("load consumed_at: %v", err)
	}
	return consumedAt
}

func expireChangeImpactAnalysis(t *testing.T, env *testEnv, id, tenantID uuid.UUID) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_change_impact_analyses
		SET expires_at = now() - interval '1 minute'
		WHERE id = $1 AND tenant_id = $2`, id, tenantID); err != nil {
		t.Fatalf("expire analysis: %v", err)
	}
}
