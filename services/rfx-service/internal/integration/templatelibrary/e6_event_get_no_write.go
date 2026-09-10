//go:build integration

package templatelibrary

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type eventGetWriteSnapshot struct {
	eventRowVersion           int
	eventUpdatedAt            time.Time
	eventDeletedAt            *time.Time
	sourceTemplateVersionID   *uuid.UUID
	linkedTemplateVersion     int
	linkedTemplateVersionAt   time.Time
	auditCount                int
	idempotencyCount          int
}

func captureEventGetWriteSnapshot(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) eventGetWriteSnapshot {
	t.Helper()
	ctx := context.Background()
	snap := eventGetWriteSnapshot{}

	if err := env.pool.QueryRow(ctx, `
		SELECT version, updated_at, deleted_at, source_template_version_id
		FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2`,
		eventID, fix.TenantID,
	).Scan(&snap.eventRowVersion, &snap.eventUpdatedAt, &snap.eventDeletedAt, &snap.sourceTemplateVersionID); err != nil {
		t.Fatalf("snapshot event row: %v", err)
	}
	if snap.sourceTemplateVersionID != nil {
		if err := env.pool.QueryRow(ctx, `
			SELECT version, updated_at
			FROM rfx.rfx_template_versions
			WHERE id = $1 AND tenant_id = $2`,
			*snap.sourceTemplateVersionID, fix.TenantID,
		).Scan(&snap.linkedTemplateVersion, &snap.linkedTemplateVersionAt); err != nil {
			t.Fatalf("snapshot linked template version row: %v", err)
		}
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`,
		fix.TenantID,
	).Scan(&snap.auditCount); err != nil {
		t.Fatalf("snapshot audit count: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND aggregate_scope = $2`,
		fix.TenantID, eventID,
	).Scan(&snap.idempotencyCount); err != nil {
		t.Fatalf("snapshot idempotency count: %v", err)
	}
	return snap
}

func assertEventGetWriteSnapshotEqual(t *testing.T, before, after eventGetWriteSnapshot) {
	t.Helper()
	if before.eventRowVersion != after.eventRowVersion {
		t.Fatalf("event.version changed: %d -> %d", before.eventRowVersion, after.eventRowVersion)
	}
	if !before.eventUpdatedAt.Equal(after.eventUpdatedAt) {
		t.Fatalf("event.updated_at changed: %v -> %v", before.eventUpdatedAt, after.eventUpdatedAt)
	}
	if !deletedAtEqual(before.eventDeletedAt, after.eventDeletedAt) {
		t.Fatalf("event.deleted_at changed: %v -> %v", before.eventDeletedAt, after.eventDeletedAt)
	}
	if !uuidPtrEqual(before.sourceTemplateVersionID, after.sourceTemplateVersionID) {
		t.Fatalf("event.source_template_version_id changed: %v -> %v", before.sourceTemplateVersionID, after.sourceTemplateVersionID)
	}
	if before.linkedTemplateVersion != after.linkedTemplateVersion {
		t.Fatalf("linked template version row version changed: %d -> %d", before.linkedTemplateVersion, after.linkedTemplateVersion)
	}
	if !before.linkedTemplateVersionAt.Equal(after.linkedTemplateVersionAt) {
		t.Fatalf("linked template version updated_at changed: %v -> %v", before.linkedTemplateVersionAt, after.linkedTemplateVersionAt)
	}
	if before.auditCount != after.auditCount {
		t.Fatalf("audit count changed: %d -> %d", before.auditCount, after.auditCount)
	}
	if before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("idempotency count changed: %d -> %d", before.idempotencyCount, after.idempotencyCount)
	}
}

func uuidPtrEqual(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
