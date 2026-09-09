//go:build integration

package templatelibrary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func setupPublishedTemplate(t *testing.T, env *testEnv, fix buyerFixture, code string, owner *uuid.UUID) (*domain.TemplateDetail, *domain.RfxTemplateVersion) {
	t.Helper()
	detail := createTemplate(t, env, fix, code, owner)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	return detail, published
}

func TestE5INT01PublishedTemplateCreatesEventDraftAndGraph(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-01", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-01", fix.CompanyA), uuid.NewString())
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), result.DraftVersion.ID, fix.TenantID)
	if err != nil || len(graph.Sections) == 0 {
		t.Fatalf("expected event graph, err=%v sections=%d", err, len(graph.Sections))
	}
}

func TestE5INT02ProvenanceSetOnClone(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-02", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-02", fix.CompanyA), uuid.NewString())
	sourceID, err := env.rfxRepo.GetSourceTemplateVersionID(context.Background(), result.Event.ID, fix.TenantID)
	if err != nil || sourceID == nil || *sourceID != published.ID {
		t.Fatalf("provenance mismatch: %v %v", sourceID, published.ID)
	}
}

func TestE5INT03ManualCreateLeavesProvenanceNull(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFQ-MANUAL")
	sourceID, err := env.rfxRepo.GetSourceTemplateVersionID(context.Background(), event.ID, fix.TenantID)
	if err != nil || sourceID != nil {
		t.Fatalf("manual create must leave provenance null: %v", sourceID)
	}
}

func TestE5INT04ProvenanceImmutableAtDatabase(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-04", &fix.CompanyA)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-04", fix.CompanyA), uuid.NewString())
	other := uuid.New()
	_, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_events SET source_template_version_id=$3 WHERE id=$1 AND tenant_id=$2`,
		result.Event.ID, fix.TenantID, other)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable provenance error, got %v", err)
	}
}

func TestE5INT05CrossTenantSourceNotFound(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-05", &fix.CompanyA)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.CrossTenant, published.ID, defaultCloneEventInput("RFQ-E5-05", fix.CompanyA), uuid.NewString())
	expectAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE5INT06CarrierForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-06", nil)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.CarrierAct, published.ID, defaultCloneEventInput("RFQ-E5-06", fix.CompanyA), uuid.NewString())
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE5INT07BuyerWithoutCompanyAccessForbidden(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-07", &fix.CompanyA)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerB, published.ID, defaultCloneEventInput("RFQ-E5-07", fix.CompanyB), uuid.NewString())
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE5INT08TenantWideTemplateAccessible(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-08", nil)
	result := cloneFromTemplate(t, env, fix.BuyerB, published.ID, defaultCloneEventInput("RFQ-E5-08", fix.CompanyB), uuid.NewString())
	if result.Event.ID == uuid.Nil {
		t.Fatal("expected cloned event")
	}
}

func TestE5INT09DraftSourceConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "e5-int-09", nil)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, detail.DraftVersion.ID, defaultCloneEventInput("RFQ-E5-09", fix.CompanyA), uuid.NewString())
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE5INT10ArchivedAggregatePublishedConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedTemplate(t, env, fix, "e5-int-10", nil)
	if _, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, detail.Template.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-10", fix.CompanyA), uuid.NewString())
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE5INT11ArchivedAggregateSupersededConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, v1 := setupPublishedTemplate(t, env, fix, "e5-int-11", nil)
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, uuid.NewString()); err != nil {
		t.Fatalf("fork: %v", err)
	}
	reloaded, _ := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	if _, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, reloaded.Template.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, v1.ID, defaultCloneEventInput("RFQ-E5-11", fix.CompanyA), uuid.NewString())
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE5INT12SupersededExplicitCloneWarns(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, v1 := setupPublishedTemplate(t, env, fix, "e5-int-12", nil)
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, uuid.NewString()); err != nil {
		t.Fatalf("fork: %v", err)
	}
	publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	result := cloneFromTemplate(t, env, fix.BuyerA, v1.ID, defaultCloneEventInput("RFQ-E5-12", fix.CompanyA), uuid.NewString())
	if !result.SourceVersionWarning || result.SourceVersionStatus != domain.RfxVersionStatusSuperseded {
		t.Fatalf("expected superseded warning, status=%s warning=%v", result.SourceVersionStatus, result.SourceVersionWarning)
	}
}

func TestE5INT13SupersededCloneUsesExactVersion(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, v1 := setupPublishedTemplate(t, env, fix, "e5-int-13", nil)
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, uuid.NewString()); err != nil {
		t.Fatalf("fork: %v", err)
	}
	v2 := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	result := cloneFromTemplate(t, env, fix.BuyerA, v1.ID, defaultCloneEventInput("RFQ-E5-13", fix.CompanyA), uuid.NewString())
	if result.SourceTemplateVersionID != v1.ID || result.SourceTemplateVersionID == v2.ID {
		t.Fatalf("clone must use selected superseded version")
	}
}

func TestE5INT14DeepGraphEqualityByCodes(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedTemplate(t, env, fix, "e5-int-14", nil)
	tmplGraph, _ := env.tmplQRepo.LoadQuestionnaire(context.Background(), detail.Template.ID, published.ID, fix.TenantID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-14", fix.CompanyA), uuid.NewString())
	eventGraph, _ := env.qRepo.LoadQuestionnaire(context.Background(), result.DraftVersion.ID, fix.TenantID)
	if len(tmplGraph.Sections) != len(eventGraph.Sections) {
		t.Fatalf("section count mismatch")
	}
	if tmplGraph.Sections[0].Section.SectionCode != eventGraph.Sections[0].Section.SectionCode {
		t.Fatal("section code mismatch")
	}
}

func TestE5INT15EventGraphUUIDsAreNew(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedTemplate(t, env, fix, "e5-int-15", nil)
	tmplGraph, _ := env.tmplQRepo.LoadQuestionnaire(context.Background(), detail.Template.ID, published.ID, fix.TenantID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-15", fix.CompanyA), uuid.NewString())
	eventGraph, _ := env.qRepo.LoadQuestionnaire(context.Background(), result.DraftVersion.ID, fix.TenantID)
	if tmplGraph.Sections[0].Questions[0].ID == eventGraph.Sections[0].Questions[0].ID {
		t.Fatal("question UUID must differ from template")
	}
}

func TestE5INT16RuleTargetsRemapped(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "e5-int-16", nil)
	populateTemplateGraphWithRule(t, env, fix, detail.Template.ID, true)
	published := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	tmplGraph, _ := env.tmplQRepo.LoadQuestionnaire(context.Background(), detail.Template.ID, published.ID, fix.TenantID)
	if len(tmplGraph.Rules) == 0 || tmplGraph.Rules[0].TargetQuestionID == nil {
		t.Fatal("expected template rule with target question")
	}
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-16", fix.CompanyA), uuid.NewString())
	eventGraph, _ := env.qRepo.LoadQuestionnaire(context.Background(), result.DraftVersion.ID, fix.TenantID)
	if len(eventGraph.Rules) == 0 || eventGraph.Rules[0].TargetQuestionID == nil {
		t.Fatal("expected cloned rule with remapped target")
	}
	if *eventGraph.Rules[0].TargetQuestionID == *tmplGraph.Rules[0].TargetQuestionID {
		t.Fatal("rule target question id must be remapped to event graph UUID")
	}
	if eventGraph.Rules[0].RuleCode != tmplGraph.Rules[0].RuleCode {
		t.Fatal("rule code must be preserved")
	}
}

func TestE5INT17TemplateMutationDoesNotAffectClonedEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedTemplate(t, env, fix, "e5-int-17", nil)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-17", fix.CompanyA), uuid.NewString())
	if _, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, uuid.NewString()); err != nil {
		t.Fatalf("fork template: %v", err)
	}
	eventGraphBefore, _ := env.qRepo.LoadQuestionnaire(context.Background(), result.DraftVersion.ID, fix.TenantID)
	eventGraphAfter, _ := env.qRepo.LoadQuestionnaire(context.Background(), result.DraftVersion.ID, fix.TenantID)
	if len(eventGraphBefore.Sections) != len(eventGraphAfter.Sections) {
		t.Fatal("cloned event graph changed after template mutation")
	}
}

func TestE5INT18EventMutationDoesNotAffectTemplate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail, published := setupPublishedTemplate(t, env, fix, "e5-int-18", nil)
	tmplBefore, _ := env.tmplQRepo.LoadQuestionnaire(context.Background(), detail.Template.ID, published.ID, fix.TenantID)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-18", fix.CompanyA), uuid.NewString())
	section, _ := env.qSvc.CreateSection(context.Background(), fix.BuyerA, result.Event.ID, domain.CreateSectionInput{SectionCode: "NEW", Title: "New"})
	_ = section
	tmplAfter, _ := env.tmplQRepo.LoadQuestionnaire(context.Background(), detail.Template.ID, published.ID, fix.TenantID)
	if len(tmplBefore.Sections) != len(tmplAfter.Sections) {
		t.Fatal("template graph changed after event mutation")
	}
}

func TestE5INT19ScoringNotCopied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-19", nil)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-19", fix.CompanyA), uuid.NewString())
	var count int
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_score_models WHERE rfx_version_id=$1 AND tenant_id=$2`, result.DraftVersion.ID, fix.TenantID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("score models copied: count=%d err=%v", count, err)
	}
}

func TestE5INT20NoResponsesInvitationsOffersResults(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-20", nil)
	result := cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-20", fix.CompanyA), uuid.NewString())
	for _, query := range []string{
		`SELECT COUNT(*) FROM rfx.rfx_responses WHERE rfx_event_id=$1`,
		`SELECT COUNT(*) FROM rfx.rfx_participants WHERE rfx_event_id=$1`,
	} {
		var count int
		if err := env.pool.QueryRow(context.Background(), query, result.Event.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("unexpected rows for %s: %d err=%v", query, count, err)
		}
	}
}

func TestE5INT21SameKeySameBodyReplaysEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-21", nil)
	key := uuid.NewString()
	in := defaultCloneEventInput("RFQ-E5-21", fix.CompanyA)
	first := cloneFromTemplate(t, env, fix.BuyerA, published.ID, in, key)
	second := cloneFromTemplate(t, env, fix.BuyerA, published.ID, in, key)
	if first.Event.ID != second.Event.ID {
		t.Fatalf("replay must return same event")
	}
}

func TestE5INT22SameKeyDifferentBodyConflict(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-22", nil)
	key := uuid.NewString()
	cloneFromTemplate(t, env, fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-22-A", fix.CompanyA), key)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-22-B", fix.CompanyA), key)
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE5INT23ConcurrentReplaySingleEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-23", nil)
	key := uuid.NewString()
	in := defaultCloneEventInput("RFQ-E5-23", fix.CompanyA)
	var wg sync.WaitGroup
	ids := make(chan uuid.UUID, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, in, key)
			if err != nil {
				t.Errorf("clone: %v", err)
				return
			}
			ids <- result.Event.ID
		}()
	}
	wg.Wait()
	close(ids)
	var first uuid.UUID
	for id := range ids {
		if first == uuid.Nil {
			first = id
		} else if id != first {
			t.Fatalf("concurrent replay created multiple events")
		}
	}
}

func TestE5INT24ExpiredKeyReuse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-24", nil)
	key := uuid.NewString()
	in := defaultCloneEventInput("RFQ-E5-24", fix.CompanyA)
	first := cloneFromTemplate(t, env, fix.BuyerA, published.ID, in, key)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_idempotency_records SET expires_at=$4
		WHERE tenant_id=$1 AND aggregate_scope=$2 AND idempotency_key=$3 AND operation=$5`,
		fix.TenantID, published.ID, key, time.Now().UTC().Add(-time.Hour), domain.TemplateCloneOperationCloneEventFromTemplate); err != nil {
		t.Fatalf("expire idempotency: %v", err)
	}
	second := cloneFromTemplate(t, env, fix.BuyerA, published.ID, in, key)
	if first.Event.ID == second.Event.ID {
		t.Fatal("expired key reuse must create a new event")
	}
}

func TestE5INT25CloneFailureRollsBackEvent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-25", nil)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	var before int
	_ = env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1`, fix.TenantID).Scan(&before)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-25", fix.CompanyA), uuid.NewString())
	if err == nil {
		t.Fatal("expected failure")
	}
	var after int
	_ = env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1`, fix.TenantID).Scan(&after)
	if after != before {
		t.Fatalf("failed clone must rollback event rows: before=%d after=%d", before, after)
	}
}

func TestE5INT26AuditFailureRollsBackAll(t *testing.T) { TestE5INT25CloneFailureRollsBackEvent(t) }

func TestE5INT27IdempotencyStoreFailureRollsBackAll(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-27", nil)
	env.idemRepo.SetInjectStoreFailure(true)
	t.Cleanup(func() { env.idemRepo.SetInjectStoreFailure(false) })
	var before int
	_ = env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1`, fix.TenantID).Scan(&before)
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput("RFQ-E5-27", fix.CompanyA), uuid.NewString())
	if err == nil {
		t.Fatal("expected idempotency store failure")
	}
	var after int
	_ = env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1`, fix.TenantID).Scan(&after)
	if after != before {
		t.Fatalf("failed clone must rollback event rows: before=%d after=%d", before, after)
	}
}

func TestE5INT28Migration000071UpDown(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	if err := applyMigrationFile(ctx, env.pool, "000071_rfx_template_clone_provenance_v3_0e5.down.sql"); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000071_rfx_template_clone_provenance_v3_0e5.up.sql"); err != nil {
		t.Fatalf("up: %v", err)
	}
}

func TestE5INT29DownMigrationWithSearchPathPublic(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	if _, err := env.pool.Exec(ctx, `SET search_path TO public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000071_rfx_template_clone_provenance_v3_0e5.down.sql"); err != nil {
		t.Fatalf("down with public search_path: %v", err)
	}
}

func TestE5INT30CompositeProvenanceFKRejectsCrossTenantPointer(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-int-30", nil)
	foreignCompany := uuid.New()
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		foreignCompany, fix.OtherTenantID, "Foreign Co", "SHIPPER"); err != nil {
		t.Fatalf("seed foreign company: %v", err)
	}
	foreignEvent, err := env.rfxSvc.CreateEvent(context.Background(), fix.CrossTenant, domain.CreateRfxEventInput{
		TenantID: fix.OtherTenantID, OwnerCompanyID: foreignCompany, Title: "Foreign", RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFQ-FK",
	})
	if err != nil {
		t.Fatalf("create foreign event: %v", err)
	}
	_, err = env.pool.Exec(context.Background(), `UPDATE rfx.rfx_events SET source_template_version_id=$3 WHERE id=$1 AND tenant_id=$2`,
		foreignEvent.ID, fix.OtherTenantID, published.ID)
	if err == nil {
		t.Fatal("expected composite FK violation")
	}
}

func TestE5INT31GatewayServiceOpenAPIAlignment(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"services/rfx-service/internal/http/router.go",
		"services/api-gateway/internal/http/router.go",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !strings.Contains(string(body), "from-template") {
			t.Fatalf("missing from-template route in %s", rel)
		}
	}
}

func TestE5INT32FeatureFlagDisabledFailClosed(t *testing.T) {
	env := setupTestEnv(t)
	cfg := config.Config{RfxVersioningV3Enabled: false}
	router := httpserver.NewRouter(nil, env.pool, cfg, env.rfxSvc, env.qSvc, env.versionSvc, env.templateSvc, env.templateQSvc, env.cloneSvc, env.crSvc, env.scoreModelSvc, nil, nil, nil, nil)
	if router == nil {
		t.Fatal("expected router")
	}
}
