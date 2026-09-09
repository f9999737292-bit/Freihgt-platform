//go:build integration

package templatelibrary

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestE5REM007CompleteCanonicalGraphEquality(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedRichTemplate(t, env, fix, "e5-rem-007", nil)
	tmplCanonical, tmplGraph := loadTemplateGraphSnapshot(t, env, fix, detail.Template.ID, published.ID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-REM-007", fix.CompanyA), uuid.NewString())
	eventCanonical, eventGraph := loadEventGraphSnapshot(t, env, fix, result.DraftVersion.ID)
	assertCanonicalGraphEqual(t, tmplCanonical, eventCanonical)
	assertTemplateAndEventCanonicalEqual(t, tmplGraph, eventGraph)
}

func TestE5REM008AllGraphUUIDSetsDisjoint(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedRichTemplate(t, env, fix, "e5-rem-008", nil)
	_, tmplGraph := loadTemplateGraphSnapshot(t, env, fix, detail.Template.ID, published.ID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-REM-008", fix.CompanyA), uuid.NewString())
	_, eventGraph := loadEventGraphSnapshot(t, env, fix, result.DraftVersion.ID)
	assertGraphUUIDSetsDisjoint(t, tmplGraph, eventGraph)
	assertEventGraphUsesNoTemplateUUIDs(t, tmplGraph, eventGraph)
}

func TestE5REM009TemplateMutationCannotAffectClonedEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedRichTemplate(t, env, fix, "e5-rem-009", nil)
	_, tmplGraph := loadTemplateGraphSnapshot(t, env, fix, detail.Template.ID, published.ID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-REM-009", fix.CompanyA), uuid.NewString())
	eventBefore, _ := loadEventGraphSnapshot(t, env, fix, result.DraftVersion.ID)
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, uuid.NewString()); err != nil {
		t.Fatalf("fork template: %v", err)
	}
	mutateTemplateDraftGraph(t, env, fix, detail.Template.ID)
	eventAfter, eventGraph := loadEventGraphSnapshot(t, env, fix, result.DraftVersion.ID)
	assertCanonicalGraphEqual(t, eventBefore, eventAfter)
	assertEventGraphUsesNoTemplateUUIDs(t, tmplGraph, eventGraph)
}

func TestE5REM010EventMutationCannotAffectSourceTemplate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedRichTemplate(t, env, fix, "e5-rem-010", nil)
	_, tmplGraphDef := loadTemplateGraphSnapshot(t, env, fix, detail.Template.ID, published.ID)
	templateBefore := capturePublishedTemplateContract(tmplGraphDef, published)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-REM-010", fix.CompanyA), uuid.NewString())
	mutateEventDraftGraph(t, env, fix, result.Event.ID)
	_, tmplGraphAfterDef := loadTemplateGraphSnapshot(t, env, fix, detail.Template.ID, published.ID)
	reloadedPublished, err := env.tmplRepo.GetVersionByID(context.Background(), published.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload published version: %v", err)
	}
	templateAfter := capturePublishedTemplateContract(tmplGraphAfterDef, reloadedPublished)
	assertPublishedTemplateContractUnchanged(t, templateBefore, templateAfter)
}

func TestE5REM011NoScoringResponsesParticipantsOffersResults(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedRichTemplate(t, env, fix, "e5-rem-011", nil)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-REM-011", fix.CompanyA), uuid.NewString())
	assertNoCloneBusinessArtifacts(t, env, fix, result.Event.ID, result.DraftVersion.ID)
}

func TestE5REM012RuleTargetsResolveOnlyInsideEventGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedRichTemplate(t, env, fix, "e5-rem-012", nil)
	_, tmplGraph := loadTemplateGraphSnapshot(t, env, fix, detail.Template.ID, published.ID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-REM-012", fix.CompanyA), uuid.NewString())
	_, eventGraph := loadEventGraphSnapshot(t, env, fix, result.DraftVersion.ID)
	assertEventRuleTargetsAreEventLocal(t, eventGraph)
	assertGraphUUIDSetsDisjoint(t, tmplGraph, eventGraph)
	for _, rule := range eventGraph.Rules {
		if rule.TargetQuestionID == nil {
			continue
		}
		for _, tmplRule := range tmplGraph.Rules {
			if tmplRule.TargetQuestionID != nil && *rule.TargetQuestionID == *tmplRule.TargetQuestionID {
				t.Fatalf("rule %s target still points at template question UUID", rule.RuleCode)
			}
		}
	}
}
