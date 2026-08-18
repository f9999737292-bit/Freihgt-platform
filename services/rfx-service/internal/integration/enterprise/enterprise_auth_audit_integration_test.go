//go:build integration

package enterprise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestRealDBCompanyIsolationDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Owned by A",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-A-1",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	_, err = env.rfxSvc.PublishEvent(ctx, fix.BuyerB, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestRealDBAuditRollbackOnCreate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })

	_, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Rollback",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-ROLL-1",
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	count, err := countRfxEvents(ctx, env.pool, fix.TenantID)
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback, found %d events", count)
	}
}

func TestLifecycleAuditAtomicity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Lifecycle",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-LC-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	updated, err := env.rfxSvc.TransitionEvent(ctx, fix.BuyerA, event.ID, domain.RfxCommandOpenResponses)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if updated.Status != domain.RfxStatusResponsesOpen {
		t.Fatalf("unexpected status %s", updated.Status)
	}
	auditCount, err := countAuditEvents(ctx, env.pool, fix.TenantID, event.ID, "transition")
	if err != nil || auditCount != 1 {
		t.Fatalf("expected transition audit, count=%d err=%v", auditCount, err)
	}

	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	before, _ := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	_, err = env.rfxSvc.TransitionEvent(ctx, fix.BuyerA, event.ID, domain.RfxCommandCloseResponses)
	if err == nil {
		t.Fatal("expected audit failure")
	}
	after, err := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if after.Status != before.Status || after.Version != before.Version {
		t.Fatalf("status/version changed on failed audit: before=%s/%d after=%s/%d",
			before.Status, before.Version, after.Status, after.Version)
	}
}

func TestDeadlineAuditAtomicity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)

	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Deadline",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-DL-1", ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newDeadline := deadline.Add(2 * time.Hour)
	updated, err := env.rfxSvc.ExtendDeadline(ctx, fix.BuyerA, event.ID, newDeadline)
	if err != nil {
		t.Fatalf("extend deadline: %v", err)
	}
	if updated.ResponseDeadline == nil || updated.ResponseDeadline.UTC().Sub(newDeadline.UTC()).Abs() > time.Second {
		t.Fatalf("deadline not updated: got %v want %v", updated.ResponseDeadline, newDeadline)
	}

	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	rollbackDeadline := newDeadline.Add(time.Hour)
	_, err = env.rfxSvc.ExtendDeadline(ctx, fix.BuyerA, event.ID, rollbackDeadline)
	if err == nil {
		t.Fatal("expected audit failure")
	}
	current, err := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if current.ResponseDeadline == nil || current.ResponseDeadline.UTC().Sub(newDeadline.UTC()).Abs() > time.Second {
		t.Fatalf("deadline changed after failed audit: got %v want %v", current.ResponseDeadline, newDeadline)
	}
}

func TestParticipantAuditAtomicity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()

	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Participants",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-P-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	})
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}

	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err = env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, CompanyID: fix.CompanyB, ParticipantType: "CARRIER",
	})
	if err == nil {
		t.Fatal("expected audit failure or conflict")
	}
	var participantCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_participants WHERE rfx_event_id = $1`, event.ID).Scan(&participantCount); err != nil {
		t.Fatalf("count participants: %v", err)
	}
	if participantCount != 1 {
		t.Fatalf("expected one participant after rollback, got %d", participantCount)
	}
}

func TestAutoCloseTransactionalAudit(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Hour)
	eventID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline
		) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', 'Auto close', $4, $5, $6)
	`, eventID, fix.TenantID, "RFX-AUTO-1", fix.CompanyA, domain.RfxStatusResponsesOpen, past)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}

	examined, closed, failures, err := env.rfxSvc.ProcessExpiredResponseDeadlines(ctx, time.Now().UTC(), 10)
	if err != nil || closed != 1 || failures != 0 {
		t.Fatalf("auto close failed examined=%d closed=%d failures=%d err=%v", examined, closed, failures, err)
	}
	event, err := env.rfxRepo.GetEventByID(ctx, eventID, fix.TenantID)
	if err != nil || event.Status != domain.RfxStatusResponsesClosed {
		t.Fatalf("event=%+v err=%v", event, err)
	}

	// Re-open manually for failure injection test.
	_, err = env.pool.Exec(ctx, `UPDATE rfx.rfx_events SET status = $3, response_deadline = $4 WHERE id = $1 AND tenant_id = $2`,
		eventID, fix.TenantID, domain.RfxStatusResponsesOpen, past)
	if err != nil {
		t.Fatalf("reset event: %v", err)
	}
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, closed, failures, err = env.rfxSvc.ProcessExpiredResponseDeadlines(ctx, time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("worker pass: %v", err)
	}
	if closed != 0 || failures != 1 {
		t.Fatalf("expected retry-safe failure closed=%d failures=%d", closed, failures)
	}
	event, err = env.rfxRepo.GetEventByID(ctx, eventID, fix.TenantID)
	if err != nil || event.Status != domain.RfxStatusResponsesOpen {
		t.Fatalf("event should remain open for retry, got status=%s err=%v", event.Status, err)
	}
}

func TestAwardTransactionalIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	carrierB := uuid.New()
	_, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, 'CARRIER')`,
		carrierB, fix.TenantID, "Carrier B")
	if err != nil {
		t.Fatalf("seed carrier b: %v", err)
	}

	frID := uuid.New()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.freight_requests (
			id, tenant_id, freight_request_number, request_type, shipper_company_id, status, response_deadline
		) VALUES ($1, $2, $3, 'MINI_TENDER', $4, $5, $6)
	`, frID, fix.TenantID, "FR-AWARD-1", fix.CompanyA, domain.FreightRequestStatusPublished, deadline)
	if err != nil {
		t.Fatalf("seed freight request: %v", err)
	}
	bidA := uuid.New()
	bidB := uuid.New()
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.bids (id, tenant_id, freight_request_id, carrier_company_id, bid_number, status, total_amount, vat_rate, vat_amount, total_amount_with_vat, submitted_at)
		VALUES ($1, $2, $3, $4, 'BID-A', $5, 100, 20, 20, 120, now()), ($6, $2, $3, $7, 'BID-B', $5, 110, 20, 22, 132, now())
	`, bidA, fix.TenantID, frID, fix.CarrierID, domain.BidStatusSubmitted, bidB, carrierB)
	if err != nil {
		t.Fatalf("seed bids: %v", err)
	}

	accepted, err := env.bidSvc.AcceptBid(ctx, fix.BuyerA, bidA)
	if err != nil {
		t.Fatalf("accept bid: %v", err)
	}
	if accepted.Status != domain.BidStatusAccepted {
		t.Fatalf("unexpected accepted status %s", accepted.Status)
	}
	var statusB string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.bids WHERE id = $1`, bidB).Scan(&statusB); err != nil {
		t.Fatalf("load bid b: %v", err)
	}
	if statusB != domain.BidStatusRejected {
		t.Fatalf("expected bid B rejected, got %s", statusB)
	}
	auditCount, err := countAuditEvents(ctx, env.pool, fix.TenantID, bidA, "accept")
	if err != nil || auditCount != 1 {
		t.Fatalf("expected accept audit, count=%d err=%v", auditCount, err)
	}

	// Reset for rollback test on a fresh freight request.
	frID2 := uuid.New()
	bidC := uuid.New()
	bidD := uuid.New()
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.freight_requests (
			id, tenant_id, freight_request_number, request_type, shipper_company_id, status, response_deadline
		) VALUES ($1, $2, 'FR-AWARD-2', 'MINI_TENDER', $3, $4, $5)
	`, frID2, fix.TenantID, fix.CompanyA, domain.FreightRequestStatusPublished, deadline)
	if err != nil {
		t.Fatalf("seed freight request 2: %v", err)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.bids (id, tenant_id, freight_request_id, carrier_company_id, bid_number, status, total_amount, vat_rate, vat_amount, total_amount_with_vat, submitted_at)
		VALUES ($1, $2, $3, $4, 'BID-C', $5, 100, 20, 20, 120, now()), ($6, $2, $3, $7, 'BID-D', $5, 110, 20, 22, 132, now())
	`, bidC, fix.TenantID, frID2, fix.CarrierID, domain.BidStatusSubmitted, bidD, carrierB)
	if err != nil {
		t.Fatalf("seed bids 2: %v", err)
	}
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err = env.bidSvc.AcceptBid(ctx, fix.BuyerA, bidC)
	if err == nil {
		t.Fatal("expected audit failure")
	}
	var statusC, statusD, frStatus string
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.bids WHERE id = $1`, bidC).Scan(&statusC); err != nil {
		t.Fatalf("load bid c: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.bids WHERE id = $1`, bidD).Scan(&statusD); err != nil {
		t.Fatalf("load bid d: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT status FROM rfx.freight_requests WHERE id = $1`, frID2).Scan(&frStatus); err != nil {
		t.Fatalf("load fr: %v", err)
	}
	if statusC != domain.BidStatusSubmitted || statusD != domain.BidStatusSubmitted || frStatus != domain.FreightRequestStatusPublished {
		t.Fatalf("award rollback failed: bidC=%s bidD=%s fr=%s", statusC, statusD, frStatus)
	}
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected code %s, got %v", code, err)
	}
}
