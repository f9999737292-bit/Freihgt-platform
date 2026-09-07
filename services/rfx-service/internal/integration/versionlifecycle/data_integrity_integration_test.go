//go:build integration

package versionlifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestE1INT21PublishedVersionCrossEventDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	eventA := createDraftEvent(t, env, fix, "RFX-E1-21-A")
	eventB := createDraftEvent(t, env, fix, "RFX-E1-21-B")
	vA := ensurePublishedVersion(t, env, fix, eventA.ID, "e1-int-21-a")
	ensurePublishedVersion(t, env, fix, eventB.ID, "e1-int-21-b")

	var before *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, eventB.ID).Scan(&before); err != nil {
		t.Fatalf("read pointer before: %v", err)
	}

	_, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_events
		SET published_version_id = $2
		WHERE id = $1 AND tenant_id = $3`,
		eventB.ID, vA.ID, fix.TenantID)
	assertPostgreSQLErrorCode(t, err, "23503")

	var after *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, eventB.ID).Scan(&after); err != nil {
		t.Fatalf("read pointer after: %v", err)
	}
	if (before == nil) != (after == nil) || (before != nil && after != nil && *before != *after) {
		t.Fatalf("published_version_id changed: before=%v after=%v", before, after)
	}
}

func TestE1INT22PublishedVersionCrossTenantDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	eventA := createDraftEvent(t, env, fix, "RFX-E1-22-A")
	vA := ensurePublishedVersion(t, env, fix, eventA.ID, "e1-int-22-a")

	otherTenant := fix.OtherTenantID
	otherCompany := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		otherCompany, otherTenant, "Other Buyer", "SHIPPER"); err != nil {
		t.Fatalf("seed other company: %v", err)
	}
	otherUser := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`,
		otherUser, otherTenant, "other-buyer@test.local", "other-buyer@test.local"); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	var buyerRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&buyerRoleID); err != nil {
		t.Fatalf("lookup buyer role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`,
		otherTenant, otherCompany, otherUser); err != nil {
		t.Fatalf("seed other membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`,
		otherTenant, otherUser, otherCompany, buyerRoleID); err != nil {
		t.Fatalf("seed other role: %v", err)
	}
	otherActor := domain.ActorContext{TenantID: otherTenant, UserID: otherUser}
	eventB, err := env.rfxSvc.CreateEvent(ctx, otherActor, domain.CreateRfxEventInput{
		TenantID:       otherTenant,
		OwnerCompanyID: otherCompany,
		Title:          "Other tenant event",
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		RfxNumber:      "RFX-E1-22-B",
	})
	if err != nil {
		t.Fatalf("create other tenant event: %v", err)
	}
	otherVersionID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (
			id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at
		) VALUES ($1, $2, $3, 1, 'PUBLISHED', TRUE, now())`,
		otherVersionID, otherTenant, eventB.ID); err != nil {
		t.Fatalf("insert other tenant published version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_events SET published_version_id = $2 WHERE id = $1 AND tenant_id = $3`,
		eventB.ID, otherVersionID, otherTenant); err != nil {
		t.Fatalf("set other tenant published pointer: %v", err)
	}

	var before *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, eventB.ID).Scan(&before); err != nil {
		t.Fatalf("read pointer before: %v", err)
	}

	_, err = env.pool.Exec(ctx, `
		UPDATE rfx.rfx_events
		SET published_version_id = $2
		WHERE id = $1 AND tenant_id = $3`,
		eventB.ID, vA.ID, otherTenant)
	assertPostgreSQLErrorCode(t, err, "23503")

	var after *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT published_version_id FROM rfx.rfx_events WHERE id = $1`, eventB.ID).Scan(&after); err != nil {
		t.Fatalf("read pointer after: %v", err)
	}
	if before == nil || after == nil || *before != *after || *after != otherVersionID {
		t.Fatalf("cross-tenant pointer mutated: before=%v after=%v", before, after)
	}
}

func TestE1INT23SupersededByCrossEventDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	eventA := createDraftEvent(t, env, fix, "RFX-E1-23-A")
	eventB := createDraftEvent(t, env, fix, "RFX-E1-23-B")
	vA := ensurePublishedVersion(t, env, fix, eventA.ID, "e1-int-23-a")
	vB := ensurePublishedVersion(t, env, fix, eventB.ID, "e1-int-23-b")

	_, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions
		SET superseded_by_version_id = $2
		WHERE id = $1 AND tenant_id = $3`,
		vB.ID, vA.ID, fix.TenantID)
	assertPostgreSQLErrorCode(t, err, "23503")

	var supersededBy *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT superseded_by_version_id FROM rfx.rfx_versions WHERE id = $1`, vB.ID).Scan(&supersededBy); err != nil {
		t.Fatalf("read superseded_by after: %v", err)
	}
	if supersededBy != nil {
		t.Fatalf("expected superseded_by unchanged nil, got %v", supersededBy)
	}
}

func TestE1INT24SupersededByCrossTenantDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	eventA := createDraftEvent(t, env, fix, "RFX-E1-24-A")
	vA := ensurePublishedVersion(t, env, fix, eventA.ID, "e1-int-24-a")

	otherTenant := fix.OtherTenantID
	otherVersionID := uuid.New()
	otherEventID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, owner_company_id, title, rfx_type, category, rfx_number, status, version
		) VALUES ($1, $2, $3, 'Other', 'SPOT_RFQ', 'FREIGHT', 'RFX-E1-24-B', 'DRAFT', 1)`,
		otherEventID, otherTenant, uuid.New()); err != nil {
		t.Fatalf("insert other tenant event: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_versions (
			id, tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, published_at
		) VALUES ($1, $2, $3, 1, 'PUBLISHED', TRUE, now())`,
		otherVersionID, otherTenant, otherEventID); err != nil {
		t.Fatalf("insert other tenant version: %v", err)
	}

	_, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions
		SET superseded_by_version_id = $2
		WHERE id = $1 AND tenant_id = $3`,
		otherVersionID, vA.ID, otherTenant)
	assertPostgreSQLErrorCode(t, err, "23503")

	var supersededBy *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT superseded_by_version_id FROM rfx.rfx_versions WHERE id = $1`, otherVersionID).Scan(&supersededBy); err != nil {
		t.Fatalf("read superseded_by after: %v", err)
	}
	if supersededBy != nil {
		t.Fatalf("expected superseded_by unchanged nil, got %v", supersededBy)
	}
}

func TestE1INT25SupersededBySelfDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-E1-25")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-25")

	_, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_versions
		SET superseded_by_version_id = $1
		WHERE id = $1 AND tenant_id = $2`,
		v1.ID, fix.TenantID)
	assertPostgreSQLErrorCode(t, err, "23514")

	var supersededBy *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT superseded_by_version_id FROM rfx.rfx_versions WHERE id = $1`, v1.ID).Scan(&supersededBy); err != nil {
		t.Fatalf("read superseded_by after: %v", err)
	}
	if supersededBy != nil {
		t.Fatalf("expected superseded_by unchanged nil, got %v", supersededBy)
	}
}

func TestE1INT26NormalPublishV1ToV2CompositeConstraintsPass(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-E1-26")
	v1 := ensurePublishedVersion(t, env, fix, event.ID, "e1-int-26-v1")
	v2 := publishV2WithoutResponses(t, env, fix, event.ID, "e1-int-26-v2", "e1-int-26-fork")

	v1Reload, err := env.qRepo.GetVersionByID(context.Background(), v1.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload v1: %v", err)
	}
	if v1Reload.Status != domain.RfxVersionStatusSuperseded {
		t.Fatalf("v1 status=%s", v1Reload.Status)
	}
	if v1Reload.SupersededByVersionID == nil || *v1Reload.SupersededByVersionID != v2.ID {
		t.Fatalf("v1 superseded_by=%v want %s", v1Reload.SupersededByVersionID, v2.ID)
	}
	if v2.Status != domain.RfxVersionStatusPublished || !v2.IsCurrentPublished {
		t.Fatalf("v2 not current published: %+v", v2)
	}

	var compositeFKExists bool
	if err := env.pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = 'fk_rfx_events_published_version_composite'
		)`).Scan(&compositeFKExists); err != nil {
		t.Fatalf("check composite fk: %v", err)
	}
	if !compositeFKExists {
		t.Fatal("expected published_version composite FK to exist")
	}
}

func assertPostgreSQLErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected postgres error code %s", code)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("expected postgres error code %s, got %v", code, err)
	}
}
