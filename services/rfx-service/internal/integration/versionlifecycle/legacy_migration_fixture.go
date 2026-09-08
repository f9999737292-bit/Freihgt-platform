//go:build integration

package versionlifecycle

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func seedLegacyPre068PublishedVersion(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	versionID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (
			id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at
		) VALUES ($1, $2, $3, 1, 'PUBLISHED', TRUE, now())`,
		versionID, fix.TenantID, eventID); err != nil {
		t.Fatalf("insert legacy published version: %v", err)
	}
	return versionID
}

func seedLegacyPre068AmbiguousPublishedVersions(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	v1ID := uuid.New()
	v2ID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (
			id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at
		) VALUES ($1, $2, $3, 1, 'PUBLISHED', TRUE, now())`,
		v1ID, fix.TenantID, eventID); err != nil {
		t.Fatalf("insert first legacy published version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (
			id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at
		) VALUES ($1, $2, $3, 2, 'PUBLISHED', TRUE, now())`,
		v2ID, fix.TenantID, eventID); err != nil {
		t.Fatalf("insert second legacy published version: %v", err)
	}
	return v1ID, v2ID
}

func legacyColumnExists(t *testing.T, env *testEnv, column string) bool {
	t.Helper()
	var exists bool
	if err := env.pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'rfx'
			  AND table_name = 'rfx_events'
			  AND column_name = $1
		)`, column).Scan(&exists); err != nil {
		t.Fatalf("check column %s: %v", column, err)
	}
	return exists
}

func legacyTableExists(t *testing.T, env *testEnv, table string) bool {
	t.Helper()
	var exists bool
	if err := env.pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'rfx'
			  AND table_name = $1
		)`, table).Scan(&exists); err != nil {
		t.Fatalf("check table rfx.%s: %v", table, err)
	}
	return exists
}
