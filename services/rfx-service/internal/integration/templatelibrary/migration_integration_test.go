//go:build integration

package templatelibrary

import (
	"context"
	"testing"
)

func TestE4INT38Migration000070Up(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	var exists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_templates'
		)`).Scan(&exists); err != nil {
		t.Fatalf("check templates table: %v", err)
	}
	if !exists {
		t.Fatal("expected rfx_templates after migration 000070")
	}
	var draftConstraint bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes WHERE schemaname='rfx' AND indexname='uq_rfx_template_versions_one_draft'
		)`).Scan(&draftConstraint); err != nil {
		t.Fatalf("check draft constraint: %v", err)
	}
	if !draftConstraint {
		t.Fatal("expected one-draft constraint")
	}
}

func TestE4INT39Migration000070DownPreservesPriorSchema(t *testing.T) {
	env, cleanup := setupLegacyMigrationTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	if err := applyMigrationFile(ctx, env.pool, "000070_rfx_template_library_v3_0e4.up.sql"); err != nil {
		t.Fatalf("apply 000070 up: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000070_rfx_template_library_v3_0e4.down.sql"); err != nil {
		t.Fatalf("apply 000070 down: %v", err)
	}
	var templatesExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_templates'
		)`).Scan(&templatesExists); err != nil {
		t.Fatalf("check templates removed: %v", err)
	}
	if templatesExists {
		t.Fatal("expected rfx_templates removed after down migration")
	}
	var versionsExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_versions'
		)`).Scan(&versionsExists); err != nil {
		t.Fatalf("check rfx_versions preserved: %v", err)
	}
	if !versionsExists {
		t.Fatal("expected prior E1 schema preserved")
	}
}
