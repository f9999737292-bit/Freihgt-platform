//go:build integration

package versionlifecycle

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestE1INT19MigrationBackfillPublishedVersionID(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-19")
	draft := makeQuestionnaireDraft(t, env, fix, event.ID)
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions
		SET status = 'PUBLISHED', published_at = now(), questionnaire_enabled = TRUE
		WHERE id = $1 AND tenant_id = $2`, draft.ID, fix.TenantID); err != nil {
		t.Fatalf("mark version published pre-068: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql"); err != nil {
		t.Fatalf("apply 000068: %v", err)
	}
	var publishedVersionID *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, event.ID).Scan(&publishedVersionID); err != nil {
		t.Fatalf("read published_version_id: %v", err)
	}
	if publishedVersionID == nil || *publishedVersionID != draft.ID {
		t.Fatalf("backfill failed: got=%v want=%s", publishedVersionID, draft.ID)
	}
	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.down.sql"); err != nil {
		t.Fatalf("down migration: %v", err)
	}
}

func TestE1INT20MigrationFailsOnAmbiguousMultiplePublished(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-20")
	v1 := makeQuestionnaireDraft(t, env, fix, event.ID)
	v2ID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at)
		VALUES ($1, $2, $3, 2, 'PUBLISHED', TRUE, now())`, v2ID, fix.TenantID, event.ID); err != nil {
		t.Fatalf("insert second published version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions SET status = 'PUBLISHED', published_at = now()
		WHERE id = $1 AND tenant_id = $2`, v1.ID, fix.TenantID); err != nil {
		t.Fatalf("mark first version published: %v", err)
	}
	err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql")
	if err == nil {
		t.Fatal("expected migration to fail on ambiguous published rows")
	}
	if !strings.Contains(err.Error(), "multiple PUBLISHED") {
		t.Fatalf("unexpected migration error: %v", err)
	}
}
