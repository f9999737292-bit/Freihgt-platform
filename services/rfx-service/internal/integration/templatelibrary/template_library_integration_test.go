//go:build integration

package templatelibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE4INT01CreateTemplateCreatesActiveDraftV1(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "lane-tender-a", &fix.CompanyA)
	if detail.Template.Status != domain.RfxTemplateStatusActive {
		t.Fatalf("status=%s", detail.Template.Status)
	}
	if detail.DraftVersion == nil || detail.DraftVersion.VersionNumber != 1 {
		t.Fatal("expected draft v1")
	}
}

func TestE4INT02DuplicateTemplateCode409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplate(t, env, fix, "dup-code", &fix.CompanyA)
	_, err := env.templateSvc.CreateTemplate(context.Background(), fix.BuyerA, createTemplateInput("dup-code", &fix.CompanyA))
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT03SameCodeAllowedInOtherTenant(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplate(t, env, fix, "shared-code", &fix.CompanyA)
	foreignCompany := uuid.New()
	if _, err := env.pool.Exec(context.Background(), `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		foreignCompany, fix.OtherTenantID, "Foreign", "SHIPPER"); err != nil {
		t.Fatalf("seed foreign company: %v", err)
	}
	if _, err := env.templateSvc.CreateTemplate(context.Background(), fix.CrossTenant, createTemplateInput("shared-code", &foreignCompany)); err != nil {
		t.Fatalf("expected cross-tenant duplicate allowed: %v", err)
	}
}

func TestE4INT04TenantWideTemplateVisibility(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "tenant-wide", nil)
	if _, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerB, detail.Template.ID); err != nil {
		t.Fatalf("buyer B should read tenant-wide template: %v", err)
	}
}

func TestE4INT05CompanyOwnedVisibility(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "company-owned", &fix.CompanyA)
	if _, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID); err != nil {
		t.Fatalf("owner buyer should read: %v", err)
	}
}

func TestE4INT06BuyerNonOwner403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "owned-by-a", &fix.CompanyA)
	_, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerB, detail.Template.ID)
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE4INT07CarrierList403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplate(t, env, fix, "carrier-deny-list", nil)
	_, _, err := env.templateSvc.ListTemplates(context.Background(), fix.CarrierAct, domain.TemplateListFilter{})
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE4INT08CarrierKnownIDGet403(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "carrier-deny-get", nil)
	_, err := env.templateSvc.GetTemplate(context.Background(), fix.CarrierAct, detail.Template.ID)
	expectAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestE4INT09CrossTenantGet404(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "tenant-a-only", &fix.CompanyA)
	_, err := env.templateSvc.GetTemplate(context.Background(), fix.CrossTenant, detail.Template.ID)
	expectAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE4INT10MetadataOptimisticUpdate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "meta-update", nil)
	updated, err := env.templateSvc.UpdateTemplate(context.Background(), fix.BuyerA, detail.Template.ID, domain.UpdateTemplateInput{
		NameI18n:        json.RawMessage(`{"en-US":"Updated name"}`),
		ExpectedVersion: detail.Template.Version,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != detail.Template.Version+1 {
		t.Fatalf("version=%d", updated.Version)
	}
}

func TestE4INT11StaleMetadataVersion409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "stale-meta", nil)
	_, err := env.templateSvc.UpdateTemplate(context.Background(), fix.BuyerA, detail.Template.ID, domain.UpdateTemplateInput{
		NameI18n:        json.RawMessage(`{"en-US":"First"}`),
		ExpectedVersion: detail.Template.Version,
	})
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	_, err = env.templateSvc.UpdateTemplate(context.Background(), fix.BuyerA, detail.Template.ID, domain.UpdateTemplateInput{
		NameI18n:        json.RawMessage(`{"en-US":"Second"}`),
		ExpectedVersion: detail.Template.Version,
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT12DraftGraphCRUD(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "graph-crud", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	q, err := env.templateQSvc.GetQuestionnaire(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil || len(q.Sections) != 1 {
		t.Fatalf("questionnaire: %v sections=%d", err, len(q.Sections))
	}
}

func TestE4INT13StableCodeUniqueness409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "stable-code", nil)
	section, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{
		SectionCode: "SEC1", Title: "One",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	_, err = env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{
		SectionCode: "SEC1", Title: "Dup",
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
	_, err = env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q1", QuestionType: domain.QuestionTypeText, Label: "Q1", Required: true,
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	_, err = env.templateQSvc.CreateQuestion(context.Background(), fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "Q1", QuestionType: domain.QuestionTypeText, Label: "Dup", Required: true,
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT14PublishReadinessFail422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "empty-graph", nil)
	_, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, "e4-14", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    detail.DraftVersion.Version,
		ChangeSummary:           "fail",
	})
	expectAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestE4INT15PublishDraftToPublished(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "publish-ok", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, "e4-15")
	if published.Status != domain.RfxVersionStatusPublished {
		t.Fatalf("status=%s", published.Status)
	}
}

func TestE4INT16PublishedGraphMutation409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "published-immutable", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-16")
	_, err := env.templateQSvc.CreateSection(context.Background(), fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{
		SectionCode: "NEW", Title: "New",
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT17ForkCreatesDraftV2(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fork-v2", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-17-pub")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-17-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if draft.VersionNumber != 2 {
		t.Fatalf("version_number=%d", draft.VersionNumber)
	}
}

func TestE4INT18ExistingDraftBlocksSecondFork409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "fork-block", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-18-pub")
	_, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-18-fork1")
	if err != nil {
		t.Fatalf("first fork: %v", err)
	}
	_, err = env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-18-fork2")
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT19ConcurrentForkOneWinner(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "concurrent-fork", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-19-pub")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, fmt.Sprintf("e4-19-fork-%d", idx))
		}(i)
	}
	wg.Wait()
	wins, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeConflict {
			conflicts++
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d errs=%v", wins, conflicts, errs)
	}
}

func TestE4INT20PublishV2SupersedesV1(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "supersede", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	v1 := publishTemplate(t, env, fix, detail.Template.ID, "e4-20-pub1")
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-20-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	detail, err = env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	v2, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, "e4-20-pub2", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    draft.Version,
		ChangeSummary:           "v2",
	})
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	var v1Status string
	if err := env.pool.QueryRow(context.Background(), `SELECT status FROM rfx.rfx_template_versions WHERE id=$1`, v1.ID).Scan(&v1Status); err != nil {
		t.Fatalf("read v1: %v", err)
	}
	if v1Status != domain.RfxVersionStatusSuperseded {
		t.Fatalf("v1 status=%s", v1Status)
	}
	if v2.Status != domain.RfxVersionStatusPublished {
		t.Fatalf("v2 status=%s", v2.Status)
	}
}

func TestE4INT22PublishIdempotentReplay(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "idem-pub", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	key := "e4-22"
	first := publishTemplate(t, env, fix, detail.Template.ID, key)
	second := publishTemplate(t, env, fix, detail.Template.ID, key)
	if first.ID != second.ID {
		t.Fatalf("idempotent replay mismatch")
	}
}

func TestE4INT26ArchiveActiveTemplate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "archive-me", nil)
	archived, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil || archived.Status != domain.RfxTemplateStatusArchived {
		t.Fatalf("archive: %v status=%s", err, archived.Status)
	}
}

func TestE4INT27ArchivedEdit409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "archived-edit", nil)
	if _, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, detail.Template.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := env.templateSvc.UpdateTemplate(context.Background(), fix.BuyerA, detail.Template.ID, domain.UpdateTemplateInput{
		NameI18n: json.RawMessage(`{"en-US":"Nope"}`), ExpectedVersion: detail.Template.Version + 1,
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT30DeleteDraftOnlyTemplate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "delete-me", nil)
	if err := env.templateSvc.DeleteTemplate(context.Background(), fix.BuyerA, detail.Template.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	expectAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestE4INT31DeletePublishedTemplate409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "no-delete", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-31")
	err := env.templateSvc.DeleteTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT32ListExcludesDeleted(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "list-hidden", nil)
	if err := env.templateSvc.DeleteTemplate(context.Background(), fix.BuyerA, detail.Template.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	items, total, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, item := range items {
		if item.ID == detail.Template.ID {
			t.Fatal("deleted template visible in list")
		}
	}
	if total < 0 {
		t.Fatal("invalid total")
	}
}

func TestE4INT34I18nValidation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, err := env.templateSvc.CreateTemplate(context.Background(), fix.BuyerA, domain.CreateTemplateInput{
		TemplateCode: "i18n-ok",
		NameI18n:     json.RawMessage(`{"ru-RU":"Шаблон","en-US":"Template","zh-CN":"模板"}`),
	})
	if err != nil {
		t.Fatalf("create with RU/EN/ZH: %v", err)
	}
}

func TestE4INT24ForkIdempotentReplay(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "idem-fork", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-24-pub")
	key := "e4-24-fork"
	first, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, key)
	if err != nil {
		t.Fatalf("first fork: %v", err)
	}
	second, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, key)
	if err != nil {
		t.Fatalf("replay fork: %v", err)
	}
	if first.ID != second.ID {
		t.Fatal("idempotent fork replay mismatch")
	}
}

func TestE4INT28ArchivedPublishFork409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "archived-lifecycle", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	publishTemplate(t, env, fix, detail.Template.ID, "e4-28-pub")
	if _, err := env.templateSvc.ArchiveTemplate(context.Background(), fix.BuyerA, detail.Template.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, "e4-28-pub2", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version + 1,
		ExpectedDraftVersion:    1,
		ChangeSummary:           "nope",
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
	_, err = env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-28-fork")
	expectAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE4INT33ListPaginationDeterministic(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	createTemplate(t, env, fix, "aaa-template", nil)
	createTemplate(t, env, fix, "bbb-template", nil)
	items, total, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{Limit: 1, Offset: 0})
	if err != nil || total < 2 || len(items) != 1 {
		t.Fatalf("list page1: err=%v total=%d len=%d", err, total, len(items))
	}
	page2, _, err := env.templateSvc.ListTemplates(context.Background(), fix.BuyerA, domain.TemplateListFilter{Limit: 1, Offset: 1})
	if err != nil || len(page2) != 1 || page2[0].ID == items[0].ID {
		t.Fatalf("list page2 not distinct: %v", err)
	}
}

func TestE4INT35InvalidLocale422(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, err := env.templateSvc.CreateTemplate(context.Background(), fix.BuyerA, domain.CreateTemplateInput{
		TemplateCode: "bad-locale",
		NameI18n:     json.RawMessage(`{"de-DE":"Name"}`),
	})
	expectAppErrorCode(t, err, apperrors.CodeValidation)
}
