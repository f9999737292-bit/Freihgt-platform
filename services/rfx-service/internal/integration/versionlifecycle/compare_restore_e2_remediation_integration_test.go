//go:build integration

package versionlifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/http/handlers"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE2INT14RestoreIdempotencySameSourceSameBodyReplay(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-14")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-14-pub")
	in := domain.RestoreVersionAsDraftInput{ChangeSummary: "Same source replay"}
	key := "e2-int-14-key"

	first, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, in)
	if err != nil {
		t.Fatalf("first restore: %v", err)
	}
	second, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, in)
	if err != nil {
		t.Fatalf("replay restore: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same draft ID on replay, got %s vs %s", first.ID, second.ID)
	}
}

func TestE2INT15RestoreSameKeyDifferentSource409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-15")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-15-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-15-v2", "e2-int-15-fork")
	key := "e2-int-15-key"
	summary := domain.RestoreVersionAsDraftInput{ChangeSummary: "Shared summary"}

	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, v1.ID, key, summary); err != nil {
		t.Fatalf("restore v1: %v", err)
	}
	_, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, v2.ID, key, summary)
	assertAppErrorCode(t, err, apperrors.CodeConflict)

	var draftCount int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND status = 'DRAFT' AND deleted_at IS NULL`,
		event.ID, fix.TenantID).Scan(&draftCount); err != nil {
		t.Fatalf("count drafts: %v", err)
	}
	if draftCount != 1 {
		t.Fatalf("expected exactly one draft, got %d", draftCount)
	}
}

func TestE2INT16RestoreSameKeySameSourceDifferentSummary409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-16")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-16-pub")
	key := "e2-int-16-key"

	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, domain.RestoreVersionAsDraftInput{
		ChangeSummary: "First summary",
	}); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	_, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, key, domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Different summary",
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE2INT17RestoreWithoutScoringNoScoreModel(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-17")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-17-pub")

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-17-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Restore without scoring",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertFullGraphEqual(t, env, fix, published.ID, draft.ID, false)
}

func TestE2INT18RestoreWithPublishedScoringCreatesDraftModel(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-18")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-18-pub")
	attachPublishedScoringToEvent(t, env, fix, event.ID)

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-18-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Restore with scoring",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertFullGraphEqual(t, env, fix, published.ID, draft.ID, true)
}

func TestE2INT19RestoreCriteriaCopiedWithNewIDs(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-19")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-19-pub")
	attachPublishedScoringToEvent(t, env, fix, event.ID)

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-19-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Criteria copy",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	ctx := context.Background()
	sourceModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("source model: %v", err)
	}
	targetModel, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("target model: %v", err)
	}
	sourceCriteria, err := env.scoreRepo.ListCriteriaByModel(ctx, sourceModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("source criteria: %v", err)
	}
	targetCriteria, err := env.scoreRepo.ListCriteriaByModel(ctx, targetModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("target criteria: %v", err)
	}
	if len(sourceCriteria) != len(targetCriteria) {
		t.Fatalf("criteria count mismatch")
	}
	for i, source := range sourceCriteria {
		target := targetCriteria[i]
		if source.ID == target.ID {
			t.Fatalf("criterion ID reused for %s", source.CriterionCode)
		}
		if source.CriterionCode != target.CriterionCode || source.Weight != target.Weight {
			t.Fatalf("criterion config mismatch for %s", source.CriterionCode)
		}
		if string(source.NormalizationJSON) != string(target.NormalizationJSON) {
			t.Fatalf("normalization_json mismatch for %s", source.CriterionCode)
		}
	}
}

func TestE2INT20RestoreBindingsRemappedToRestoredQuestions(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-20")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-20-pub")
	attachPublishedScoringToEvent(t, env, fix, event.ID)

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-20-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Binding remap",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	ctx := context.Background()
	sourceGraph, err := env.qRepo.LoadQuestionnaire(ctx, published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("source graph: %v", err)
	}
	targetGraph, err := env.qRepo.LoadQuestionnaire(ctx, draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("target graph: %v", err)
	}
	sourceModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("source model: %v", err)
	}
	targetModel, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("target model: %v", err)
	}
	sourceBindings, err := env.scoreRepo.ListBindingsByModel(ctx, sourceModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("source bindings: %v", err)
	}
	targetBindings, err := env.scoreRepo.ListBindingsByModel(ctx, targetModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("target bindings: %v", err)
	}
	if len(sourceBindings) != len(targetBindings) {
		t.Fatalf("binding count mismatch")
	}
	sourceQuestions := questionCodeByID(sourceGraph)
	targetQuestions := questionCodeByID(targetGraph)
	for i, sourceBinding := range sourceBindings {
		targetBinding := targetBindings[i]
		if sourceQuestions[sourceBinding.QuestionID] != targetQuestions[targetBinding.QuestionID] {
			t.Fatalf("binding question code mismatch")
		}
		if sourceBinding.QuestionID == targetBinding.QuestionID {
			t.Fatal("binding reused source question ID")
		}
	}
}

func TestE2INT21RestoreScoringJSONPreserved(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-21")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-21-pub")
	attachPublishedScoringToEvent(t, env, fix, event.ID)

	draft, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-21-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "JSON preserve",
	})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	ctx := context.Background()
	sourceModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("source model: %v", err)
	}
	targetModel, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, draft.ID)
	if err != nil {
		t.Fatalf("target model: %v", err)
	}
	sourceBindings, err := env.scoreRepo.ListBindingsByModel(ctx, sourceModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("source bindings: %v", err)
	}
	targetBindings, err := env.scoreRepo.ListBindingsByModel(ctx, targetModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("target bindings: %v", err)
	}
	for i, source := range sourceBindings {
		target := targetBindings[i]
		if string(source.ScoringRuleJSON) != string(target.ScoringRuleJSON) {
			t.Fatalf("scoring_rule_json mismatch")
		}
		if string(source.KnockoutRuleJSON) != string(target.KnockoutRuleJSON) {
			t.Fatalf("knockout_rule_json mismatch")
		}
		if source.BindingType != target.BindingType {
			t.Fatalf("binding_type mismatch")
		}
	}
}

func TestE2INT22RestoreSourceScoringUnchanged(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-22")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-22-pub")
	attachPublishedScoringToEvent(t, env, fix, event.ID)

	ctx := context.Background()
	beforeModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("before model: %v", err)
	}
	beforeCriteria, err := env.scoreRepo.ListCriteriaByModel(ctx, beforeModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("before criteria: %v", err)
	}
	beforeBindings, err := env.scoreRepo.ListBindingsByModel(ctx, beforeModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("before bindings: %v", err)
	}

	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-22-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Leave source untouched",
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}

	afterModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("after model: %v", err)
	}
	if beforeModel.ID != afterModel.ID || beforeModel.Status != afterModel.Status || beforeModel.ModelVersion != afterModel.ModelVersion {
		t.Fatalf("source score model changed: before=%+v after=%+v", beforeModel, afterModel)
	}
	afterCriteria, err := env.scoreRepo.ListCriteriaByModel(ctx, afterModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("after criteria: %v", err)
	}
	afterBindings, err := env.scoreRepo.ListBindingsByModel(ctx, afterModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("after bindings: %v", err)
	}
	if len(beforeCriteria) != len(afterCriteria) || len(beforeBindings) != len(afterBindings) {
		t.Fatal("source criteria/bindings count changed")
	}
	for i := range beforeCriteria {
		if beforeCriteria[i].ID != afterCriteria[i].ID {
			t.Fatalf("source criterion ID changed at %d", i)
		}
	}
	for i := range beforeBindings {
		if beforeBindings[i].ID != afterBindings[i].ID {
			t.Fatalf("source binding ID changed at %d", i)
		}
	}
}

func TestE2INT23RestorePreservesResponseScoresAndQualification(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-23")
	published := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-23-pub")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	addParticipantAndOpenResponses(t, env, fix, event.ID)
	ws, err := env.crSvc.StartOrResume(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response: %v", err)
	}
	insertQualificationForResponse(t, env, fix.TenantID, &ws.Response)

	ctx := context.Background()
	beforeModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("source model: %v", err)
	}
	beforeCriteriaCount := len(mustListCriteria(t, env, beforeModel.ID, fix.TenantID))
	beforeBindingsCount := len(mustListBindings(t, env, beforeModel.ID, fix.TenantID))

	if _, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, published.ID, "e2-int-23-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Preserve response history",
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}

	var scoreCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results WHERE tenant_id = $1 AND rfx_response_id = $2`,
		fix.TenantID, ws.Response.ID).Scan(&scoreCount); err != nil {
		t.Fatalf("count qualification: %v", err)
	}
	if scoreCount != 1 {
		t.Fatalf("qualification history changed: count=%d", scoreCount)
	}
	afterModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, published.ID)
	if err != nil {
		t.Fatalf("after source model: %v", err)
	}
	if len(mustListCriteria(t, env, afterModel.ID, fix.TenantID)) != beforeCriteriaCount {
		t.Fatal("source criteria count changed after restore")
	}
	if len(mustListBindings(t, env, afterModel.ID, fix.TenantID)) != beforeBindingsCount {
		t.Fatal("source bindings count changed after restore")
	}
}

func TestE2INT24UnresolvableScoringBindingRollsBack(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-24")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-int-24-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-24-v2", "e2-int-24-fork")

	ctx := context.Background()
	orphanQuestionID := mustFirstQuestionID(t, env, v2.ID, fix.TenantID)
	insertOrphanScoringBinding(t, env, fix, v1.ID, orphanQuestionID)

	before := captureCompareWriteSnapshot(t, env, fix.TenantID, event.ID)
	var restoreAuditBefore int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id = $1 AND action = 'rfx.version.restored_as_draft.v1'`,
		fix.TenantID).Scan(&restoreAuditBefore); err != nil {
		t.Fatalf("count restore audit before: %v", err)
	}
	var draftPointer *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT draft_version_id FROM rfx.rfx_events WHERE id = $1 AND tenant_id = $2`, event.ID, fix.TenantID).Scan(&draftPointer); err != nil {
		t.Fatalf("draft pointer: %v", err)
	}
	if draftPointer != nil {
		t.Fatal("expected no draft before failed restore")
	}

	_, err := env.versionSvc.RestoreVersionAsDraft(context.Background(), fix.BuyerA, event.ID, v1.ID, "e2-int-24-restore", domain.RestoreVersionAsDraftInput{
		ChangeSummary: "Should rollback",
	})
	if err == nil {
		t.Fatal("expected restore failure")
	}

	after := captureCompareWriteSnapshot(t, env, fix.TenantID, event.ID)
	assertCompareWriteSnapshotEqual(t, before, after)
	if err := env.pool.QueryRow(ctx, `SELECT draft_version_id FROM rfx.rfx_events WHERE id = $1 AND tenant_id = $2`, event.ID, fix.TenantID).Scan(&draftPointer); err != nil {
		t.Fatalf("draft pointer after: %v", err)
	}
	if draftPointer != nil {
		t.Fatal("draft pointer set after rollback")
	}
	var restoreAuditAfter int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id = $1 AND action = 'rfx.version.restored_as_draft.v1'`,
		fix.TenantID).Scan(&restoreAuditAfter); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	if restoreAuditAfter != restoreAuditBefore {
		t.Fatalf("restore audit events changed: before=%d after=%d", restoreAuditBefore, restoreAuditAfter)
	}
	var idemCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records WHERE tenant_id = $1 AND aggregate_scope = $2`,
		fix.TenantID, event.ID).Scan(&idemCount); err != nil {
		t.Fatalf("idempotency count: %v", err)
	}
	if idemCount != before.idempotencyCount {
		t.Fatalf("idempotency records changed: before=%d after=%d", before.idempotencyCount, idemCount)
	}
}

func TestE2INT25CompareReadOnlyNoWrites(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-25")
	v1 := ensureRichPublishedVersion(t, env, fix, event.ID, "e2-int-25-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-25-v2", "e2-int-25-fork")

	before := captureCompareWriteSnapshot(t, env, fix.TenantID, event.ID)
	result, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if result.CanonicalDiffHash == "" {
		t.Fatal("expected compare result")
	}
	after := captureCompareWriteSnapshot(t, env, fix.TenantID, event.ID)
	assertCompareWriteSnapshotEqual(t, before, after)
}

func TestE2INT26CompareIncludesOptionsAndRules(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-26")
	v1 := ensureRichPublishedVersion(t, env, fix, event.ID, "e2-int-26-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-26-v2", "e2-int-26-fork")

	result, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if len(result.Options) == 0 {
		t.Fatal("expected option diffs from persisted graph")
	}
	if len(result.Rules) == 0 {
		t.Fatal("expected rule diffs from persisted graph")
	}
}

func TestE2INT27CompareIncludesScoringChanges(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-27")
	v1 := ensureRichPublishedVersion(t, env, fix, event.ID, "e2-int-27-v1")
	attachPublishedScoringToEvent(t, env, fix, event.ID)
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-27-v2", "e2-int-27-fork")

	result, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if len(result.Scoring.Criteria) == 0 && result.Scoring.Model == nil {
		t.Fatal("expected scoring diffs from persisted graph")
	}
}

func TestE2INT28EquivalentGraphsSameCanonicalHash(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-28")
	v1 := ensureRichPublishedVersion(t, env, fix, event.ID, "e2-int-28-v1")
	v2 := publishIdenticalSecondVersion(t, env, fix, event.ID, "e2-int-28-v2", "e2-int-28-fork")

	in := domain.CompareVersionsInput{SourceVersionID: v1.ID, TargetVersionID: v2.ID}
	first, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, in)
	if err != nil {
		t.Fatalf("first compare: %v", err)
	}
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_sections SET updated_at = now() + interval '1 second'
		WHERE rfx_version_id = $1 AND tenant_id = $2`, v2.ID, fix.TenantID); err != nil {
		t.Fatalf("shuffle section timestamps: %v", err)
	}
	second, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, in)
	if err != nil {
		t.Fatalf("second compare: %v", err)
	}
	if first.CanonicalDiffHash != second.CanonicalDiffHash {
		t.Fatalf("hash not deterministic across reload: %s vs %s", first.CanonicalDiffHash, second.CanonicalDiffHash)
	}
}

func TestE2INT29ReverseCompareDeterministicSemantics(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-29")
	v1 := ensureRichPublishedVersion(t, env, fix, event.ID, "e2-int-29-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-int-29-v2", "e2-int-29-fork")

	forward, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	if err != nil {
		t.Fatalf("forward compare: %v", err)
	}
	reverse, err := env.versionSvc.CompareVersions(context.Background(), fix.BuyerA, event.ID, domain.CompareVersionsInput{
		SourceVersionID: v2.ID,
		TargetVersionID: v1.ID,
	})
	if err != nil {
		t.Fatalf("reverse compare: %v", err)
	}
	if forward.Summary.AddedCount != reverse.Summary.RemovedCount {
		t.Fatalf("added/removed mismatch: forward added=%d reverse removed=%d", forward.Summary.AddedCount, reverse.Summary.RemovedCount)
	}
	if forward.Summary.RemovedCount != reverse.Summary.AddedCount {
		t.Fatalf("removed/added mismatch: forward removed=%d reverse added=%d", forward.Summary.RemovedCount, reverse.Summary.AddedCount)
	}
}

func TestE2CompareHTTPReturns200(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E2-HTTP")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e2-http-v1")
	v2 := publishSecondVersion(t, env, fix, event.ID, "e2-http-v2", "e2-http-fork")

	handler := handlers.NewVersionLifecycleHandler(env.versionSvc)
	r := chi.NewRouter()
	r.Post("/v1/rfx-events/{id}/versions/compare", handler.CompareVersions)

	body, err := json.Marshal(domain.CompareVersionsInput{
		SourceVersionID: v1.ID,
		TargetVersionID: v2.ID,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/"+event.ID.String()+"/versions/compare", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("compare HTTP status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
}

func publishIdenticalSecondVersion(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, publishKey, forkKey string) *domain.RfxVersion {
	t.Helper()
	draft, err := env.versionSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, eventID, forkKey)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	draft, err = env.qRepo.GetVersionByID(context.Background(), draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	event, err := env.rfxRepo.GetEventByID(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	published, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, eventID, publishKey, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Identical republish",
	})
	if err != nil {
		t.Fatalf("publish identical v2: %v", err)
	}
	return published
}

func insertOrphanScoringBinding(t *testing.T, env *testEnv, fix buyerFixture, sourceVersionID, orphanQuestionID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	model, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, sourceVersionID)
	if err != nil {
		t.Fatalf("source model: %v", err)
	}
	criteria, err := env.scoreRepo.ListCriteriaByModel(ctx, model.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("criteria: %v", err)
	}
	if len(criteria) == 0 {
		t.Fatal("expected source criteria")
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_score_bindings (
			id, tenant_id, score_model_id, criterion_id, question_id, binding_type,
			scoring_rule_json, knockout_rule_json
		) VALUES ($1, $2, $3, $4, $5, 'QUESTION', '{}'::jsonb, NULL)`,
		uuid.New(), fix.TenantID, model.ID, criteria[0].ID, orphanQuestionID); err != nil {
		t.Fatalf("insert orphan binding: %v", err)
	}
}

func mustFirstQuestionID(t *testing.T, env *testEnv, versionID, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), versionID, tenantID)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	for _, section := range graph.Sections {
		if len(section.Questions) > 0 {
			return section.Questions[0].ID
		}
	}
	t.Fatal("no questions found")
	return uuid.Nil
}

func mustListCriteria(t *testing.T, env *testEnv, modelID, tenantID uuid.UUID) []domain.ScoreCriterion {
	t.Helper()
	criteria, err := env.scoreRepo.ListCriteriaByModel(context.Background(), modelID, tenantID)
	if err != nil {
		t.Fatalf("list criteria: %v", err)
	}
	return criteria
}

func mustListBindings(t *testing.T, env *testEnv, modelID, tenantID uuid.UUID) []domain.ScoreBinding {
	t.Helper()
	bindings, err := env.scoreRepo.ListBindingsByModel(context.Background(), modelID, tenantID)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	return bindings
}

func questionCodeByID(graph *domain.QuestionnaireDefinition) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string)
	for _, section := range graph.Sections {
		for _, question := range section.Questions {
			out[question.ID] = question.QuestionCode
		}
	}
	return out
}

func assertCompareWriteSnapshotEqual(t *testing.T, before, after compareWriteSnapshot) {
	t.Helper()
	if before.versionCount != after.versionCount {
		t.Fatalf("version count changed: %d -> %d", before.versionCount, after.versionCount)
	}
	if before.sectionCount != after.sectionCount {
		t.Fatalf("section count changed: %d -> %d", before.sectionCount, after.sectionCount)
	}
	if before.questionCount != after.questionCount {
		t.Fatalf("question count changed: %d -> %d", before.questionCount, after.questionCount)
	}
	if before.optionCount != after.optionCount {
		t.Fatalf("option count changed: %d -> %d", before.optionCount, after.optionCount)
	}
	if before.ruleCount != after.ruleCount {
		t.Fatalf("rule count changed: %d -> %d", before.ruleCount, after.ruleCount)
	}
	if before.scoreModelCount != after.scoreModelCount {
		t.Fatalf("score model count changed: %d -> %d", before.scoreModelCount, after.scoreModelCount)
	}
	if before.auditCount != after.auditCount {
		t.Fatalf("audit count changed: %d -> %d", before.auditCount, after.auditCount)
	}
	if before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("idempotency count changed: %d -> %d", before.idempotencyCount, after.idempotencyCount)
	}
	if !before.versionUpdatedMax.Equal(after.versionUpdatedMax) {
		t.Fatalf("version updated_at changed: %v -> %v", before.versionUpdatedMax, after.versionUpdatedMax)
	}
}
