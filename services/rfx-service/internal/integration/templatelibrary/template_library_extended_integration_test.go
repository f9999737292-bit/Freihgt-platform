//go:build integration

package templatelibrary

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE4INT21ConcurrentPublishOneWinner(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "concurrent-pub", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	in := domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    detail.DraftVersion.Version,
		ChangeSummary:           "Concurrent publish",
	}
	var wg sync.WaitGroup
	results := make([]*domain.RfxTemplateVersion, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("e4-21-%d", idx)
			results[idx], errs[idx] = env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, key, in)
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

func TestE4INT23PublishKeyBodyMismatch409(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "idem-mismatch", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	key := "e4-23"
	in := domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    detail.DraftVersion.Version,
		ChangeSummary:           "First body",
	}
	if _, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, key, in); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	var beforeHash string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT request_body_hash FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, detail.Template.ID, key).Scan(&beforeHash); err != nil {
		t.Fatalf("read idempotency hash: %v", err)
	}
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-23-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	detail, err = env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	_, err = env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, key, domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    draft.Version,
		ChangeSummary:           "Different body",
	})
	expectAppErrorCode(t, err, apperrors.CodeConflict)
	var afterHash string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT request_body_hash FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3`,
		fix.TenantID, detail.Template.ID, key).Scan(&afterHash); err != nil {
		t.Fatalf("read idempotency hash after conflict: %v", err)
	}
	if afterHash != beforeHash {
		t.Fatal("idempotency record mutated on body mismatch")
	}
}

func TestE4INT25ExpiredKeyReuse(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "expired-key", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	key := "e4-25"
	first := publishTemplate(t, env, fix, detail.Template.ID, key)
	expireTemplateIdempotencyKey(t, env, fix, detail.Template.ID, key, domain.TemplateLifecycleOperationPublish)
	draft, err := env.templateSvc.ForkDraftFromPublished(context.Background(), fix.BuyerA, detail.Template.ID, "e4-25-fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	detail, err = env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, detail.Template.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	second, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, key, domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    draft.Version,
		ChangeSummary:           "After expiry",
	})
	if err != nil {
		t.Fatalf("publish after expired key reuse: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("expected new published version after expired key reuse")
	}
}

func TestE4INT29UnarchiveRouteAbsent(t *testing.T) {
	root := repoRoot(t)
	routerPath := filepath.Join(root, "services", "rfx-service", "internal", "http", "router.go")
	gatewayPath := filepath.Join(root, "services", "api-gateway", "internal", "http", "router.go")
	for _, path := range []string{routerPath, gatewayPath} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(body), "unarchive") {
			t.Fatalf("unarchive route must be absent in %s", path)
		}
	}
}

func TestE4INT36AuditEventsAtomic(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	var before int
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`, fix.TenantID).Scan(&before); err != nil {
		t.Fatalf("count audit before: %v", err)
	}
	_, err := env.templateSvc.CreateTemplate(context.Background(), fix.BuyerA, createTemplateInput("audit-atomic", nil))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var after int
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`, fix.TenantID).Scan(&after); err != nil {
		t.Fatalf("count audit after: %v", err)
	}
	if after != before+1 {
		t.Fatalf("expected one audit event, before=%d after=%d", before, after)
	}
	_, err = env.templateSvc.CreateTemplate(context.Background(), fix.BuyerA, createTemplateInput("audit-atomic", nil))
	expectAppErrorCode(t, err, apperrors.CodeConflict)
	var afterDup int
	if err := env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`, fix.TenantID).Scan(&afterDup); err != nil {
		t.Fatalf("count audit after dup: %v", err)
	}
	if afterDup != after {
		t.Fatalf("failed create must not write audit: after=%d afterDup=%d", after, afterDup)
	}
}

func TestE4INT37TransactionRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "rollback", nil)
	var publishedBefore int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_template_versions
		WHERE template_id = $1 AND status = $2 AND deleted_at IS NULL`,
		detail.Template.ID, domain.RfxVersionStatusPublished).Scan(&publishedBefore); err != nil {
		t.Fatalf("count published before: %v", err)
	}
	_, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, detail.Template.ID, "e4-37", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    detail.DraftVersion.Version,
		ChangeSummary:           "Should fail readiness",
	})
	expectAppErrorCode(t, err, apperrors.CodeValidation)
	var publishedAfter int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_template_versions
		WHERE template_id = $1 AND status = $2 AND deleted_at IS NULL`,
		detail.Template.ID, domain.RfxVersionStatusPublished).Scan(&publishedAfter); err != nil {
		t.Fatalf("count published after: %v", err)
	}
	if publishedAfter != publishedBefore {
		t.Fatalf("failed publish must rollback: before=%d after=%d", publishedBefore, publishedAfter)
	}
	var draftStatus string
	if err := env.pool.QueryRow(context.Background(), `SELECT status FROM rfx.rfx_template_versions WHERE id = $1`, detail.DraftVersion.ID).Scan(&draftStatus); err != nil {
		t.Fatalf("read draft status: %v", err)
	}
	if draftStatus != domain.RfxVersionStatusDraft {
		t.Fatalf("draft status changed on failed publish: %s", draftStatus)
	}
}

func TestE4INT40CompositeIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "composite-fk", nil)
	otherTenant := uuid.New()
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_template_sections (
			tenant_id, template_id, rfx_template_version_id, section_code, title
		) VALUES ($1, $2, $3, $4, $5)`,
		otherTenant, detail.Template.ID, detail.DraftVersion.ID, "BAD", "Bad")
	if err == nil {
		t.Fatal("expected composite FK violation for cross-tenant section insert")
	}
}

func TestE4INT41OpenAPIRouterGatewayParity(t *testing.T) {
	root := repoRoot(t)
	openapiPath := filepath.Join(root, "packages", "openapi", "rfx-service.yaml")
	rfxRouterPath := filepath.Join(root, "services", "rfx-service", "internal", "http", "router.go")
	gatewayPath := filepath.Join(root, "services", "api-gateway", "internal", "http", "router.go")
	openapiBody, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	rfxBody, err := os.ReadFile(rfxRouterPath)
	if err != nil {
		t.Fatalf("read rfx router: %v", err)
	}
	gatewayBody, err := os.ReadFile(gatewayPath)
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}
	required := []string{
		"/api/v1/rfx-templates",
		"/api/v1/rfx-templates/{id}/versions/publish",
		"/api/v1/rfx-templates/{id}/versions/fork-draft",
		"/api/v1/rfx-templates/{id}/questionnaire",
	}
	for _, path := range required {
		if !strings.Contains(string(openapiBody), path) {
			t.Fatalf("openapi missing path %s", path)
		}
		rfxPath := strings.ReplaceAll(path, "/api/v1", "/v1")
		if !strings.Contains(string(rfxBody), rfxPath) {
			t.Fatalf("rfx router missing path %s", rfxPath)
		}
		if !strings.Contains(string(gatewayBody), path) {
			t.Fatalf("gateway missing path %s", path)
		}
	}
}

func TestE4INT42E5CloneRouteAbsent(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"services/rfx-service/internal/http/router.go",
		"services/api-gateway/internal/http/router.go",
		"scripts/openapi/generate_openapi.py",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if strings.Contains(string(body), "from-template") {
			t.Fatalf("E5 clone route must be absent in %s", rel)
		}
	}
}

func expireTemplateIdempotencyKey(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, key, operation string) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_idempotency_records
		SET expires_at = $4
		WHERE tenant_id = $1 AND aggregate_scope = $2 AND idempotency_key = $3 AND operation = $5`,
		fix.TenantID, templateID, key, time.Now().UTC().Add(-time.Hour), operation); err != nil {
		t.Fatalf("expire idempotency key: %v", err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.work not found")
		}
		dir = parent
	}
}
