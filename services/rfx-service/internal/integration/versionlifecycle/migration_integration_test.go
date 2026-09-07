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
	publishedVersionID := seedLegacyPre068PublishedVersion(t, env, fix, event.ID)

	if legacyColumnExists(t, env, "published_version_id") {
		t.Fatal("published_version_id must not exist before migration 000068")
	}

	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql"); err != nil {
		t.Fatalf("apply 000068: %v", err)
	}

	var publishedVersionIDAfter *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, event.ID).Scan(&publishedVersionIDAfter); err != nil {
		t.Fatalf("read published_version_id: %v", err)
	}
	if publishedVersionIDAfter == nil || *publishedVersionIDAfter != publishedVersionID {
		t.Fatalf("backfill failed: got=%v want=%s", publishedVersionIDAfter, publishedVersionID)
	}

	var draftConstraintExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'rfx' AND indexname = 'uq_rfx_versions_one_draft_per_event'
		)`).Scan(&draftConstraintExists); err != nil {
		t.Fatalf("check draft constraint: %v", err)
	}
	if !draftConstraintExists {
		t.Fatal("expected one-draft constraint after migration up")
	}

	var publishedConstraintExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'rfx' AND indexname = 'uq_rfx_versions_one_published_per_event'
		)`).Scan(&publishedConstraintExists); err != nil {
		t.Fatalf("check published constraint: %v", err)
	}
	if !publishedConstraintExists {
		t.Fatal("expected one-published constraint after migration up")
	}

	if err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.down.sql"); err != nil {
		t.Fatalf("down migration: %v", err)
	}

	if legacyColumnExists(t, env, "published_version_id") {
		t.Fatal("published_version_id must be removed after down migration")
	}
	if legacyTableExists(t, env, "rfx_idempotency_records") {
		t.Fatal("idempotency table must be removed after down migration")
	}

	var versionStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.rfx_versions WHERE id = $1`, publishedVersionID).Scan(&versionStatus); err != nil {
		t.Fatalf("reload legacy version after down: %v", err)
	}
	if versionStatus != "PUBLISHED" {
		t.Fatalf("legacy published version altered by down migration: status=%s", versionStatus)
	}
}

func TestE1INT20MigrationFailsOnAmbiguousMultiplePublished(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-20")
	v1ID, v2ID := seedLegacyPre068AmbiguousPublishedVersions(t, env, fix, event.ID)

	err := applyMigrationFile(ctx, env.pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql")
	if err == nil {
		t.Fatal("expected migration to fail on ambiguous published rows")
	}
	if !strings.Contains(err.Error(), "multiple PUBLISHED") {
		t.Fatalf("unexpected migration error: %v", err)
	}

	if legacyColumnExists(t, env, "published_version_id") {
		t.Fatal("partial migration must not add published_version_id")
	}

	var publishedVersionID *uuid.UUID
	if legacyColumnExists(t, env, "published_version_id") {
		if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, event.ID).Scan(&publishedVersionID); err != nil {
			t.Fatalf("read published_version_id after failed migration: %v", err)
		}
		if publishedVersionID != nil {
			t.Fatalf("published_version_id must remain unset after failed migration, got %v", publishedVersionID)
		}
	}

	var v1Status, v2Status string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.rfx_versions WHERE id = $1`, v1ID).Scan(&v1Status); err != nil {
		t.Fatalf("reload v1: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.rfx_versions WHERE id = $1`, v2ID).Scan(&v2Status); err != nil {
		t.Fatalf("reload v2: %v", err)
	}
	if v1Status != "PUBLISHED" || v2Status != "PUBLISHED" {
		t.Fatalf("ambiguous published rows must remain unchanged: v1=%s v2=%s", v1Status, v2Status)
	}
}
