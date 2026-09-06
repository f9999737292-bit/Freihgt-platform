//go:build integration

package questionnaire

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestMigration000065LegacyDataPreserved(t *testing.T) {
	env, _ := setupLegacyMigrationTestEnv(t)
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)

	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID:       fix.TenantID,
		OwnerCompanyID: fix.CompanyA,
		Title:          "Legacy RFQ",
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		RfxNumber:      "RFX-LEGACY-PRE65",
	})
	if err != nil {
		t.Fatalf("create legacy event before 065: %v", err)
	}

	var eventCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_events WHERE id=$1 AND tenant_id=$2`, event.ID, fix.TenantID).Scan(&eventCount); err != nil {
		t.Fatalf("count legacy event: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected legacy event row before 065")
	}

	if err := applyMigrationFile(ctx, env.pool, "000065_rfx_questionnaire_v3_0b.up.sql"); err != nil {
		t.Fatalf("apply migration 000065: %v", err)
	}

	var exists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_versions'
		)`).Scan(&exists); err != nil {
		t.Fatalf("check rfx_versions after 065: %v", err)
	}
	if !exists {
		t.Fatal("rfx_versions table missing after 000065")
	}

	got, err := env.rfxSvc.GetEvent(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("legacy event unreadable after 065: %v", err)
	}
	if got.ID != event.ID || got.RfxNumber != event.RfxNumber {
		t.Fatalf("legacy event data not preserved")
	}

	var draftVersionID *uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT draft_version_id FROM rfx.rfx_events WHERE id=$1`, event.ID).Scan(&draftVersionID); err != nil {
		t.Fatalf("read draft_version_id column: %v", err)
	}
	if draftVersionID != nil {
		t.Fatalf("legacy event should not have draft_version_id until questionnaire accessed")
	}
}
