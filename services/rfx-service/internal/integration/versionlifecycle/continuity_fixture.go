//go:build integration

package versionlifecycle

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// promoteVersionForContinuityFixture is test-only SQL state setup.
// It models v2 becoming current PUBLISHED while an existing response remains pinned to v1.
func promoteVersionForContinuityFixture(t *testing.T, env *testEnv, fix buyerFixture, eventID, v1ID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	v2ID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions
		SET status = 'SUPERSEDED', superseded_at = now()
		WHERE id = $1 AND tenant_id = $2`, v1ID, fix.TenantID); err != nil {
		t.Fatalf("supersede v1 before continuity v2 insert: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (
			id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled,
			published_at, published_by, change_summary
		)
		SELECT $1, tenant_id, rfx_event_id, version_number + 1, 'PUBLISHED', questionnaire_enabled,
			now(), $4, 'Continuity fixture v2'
		FROM rfx.rfx_versions
		WHERE id = $2 AND tenant_id = $3`,
		v2ID, v1ID, fix.TenantID, fix.BuyerA.UserID); err != nil {
		t.Fatalf("insert continuity v2 published version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions
		SET superseded_by_version_id = $3
		WHERE id = $1 AND tenant_id = $2`, v1ID, fix.TenantID, v2ID); err != nil {
		t.Fatalf("link superseded v1 to v2: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_events
		SET published_version_id = $3, draft_version_id = NULL, updated_at = now(), version = version + 1
		WHERE id = $1 AND tenant_id = $2`, eventID, fix.TenantID, v2ID); err != nil {
		t.Fatalf("update event published pointer: %v", err)
	}
	return v2ID
}
