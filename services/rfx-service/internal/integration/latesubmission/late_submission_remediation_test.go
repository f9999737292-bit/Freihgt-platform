//go:build integration

package latesubmission

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestE7REM001RerequestAfterExpiredApprovedMaterializesExpired(t *testing.T) {
	env, fix, event := seedExpiredApprovedPermission(t)
	ctx := context.Background()

	created, err := env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "retry", RequestedUntil: fixedLateNow().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create after expiry: %v", err)
	}
	if created.Status != domain.LateSubmissionStatusRequested {
		t.Fatalf("new status=%s", created.Status)
	}

	items, err := env.lateSvc.ListOwnRequests(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 historical rows, got %d", len(items))
	}
	var expiredCount, requestedCount int
	for _, item := range items {
		switch item.Status {
		case domain.LateSubmissionStatusExpired:
			expiredCount++
		case domain.LateSubmissionStatusRequested:
			requestedCount++
		}
	}
	if expiredCount != 1 || requestedCount != 1 {
		t.Fatalf("expired=%d requested=%d", expiredCount, requestedCount)
	}

	var dbStatus string
	err = env.pool.QueryRow(ctx, `
		SELECT status FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3 AND status = 'EXPIRED'
	`, fix.TenantID, event.ID, fix.CarrierID).Scan(&dbStatus)
	if err != nil {
		t.Fatalf("expired row missing in db: %v", err)
	}
}

func TestE7REM002RequestedBlocksNewRequest(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	event, _ := seedPublishedEventAfterDeadline(t, env, fix)
	ctx := context.Background()
	_, err := env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "first", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "second", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE7REM003ActiveApprovedBlocksNewRequest(t *testing.T) {
	env, fix, event, _, _ := seedApprovedLateWindow(t)
	ctx := context.Background()
	_, err := env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "blocked", RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
	})
	assertAppErrorCode(t, err, apperrors.CodeConflict)
}

func TestE7REM004ConcurrentRerequestAfterExpiryOneBusinessResult(t *testing.T) {
	env, fix, event := seedExpiredApprovedPermission(t)
	ctx := context.Background()
	in := domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "retry", RequestedUntil: fixedLateNow().Add(24 * time.Hour),
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wins, fails int
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "shared-rerequest-key"
			if i > 0 {
				key = uuid.NewString()
			}
			_, err := env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, key, in)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				wins++
			} else {
				fails++
			}
		}(i)
	}
	wg.Wait()
	if wins < 1 {
		t.Fatalf("expected at least one successful rerequest, wins=%d fails=%d", wins, fails)
	}
	active, err := env.lateRepo.GetActiveByEventAndCarrier(ctx, fix.TenantID, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("active lookup: %v", err)
	}
	if active == nil || active.Status != domain.LateSubmissionStatusRequested {
		t.Fatalf("expected single active REQUESTED, got %+v", active)
	}
}

func TestE7REM005OtherCarrierCannotMaterializeOrRerequest(t *testing.T) {
	env, fix, event := seedExpiredApprovedPermission(t)
	ctx := context.Background()
	_, err := env.lateSvc.CreateRequest(ctx, fix.CarrierBAct, event.ID, fix.CarrierBID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "other", RequestedUntil: fixedLateNow().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("carrier B create: %v", err)
	}
	_, err = env.lateSvc.CreateRequest(ctx, fix.CarrierBAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "spoof", RequestedUntil: fixedLateNow().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("carrier B must not create request for carrier A permission")
	}
}

func TestE7REM006AuditFailureRollsBackExpiryAndCreate(t *testing.T) {
	env, fix, event := seedExpiredApprovedPermission(t)
	env.auditRepo.SetInjectRecordFailure(true)
	t.Cleanup(func() { env.auditRepo.SetInjectRecordFailure(false) })
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "audit fail", RequestedUntil: fixedLateNow().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected audit failure rollback")
	}
	var approvedCount int
	err = env.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3 AND status = 'APPROVED'
	`, fix.TenantID, event.ID, fix.CarrierID).Scan(&approvedCount)
	if err != nil {
		t.Fatalf("count approved: %v", err)
	}
	if approvedCount != 1 {
		t.Fatalf("approved row must remain when audit fails, count=%d", approvedCount)
	}
	items, _ := env.lateSvc.ListOwnRequests(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID)
	if len(items) != 1 || items[0].Status != domain.LateSubmissionStatusExpired {
		t.Fatalf("expected single computed expired row, got %+v", items)
	}
}

func TestE7REM007IdempotencyFailureRollsBackExpiryAndCreate(t *testing.T) {
	env, fix, event := seedExpiredApprovedPermission(t)
	env.idemRepo.SetInjectStoreFailure(true)
	t.Cleanup(func() { env.idemRepo.SetInjectStoreFailure(false) })
	_, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "idem fail", RequestedUntil: fixedLateNow().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected idempotency failure rollback")
	}
	var approvedCount int
	err = env.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3 AND status = 'APPROVED'
	`, fix.TenantID, event.ID, fix.CarrierID).Scan(&approvedCount)
	if err != nil {
		t.Fatalf("count approved: %v", err)
	}
	if approvedCount != 1 {
		t.Fatalf("approved row must remain when idempotency fails, count=%d", approvedCount)
	}
}

func TestE7REM008SubmitInValidWindowNotBrokenByConcurrentRerequestAfterSeparateExpiry(t *testing.T) {
	env, fix, event, q, saved := seedApprovedLateWindow(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, _ = env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"ok"`)}},
	})
	ws, _ := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	key := uuid.NewString()
	var wg sync.WaitGroup
	var submitErr, createErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, submitErr = env.crSvc.Submit(ctx, fix.CarrierAct, event.ID, fix.CarrierID, ws.Response.SaveVersion, key)
	}()
	go func() {
		defer wg.Done()
		_, createErr = env.lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
			ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "late", RequestedUntil: now.Add(24 * time.Hour),
		})
	}()
	wg.Wait()
	if submitErr != nil {
		t.Fatalf("submit in valid window failed: %v", submitErr)
	}
	if createErr == nil {
		t.Fatal("concurrent create must fail while permission is consumed or active")
	}
}

func seedExpiredApprovedPermission(t *testing.T) (*testEnv, buyerFixture, *domain.RfxEvent) {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	env.lateSvc.SetNowFunc(func() time.Time { return now })
	event, _ := seedPublishedEventAfterDeadlineAt(t, env, fix, now)
	created, err := env.lateSvc.CreateRequest(context.Background(), fix.CarrierAct, event.ID, fix.CarrierID, uuid.NewString(), domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "first", RequestedUntil: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	from := now.Add(-2 * time.Hour)
	until := now
	approveRequest(t, env, fix, event.ID, created.ID, created.Version, from, until)
	return env, fix, event
}

func fixedLateNow() time.Time {
	return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
}
