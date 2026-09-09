//go:build integration

package templatelibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func postCloneFromTemplateHTTP(t *testing.T, env *testEnv, cfg config.Config, fix buyerFixture, templateVersionID uuid.UUID, rfxNumber, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, cfg, env.rfxSvc, env.qSvc, env.versionSvc, env.templateSvc, env.templateQSvc, env.cloneSvc, env.crSvc, env.scoreModelSvc, nil, nil, nil, nil)
	body, err := json.Marshal(map[string]any{
		"template_version_id": templateVersionID.String(),
		"rfx_number":          rfxNumber,
		"rfx_type":            "SPOT_RFQ",
		"category":            "FREIGHT",
		"title":               "Clone HTTP Test",
		"owner_company_id":    fix.CompanyA.String(),
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/rfx-events/from-template", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	req.Header.Set("Idempotency-Key", idempotencyKey)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestE5REM001GraphMaterializationFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	detail := createTemplate(t, env, fix, "e5-rem-001", nil)
	populateTemplateGraphWithOptions(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())

	repository.SetInjectTemplateGraphCopyFailure("option")
	t.Cleanup(func() { repository.ClearInjectTemplateGraphCopyFailure() })

	rfxNumber := "RFQ-E5-REM-001"
	key := uuid.NewString()
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput(rfxNumber, fix.CompanyA), key)
	if err == nil {
		t.Fatal("expected graph materialization failure")
	}
	assertZeroCloneArtifacts(t, env, fix, rfxNumber, key, published.ID)
}

func TestE5REM002AuditFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-rem-002", nil)

	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	rfxNumber := "RFQ-E5-REM-002"
	key := uuid.NewString()
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput(rfxNumber, fix.CompanyA), key)
	if err == nil {
		t.Fatal("expected audit failure")
	}
	assertZeroCloneArtifacts(t, env, fix, rfxNumber, key, published.ID)
}

func TestE5REM003IdempotencyStoreFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-rem-003", nil)

	env.idemRepo.SetInjectStoreFailure(true)
	t.Cleanup(func() { env.idemRepo.SetInjectStoreFailure(false) })

	rfxNumber := "RFQ-E5-REM-003"
	key := uuid.NewString()
	_, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), fix.BuyerA, published.ID, defaultCloneEventInput(rfxNumber, fix.CompanyA), key)
	if err == nil {
		t.Fatal("expected idempotency store failure")
	}
	assertZeroCloneArtifacts(t, env, fix, rfxNumber, key, published.ID)
}

func TestE5REM004DownMigrationCompleteCleanupWithSearchPathPublic(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	detail := createTemplate(t, env, fix, "e5-rem-004-tpl", nil)
	populateTemplateGraph(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	event := createDraftEvent(t, env, fix, "RFQ-REM-004-LEGACY")

	var templateCountBefore, eventCountBefore int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_templates WHERE tenant_id=$1`, fix.TenantID).Scan(&templateCountBefore); err != nil {
		t.Fatalf("count templates: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1`, fix.TenantID).Scan(&eventCountBefore); err != nil {
		t.Fatalf("count events: %v", err)
	}
	_ = published

	if _, err := env.pool.Exec(ctx, `SET search_path TO public`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000071_rfx_template_clone_provenance_v3_0e5.down.sql"); err != nil {
		t.Fatalf("down migration: %v", err)
	}
	assertMigration000071Absent(t, ctx, env.pool)

	var templateCountAfter, eventCountAfter int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_templates WHERE tenant_id=$1`, fix.TenantID).Scan(&templateCountAfter); err != nil {
		t.Fatalf("count templates after down: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1`, fix.TenantID).Scan(&eventCountAfter); err != nil {
		t.Fatalf("count events after down: %v", err)
	}
	if templateCountAfter != templateCountBefore || eventCountAfter != eventCountBefore {
		t.Fatalf("legacy data changed: templates before=%d after=%d events before=%d after=%d event=%s",
			templateCountBefore, templateCountAfter, eventCountBefore, eventCountAfter, event.ID)
	}

	if err := applyMigrationFile(ctx, env.pool, "000071_rfx_template_clone_provenance_v3_0e5.up.sql"); err != nil {
		t.Fatalf("up migration: %v", err)
	}
	assertMigration000071Present(t, ctx, env.pool)
}

func TestE5REM005FeatureFlagDisabledRealHTTPDenial(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-rem-005", nil)

	rfxNumber := "RFQ-E5-REM-005"
	key := uuid.NewString()
	rec := postCloneFromTemplateHTTP(t, env, config.Config{RfxVersioningV3Enabled: false}, fix, published.ID, rfxNumber, key)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("feature disabled HTTP status=%d want=%d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	assertZeroCloneArtifacts(t, env, fix, rfxNumber, key, published.ID)
}

func TestE5REM006FeatureFlagEnabledHTTP201(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	_, published := setupPublishedTemplate(t, env, fix, "e5-rem-006", nil)

	rfxNumber := "RFQ-E5-REM-006"
	key := uuid.NewString()
	rec := postCloneFromTemplateHTTP(t, env, config.Config{RfxVersioningV3Enabled: true}, fix, published.ID, rfxNumber, key)
	if rec.Code != http.StatusCreated {
		t.Fatalf("feature enabled HTTP status=%d want=%d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	counts, err := countCloneArtifacts(context.Background(), env, fix, rfxNumber, key, published.ID)
	if err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if counts.events != 1 || counts.versions != 1 || counts.idempotency != 1 {
		t.Fatalf("expected persisted clone artifacts, got events=%d versions=%d idempotency=%d",
			counts.events, counts.versions, counts.idempotency)
	}
}
