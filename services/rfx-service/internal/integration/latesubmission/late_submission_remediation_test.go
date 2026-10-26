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
	fixup := prepareOpenLateWindowSubmission(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	var submitResult *domain.SubmitResult
	var submitErr error
	var created *domain.LateSubmissionRequest
	var createErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		submitResult, submitErr = fixup.env.crSvc.Submit(ctx, fixup.fix.CarrierAct, fixup.event.ID, fixup.fix.CarrierID, fixup.saveVersion, uuid.NewString())
	}()
	go func() {
		defer wg.Done()
		created, createErr = fixup.env.lateSvc.CreateRequest(ctx, fixup.fix.CarrierAct, fixup.event.ID, fixup.fix.CarrierID, uuid.NewString(), lateCreateInput(fixup.now))
	}()
	wg.Wait()
	assertOpenWindowSubmitAndCreateOutcome(t, fixup, submitResult, submitErr, created, createErr)
}

func TestE7REM008SequentialSubmitThenCreateAllowsRequested(t *testing.T) {
	fixup := prepareOpenLateWindowSubmission(t)
	ctx := context.Background()
	submitResult, submitErr := fixup.env.crSvc.Submit(ctx, fixup.fix.CarrierAct, fixup.event.ID, fixup.fix.CarrierID, fixup.saveVersion, uuid.NewString())
	created, createErr := fixup.env.lateSvc.CreateRequest(ctx, fixup.fix.CarrierAct, fixup.event.ID, fixup.fix.CarrierID, uuid.NewString(), lateCreateInput(fixup.now))
	assertOpenWindowSubmitAndCreateOutcome(t, fixup, submitResult, submitErr, created, createErr)
	if createErr != nil || created == nil {
		t.Fatal("sequential create after a consumed permission must succeed")
	}
}

func TestE7REM008SequentialApprovedCreateConflictsThenSubmit(t *testing.T) {
	fixup := prepareOpenLateWindowSubmission(t)
	ctx := context.Background()
	created, createErr := fixup.env.lateSvc.CreateRequest(ctx, fixup.fix.CarrierAct, fixup.event.ID, fixup.fix.CarrierID, uuid.NewString(), lateCreateInput(fixup.now))
	if created != nil {
		t.Fatalf("create while APPROVED returned a request and error=%v", createErr)
	}
	assertAppErrorCode(t, createErr, apperrors.CodeConflict)
	if countTenantActiveLateRequests(t, fixup.env, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID) != 1 {
		t.Fatal("rejected create changed the active request count")
	}
	stillActive, err := fixup.env.lateRepo.GetActiveByEventAndCarrier(ctx, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID)
	if err != nil {
		t.Fatalf("active lookup after rejected create: %v", err)
	}
	if stillActive == nil || stillActive.ID != fixup.original.ID || stillActive.Status != domain.LateSubmissionStatusApproved {
		t.Fatalf("original APPROVED permission changed before submit: %+v", stillActive)
	}
	submitResult, submitErr := fixup.env.crSvc.Submit(ctx, fixup.fix.CarrierAct, fixup.event.ID, fixup.fix.CarrierID, fixup.saveVersion, uuid.NewString())
	assertOpenWindowSubmitAndCreateOutcome(t, fixup, submitResult, submitErr, nil, createErr)
}

type openLateWindowSubmission struct {
	env         *testEnv
	fix         buyerFixture
	event       *domain.RfxEvent
	original    *domain.LateSubmissionRequest
	saveVersion int64
	now         time.Time
}

func prepareOpenLateWindowSubmission(t *testing.T) openLateWindowSubmission {
	t.Helper()
	env, fix, event, q, saved := seedApprovedLateWindow(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: saved.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: q.ID, Value: json.RawMessage(`"ok"`)}},
	}); err != nil {
		t.Fatalf("save answers: %v", err)
	}
	ws, err := env.crSvc.GetWorkspace(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	original, err := env.lateRepo.GetActiveByEventAndCarrier(ctx, fix.TenantID, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("load approved permission: %v", err)
	}
	if original == nil || original.Status != domain.LateSubmissionStatusApproved || original.CarrierCompanyID != fix.CarrierID {
		t.Fatalf("expected one APPROVED permission, got %+v", original)
	}
	if status := tenantLateRequestStatus(t, env, original.ID, fix.TenantID, event.ID, fix.CarrierID); status != string(domain.LateSubmissionStatusApproved) {
		t.Fatalf("tenant-scoped status before race=%s", status)
	}
	return openLateWindowSubmission{env: env, fix: fix, event: event, original: original, saveVersion: ws.Response.SaveVersion, now: now}
}

func lateCreateInput(now time.Time) domain.CreateLateSubmissionRequestInput {
	return domain.CreateLateSubmissionRequestInput{
		ReasonCode: domain.LateSubmissionReasonOther, ReasonText: "late", RequestedUntil: now.Add(24 * time.Hour),
	}
}

func assertOpenWindowSubmitAndCreateOutcome(t *testing.T, fixup openLateWindowSubmission, submitResult *domain.SubmitResult, submitErr error, created *domain.LateSubmissionRequest, createErr error) {
	t.Helper()
	if submitErr != nil || submitResult == nil {
		t.Fatalf("submit in valid window failed: result=%v err=%v", submitResult, submitErr)
	}
	ctx := context.Background()
	stored, err := fixup.env.lateRepo.GetByID(ctx, fixup.original.ID, fixup.fix.TenantID, fixup.event.ID)
	if err != nil {
		t.Fatalf("reload original permission: %v", err)
	}
	if stored.Status != domain.LateSubmissionStatusConsumed || stored.CarrierCompanyID != fixup.fix.CarrierID || stored.TenantID != fixup.fix.TenantID {
		t.Fatalf("original permission after submit: status=%s tenant=%s carrier=%s", stored.Status, stored.TenantID, stored.CarrierCompanyID)
	}
	if status := tenantLateRequestStatus(t, fixup.env, fixup.original.ID, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID); status != string(domain.LateSubmissionStatusConsumed) {
		t.Fatalf("tenant-scoped original status=%s", status)
	}
	activeCount := countTenantActiveLateRequests(t, fixup.env, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID)
	if activeCount > 1 {
		t.Fatalf("active REQUESTED/APPROVED count=%d", activeCount)
	}
	listed, err := fixup.env.lateRepo.ListOwnByEventAndCarrier(ctx, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID)
	if err != nil {
		t.Fatalf("list own requests: %v", err)
	}

	switch {
	case createErr == nil && created != nil:
		if created.ID == fixup.original.ID {
			t.Fatal("new request reuses the consumed permission id")
		}
		if created.Status != domain.LateSubmissionStatusRequested || created.CarrierCompanyID != fixup.fix.CarrierID || created.TenantID != fixup.fix.TenantID {
			t.Fatalf("created request status=%s tenant=%s carrier=%s", created.Status, created.TenantID, created.CarrierCompanyID)
		}
		row, err := fixup.env.lateRepo.GetByID(ctx, created.ID, fixup.fix.TenantID, fixup.event.ID)
		if err != nil {
			t.Fatalf("created request missing from repository: %v", err)
		}
		if row.Status != domain.LateSubmissionStatusRequested {
			t.Fatalf("stored created status=%s", row.Status)
		}
		if status := tenantLateRequestStatus(t, fixup.env, created.ID, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID); status != string(domain.LateSubmissionStatusRequested) {
			t.Fatalf("tenant-scoped created status=%s", status)
		}
		if activeCount != 1 {
			t.Fatalf("successful create left %d active requests", activeCount)
		}
		active, err := fixup.env.lateRepo.GetActiveByEventAndCarrier(ctx, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID)
		if err != nil {
			t.Fatalf("active lookup: %v", err)
		}
		if active == nil || active.ID != created.ID || active.Status != domain.LateSubmissionStatusRequested {
			t.Fatalf("active request=%+v", active)
		}
		if !lateRequestListContains(listed, created.ID, domain.LateSubmissionStatusRequested) || !lateRequestListContains(listed, fixup.original.ID, domain.LateSubmissionStatusConsumed) {
			t.Fatalf("repository list missing consumed permission or new request: %+v", listed)
		}
	case createErr != nil && created == nil:
		assertAppErrorCode(t, createErr, apperrors.CodeConflict)
		if activeCount != 0 {
			t.Fatalf("rejected create left %d active requests", activeCount)
		}
		active, err := fixup.env.lateRepo.GetActiveByEventAndCarrier(ctx, fixup.fix.TenantID, fixup.event.ID, fixup.fix.CarrierID)
		if err != nil {
			t.Fatalf("active lookup after conflict: %v", err)
		}
		if active != nil {
			t.Fatalf("rejected create left active request %s", active.ID)
		}
		if len(listed) != 1 || !lateRequestListContains(listed, fixup.original.ID, domain.LateSubmissionStatusConsumed) {
			t.Fatalf("rejected create persisted an extra request: %+v", listed)
		}
	default:
		t.Fatalf("unexpected create outcome: created=%v err=%v", created != nil, createErr)
	}
	if createErr == nil {
		t.Log("late-create-outcome=REQUESTED_AFTER_CONSUMED")
	} else {
		t.Log("late-create-outcome=CONFLICT")
	}
}

func countTenantActiveLateRequests(t *testing.T, env *testEnv, tenantID, eventID, carrierID uuid.UUID) int {
	t.Helper()
	var count int
	err := env.pool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3
			AND status IN ('REQUESTED', 'APPROVED')
	`, tenantID, eventID, carrierID).Scan(&count)
	if err != nil {
		t.Fatalf("count active late requests: %v", err)
	}
	return count
}

func tenantLateRequestStatus(t *testing.T, env *testEnv, id, tenantID, eventID, carrierID uuid.UUID) string {
	t.Helper()
	var status string
	err := env.pool.QueryRow(context.Background(), `
		SELECT status
		FROM rfx.rfx_late_submission_requests
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3 AND carrier_company_id = $4
	`, id, tenantID, eventID, carrierID).Scan(&status)
	if err != nil {
		t.Fatalf("tenant-scoped late request %s: %v", id, err)
	}
	return status
}

func lateRequestListContains(items []domain.LateSubmissionRequest, id uuid.UUID, status domain.LateSubmissionStatus) bool {
	for _, item := range items {
		if item.ID == id && item.Status == status {
			return true
		}
	}
	return false
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
