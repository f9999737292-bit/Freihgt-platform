//go:build integration

package templatelibrary

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type templateVersionQuestionnaireWriteSnapshot struct {
	templateRowVersion int
	templateUpdatedAt  time.Time
	templateDeletedAt  *time.Time
	versionRowVersion  int
	versionUpdatedAt   time.Time
	versionDeletedAt   *time.Time
	templateVersionCnt int
	sectionCount       int
	questionCount      int
	optionCount        int
	ruleCount          int
	auditCount         int
	idempotencyCount   int
}

func captureTemplateVersionQuestionnaireWriteSnapshot(t *testing.T, env *testEnv, fix buyerFixture, templateID, versionID uuid.UUID) templateVersionQuestionnaireWriteSnapshot {
	t.Helper()
	ctx := context.Background()
	snap := templateVersionQuestionnaireWriteSnapshot{}

	if err := env.pool.QueryRow(ctx, `
		SELECT version, updated_at, deleted_at
		FROM rfx.rfx_templates
		WHERE id = $1 AND tenant_id = $2`,
		templateID, fix.TenantID,
	).Scan(&snap.templateRowVersion, &snap.templateUpdatedAt, &snap.templateDeletedAt); err != nil {
		t.Fatalf("snapshot template row: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT version, updated_at, deleted_at
		FROM rfx.rfx_template_versions
		WHERE id = $1 AND tenant_id = $2 AND template_id = $3`,
		versionID, fix.TenantID, templateID,
	).Scan(&snap.versionRowVersion, &snap.versionUpdatedAt, &snap.versionDeletedAt); err != nil {
		t.Fatalf("snapshot template version row: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_versions
		WHERE template_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		templateID, fix.TenantID,
	).Scan(&snap.templateVersionCnt); err != nil {
		t.Fatalf("snapshot template version count: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_sections
		WHERE template_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		templateID, fix.TenantID,
	).Scan(&snap.sectionCount); err != nil {
		t.Fatalf("snapshot section count: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_questions q
		INNER JOIN rfx.rfx_template_sections s ON s.id = q.section_id AND s.tenant_id = q.tenant_id
		WHERE q.template_id = $1 AND q.tenant_id = $2 AND q.deleted_at IS NULL AND s.deleted_at IS NULL`,
		templateID, fix.TenantID,
	).Scan(&snap.questionCount); err != nil {
		t.Fatalf("snapshot question count: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_question_options o
		INNER JOIN rfx.rfx_template_questions q ON q.id = o.question_id AND q.tenant_id = o.tenant_id
		WHERE q.template_id = $1 AND q.tenant_id = $2 AND o.deleted_at IS NULL AND q.deleted_at IS NULL`,
		templateID, fix.TenantID,
	).Scan(&snap.optionCount); err != nil {
		t.Fatalf("snapshot option count: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_template_question_rules
		WHERE template_id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		templateID, fix.TenantID,
	).Scan(&snap.ruleCount); err != nil {
		t.Fatalf("snapshot rule count: %v", err)
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
		fix.TenantID, templateID,
	).Scan(&snap.idempotencyCount); err != nil {
		t.Fatalf("snapshot idempotency count: %v", err)
	}
	return snap
}

func assertTemplateVersionQuestionnaireWriteSnapshotEqual(t *testing.T, before, after templateVersionQuestionnaireWriteSnapshot) {
	t.Helper()
	if before.templateRowVersion != after.templateRowVersion {
		t.Fatalf("template.version changed: %d -> %d", before.templateRowVersion, after.templateRowVersion)
	}
	if !before.templateUpdatedAt.Equal(after.templateUpdatedAt) {
		t.Fatalf("template.updated_at changed: %v -> %v", before.templateUpdatedAt, after.templateUpdatedAt)
	}
	if !deletedAtEqual(before.templateDeletedAt, after.templateDeletedAt) {
		t.Fatalf("template.deleted_at changed: %v -> %v", before.templateDeletedAt, after.templateDeletedAt)
	}
	if before.versionRowVersion != after.versionRowVersion {
		t.Fatalf("template_version.version changed: %d -> %d", before.versionRowVersion, after.versionRowVersion)
	}
	if !before.versionUpdatedAt.Equal(after.versionUpdatedAt) {
		t.Fatalf("template_version.updated_at changed: %v -> %v", before.versionUpdatedAt, after.versionUpdatedAt)
	}
	if !deletedAtEqual(before.versionDeletedAt, after.versionDeletedAt) {
		t.Fatalf("template_version.deleted_at changed: %v -> %v", before.versionDeletedAt, after.versionDeletedAt)
	}
	if before.templateVersionCnt != after.templateVersionCnt {
		t.Fatalf("template version count changed: %d -> %d", before.templateVersionCnt, after.templateVersionCnt)
	}
	if before.sectionCount != after.sectionCount {
		t.Fatalf("section count changed: %d -> %d", before.sectionCount, after.sectionCount)
	}
	if before.questionCount != after.questionCount {
		t.Fatalf("question count changed: %d -> %d", before.questionCount, after.questionCount)
	}
	if before.optionCount != after.optionCount {
		t.Fatalf("option count changed: %d -> %d", before.optionCount, after.optionCount)
	}
	if before.ruleCount != after.ruleCount {
		t.Fatalf("rule count changed: %d -> %d", before.ruleCount, after.ruleCount)
	}
	if before.auditCount != after.auditCount {
		t.Fatalf("audit count changed: %d -> %d", before.auditCount, after.auditCount)
	}
	if before.idempotencyCount != after.idempotencyCount {
		t.Fatalf("idempotency count changed: %d -> %d", before.idempotencyCount, after.idempotencyCount)
	}
}

func deletedAtEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}
