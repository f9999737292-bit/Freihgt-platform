//go:build integration

package carrierresponse

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestMigration000066FreshAndLegacy(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	var answersExists bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='rfx' AND table_name='rfx_answers'
		)`).Scan(&answersExists); err != nil {
		t.Fatalf("check rfx_answers: %v", err)
	}
	if !answersExists {
		t.Fatal("migration 000066: rfx_answers missing on fresh DB")
	}

	var saveVersionCol bool
	if err := env.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='rfx' AND table_name='rfx_responses' AND column_name='save_version'
		)`).Scan(&saveVersionCol); err != nil {
		t.Fatalf("check save_version column: %v", err)
	}
	if !saveVersionCol {
		t.Fatal("migration 000066: save_version column missing on fresh DB")
	}
}

func TestMigration000066LegacyDataPreserved(t *testing.T) {
	env, _ := setupLegacyMigrationTestEnv(t)
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)

	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Legacy Response Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-LEGACY-066",
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var legacyID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO rfx.rfx_responses (tenant_id, rfx_event_id, participant_company_id, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		fix.TenantID, event.ID, fix.CarrierID, domain.RfxResponseStatusDraft,
	).Scan(&legacyID); err != nil {
		t.Fatalf("insert legacy response before 066: %v", err)
	}

	if err := applyMigrationFile(ctx, env.pool, "000066_rfx_carrier_response_v3_0c.up.sql"); err != nil {
		t.Fatalf("apply migration 000066: %v", err)
	}

	var status string
	var saveVersion int64
	var completion float64
	if err := env.pool.QueryRow(ctx, `
		SELECT status, save_version, completion_percent
		FROM rfx.rfx_responses WHERE id=$1 AND tenant_id=$2`,
		legacyID, fix.TenantID).Scan(&status, &saveVersion, &completion); err != nil {
		t.Fatalf("read legacy response after 066: %v", err)
	}
	if status != domain.RfxResponseStatusDraft {
		t.Fatalf("legacy status not preserved: %s", status)
	}
	if saveVersion != 0 {
		t.Fatalf("expected default save_version=0, got %d", saveVersion)
	}
	if completion != 0 {
		t.Fatalf("expected default completion_percent=0, got %v", completion)
	}

	got, err := env.rfxSvc.GetResponse(ctx, fix.CarrierAct, legacyID)
	if err != nil {
		t.Fatalf("legacy response unreadable after 066: %v", err)
	}
	if got.ID != legacyID {
		t.Fatal("legacy response id not preserved")
	}
}
