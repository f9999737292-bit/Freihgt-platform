//go:build integration

package latesubmission

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestE7INT35MigrationUpPresent(t *testing.T) {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	up := filepath.Join(migrationsDir, "000072_rfx_late_submission_v3_0e7.up.sql")
	if _, err := os.Stat(up); err != nil {
		t.Fatalf("missing migration up: %v", err)
	}
}

func TestE7INT36MigrationDownSearchPath(t *testing.T) {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	down, err := os.ReadFile(filepath.Join(migrationsDir, "000072_rfx_late_submission_v3_0e7.down.sql"))
	if err != nil {
		t.Fatalf("read down: %v", err)
	}
	if !strings.Contains(string(down), "SET search_path = public") {
		t.Fatal("down migration must set search_path=public")
	}
}

func TestE7INT37UpAfterDown(t *testing.T) {
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer dropDB(context.Background())
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("up all: %v", err)
	}
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	downSQL, _ := os.ReadFile(filepath.Join(migrationsDir, "000072_rfx_late_submission_v3_0e7.down.sql"))
	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("down 000072: %v", err)
	}
	upSQL, _ := os.ReadFile(filepath.Join(migrationsDir, "000072_rfx_late_submission_v3_0e7.up.sql"))
	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("up 000072 again: %v", err)
	}
}

func TestE7INT38InvalidStatusReasonRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	eventID := uuid.New()
	tenantID := fix.TenantID
	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_events (id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline)
		VALUES ($1, $2, 'RFX-CHK-1', 'SPOT_RFQ', 'FREIGHT', 'Check', $3, 'PUBLISHED', now() - interval '1 hour')
	`, eventID, tenantID, fix.CompanyA)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_participants (id, tenant_id, rfx_event_id, company_id, participant_type, status)
		VALUES ($1, $2, $3, $4, 'CARRIER', 'INVITED')
	`, uuid.New(), tenantID, eventID, fix.CarrierID)
	if err != nil {
		t.Fatalf("seed participant: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_late_submission_requests (
			tenant_id, rfx_event_id, carrier_company_id, reason_code, reason_text, requested_until, status, requested_by
		) VALUES ($1, $2, $3, 'OTHER', 'x', $4, 'INVALID', $5)
	`, tenantID, eventID, fix.CarrierID, time.Now().UTC().Add(time.Hour), fix.CarrierAct.UserID)
	if err == nil {
		t.Fatal("expected invalid status rejection")
	}
}

func TestE7INT39CrossEventFKRejected(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	eventA, eventB := uuid.New(), uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_events (id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline)
		VALUES ($1, $2, 'RFX-A', 'SPOT_RFQ', 'FREIGHT', 'A', $3, 'PUBLISHED', now() - interval '1 hour'),
		       ($4, $2, 'RFX-B', 'SPOT_RFQ', 'FREIGHT', 'B', $3, 'PUBLISHED', now() - interval '1 hour')
	`, eventA, fix.TenantID, fix.CompanyA, eventB)
	if err != nil {
		t.Fatalf("seed events: %v", err)
	}
	partID := uuid.New()
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_participants (id, tenant_id, rfx_event_id, company_id, participant_type, status)
		VALUES ($1, $2, $3, $4, 'CARRIER', 'INVITED')
	`, partID, fix.TenantID, eventA, fix.CarrierID)
	if err != nil {
		t.Fatalf("participant: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_late_submission_requests (
			tenant_id, rfx_event_id, carrier_company_id, participant_id, reason_code, reason_text, requested_until, status, requested_by
		) VALUES ($1, $2, $3, $4, 'OTHER', 'x', $5, 'REQUESTED', $6)
	`, fix.TenantID, eventB, fix.CarrierID, partID, time.Now().UTC().Add(time.Hour), fix.CarrierAct.UserID)
	if err == nil {
		t.Fatal("expected cross-event participant FK rejection")
	}
}
