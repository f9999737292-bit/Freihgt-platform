//go:build integration

package excelexchange

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestE7P2INT01Migration073UpPresent(t *testing.T) {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	up := filepath.Join(migrationsDir, "000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql")
	if _, err := os.Stat(up); err != nil {
		t.Fatalf("missing migration up: %v", err)
	}
}

func TestE7P2INT02Migration073DownSearchPath(t *testing.T) {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	down, err := os.ReadFile(filepath.Join(migrationsDir, "000073_rfx_excel_erp_exchange_v3_0e7_phase2.down.sql"))
	if err != nil {
		t.Fatalf("read down: %v", err)
	}
	if !strings.Contains(string(down), "SET search_path = public") {
		t.Fatal("down migration must set search_path=public")
	}
}

func TestE7P2INT03Migration073UpDownUp(t *testing.T) {
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer dropDB(context.Background())
	pool, err := connectPool(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("up all: %v", err)
	}
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	downSQL, _ := os.ReadFile(filepath.Join(migrationsDir, "000073_rfx_excel_erp_exchange_v3_0e7_phase2.down.sql"))
	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("down 000073: %v", err)
	}
	upSQL, _ := os.ReadFile(filepath.Join(migrationsDir, "000073_rfx_excel_erp_exchange_v3_0e7_phase2.up.sql"))
	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("up 000073 again: %v", err)
	}
}

func TestE7P2INT04CreationChannelBackfill(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	companyID := uuid.New()
	templateID := uuid.New()
	templateVersionID := uuid.New()
	manualEventID := uuid.New()
	templateEventID := uuid.New()
	actorID := uuid.New()

	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_templates (
			id, tenant_id, template_code, name_i18n_json, status, created_by
		) VALUES ($1, $2, 'TPL-001', '{"en":"Template"}'::jsonb, 'ACTIVE', $3)
	`, templateID, tenantID, actorID)
	if err != nil {
		t.Fatalf("seed template: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_template_versions (
			id, tenant_id, template_id, version_number, status, created_by
		) VALUES ($1, $2, $3, 1, 'PUBLISHED', $4)
	`, templateVersionID, tenantID, templateID, actorID)
	if err != nil {
		t.Fatalf("seed template version: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, source_template_version_id
		) VALUES
			($1, $2, 'MAN-1', 'SPOT_RFQ', 'FREIGHT', 'Manual', $3, 'DRAFT', NULL),
			($4, $2, 'TPL-1', 'SPOT_RFQ', 'FREIGHT', 'Template', $3, 'DRAFT', $5)
	`, manualEventID, tenantID, companyID, templateEventID, templateVersionID)
	if err != nil {
		t.Fatalf("seed events: %v", err)
	}

	var manualChannel, templateChannel string
	if err := env.pool.QueryRow(ctx, `
		SELECT creation_channel FROM rfx.rfx_events WHERE id = $1
	`, manualEventID).Scan(&manualChannel); err != nil {
		t.Fatalf("manual channel: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT creation_channel FROM rfx.rfx_events WHERE id = $1
	`, templateEventID).Scan(&templateChannel); err != nil {
		t.Fatalf("template channel: %v", err)
	}
	if manualChannel != "MANUAL" {
		t.Fatalf("manual channel = %q want MANUAL", manualChannel)
	}
	if templateChannel != "TEMPLATE" {
		t.Fatalf("template channel = %q want TEMPLATE", templateChannel)
	}
}

func TestE7P2INT05ImportAnalysisPayloadImmutable(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	analysisID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_import_analyses (
			id, tenant_id, actor_id, actor_company_id, workbook_type, schema_version,
			target_type, canonical_payload_json, canonical_hash, status, validation_summary, expires_at
		) VALUES (
			$1, $2, $3, $4, 'BUYER_TENDER', 'BINTRANS_RFX_BUYER_XLSX_V1',
			'NEW_EVENT', '{"event":{"title":"X"}}'::jsonb,
			'0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
			'PREVIEWED', '{}'::jsonb, now() + interval '1 hour'
		)
	`, analysisID, tenantID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("insert analysis: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		UPDATE rfx.rfx_import_analyses
		SET canonical_payload_json = '{"event":{"title":"Y"}}'::jsonb
		WHERE id = $1
	`, analysisID)
	if err == nil {
		t.Fatal("expected immutable payload rejection")
	}
}
